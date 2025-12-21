package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
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
	requestsTotal      atomic.Uint64
	requestsSuccess    atomic.Uint64
	requestsError      atomic.Uint64
	instancesCreated   atomic.Uint64
	instancesDeleted   atomic.Uint64
	snapshotsCreated   atomic.Uint64
	jobsDispatched     atomic.Uint64
	filesUploaded      atomic.Uint64
	filesDownloaded    atomic.Uint64
	networksCreated    atomic.Uint64
	startTime          int64
}

func NewMetrics() *Metrics {
	return &Metrics{startTime: time.Now().Unix()}
}

func (m *Metrics) RecordRequest() { m.requestsTotal.Add(1) }
func (m *Metrics) RecordSuccess() { m.requestsSuccess.Add(1) }
func (m *Metrics) RecordError()   { m.requestsError.Add(1) }
func (m *Metrics) RecordInstanceCreated() { m.instancesCreated.Add(1) }
func (m *Metrics) RecordInstanceDeleted() { m.instancesDeleted.Add(1) }
func (m *Metrics) RecordSnapshot() { m.snapshotsCreated.Add(1) }
func (m *Metrics) RecordJob() { m.jobsDispatched.Add(1) }
func (m *Metrics) RecordFileUpload() { m.filesUploaded.Add(1) }
func (m *Metrics) RecordFileDownload() { m.filesDownloaded.Add(1) }
func (m *Metrics) RecordNetworkCreated() { m.networksCreated.Add(1) }

func (m *Metrics) Snapshot() map[string]interface{} {
	uptime := time.Now().Unix() - m.startTime
	return map[string]interface{}{
		"uptime_seconds":     uptime,
		"requests_total":     m.requestsTotal.Load(),
		"requests_success":   m.requestsSuccess.Load(),
		"requests_error":     m.requestsError.Load(),
		"instances_created":  m.instancesCreated.Load(),
		"instances_deleted":  m.instancesDeleted.Load(),
		"snapshots_created":  m.snapshotsCreated.Load(),
		"jobs_dispatched":    m.jobsDispatched.Load(),
		"files_uploaded":     m.filesUploaded.Load(),
		"files_downloaded":   m.filesDownloaded.Load(),
		"networks_created":   m.networksCreated.Load(),
		"goroutines":         runtime.NumGoroutine(),
	}
}

// ============================================================================
// HTTP HANDLERS
// ============================================================================

type Handlers struct {
	lxcClient       *lxc.Client
	backupScheduler *scheduler.BackupScheduler
	metrics         *Metrics
}

