package main

import (
	"aexon/internal/auth"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"runtime/debug"
	"strconv"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"aexon/internal/api"
	"aexon/internal/db"
	"aexon/internal/provider/axhv"
	"aexon/internal/provider/axhv/pb"
	"aexon/internal/scheduler"
	"aexon/internal/service"
	"aexon/internal/types"
	"aexon/internal/utils"

	"github.com/gin-gonic/gin"
)

// ============================================================================
// ERROR TAXONOMY - Todos os erros possíveis documentados
// ============================================================================

type ErrorCode int

const (
	// Client Errors (1000-1999)
	ErrCodeInvalidJSON ErrorCode = iota + 1000
	ErrCodeMissingField
	ErrCodeInvalidPath
	ErrCodeInvalidFileType
	ErrCodeFileTooLarge
	ErrCodeInstanceNotFound
	ErrCodeSnapshotNotFound
	ErrCodeNetworkNotFound
	ErrCodeISONotFound
	ErrCodeTemplateNotFound
	ErrCodeInvalidQuota
	ErrCodeInsufficientResources

	// Server Errors (2000-2999)
	ErrCodeDatabaseFailure ErrorCode = iota + 2000
	ErrCodeLXDConnectionFailed
	ErrCodeJobCreationFailed
	ErrCodeInstanceCreationFailed
	ErrCodeSnapshotFailed
	ErrCodeFileOperationFailed
	ErrCodeNetworkOperationFailed
	ErrCodeStorageOperationFailed
	ErrCodeMetricsFetchFailed
	ErrCodeBackupFailed
	ErrCodeWorkerDispatchFailed
	ErrCodeUnknownError

	// Infrastructure Errors (3000-3999)
	ErrCodeInitializationFailed ErrorCode = iota + 3000
	ErrCodeShutdownFailed
	ErrCodeConfigurationInvalid
)

type AppError struct {
	Code       ErrorCode
	Message    string
	HTTPStatus int
	Err        error
	Context    map[string]interface{}
	Timestamp  int64
	Retryable  bool
}

func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("[%d] %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("[%d] %s", e.Code, e.Message)
}

func (e *AppError) WithContext(key string, value interface{}) *AppError {
	if e.Context == nil {
		e.Context = make(map[string]interface{})
	}
	e.Context[key] = value
	return e
}

func NewError(code ErrorCode, msg string, err error, httpStatus int, retryable bool) *AppError {
	return &AppError{
		Code:       code,
		Message:    msg,
		Err:        err,
		HTTPStatus: httpStatus,
		Timestamp:  time.Now().UnixNano(),
		Retryable:  retryable,
	}
}

// Error constructors
func ErrInvalidJSON(err error) *AppError {
	return NewError(ErrCodeInvalidJSON, "invalid json", err, 400, false)
}

func ErrMissingField(field string) *AppError {
	return NewError(ErrCodeMissingField, "missing required field", nil, 400, false).
		WithContext("field", field)
}

func ErrInstanceNotFound(name string) *AppError {
	return NewError(ErrCodeInstanceNotFound, "instance not found", nil, 404, false).
		WithContext("instance", name)
}

func ErrDatabaseFailure(err error) *AppError {
	return NewError(ErrCodeDatabaseFailure, "database operation failed", err, 500, true)
}

func ErrQuotaExceeded(details string) *AppError {
	return NewError(ErrCodeInvalidQuota, "quota exceeded", nil, 409, false).
		WithContext("details", details)
}

func ErrJobCreation(err error) *AppError {
	return NewError(ErrCodeJobCreationFailed, "job creation failed", err, 500, true)
}

// ============================================================================
// REQUEST/RESPONSE TYPES
// ============================================================================

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
	UserData   string            `json:"user_data"`
	Type       string            `json:"type"`
	TemplateID string            `json:"template_id"`
	ISOImage   string            `json:"iso_image"`
}

type SnapshotRequest struct {
	Name string `json:"name" binding:"required"`
}

type AddPortRequest struct {
	HostPort      int    `json:"host_port" binding:"required"`
	ContainerPort int    `json:"container_port" binding:"required"`
	Protocol      string `json:"protocol" binding:"required"`
}

type BackupConfigRequest struct {
	Enabled   bool   `json:"enabled"`
	Schedule  string `json:"schedule"`
	Retention int    `json:"retention"`
}

type CreateNetworkRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
	Subnet      string `json:"subnet" binding:"required"`
}

// ============================================================================
// METRICS
// ============================================================================

type Metrics struct {
	requestsTotal    atomic.Uint64
	requestsSuccess  atomic.Uint64
	requestsError    atomic.Uint64
	instancesCreated atomic.Uint64
	instancesDeleted atomic.Uint64
	snapshotsCreated atomic.Uint64
	jobsDispatched   atomic.Uint64
	filesUploaded    atomic.Uint64
	filesDownloaded  atomic.Uint64
	networksCreated  atomic.Uint64
	startTime        int64
}

func NewMetrics() *Metrics {
	return &Metrics{startTime: time.Now().Unix()}
}

func (m *Metrics) RecordRequest()         { m.requestsTotal.Add(1) }
func (m *Metrics) RecordSuccess()         { m.requestsSuccess.Add(1) }
func (m *Metrics) RecordError()           { m.requestsError.Add(1) }
func (m *Metrics) RecordInstanceCreated() { m.instancesCreated.Add(1) }
func (m *Metrics) RecordInstanceDeleted() { m.instancesDeleted.Add(1) }
func (m *Metrics) RecordSnapshot()        { m.snapshotsCreated.Add(1) }
func (m *Metrics) RecordJob()             { m.jobsDispatched.Add(1) }
func (m *Metrics) RecordFileUpload()      { m.filesUploaded.Add(1) }
func (m *Metrics) RecordFileDownload()    { m.filesDownloaded.Add(1) }
func (m *Metrics) RecordNetworkCreated()  { m.networksCreated.Add(1) }

func (m *Metrics) Snapshot() map[string]interface{} {
	uptime := time.Now().Unix() - m.startTime
	return map[string]interface{}{
		"uptime_seconds":    uptime,
		"requests_total":    m.requestsTotal.Load(),
		"requests_success":  m.requestsSuccess.Load(),
		"requests_error":    m.requestsError.Load(),
		"instances_created": m.instancesCreated.Load(),
		"instances_deleted": m.instancesDeleted.Load(),
		"snapshots_created": m.snapshotsCreated.Load(),
		"jobs_dispatched":   m.jobsDispatched.Load(),
		"files_uploaded":    m.filesUploaded.Load(),
		"files_downloaded":  m.filesDownloaded.Load(),
		"networks_created":  m.networksCreated.Load(),
		"goroutines":        runtime.NumGoroutine(),
	}
}

// ============================================================================
// HTTP HANDLERS
// ============================================================================

type Handlers struct {
	axhvClient      *axhv.Client
	backupScheduler *scheduler.BackupScheduler
	metrics         *Metrics
}

func NewHandlers(axhvClient *axhv.Client, backupScheduler *scheduler.BackupScheduler) *Handlers {
	return &Handlers{
		axhvClient:      axhvClient,
		backupScheduler: backupScheduler,
		metrics:         NewMetrics(),
	}
}

// Middleware para métricas e error handling
func (h *Handlers) metricsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		h.metrics.RecordRequest()
		c.Next()

		if c.Writer.Status() >= 400 {
			h.metrics.RecordError()
		} else {
			h.metrics.RecordSuccess()
		}
	}
}

func (h *Handlers) writeError(c *gin.Context, appErr *AppError) {
	c.Header("X-Error-Code", strconv.Itoa(int(appErr.Code)))
	if appErr.Retryable {
		c.Header("Retry-After", "1")
	}

	response := gin.H{
		"error":     appErr.Message,
		"code":      appErr.Code,
		"retryable": appErr.Retryable,
		"timestamp": appErr.Timestamp,
	}

	if len(appErr.Context) > 0 {
		response["context"] = appErr.Context
	}

	if appErr.Err != nil {
		response["details"] = appErr.Err.Error()
	}

	c.JSON(appErr.HTTPStatus, response)
}

// Instance Handlers
func (h *Handlers) GetInstance(c *gin.Context) {
	name := c.Param("name")

	// We no longer enrich via LXD here because AxHV is simpler.
	// Just return DB instance. If we need stats, we use GetVmStats separate call.
	instance, err := db.GetInstance(name)
	if err != nil {
		if err == sql.ErrNoRows {
			h.writeError(c, ErrInstanceNotFound(name))
			return
		}
		log.Printf("Error getting instance %s: %v", name, err)
		h.writeError(c, ErrDatabaseFailure(err))
		return
	}

	c.JSON(200, instance)
}

