package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"aexon/internal/api"
	"aexon/internal/auth"
	"aexon/internal/db"
	"aexon/internal/monitor"
	"aexon/internal/provider/lxc"
	"aexon/internal/scheduler"
	"aexon/internal/service"
	"aexon/internal/types"
	"aexon/internal/utils"
	"aexon/internal/worker"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type InstanceActionRequest struct {
	Action string `json:"action" binding:"required"`
}

type InstanceLimitsRequest struct {
	Memory string `json:"memory"`
	CPU    string `json:"cpu"`
}

type CreateInstanceRequest struct {
	Name       string            `json:"name" binding:"required"`
	Image      string            `json:"image" binding:"required"`
	Limits     map[string]string `json:"limits"`
	UserData   string            `json:"user_data"`   // Opcional: Cloud-Init
	Type       string            `json:"type"`        // Instance type: "container" or "virtual-machine"
	TemplateID string            `json:"template_id"` // Opcional: ID do template a ser usado
	ISOImage   string            `json:"iso_image"`   // Nome do arquivo ISO para boot customizado (opcional)
}

type SnapshotRequest struct {
	Name string `json:"name" binding:"required"`
}

type AddPortRequest struct {
	HostPort      int    `json:"host_port" binding:"required"`
	ContainerPort int    `json:"container_port" binding:"required"`
	Protocol      string `json:"protocol" binding:"required"`
}