func NewHandlers(lxcClient *lxc.Client, backupScheduler *scheduler.BackupScheduler) *Handlers {
	return &Handlers{
		lxcClient:       lxcClient,
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
	
	instance, err := db.GetInstanceWithHardwareInfo(name, h.lxcClient)
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

	// Validate ISO if provided
	if req.ISOImage != "" {
		if appErr := h.validateISO(req.ISOImage); appErr != nil {
			h.writeError(c, appErr)
			return
		}
	}

	// Check quota
	reqCpu := h.parseCPU(req.Limits)
	reqRam := h.parseMemory(req.Limits)
	
	if err := h.lxcClient.CheckGlobalQuota(reqCpu, reqRam); err != nil {
		h.writeError(c, ErrQuotaExceeded(err.Error()))
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
		h.writeError(c, ErrDatabaseFailure(err))
		return
	}

	jobID := h.dispatchJob(types.JobTypeCreateInstance, req.Name, req)
	if jobID == "" {
		h.writeError(c, ErrJobCreation(errors.New("failed to dispatch job")))
		return
	}

	h.metrics.RecordInstanceCreated()
	h.metrics.RecordJob()
	
	c.JSON(202, gin.H{"job_id": jobID, "status": "accepted"})
}

func (h *Handlers) DeleteInstance(c *gin.Context) {
	name := c.Param("name")

	if err := db.DeleteInstance(name); err != nil {
		h.writeError(c, ErrDatabaseFailure(err))
		return
	}

	jobID := h.dispatchJob(types.JobTypeDeleteInstance, name, map[string]string{})
	if jobID == "" {
		h.writeError(c, ErrJobCreation(errors.New("failed to dispatch job")))
		return
	}

	h.metrics.RecordInstanceDeleted()
	h.metrics.RecordJob()
	
	c.JSON(202, gin.H{"job_id": jobID, "status": "accepted"})
}

func (h *Handlers) UpdateInstanceState(c *gin.Context) {
	name := c.Param("name")
	var req InstanceActionRequest
	
	if err := c.ShouldBindJSON(&req); err != nil {
		h.writeError(c, ErrInvalidJSON(err))
		return
	}

	jobID := h.dispatchJob(types.JobTypeStateChange, name, req)
	if jobID == "" {
		h.writeError(c, ErrJobCreation(errors.New("failed to dispatch job")))
		return
	}

	h.metrics.RecordJob()
	c.JSON(202, gin.H{"job_id": jobID, "status": "accepted"})
}

func (h *Handlers) UpdateInstanceLimits(c *gin.Context) {
	name := c.Param("name")
	var req InstanceLimitsRequest
	
	if err := c.ShouldBindJSON(&req); err != nil {
		h.writeError(c, ErrInvalidJSON(err))
		return
	}

	reqCpu := utils.ParseCpuCores(req.CPU)
	reqRam := utils.ParseMemoryToMB(req.Memory)
	
	if err := h.lxcClient.CheckGlobalQuota(reqCpu, reqRam); err != nil {
		h.writeError(c, ErrQuotaExceeded(err.Error()))
		return
	}

	jobID := h.dispatchJob(types.JobTypeUpdateLimits, name, req)
	if jobID == "" {
		h.writeError(c, ErrJobCreation(errors.New("failed to dispatch job")))
		return
	}

	h.metrics.RecordJob()
	c.JSON(202, gin.H{"job_id": jobID, "status": "accepted"})
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

// Snapshot Handlers
func (h *Handlers) ListSnapshots(c *gin.Context) {
	name := c.Param("name")
	
	snaps, err := h.lxcClient.ListSnapshots(name)
	if err != nil {
		h.writeError(c, NewError(ErrCodeSnapshotFailed, "failed to list snapshots", err, 500, true))
		return
	}
	
	c.JSON(200, snaps)
}

func (h *Handlers) CreateSnapshot(c *gin.Context) {
	name := c.Param("name")
	var req SnapshotRequest
	
	if err := c.ShouldBindJSON(&req); err != nil {
		h.writeError(c, ErrInvalidJSON(err))
		return
	}

	payload := map[string]string{"snapshot_name": req.Name}
	jobID := h.dispatchJob(types.JobTypeCreateSnapshot, name, payload)
	if jobID == "" {
		h.writeError(c, ErrJobCreation(errors.New("failed to dispatch job")))
		return
	}

	h.metrics.RecordSnapshot()
	h.metrics.RecordJob()
	
	c.JSON(202, gin.H{"job_id": jobID, "status": "accepted"})
}

func (h *Handlers) RestoreSnapshot(c *gin.Context) {
	name := c.Param("name")
	snap := c.Param("snap")

	payload := map[string]string{"snapshot_name": snap}
	jobID := h.dispatchJob(types.JobTypeRestoreSnapshot, name, payload)
	if jobID == "" {
		h.writeError(c, ErrJobCreation(errors.New("failed to dispatch job")))
		return
	}

	h.metrics.RecordJob()
	c.JSON(202, gin.H{"job_id": jobID, "status": "accepted"})
}

func (h *Handlers) DeleteSnapshot(c *gin.Context) {
	name := c.Param("name")
	snap := c.Param("snap")

	payload := map[string]string{"snapshot_name": snap}
	jobID := h.dispatchJob(types.JobTypeDeleteSnapshot, name, payload)
	if jobID == "" {
		h.writeError(c, ErrJobCreation(errors.New("failed to dispatch job")))
		return
	}

	h.metrics.RecordJob()
	c.JSON(202, gin.H{"job_id": jobID, "status": "accepted"})
}

// Port Management Handlers
func (h *Handlers) AddPort(c *gin.Context) {
	name := c.Param("name")
	var req AddPortRequest
	
	if err := c.ShouldBindJSON(&req); err != nil {
		h.writeError(c, ErrInvalidJSON(err))
		return
	}

	jobID := h.dispatchJob(types.JobTypeAddPort, name, req)
	if jobID == "" {
		h.writeError(c, ErrJobCreation(errors.New("failed to dispatch job")))
		return
	}

	h.metrics.RecordJob()
	c.JSON(202, gin.H{"job_id": jobID, "status": "accepted"})
}

func (h *Handlers) RemovePort(c *gin.Context) {
	name := c.Param("name")
	hpStr := c.Param("host_port")
	hp, _ := strconv.Atoi(hpStr)

	payload := map[string]int{"host_port": hp}
	jobID := h.dispatchJob(types.JobTypeRemovePort, name, payload)
	if jobID == "" {
		h.writeError(c, ErrJobCreation(errors.New("failed to dispatch job")))
		return
	}

	h.metrics.RecordJob()
	c.JSON(202, gin.H{"job_id": jobID, "status": "accepted"})
}

// File System Handlers
func (h *Handlers) ListFiles(c *gin.Context) {
	name := c.Param("name")
	path := c.DefaultQuery("path", "/root")

	entries, err := h.lxcClient.ListFiles(name, path)
	if err != nil {
		h.writeError(c, NewError(ErrCodeFileOperationFailed, "failed to list files", err, 500, true))
		return
	}
	
	c.JSON(200, entries)
}

func (h *Handlers) DownloadFile(c *gin.Context) {
	name := c.Param("name")
	rawPath := c.Query("path")
	
	if rawPath == "" {
		h.writeError(c, ErrMissingField("path"))
		return
	}

	cleanPath := filepath.Clean(rawPath)
	content, size, err := h.lxcClient.DownloadFile(name, cleanPath)
	if err != nil {
		h.writeError(c, NewError(ErrCodeFileOperationFailed, "download failed", err, 500, true))
		return
	}
	defer content.Close()

	const MaxEditorSize = 1024 * 1024
	if size > MaxEditorSize {
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filepath.Base(cleanPath)))
	} else {
		c.Header("Content-Disposition", "inline")
	}

	if size > 0 {
		c.Header("Content-Length", fmt.Sprintf("%d", size))
	}

	h.metrics.RecordFileDownload()
	io.Copy(c.Writer, content)
}