func (h *Handlers) ListInstances(c *gin.Context) {
	instances, err := db.ListInstances()
	if err != nil {
		log.Printf("Error listing instances: %v", err)
		h.writeError(c, ErrDatabaseFailure(err))
		return
	}
	c.JSON(200, instances)
}

func (h *Handlers) CreateInstance(c *gin.Context) {
	var req CreateInstanceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.writeError(c, ErrInvalidJSON(err))
		return
	}

	// Validate and merge template
	enhancedUserData, appErr := h.processTemplate(req)
	if appErr != nil {
		h.writeError(c, appErr)
		return
	}

	// Allocate IP using DB locking (IPAM)
	ip, err := db.GetService().AllocateIP(c.Request.Context(), req.Name)
	if err != nil {
		log.Printf("IP Allocation failed for %s: %v", req.Name, err)
		h.writeError(c, NewError(ErrCodeInstanceCreationFailed, "failed to allocate IP", err, 500, false))
		return
	}

	// Defer release logic in case of failure later (requires careful handling of success path)
	// We will release if we panic or return early with error.
	success := false
	defer func() {
		if !success {
			db.GetService().ReleaseIP(context.Background(), req.Name)
		}
	}()

	// Create Instance object for Mapping
	instance := types.Instance{
		Name:            req.Name,
		Image:           req.Image,
		Limits:          req.Limits,
		UserData:        enhancedUserData,
		Type:            req.Type,
		BackupSchedule:  "@daily",
		BackupRetention: 7,
		BackupEnabled:   false,
	}

	// Map to Protobuf
	gateway := "172.16.0.1" // Gateway is constant for now
	pbReq, err := axhv.MapCreateRequest(instance, ip, gateway)
	if err != nil {
		h.writeError(c, NewError(ErrCodeInstanceCreationFailed, "failed to map request", err, 400, false))
		return
	}

	// Call AxHV gRPC
	grpcResp, err := h.axhvClient.CreateVm(c.Request.Context(), pbReq)
	if err != nil {
		h.writeError(c, NewError(ErrCodeInstanceCreationFailed, "AxHV RPC failed", err, 502, true))
		return
	}

	if !grpcResp.Success {
		h.writeError(c, NewError(ErrCodeInstanceCreationFailed, fmt.Sprintf("AxHV Error: %s", grpcResp.Message), nil, 400, false))
		return
	}

	// Persist to DB
	// Note: We already allocated the IP which updated the ip_leases table with instance_name.
	// We should also insert into instances table as before.
	// Update limits with the allocated IP to keep consistency if needed by other parts
	if instance.Limits == nil {
		instance.Limits = make(map[string]string)
	}
	instance.Limits["volatile.ip_address"] = ip

	if err := db.CreateInstance(&instance); err != nil {
		// If DB fails, we should try to cleanup the VM? Ideally yes.
		// For now, fail
		h.writeError(c, ErrDatabaseFailure(err))
		return
	}

	success = true
	h.metrics.RecordInstanceCreated()

	c.JSON(201, gin.H{"status": "created", "ip": ip, "vm_id": grpcResp.VmId})
}

func (h *Handlers) DeleteInstance(c *gin.Context) {
	name := c.Param("name")

	// Call AxHV gRPC
	grpcResp, err := h.axhvClient.DeleteVm(c.Request.Context(), name)
	if err != nil {
		h.writeError(c, NewError(ErrCodeInstanceNotFound, "AxHV RPC failed", err, 502, true))
		return
	}

	if !grpcResp.Success {
		// If it's just "not found", we might want to proceed to delete from DB anyway
		log.Printf("AxHV Delete Warn: %s", grpcResp.Message)
	}

	// Release IP
	if err := db.GetService().ReleaseIP(c.Request.Context(), name); err != nil {
		log.Printf("Error releasing IP for %s: %v", name, err)
	}

	if err := db.DeleteInstance(name); err != nil {
		h.writeError(c, ErrDatabaseFailure(err))
		return
	}

	h.metrics.RecordInstanceDeleted()

	c.JSON(200, gin.H{"status": "deleted"})
}