func main() {
	log.SetOutput(os.Stdout)
	log.Println("Iniciando Axion Control Plane...")

	if err := db.Init("axion.db"); err != nil {
		log.Fatalf("[ERRO CRÍTICO] Falha ao inicializar banco de dados: %v", err)
	}
	log.Println("Database axion.db inicializado.")

	lxcClient, err := lxc.NewClient()
	if err != nil {
		log.Fatalf("[ERRO CRÍTICO] Falha na inicialização do provider LXD: %v", err)
	}
	log.Println("Conexão com LXD estabelecida.")

	// Run sync and start collectors
	scheduler.RunStartupSync(db.DB, lxcClient)
	go monitor.StartHistoricalCollector(db.DB, lxcClient)

	worker.Init(2, lxcClient)
	api.InitBroadcaster()

	backupScheduler := scheduler.NewBackupScheduler(db.DB, lxcClient)
	backupScheduler.Start()
	backupScheduler.SyncJobs()

	r := gin.Default()

	r.Use(cors.New(cors.Config{
		AllowAllOrigins: true,
		AllowMethods:    []string{"GET", "POST", "OPTIONS", "DELETE", "PUT"},
		AllowHeaders:    []string{"Origin", "Content-Type", "Accept", "Authorization"},
		ExposeHeaders:   []string{"Content-Length", "Content-Disposition"},
		MaxAge:          12 * time.Hour,
	}))

	r.POST("/login", auth.LoginHandler)
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	protected := r.Group("/")
	protected.Use(auth.AuthMiddleware())
	{
		// Instances
		protected.GET("/instances/:name", func(c *gin.Context) {
			name := c.Param("name")
			instance, err := db.GetInstanceWithHardwareInfo(name, lxcClient)
			if err != nil {
				if err == sql.ErrNoRows {
					c.JSON(404, gin.H{"error": "Instance not found"})
					return
				}
				log.Printf("Error getting instance %s: %v", name, err)
				c.JSON(500, gin.H{"error": "Failed to get instance"})
				return
			}
			c.JSON(200, instance)
		})

		protected.GET("/instances", func(c *gin.Context) {
			instances, err := db.ListInstances()
			if err != nil {
				log.Printf("Erro ao processar ListInstances: %v", err)
				c.JSON(500, gin.H{"error": "Falha ao obter métricas"})
				return
			}
			c.JSON(200, instances)
		})

		protected.POST("/instances", func(c *gin.Context) {
			var req CreateInstanceRequest
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(400, gin.H{"error": "JSON inválido. Campos obrigatórios: name, image"})
				return
			}

			// If template is specified, process it
			enhancedUserData := req.UserData
			if req.TemplateID != "" {
				templates := service.GetTemplates()
				templateFound := false
				for _, template := range templates {
					if template.ID == req.TemplateID {
						// Validate that the instance meets the template's minimum requirements
						reqCpu := 1
						if val, ok := req.Limits["limits.cpu"]; ok {
							reqCpu = utils.ParseCpuCores(val)
						}
						reqRam := int64(512)
						if val, ok := req.Limits["limits.memory"]; ok {
							reqRam = utils.ParseMemoryToMB(val)
						}

						// Check if template requirements are met
						if reqCpu < template.MinCPU {
							c.JSON(400, gin.H{"error": fmt.Sprintf("CPU insuficiente. Template %s requer no mínimo %d CPU(s)", template.Name, template.MinCPU)})
							return
						}
						if reqRam < int64(template.MinRAM) {
							c.JSON(400, gin.H{"error": fmt.Sprintf("RAM insuficiente. Template %s requer no mínimo %d MB", template.Name, template.MinRAM)})
							return
						}

						// Merge template cloud config with existing user data
						if req.UserData != "" {
							// If both template and user data exist, we need to merge them
							enhancedUserData = template.CloudConfig + "\n" + req.UserData
						} else {
							// Only use template cloud config
							enhancedUserData = template.CloudConfig
						}
						templateFound = true
						break
					}
				}

				if !templateFound {
					c.JSON(404, gin.H{"error": fmt.Sprintf("Template com ID %s não encontrado", req.TemplateID)})
					return
				}
			}

			// If ISOImage is provided, validate that the ISO exists
			if req.ISOImage != "" {
				storageService, err := service.NewStorageService()
				if err != nil {
					c.JSON(500, gin.H{"error": "Failed to initialize storage service"})
					return
				}

				// Validate that the ISO file exists
				isoPath := storageService.GetISOPath(req.ISOImage)
				if _, err := os.Stat(isoPath); os.IsNotExist(err) {
					c.JSON(400, gin.H{"error": "Specified ISO file does not exist"})
					return
				}
			}

			reqCpu := 1
			if val, ok := req.Limits["limits.cpu"]; ok {
				reqCpu = utils.ParseCpuCores(val)
			}
			reqRam := int64(512)
			if val, ok := req.Limits["limits.memory"]; ok {
				reqRam = utils.ParseMemoryToMB(val)
			}

			if err := lxcClient.CheckGlobalQuota(reqCpu, reqRam); err != nil {
				c.JSON(409, gin.H{"error": "Quota Exceeded", "details": err.Error()})
				return
			}

			instance := &types.Instance{
				Name:            req.Name,
				Image:           req.Image,
				Limits:          req.Limits,
				UserData:        enhancedUserData,
				Type:            req.Type,
				BackupSchedule:  "@daily",
				BackupRetention: 7,
				BackupEnabled:   false,
			}

			if err := db.CreateInstance(instance); err != nil {
				c.JSON(500, gin.H{"error": err.Error()})
				return
			}

			jobID := uuid.New().String()
			payloadBytes, _ := json.Marshal(req)
			job := &db.Job{ID: jobID, Type: types.JobTypeCreateInstance, Target: req.Name, Payload: string(payloadBytes)}
			if err := db.CreateJob(job); err != nil {
				c.JSON(500, gin.H{"error": err.Error()})
				return
			}
			worker.DispatchJob(jobID)
			c.JSON(202, gin.H{"job_id": jobID, "status": "accepted"})
		})

		protected.DELETE("/instances/:name", func(c *gin.Context) {
			name := c.Param("name")

			if err := db.DeleteInstance(name); err != nil {
				c.JSON(500, gin.H{"error": err.Error()})
				return
			}

			jobID := uuid.New().String()
			job := &db.Job{ID: jobID, Type: types.JobTypeDeleteInstance, Target: name, Payload: "{}"}
			if err := db.CreateJob(job); err != nil {
				c.JSON(500, gin.H{"error": err.Error()})
				return
			}
			worker.DispatchJob(jobID)
			c.JSON(202, gin.H{"job_id": jobID, "status": "accepted"})
		})

		protected.POST("/instances/:name/state", func(c *gin.Context) {
			name := c.Param("name")
			var req InstanceActionRequest
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(400, gin.H{"error": "JSON inválido"})
				return
			}
			jobID := uuid.New().String()
			payloadBytes, _ := json.Marshal(req)
			job := &db.Job{ID: jobID, Type: types.JobTypeStateChange, Target: name, Payload: string(payloadBytes)}
			if err := db.CreateJob(job); err != nil {
				c.JSON(500, gin.H{"error": err.Error()})
				return
			}
			worker.DispatchJob(jobID)
			c.JSON(202, gin.H{"job_id": jobID, "status": "accepted"})
		})

		protected.POST("/instances/:name/backups/config", func(c *gin.Context) {
			name := c.Param("name")
			var req struct {
				Enabled   bool   `json:"enabled"`
				Schedule  string `json:"schedule"`
				Retention int    `json:"retention"`
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(400, gin.H{"error": "Invalid JSON: " + err.Error()})
				return
			}
			if err := db.UpdateInstanceBackupConfig(name, req.Enabled, req.Schedule, req.Retention); err != nil {
				c.JSON(500, gin.H{"error": err.Error()})
				return
			}
			backupScheduler.ReloadInstance(name)
			c.JSON(200, gin.H{"status": "updated"})
		})

		protected.POST("/instances/:name/limits", func(c *gin.Context) {
			name := c.Param("name")
			var req InstanceLimitsRequest
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(400, gin.H{"error": "JSON inválido"})
				return
			}
			reqCpu := utils.ParseCpuCores(req.CPU)
			reqRam := utils.ParseMemoryToMB(req.Memory)
			if err := lxcClient.CheckGlobalQuota(reqCpu, reqRam); err != nil {
				c.JSON(409, gin.H{"error": "Quota Exceeded", "details": err.Error()})
				return
			}
			jobID := uuid.New().String()
			payloadBytes, _ := json.Marshal(req)
			job := &db.Job{ID: jobID, Type: types.JobTypeUpdateLimits, Target: name, Payload: string(payloadBytes)}
			if err := db.CreateJob(job); err != nil {
				c.JSON(500, gin.H{"error": err.Error()})
				return
			}
			worker.DispatchJob(jobID)
			c.JSON(202, gin.H{"job_id": jobID, "status": "accepted"})
		})

		// Snapshots
		protected.GET("/instances/:name/snapshots", func(c *gin.Context) {
			name := c.Param("name")
			snaps, err := lxcClient.ListSnapshots(name)
			if err != nil {
				c.JSON(500, gin.H{"error": err.Error()})
				return
			}
			c.JSON(200, snaps)
		})
		protected.POST("/instances/:name/snapshots", func(c *gin.Context) {
			name := c.Param("name")
			var req SnapshotRequest
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(400, gin.H{"error": "Nome obrigatório"})
				return
			}
			jobID := uuid.New().String()
			payload, _ := json.Marshal(map[string]string{"snapshot_name": req.Name})
			job := &db.Job{ID: jobID, Type: types.JobTypeCreateSnapshot, Target: name, Payload: string(payload)}
			if err := db.CreateJob(job); err != nil {
				c.JSON(500, gin.H{"error": err.Error()})
				return
			}
			worker.DispatchJob(jobID)
			c.JSON(202, gin.H{"job_id": jobID, "status": "accepted"})
		})
		protected.POST("/instances/:name/snapshots/:snap/restore", func(c *gin.Context) {
			name := c.Param("name")
			snap := c.Param("snap")
			jobID := uuid.New().String()
			payload, _ := json.Marshal(map[string]string{"snapshot_name": snap})
			job := &db.Job{ID: jobID, Type: types.JobTypeRestoreSnapshot, Target: name, Payload: string(payload)}
			if err := db.CreateJob(job); err != nil {
				c.JSON(500, gin.H{"error": err.Error()})
				return
			}
			worker.DispatchJob(jobID)
			c.JSON(202, gin.H{"job_id": jobID, "status": "accepted"})
		})
		protected.DELETE("/instances/:name/snapshots/:snap", func(c *gin.Context) {
			name := c.Param("name")
			snap := c.Param("snap")
			jobID := uuid.New().String()
			payload, _ := json.Marshal(map[string]string{"snapshot_name": snap})
			job := &db.Job{ID: jobID, Type: types.JobTypeDeleteSnapshot, Target: name, Payload: string(payload)}
			if err := db.CreateJob(job); err != nil {
				c.JSON(500, gin.H{"error": err.Error()})
				return
			}
			worker.DispatchJob(jobID)
			c.JSON(202, gin.H{"job_id": jobID, "status": "accepted"})
		})

		// Ports
		protected.POST("/instances/:name/ports", func(c *gin.Context) {
			name := c.Param("name")
			var req AddPortRequest
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(400, gin.H{"error": err.Error()})
				return
			}
			jobID := uuid.New().String()
			payload, _ := json.Marshal(req)
			job := &db.Job{ID: jobID, Type: types.JobTypeAddPort, Target: name, Payload: string(payload)}
			if err := db.CreateJob(job); err != nil {
				c.JSON(500, gin.H{"error": err.Error()})
				return
			}
			worker.DispatchJob(jobID)
			c.JSON(202, gin.H{"job_id": jobID, "status": "accepted"})
		})
		protected.DELETE("/instances/:name/ports/:host_port", func(c *gin.Context) {
			name := c.Param("name")
			hpStr := c.Param("host_port")
			hp, _ := strconv.Atoi(hpStr)
			jobID := uuid.New().String()
			payload, _ := json.Marshal(map[string]int{"host_port": hp})
			job := &db.Job{ID: jobID, Type: types.JobTypeRemovePort, Target: name, Payload: string(payload)}
			if err := db.CreateJob(job); err != nil {
				c.JSON(500, gin.H{"error": err.Error()})
				return
			}
			worker.DispatchJob(jobID)
			c.JSON(202, gin.H{"job_id": jobID, "status": "accepted"})
		})

		// --- FILE SYSTEM (EXPLORER) ---

		// List
		protected.GET("/instances/:name/files/list", func(c *gin.Context) {
			name := c.Param("name")
			path := c.Query("path")
			if path == "" {
				path = "/root"
			}

			entries, err := lxcClient.ListFiles(name, path)
			if err != nil {
				c.JSON(500, gin.H{"error": err.Error()})
				return
			}
			c.JSON(200, entries)
		})

		// Download
		protected.GET("/instances/:name/files/content", func(c *gin.Context) {
			name := c.Param("name")
			rawPath := c.Query("path")
			if rawPath == "" {
				c.JSON(400, gin.H{"error": "Path missing"})
				return
			}

			cleanPath := filepath.Clean(rawPath)

			content, size, err := lxcClient.DownloadFile(name, cleanPath)
			if err != nil {
				c.JSON(500, gin.H{"error": err.Error()})
				return
			}
			defer content.Close()

			const MaxEditorSize = 1024 * 1024 // 1MB

			if size > MaxEditorSize {
				// Force download ONLY for files confirmed to be large
				c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filepath.Base(cleanPath)))
			} else {
				c.Header("Content-Disposition", "inline")
			}

			if size > 0 {
				c.Header("Content-Length", fmt.Sprintf("%d", size))
			}
			io.Copy(c.Writer, content)
		})

		// Upload
		protected.POST("/instances/:name/files", func(c *gin.Context) {
			name := c.Param("name")
			path := c.Query("path")
			if path == "" {
				c.JSON(400, gin.H{"error": "Target path required"})
				return
			}

			fileHeader, err := c.FormFile("file")
			if err != nil {
				c.JSON(400, gin.H{"error": "File missing"})
				return
			}

			log.Printf("[Upload Debug] Filename: %s, Header Size: %d", fileHeader.Filename, fileHeader.Size)

			file, err := fileHeader.Open()
			if err != nil {
				c.JSON(500, gin.H{"error": "Failed to open file"})
				return
			}
			defer file.Close()

			if err := lxcClient.UploadFile(name, path, file); err != nil {
				c.JSON(500, gin.H{"error": "Upload failed", "details": err.Error()})
				return
			}

			c.JSON(200, gin.H{"status": "uploaded"})
		})

		// Delete File
		protected.DELETE("/instances/:name/files", func(c *gin.Context) {
			name := c.Param("name")
			path := c.Query("path")
			if path == "" {
				c.JSON(400, gin.H{"error": "Path missing"})
				return
			}

			if err := lxcClient.DeleteFile(name, path); err != nil {
				c.JSON(500, gin.H{"error": "Delete failed", "details": err.Error()})
				return
			}
			c.JSON(200, gin.H{"status": "deleted"})
		})

		protected.GET("/jobs", func(c *gin.Context) {
			jobs, err := db.ListRecentJobs(50)
			if err != nil {
				c.JSON(500, gin.H{"error": err.Error()})
				return
			}
			c.JSON(200, jobs)
		})
		protected.GET("/jobs/:id", func(c *gin.Context) {
			id := c.Param("id")
			job, err := db.GetJob(id)
			if err != nil {
				c.JSON(404, gin.H{"error": "Not found"})
				return
			}
			c.JSON(200, job)
		})

		protected.GET("/templates", func(c *gin.Context) {
			templates := service.GetTemplates()
			c.JSON(200, templates)
		})

		protected.GET("/ws/telemetry", func(c *gin.Context) {
			api.StreamTelemetry(c, lxcClient, func(metrics []lxc.InstanceMetric) {
				for _, m := range metrics {
					// Convert lxc.InstanceMetric to db.Metric
					dbMetric := &db.Metric{
						InstanceName: m.Name,
						CPUPercent:   float64(m.CPUUsageSeconds), // This might need adjustment depending on how you calculate percentage
						MemoryUsage:  m.MemoryUsageBytes,
						DiskUsage:    m.DiskUsageBytes,
					}
					if err := db.InsertMetric(dbMetric); err != nil {
						log.Printf("Error inserting metric for instance %s: %v", m.Name, err)
					}
				}
			})
		})

		protected.GET("/instances/:name/metrics/history", func(c *gin.Context) {
			name := c.Param("name")
			rangeParam := c.DefaultQuery("range", "1h")

			intervalMap := map[string]string{
				"1h":  "1 hour",
				"24h": "24 hours",
				"7d":  "7 days",
			}

			interval, ok := intervalMap[rangeParam]
			if !ok {
				c.JSON(400, gin.H{"error": "Invalid range. Valid ranges are 1h, 24h, 7d"})
				return
			}

			metrics, err := db.GetInstanceMetrics(name, interval)
			if err != nil {
				log.Printf("Error fetching metrics history for %s: %v", name, err)
				c.JSON(500, gin.H{"error": "Failed to fetch metrics history"})
				return
			}

			c.JSON(200, metrics)
		})

		// Current metrics endpoint
		protected.GET("/instances/:name/metrics", func(c *gin.Context) {
			name := c.Param("name")

			// Get current metrics for the instance
			lxdMetrics, err := lxcClient.ListInstances()
			if err != nil {
				log.Printf("Error fetching metrics for instance %s: %v", name, err)
				c.JSON(500, gin.H{"error": "Failed to fetch metrics"})
				return
			}

			// Find the specific instance
			var targetMetric *lxc.InstanceMetric
			for _, metric := range lxdMetrics {
				if metric.Name == name {
					metric := metric // Create a copy of the loop variable
					targetMetric = &metric
					break
				}
			}

			if targetMetric == nil {
				c.JSON(404, gin.H{"error": "Instance not found"})
				return
			}

			c.JSON(200, targetMetric)
		})

		// Instance logs endpoint
		protected.GET("/instances/:name/logs", func(c *gin.Context) {
			name := c.Param("name")
			logContent, err := lxcClient.GetInstanceLog(name)
			if err != nil {
				log.Printf("Erro ao obter log da instância %s: %v", name, err)
				c.JSON(500, gin.H{"error": "Falha ao obter log da instância", "details": err.Error()})
				return
			}
			c.JSON(200, gin.H{"log": logContent})
		})

		// Network management endpoints
		protected.GET("/networks", func(c *gin.Context) {
			networks, err := lxcClient.ListNetworks()
			if err != nil {
				log.Printf("Erro ao listar redes: %v", err)
				c.JSON(500, gin.H{"error": "Falha ao obter redes", "details": err.Error()})
				return
			}
			c.JSON(200, networks)
		})

		protected.POST("/networks", func(c *gin.Context) {
			var req struct {
				Name        string `json:"name" binding:"required"`
				Description string `json:"description"`
				Subnet      string `json:"subnet" binding:"required"`
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(400, gin.H{"error": "JSON inválido. Campos obrigatórios: name, subnet"})
				return
			}

			err := lxcClient.CreateNetwork(req.Name, req.Description, req.Subnet)
			if err != nil {
				log.Printf("Erro ao criar rede %s: %v", req.Name, err)
				c.JSON(500, gin.H{"error": "Falha ao criar rede", "details": err.Error()})
				return
			}
			c.JSON(201, gin.H{"status": "created"})
		})

		protected.DELETE("/networks/:name", func(c *gin.Context) {
			name := c.Param("name")
			err := lxcClient.DeleteNetwork(name)
			if err != nil {
				log.Printf("Erro ao deletar rede %s: %v", name, err)
				c.JSON(500, gin.H{"error": "Falha ao deletar rede", "details": err.Error()})
				return
			}
			c.JSON(200, gin.H{"status": "deleted"})
		})

		// Storage - ISO uploads
		protected.POST("/storage/isos", func(c *gin.Context) {
			// Retrieve storage service or create a new one for this endpoint
			storageService, err := service.NewStorageService()
			if err != nil {
				log.Printf("Error initializing storage service: %v", err)
				c.JSON(500, gin.H{"error": "Failed to initialize storage service"})
				return
			}

			// Get file from multipart form
			file, header, err := c.Request.FormFile("iso_file")
			if err != nil {
				c.JSON(400, gin.H{"error": "ISO file is required"})
				return
			}
			defer file.Close()

			// Validate file extension
			filename := header.Filename
			if !strings.HasSuffix(strings.ToLower(filename), ".iso") {
				c.JSON(400, gin.H{"error": "Only .iso files are allowed"})
				return
			}

			// Save ISO file using streaming and get info
			isoInfo, err := storageService.SaveISOWithInfo(filename, file)
			if err != nil {
				log.Printf("Error saving ISO file: %v", err)
				c.JSON(500, gin.H{"error": "Failed to save ISO file", "details": err.Error()})
				return
			}

			c.JSON(200, gin.H{
				"filename": isoInfo.Name,
				"size":     isoInfo.Size,
				"path":     isoInfo.Path,
			})
		})

		// List uploaded ISOs
		protected.GET("/storage/isos", func(c *gin.Context) {
			storageService, err := service.NewStorageService()
			if err != nil {
				log.Printf("Error initializing storage service: %v", err)
				c.JSON(500, gin.H{"error": "Failed to initialize storage service"})
				return
			}

			isos, err := storageService.ListISOs()
			if err != nil {
				log.Printf("Error listing ISOs: %v", err)
				c.JSON(500, gin.H{"error": "Failed to list ISOs", "details": err.Error()})
				return
			}

			// Get detailed information for each ISO
			var isoInfos []gin.H
			for _, isoName := range isos {
				info, err := storageService.GetISOInfo(isoName)
				if err != nil {
					log.Printf("Error getting info for ISO %s: %v", isoName, err)
					continue
				}

				isoInfos = append(isoInfos, gin.H{
					"name": isoName,
					"size": info.Size(),
					"path": storageService.GetISOPath(isoName),
				})
			}

			c.JSON(200, gin.H{
				"isos": isoInfos,
			})
		})

		// Delete an ISO
		protected.DELETE("/storage/isos/:name", func(c *gin.Context) {
			name := c.Param("name")

			storageService, err := service.NewStorageService()
			if err != nil {
				log.Printf("Error initializing storage service: %v", err)
				c.JSON(500, gin.H{"error": "Failed to initialize storage service"})
				return
			}

			err = storageService.DeleteISO(name)
			if err != nil {
				log.Printf("Error deleting ISO %s: %v", name, err)
				c.JSON(500, gin.H{"error": "Failed to delete ISO", "details": err.Error()})
				return
			}

			c.JSON(200, gin.H{
				"status": "deleted",
			})
		})

		// Cluster members endpoint
		protected.GET("/cluster/members", func(c *gin.Context) {
			members, err := lxcClient.GetClusterMembers()
			if err != nil {
				// Check specifically for not clustered error and return single-node representation
				if strings.Contains(err.Error(), "not clustered") {
					// Return single-node representation showing the machine state
					response := []map[string]interface{}{
						{
							"name":    "local-server", // Use actual server name if available
							"status":  "Online",
							"address": "127.0.0.1", // Local address
							"roles":   []string{"standalone"},
						},
					}
					c.JSON(200, response)
					return
				}

				log.Printf("Erro ao processar GetClusterMembers: %v", err)
				c.JSON(500, gin.H{"error": "Falha ao obter membros do cluster", "details": err.Error()})
				return
			}

			// Format the cluster members response with Name, Status, Address and Role
			type ClusterMemberResponse struct {
				Name    string   `json:"name"`
				Status  string   `json:"status"`
				Address string   `json:"address"`
				Roles   []string `json:"roles"`
			}

			var response []ClusterMemberResponse
			for _, member := range members {
				response = append(response, ClusterMemberResponse{
					Name:    member.ServerName,
					Status:  member.Status,
					Address: member.URL,
					Roles:   member.Roles,
				})
			}

			c.JSON(200, response)
		})
	}

	r.GET("/ws/terminal/:name", func(c *gin.Context) {
		api.TerminalHandler(c, lxcClient)
	})

	port := "8500"
	log.Printf("Axion Control Plane rodando na porta %s", port)
	if err := r.Run("0.0.0.0:" + port); err != nil {
		log.Fatalf("Falha ao iniciar servidor web: %v", err)
	}
}