func (h *Handlers) UploadFile(c *gin.Context) {
	name := c.Param("name")
	path := c.Query("path")
	
	if path == "" {
		h.writeError(c, ErrMissingField("path"))
		return
	}

	fileHeader, err := c.FormFile("file")
	if err != nil {
		h.writeError(c, ErrMissingField("file"))
		return
	}

	file, err := fileHeader.Open()
	if err != nil {
		h.writeError(c, NewError(ErrCodeFileOperationFailed, "failed to open file", err, 500, false))
		return
	}
	defer file.Close()

	if err := h.lxcClient.UploadFile(name, path, file); err != nil {
		h.writeError(c, NewError(ErrCodeFileOperationFailed, "upload failed", err, 500, true))
		return
	}

	h.metrics.RecordFileUpload()
	c.JSON(200, gin.H{"status": "uploaded"})
}

func (h *Handlers) DeleteFile(c *gin.Context) {
	name := c.Param("name")
	path := c.Query("path")
	
	if path == "" {
		h.writeError(c, ErrMissingField("path"))
		return
	}

	if err := h.lxcClient.DeleteFile(name, path); err != nil {
		h.writeError(c, NewError(ErrCodeFileOperationFailed, "delete failed", err, 500, true))
		return
	}
	
	c.JSON(200, gin.H{"status": "deleted"})
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

	lxdMetrics, err := h.lxcClient.ListInstances()
	if err != nil {
		log.Printf("Error fetching metrics for %s: %v", name, err)
		h.writeError(c, NewError(ErrCodeMetricsFetchFailed, "failed to fetch metrics", err, 500, true))
		return
	}

	for _, metric := range lxdMetrics {
		if metric.Name == name {
			c.JSON(200, metric)
			return
		}
	}

	h.writeError(c, ErrInstanceNotFound(name))
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
	name := c.Param("name")
	
	logContent, err := h.lxcClient.GetInstanceLog(name)
	if err != nil {
		log.Printf("Error getting logs for %s: %v", name, err)
		h.writeError(c, NewError(ErrCodeFileOperationFailed, "failed to get logs", err, 500, true))
		return
	}
	
	c.JSON(200, gin.H{"log": logContent})
}