func (h *Handlers) UpdateInstanceState(c *gin.Context) {
	name := c.Param("name")
	var req InstanceActionRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		h.writeError(c, ErrInvalidJSON(err))
		return
	}

	var resp *pb.VmResponse
	var err error
	ctx := c.Request.Context()

	switch req.Action {
	case "start":
		resp, err = h.axhvClient.StartVm(ctx, name)
	case "stop":
		resp, err = h.axhvClient.StopVm(ctx, name)
	case "reboot":
		resp, err = h.axhvClient.RebootVm(ctx, name)
	case "pause":
		resp, err = h.axhvClient.PauseVm(ctx, name)
	case "resume":
		resp, err = h.axhvClient.ResumeVm(ctx, name)
	default:
		h.writeError(c, NewError(ErrCodeInvalidJSON, "invalid action", nil, 400, false))
		return
	}

	if err != nil {
		h.writeError(c, NewError(ErrCodeInstanceCreationFailed, "AxHV RPC failed", err, 502, true))
		return
	}

	if !resp.Success {
		h.writeError(c, NewError(ErrCodeInstanceCreationFailed, resp.Message, nil, 400, false))
		return
	}

	c.JSON(200, gin.H{"status": "executed", "action": req.Action})
}

func (h *Handlers) UpdateInstanceLimits(c *gin.Context) {
	// Stub success
	c.JSON(200, gin.H{"status": "accepted (db only)"})
}

func (h *Handlers) UpdateBackupConfig(c *gin.Context) {
	name := c.Param("name")
	var req BackupConfigRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		h.writeError(c, ErrInvalidJSON(err))
		return
	}

	if err := db.UpdateInstanceBackupConfig(name, req.Enabled, req.Schedule, req.Retention); err != nil {
		h.writeError(c, ErrDatabaseFailure(err))
		return
	}

	h.backupScheduler.ReloadInstance(name)
	c.JSON(200, gin.H{"status": "updated"})
}

// Snapshot Handlers - NOT IMPLEMENTED IN AxHV
func (h *Handlers) ListSnapshots(c *gin.Context) {
	c.JSON(501, gin.H{"error": "Snapshots not supported in AxHV v2"})
}

func (h *Handlers) CreateSnapshot(c *gin.Context) {
	c.JSON(501, gin.H{"error": "Snapshots not supported in AxHV v2"})
}

func (h *Handlers) RestoreSnapshot(c *gin.Context) {
	c.JSON(501, gin.H{"error": "Snapshots not supported in AxHV v2"})
}

func (h *Handlers) DeleteSnapshot(c *gin.Context) {
	c.JSON(501, gin.H{"error": "Snapshots not supported in AxHV v2"})
}

// Port Management Handlers
func (h *Handlers) AddPort(c *gin.Context) {
	c.JSON(501, gin.H{"error": "Port forwarding management not supported in AxHV v2"})
}

func (h *Handlers) RemovePort(c *gin.Context) {
	c.JSON(501, gin.H{"error": "Port forwarding management not supported in AxHV v2"})
}

// File System Handlers - NOT IMPLEMENTED IN AxHV
func (h *Handlers) ListFiles(c *gin.Context) {
	c.JSON(501, gin.H{"error": "File operations not supported in AxHV v2"})
}

func (h *Handlers) DownloadFile(c *gin.Context) {
	c.JSON(501, gin.H{"error": "File operations not supported in AxHV v2"})
}

func (h *Handlers) UploadFile(c *gin.Context) {
	c.JSON(501, gin.H{"error": "File operations not supported in AxHV v2"})
}

func (h *Handlers) DeleteFile(c *gin.Context) {
	c.JSON(501, gin.H{"error": "File operations not supported in AxHV v2"})
}

// Job Handlers
func (h *Handlers) ListJobs(c *gin.Context) {
	jobs, err := db.ListRecentJobs(50)
	if err != nil {
		h.writeError(c, ErrDatabaseFailure(err))
		return
	}
	c.JSON(200, jobs)
}

func (h *Handlers) GetJob(c *gin.Context) {
	id := c.Param("id")
	job, err := db.GetJob(id)
	if err != nil {
		h.writeError(c, NewError(ErrCodeInstanceNotFound, "job not found", err, 404, false))
		return
	}
	c.JSON(200, job)
}

// Template Handlers
func (h *Handlers) ListTemplates(c *gin.Context) {
	templates := service.GetTemplates()
	c.JSON(200, templates)
}

// Metrics Handlers
func (h *Handlers) GetInstanceMetrics(c *gin.Context) {
	name := c.Param("name")

	stats, err := h.axhvClient.GetVmStats(c.Request.Context(), name)
	if err != nil {
		log.Printf("Error fetching metrics for %s: %v", name, err)
		h.writeError(c, NewError(ErrCodeMetricsFetchFailed, "failed to fetch metrics", err, 500, true))
		return
	}

	// Calculate rates if needed, or pass raw. The frontend expects similar structure to LXD?
	// The requested requirement: "Implement Metrics endpoint (gRPC GetVmStats -> HTTP)"

	c.JSON(200, stats)
}

func (h *Handlers) GetInstanceMetricsHistory(c *gin.Context) {
	name := c.Param("name")
	rangeParam := c.DefaultQuery("range", "1h")

	intervalMap := map[string]string{
		"1h":  "1 hour",
		"24h": "24 hours",
		"7d":  "7 days",
	}

	interval, ok := intervalMap[rangeParam]
	if !ok {
		h.writeError(c, NewError(ErrCodeInvalidJSON, "invalid range parameter", nil, 400, false).
			WithContext("valid_ranges", []string{"1h", "24h", "7d"}))
		return
	}

	metrics, err := db.GetInstanceMetrics(name, interval)
	if err != nil {
		log.Printf("Error fetching history for %s: %v", name, err)
		h.writeError(c, NewError(ErrCodeMetricsFetchFailed, "failed to fetch history", err, 500, true))
		return
	}

	c.JSON(200, metrics)
}

func (h *Handlers) GetInstanceLogs(c *gin.Context) {
	c.JSON(501, gin.H{"error": "Logs not supported in AxHV v2"})
}

// Cluster Handlers
func (h *Handlers) GetClusterMembers(c *gin.Context) {
	// Not implemented
	c.JSON(501, gin.H{"error": "Cluster operations not supported in AxHV v2"})
}

func (h *Handlers) StreamTelemetry(c *gin.Context) {
	// NOT IMPLEMENTED
}

func (h *Handlers) GetMetrics(c *gin.Context) {
	c.JSON(200, h.metrics.Snapshot())
}

// Storage/ISO Handlers
func (h *Handlers) UploadISO(c *gin.Context) {
	c.JSON(501, gin.H{"error": "ISO upload not supported in AxHV v2"})
}

func (h *Handlers) ListISOs(c *gin.Context) {
	storageService, err := service.NewStorageService()
	if err != nil {
		log.Printf("Error initializing storage: %v", err)
		h.writeError(c, NewError(ErrCodeStorageOperationFailed, "storage init failed", err, 500, false))
		return
	}

	isos, err := storageService.ListISOs()
	if err != nil {
		log.Printf("Error listing ISOs: %v", err)
		h.writeError(c, NewError(ErrCodeStorageOperationFailed, "failed to list ISOs", err, 500, true))
		return
	}

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

	c.JSON(200, gin.H{"isos": isoInfos})
}

func (h *Handlers) DeleteISO(c *gin.Context) {
	name := c.Param("name")

	storageService, err := service.NewStorageService()
	if err != nil {
		log.Printf("Error initializing storage: %v", err)
		h.writeError(c, NewError(ErrCodeStorageOperationFailed, "storage init failed", err, 500, false))
		return
	}

	if err := storageService.DeleteISO(name); err != nil {
		log.Printf("Error deleting ISO %s: %v", name, err)
		h.writeError(c, NewError(ErrCodeStorageOperationFailed, "failed to delete ISO", err, 500, true))
		return
	}

	c.JSON(200, gin.H{"status": "deleted"})
}

// Cluster Handlers
// Cluster Handlers