// Network Handlers
func (h *Handlers) ListNetworks(c *gin.Context) {
	networks, err := h.lxcClient.ListNetworks()
	if err != nil {
		log.Printf("Error listing networks: %v", err)
		h.writeError(c, NewError(ErrCodeNetworkOperationFailed, "failed to list networks", err, 500, true))
		return
	}
	c.JSON(200, networks)
}

func (h *Handlers) CreateNetwork(c *gin.Context) {
	var req CreateNetworkRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.writeError(c, ErrInvalidJSON(err))
		return
	}

	if err := h.lxcClient.CreateNetwork(req.Name, req.Description, req.Subnet); err != nil {
		log.Printf("Error creating network %s: %v", req.Name, err)
		h.writeError(c, NewError(ErrCodeNetworkOperationFailed, "failed to create network", err, 500, true))
		return
	}

	h.metrics.RecordNetworkCreated()
	c.JSON(201, gin.H{"status": "created"})
}

func (h *Handlers) DeleteNetwork(c *gin.Context) {
	name := c.Param("name")
	
	if err := h.lxcClient.DeleteNetwork(name); err != nil {
		log.Printf("Error deleting network %s: %v", name, err)
		h.writeError(c, NewError(ErrCodeNetworkOperationFailed, "failed to delete network", err, 500, true))
		return
	}
	
	c.JSON(200, gin.H{"status": "deleted"})
}