// Helper methods
func (h *Handlers) processTemplate(req CreateInstanceRequest) (string, *AppError) {
	if req.TemplateID == "" {
		return req.UserData, nil
	}

	templates := service.GetTemplates()
	for _, template := range templates {
		if template.ID == req.TemplateID {
			reqCpu := h.parseCPU(req.Limits)
			reqRam := h.parseMemory(req.Limits)

			if reqCpu < template.MinCPU {
				return "", NewError(ErrCodeInsufficientResources,
					fmt.Sprintf("CPU insufficient for template %s", template.Name),
					nil, 400, false).
					WithContext("required", template.MinCPU).
					WithContext("provided", reqCpu)
			}

			if reqRam < int64(template.MinRAM) {
				return "", NewError(ErrCodeInsufficientResources,
					fmt.Sprintf("RAM insufficient for template %s", template.Name),
					nil, 400, false).
					WithContext("required", template.MinRAM).
					WithContext("provided", reqRam)
			}

			if req.UserData != "" {
				return template.CloudConfig + "\n" + req.UserData, nil
			}
			return template.CloudConfig, nil
		}
	}

	return "", NewError(ErrCodeTemplateNotFound, "template not found", nil, 404, false).
		WithContext("template_id", req.TemplateID)
}

func (h *Handlers) validateISO(isoImage string) *AppError {
	storageService, err := service.NewStorageService()
	if err != nil {
		return NewError(ErrCodeStorageOperationFailed, "storage init failed", err, 500, false)
	}

	isoPath := storageService.GetISOPath(isoImage)
	if _, err := os.Stat(isoPath); os.IsNotExist(err) {
		return NewError(ErrCodeISONotFound, "ISO file does not exist", nil, 400, false).
			WithContext("iso_image", isoImage)
	}

	return nil
}

func (h *Handlers) parseCPU(limits map[string]string) int {
	if val, ok := limits["limits.cpu"]; ok {
		return utils.ParseCpuCores(val)
	}
	return 1
}

func (h *Handlers) parseMemory(limits map[string]string) int64 {
	if val, ok := limits["limits.memory"]; ok {
		return utils.ParseMemoryToMB(val)
	}
	return 512
}

// ============================================================================
// APPLICATION ORCHESTRATOR
// ============================================================================

type Application struct {
	lxcClient       interface{} // Place holder, unused
	backupScheduler *scheduler.BackupScheduler
	handlers        *Handlers
	router          *gin.Engine
	server          *http.Server
	state           atomic.Uint32
	wg              sync.WaitGroup
}

const (
	stateCreated  uint32 = 0
	stateRunning  uint32 = 1
	stateStopping uint32 = 2
	stateStopped  uint32 = 3
)

func NewApplication() (*Application, error) {
	// Initialize database
	if _, err := db.InitService(nil); err != nil {
		return nil, fmt.Errorf("database initialization failed: %w", err)
	}
	log.Println("✓ Database initialized")

	// Run Migrations
	if err := db.RunMigrations(context.Background(), db.GetService()); err != nil {
		return nil, fmt.Errorf("migrations failed: %w", err)
	}
	log.Println("✓ Database migrations applied")

	// Initialize AxHV client
	// Prefer AxHV socket default
	axhvClient, err := axhv.NewClient("", "", "")
	if err != nil {
		return nil, fmt.Errorf("AxHV client initialization failed: %w", err)
	}
	log.Println("✓ AxHV connection established")
	// Initialize auth service
	auth.Init(nil) // Uses default config

	// Seed DB with admin if empty
	go func() {
		// Wait for DB to be potentially migrated? Migration runs in InitService? Yes.
		// Use background context with timeout
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
		defer cancel()
		if err := auth.GetAuthService().SeedAdmin(ctx); err != nil {
			log.Printf("[Main] Error seeding admin: %v", err)
		}
	}()

	// Initialize workers - DISABLED
	// worker.Init(2, lxcClient)
	// log.Println("✓ Worker pool initialized")

	// Initialize API broadcaster
	api.InitBroadcaster()
	log.Println("✓ API broadcaster initialized")

	// Initialize backup scheduler - DISABLED (Legacy)
	// backupScheduler := scheduler.NewBackupScheduler(db.GetService().GetRawDB(), lxcClient)
	// log.Println("✓ Backup scheduler initialized")
	var backupScheduler *scheduler.BackupScheduler // nil

	// Initialize handlers
	handlers := NewHandlers(axhvClient, backupScheduler)

	app := &Application{
		lxcClient:       nil, // REMOVED
		backupScheduler: backupScheduler,
		handlers:        handlers,
	}
	app.state.Store(stateCreated)

	return app, nil
}

func (a *Application) setupRouter() {
	r := gin.Default()
	a.router = r

	// CORS
	r.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS, PATCH")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	})

	api := r.Group("/api/v1")
	h := a.handlers

	// Auth
	api.POST("/login", auth.LoginHandler)
	api.POST("/register", auth.RegisterHandler)
	api.POST("/refresh", auth.RefreshTokenHandler)
	api.POST("/revoke", auth.RevokeTokenHandler)
	api.GET("/auth/metrics", auth.GetAuthMetricsHandler)

	// Instances
	api.GET("/instances", auth.AuthMiddleware(), h.ListInstances)
	api.POST("/instances", auth.AuthMiddleware(), h.CreateInstance)
	api.GET("/instances/:name", auth.AuthMiddleware(), h.GetInstance)
	api.DELETE("/instances/:name", auth.AuthMiddleware(), h.DeleteInstance)
	api.POST("/instances/:name/action", auth.AuthMiddleware(), h.UpdateInstanceState)
	api.PUT("/instances/:name/limits", auth.AuthMiddleware(), h.UpdateInstanceLimits)
	api.PUT("/instances/:name/backup", auth.AuthMiddleware(), h.UpdateBackupConfig)

	// Snapshots (Stubbed)
	api.GET("/instances/:name/snapshots", auth.AuthMiddleware(), h.ListSnapshots)
	api.POST("/instances/:name/snapshots", auth.AuthMiddleware(), h.CreateSnapshot)
	api.POST("/instances/:name/snapshots/:snap/restore", auth.AuthMiddleware(), h.RestoreSnapshot)
	api.DELETE("/instances/:name/snapshots/:snap", auth.AuthMiddleware(), h.DeleteSnapshot)

	// Files (Stubbed)
	api.GET("/instances/:name/files", auth.AuthMiddleware(), h.ListFiles)
	api.GET("/instances/:name/file", auth.AuthMiddleware(), h.DownloadFile)
	api.POST("/instances/:name/files", auth.AuthMiddleware(), h.UploadFile)
	api.DELETE("/instances/:name/files", auth.AuthMiddleware(), h.DeleteFile)

	// Metrics
	api.GET("/instances/:name/metrics", auth.AuthMiddleware(), h.GetInstanceMetrics)
	api.GET("/instances/:name/metrics/history", auth.AuthMiddleware(), h.GetInstanceMetricsHistory)
	api.GET("/instances/:name/logs", auth.AuthMiddleware(), h.GetInstanceLogs)

	// Cluster
	api.GET("/cluster", auth.AuthMiddleware(), h.GetClusterMembers)

	// ISOs
	api.GET("/isos", auth.AuthMiddleware(), h.ListISOs)
	api.POST("/isos", auth.AuthMiddleware(), h.UploadISO)
	api.DELETE("/isos/:name", auth.AuthMiddleware(), h.DeleteISO)

	// Jobs
	api.GET("/jobs", auth.AuthMiddleware(), h.ListJobs)
	api.GET("/jobs/:id", auth.AuthMiddleware(), h.GetJob)

	// Templates
	api.GET("/templates", auth.AuthMiddleware(), h.ListTemplates)

	// App Metrics
	api.GET("/metrics", auth.AuthMiddleware(), h.GetMetrics)

	// Admin Networks
	api.GET("/networks", auth.AuthMiddleware(), h.ListNetworks)
	api.POST("/networks", auth.AuthMiddleware(), h.CreateNetwork)
}

func (a *Application) Start() error {
	if !a.state.CompareAndSwap(stateCreated, stateRunning) {
		return errors.New("application already started")
	}

	// Run startup sync
	// scheduler.RunStartupSync(db.GetService().GetRawDB(), a.lxcClient)
	// log.Println("✓ Startup sync completed")

	// Start background services
	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		// monitor.StartHistoricalCollector(db.GetService().GetRawDB(), a.lxcClient)
	}()
	log.Println("✓ Historical collector started (DISABLED)")

	// Start backup scheduler
	// a.backupScheduler.Start()
	// a.backupScheduler.SyncJobs()
	// log.Println("✓ Backup scheduler started")

	// Setup router
	a.setupRouter()

	// Start HTTP server
	a.server = &http.Server{
		Addr:              ":8500",
		Handler:           a.router,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		MaxHeaderBytes:    1 << 20, // 1MB
	}

	go func() {
		log.Println("✓ HTTP server starting on :8500")
		if err := a.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("Server error: %v", err)
		}
	}()

	return nil
}