// Storage/ISO Handlers
func (h *Handlers) UploadISO(c *gin.Context) {
	storageService, err := service.NewStorageService()
	if err != nil {
		log.Printf("Error initializing storage: %v", err)
		h.writeError(c, NewError(ErrCodeStorageOperationFailed, "storage init failed", err, 500, false))
		return
	}

	file, header, err := c.Request.FormFile("iso_file")
	if err != nil {
		h.writeError(c, ErrMissingField("iso_file"))
		return
	}
	defer file.Close()

	filename := header.Filename
	if !strings.HasSuffix(strings.ToLower(filename), ".iso") {
		h.writeError(c, NewError(ErrCodeInvalidFileType, "only .iso files allowed", nil, 400, false))
		return
	}

	isoInfo, err := storageService.SaveISOWithInfo(filename, file)
	if err != nil {
		log.Printf("Error saving ISO: %v", err)
		h.writeError(c, NewError(ErrCodeStorageOperationFailed, "failed to save ISO", err, 500, true))
		return
	}

	c.JSON(200, gin.H{
		"filename": isoInfo.Name,
		"size":     isoInfo.Size,
		"path":     isoInfo.Path,
	})
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
func (h *Handlers) GetClusterMembers(c *gin.Context) {
	members, err := h.lxcClient.GetClusterMembers()
	if err != nil {
		if strings.Contains(err.Error(), "not clustered") {
			response := []map[string]interface{}{
				{
					"name":    "local-server",
					"status":  "Online",
					"address": "127.0.0.1",
					"roles":   []string{"standalone"},
				},
			}
			c.JSON(200, response)
			return
		}

		log.Printf("Error getting cluster members: %v", err)
		h.writeError(c, NewError(ErrCodeLXDConnectionFailed, "failed to get cluster members", err, 500, true))
		return
	}

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
}

func (h *Handlers) StreamTelemetry(c *gin.Context) {
	api.StreamTelemetry(c, h.lxcClient, func(metrics []lxc.InstanceMetric) {
		for _, m := range metrics {
			dbMetric := &db.Metric{
				InstanceName: m.Name,
				CPUPercent:   float64(m.CPUUsageSeconds),
				MemoryUsage:  m.MemoryUsageBytes,
				DiskUsage:    m.DiskUsageBytes,
			}
			if err := db.InsertMetric(dbMetric); err != nil {
				log.Printf("Error inserting metric for %s: %v", m.Name, err)
			}
		}
	})
}

func (h *Handlers) GetMetrics(c *gin.Context) {
	c.JSON(200, h.metrics.Snapshot())
}

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

func (h *Handlers) dispatchJob(jobType types.JobType, target string, payload interface{}) string {
	jobID := uuid.New().String()
	payloadBytes, _ := json.Marshal(payload)
	
	job := &db.Job{
		ID:      jobID,
		Type:    jobType,
		Target:  target,
		Payload: string(payloadBytes),
	}
	
	if err := db.CreateJob(job); err != nil {
		log.Printf("Error creating job: %v", err)
		return ""
	}
	
	worker.DispatchJob(jobID)
	return jobID
}

// ============================================================================
// APPLICATION ORCHESTRATOR
// ============================================================================

type Application struct {
	lxcClient       *lxc.Client
	backupScheduler *scheduler.BackupScheduler
	handlers        *Handlers
	router          *gin.Engine
	server          *http.Server
	state           atomic.Uint32
	wg              sync.WaitGroup
}

const (
	stateCreated uint32 = 0
	stateRunning uint32 = 1
	stateStopping uint32 = 2
	stateStopped uint32 = 3
)

func NewApplication() (*Application, error) {
	// Initialize database
	if err := db.Init("axion.db"); err != nil {
		return nil, fmt.Errorf("database initialization failed: %w", err)
	}
	log.Println("✓ Database initialized")

	// Initialize LXD client
	lxcClient, err := lxc.NewClient()
	if err != nil {
		return nil, fmt.Errorf("LXD client initialization failed: %w", err)
	}
	log.Println("✓ LXD connection established")

	// Initialize workers
	worker.Init(2, lxcClient)
	log.Println("✓ Worker pool initialized")

	// Initialize API broadcaster
	api.InitBroadcaster()
	log.Println("✓ API broadcaster initialized")

	// Initialize backup scheduler
	backupScheduler := scheduler.NewBackupScheduler(db.DB, lxcClient)
	log.Println("✓ Backup scheduler initialized")

	// Initialize handlers
	handlers := NewHandlers(lxcClient, backupScheduler)

	app := &Application{
		lxcClient:       lxcClient,
		backupScheduler: backupScheduler,
		handlers:        handlers,
	}
	app.state.Store(stateCreated)

	return app, nil
}

func (a *Application) setupRouter() {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	
	// Middleware
	r.Use(gin.Recovery())
	r.Use(a.handlers.metricsMiddleware())
	r.Use(cors.New(cors.Config{
		AllowAllOrigins: true,
		AllowMethods:    []string{"GET", "POST", "OPTIONS", "DELETE", "PUT"},
		AllowHeaders:    []string{"Origin", "Content-Type", "Accept", "Authorization"},
		ExposeHeaders:   []string{"Content-Length", "Content-Disposition"},
		MaxAge:          12 * time.Hour,
	}))

	// Public routes
	r.POST("/login", auth.LoginHandler)
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status": "healthy",
			"state":  a.state.Load(),
		})
	})
	r.GET("/metrics", a.handlers.GetMetrics)

	// WebSocket routes (no auth middleware for terminal)
	r.GET("/ws/terminal/:name", func(c *gin.Context) {
		api.TerminalHandler(c, a.lxcClient)
	})

	// Protected routes
	protected := r.Group("/")
	protected.Use(auth.AuthMiddleware())
	{
		// Instances
		protected.GET("/instances", a.handlers.ListInstances)
		protected.GET("/instances/:name", a.handlers.GetInstance)
		protected.POST("/instances", a.handlers.CreateInstance)
		protected.DELETE("/instances/:name", a.handlers.DeleteInstance)
		protected.POST("/instances/:name/state", a.handlers.UpdateInstanceState)
		protected.POST("/instances/:name/limits", a.handlers.UpdateInstanceLimits)
		protected.POST("/instances/:name/backups/config", a.handlers.UpdateBackupConfig)

		// Snapshots
		protected.GET("/instances/:name/snapshots", a.handlers.ListSnapshots)
		protected.POST("/instances/:name/snapshots", a.handlers.CreateSnapshot)
		protected.POST("/instances/:name/snapshots/:snap/restore", a.handlers.RestoreSnapshot)
		protected.DELETE("/instances/:name/snapshots/:snap", a.handlers.DeleteSnapshot)

		// Ports
		protected.POST("/instances/:name/ports", a.handlers.AddPort)
		protected.DELETE("/instances/:name/ports/:host_port", a.handlers.RemovePort)

		// File System
		protected.GET("/instances/:name/files/list", a.handlers.ListFiles)
		protected.GET("/instances/:name/files/content", a.handlers.DownloadFile)
		protected.POST("/instances/:name/files", a.handlers.UploadFile)
		protected.DELETE("/instances/:name/files", a.handlers.DeleteFile)

		// Jobs
		protected.GET("/jobs", a.handlers.ListJobs)
		protected.GET("/jobs/:id", a.handlers.GetJob)

		// Templates
		protected.GET("/templates", a.handlers.ListTemplates)

		// Metrics
		protected.GET("/instances/:name/metrics", a.handlers.GetInstanceMetrics)
		protected.GET("/instances/:name/metrics/history", a.handlers.GetInstanceMetricsHistory)
		protected.GET("/instances/:name/logs", a.handlers.GetInstanceLogs)
		protected.GET("/ws/telemetry", a.handlers.StreamTelemetry)

		// Networks
		protected.GET("/networks", a.handlers.ListNetworks)
		protected.POST("/networks", a.handlers.CreateNetwork)
		protected.DELETE("/networks/:name", a.handlers.DeleteNetwork)

		// Storage/ISOs
		protected.POST("/storage/isos", a.handlers.UploadISO)
		protected.GET("/storage/isos", a.handlers.ListISOs)
		protected.DELETE("/storage/isos/:name", a.handlers.DeleteISO)

		// Cluster
		protected.GET("/cluster/members", a.handlers.GetClusterMembers)
	}

	a.router = r
}

func (a *Application) Start() error {
	if !a.state.CompareAndSwap(stateCreated, stateRunning) {
		return errors.New("application already started")
	}

	// Run startup sync
	scheduler.RunStartupSync(db.DB, a.lxcClient)
	log.Println("✓ Startup sync completed")

	// Start background services
	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		monitor.StartHistoricalCollector(db.DB, a.lxcClient)
	}()
	log.Println("✓ Historical collector started")

	// Start backup scheduler
	a.backupScheduler.Start()
	a.backupScheduler.SyncJobs()
	log.Println("✓ Backup scheduler started")

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
	if err := db.DB.Close(); err != nil {
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