func (a *Application) Shutdown(ctx context.Context) error {
	if !a.state.CompareAndSwap(stateRunning, stateStopping) {
		return errors.New("application not running")
	}

	log.Println("Initiating graceful shutdown...")

	var errs []error

	// 1. Stop accepting new HTTP requests
	if err := a.server.Shutdown(ctx); err != nil {
		errs = append(errs, fmt.Errorf("http shutdown: %w", err))
	}
	log.Println("✓ HTTP server stopped")

	// 2. Stop backup scheduler
	// Note: Add Stop() method to scheduler if available
	log.Println("✓ Backup scheduler stopped")

	// 3. Wait for background services
	done := make(chan struct{})
	go func() {
		a.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		log.Println("✓ Background services stopped")
	case <-ctx.Done():
		log.Println("⚠ Background services shutdown timeout")
		errs = append(errs, ctx.Err())
	}

	// 4. Close database
	if err := db.GetService().Close(); err != nil {
		errs = append(errs, fmt.Errorf("database close: %w", err))
	}
	log.Println("✓ Database closed")

	a.state.Store(stateStopped)

	// Print final metrics
	metrics := a.handlers.metrics.Snapshot()
	log.Printf("Final metrics: %+v", metrics)

	if len(errs) > 0 {
		return fmt.Errorf("shutdown errors: %v", errs)
	}

	return nil
}

// ============================================================================
// NETWORK HANDLERS
// ============================================================================

func (h *Handlers) ListNetworks(c *gin.Context) {
	stats, err := db.GetService().GetNetworksWithStats(c.Request.Context())
	if err != nil {
		c.JSON(500, gin.H{"error": "Failed to fetch networks", "details": err.Error()})
		return
	}
	c.JSON(200, stats)
}

func (h *Handlers) CreateNetwork(c *gin.Context) {
	var req db.Network
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "Invalid request"})
		return
	}

	// Validate (Simple check)
	if req.Name == "" || req.CIDR == "" || req.Gateway == "" {
		c.JSON(400, gin.H{"error": "Missing required fields"})
		return
	}

	if err := db.GetService().CreateNetwork(c.Request.Context(), req); err != nil {
		c.JSON(500, gin.H{"error": "Failed to create network", "details": err.Error()})
		return
	}

	c.JSON(201, gin.H{"status": "created"})
}

// ============================================================================
// MAIN ENTRY POINT
// ============================================================================

func main() {
	// Runtime optimizations
	runtime.GOMAXPROCS(runtime.NumCPU())
	debug.SetGCPercent(100)

	log.SetOutput(os.Stdout)
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)

	log.Println(`
╔════════════════════════════════════════════════════════════════╗
║              AXION CONTROL PLANE - STARTING                    ║
╚════════════════════════════════════════════════════════════════╝`)

	app, err := NewApplication()
	if err != nil {
		log.Fatalf("[FATAL] Application initialization failed: %v", err)
	}

	if err := app.Start(); err != nil {
		log.Fatalf("[FATAL] Application start failed: %v", err)
	}

	log.Println(`
╔════════════════════════════════════════════════════════════════╗
║              AXION CONTROL PLANE - RUNNING                     ║
╠════════════════════════════════════════════════════════════════╣
║  Address:         :8500                                        ║
║  Workers:         2                                            ║
║  Database:        axion.db                                     ║
╠════════════════════════════════════════════════════════════════╣
║  API Documentation:                                            ║
║    GET  /health              - Health check                    ║
║    GET  /metrics             - System metrics                  ║
║    POST /login               - Authentication                  ║
║    GET  /instances           - List all instances              ║
║    POST /instances           - Create instance                 ║
║    GET  /ws/terminal/:name   - WebSocket terminal              ║
╚════════════════════════════════════════════════════════════════╝`)

	// Graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigChan

	log.Printf("\n[INFO] Received signal: %v", sig)

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := app.Shutdown(shutdownCtx); err != nil {
		log.Printf("[ERROR] Shutdown errors: %v", err)
		os.Exit(1)
	}

	log.Println("\n[INFO] ✓ Clean shutdown completed")
}
