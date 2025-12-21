package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"aexon/internal/auth"
	"aexon/internal/provider/lxc"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// ============================================================================
// ERROR TAXONOMY
// ============================================================================

type ErrorCode int

const (
	ErrCodeAuthFailed ErrorCode = iota + 4000
	ErrCodeTokenMissing
	ErrCodeTokenInvalid
	ErrCodeWebSocketUpgradeFailed
	ErrCodeInstanceNotFound
	ErrCodeExecFailed
	ErrCodeControlTimeout
	ErrCodeResizeFailed
	ErrCodeMessageParseFailed
	ErrCodeConnectionClosed
	ErrCodeWriteFailed
	ErrCodeUnexpectedMessage
)

type TerminalError struct {
	Code      ErrorCode
	Message   string
	Err       error
	Timestamp int64
}

func (e *TerminalError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("[%d] %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("[%d] %s", e.Code, e.Message)
}

func NewTerminalError(code ErrorCode, msg string, err error) *TerminalError {
	return &TerminalError{
		Code:      code,
		Message:   msg,
		Err:       err,
		Timestamp: time.Now().UnixNano(),
	}
}

// Error constructors
func ErrTokenMissing() *TerminalError {
	return NewTerminalError(ErrCodeTokenMissing, "authentication token missing", nil)
}

func ErrTokenInvalid(err error) *TerminalError {
	return NewTerminalError(ErrCodeTokenInvalid, "invalid authentication token", err)
}

func ErrUpgradeFailed(err error) *TerminalError {
	return NewTerminalError(ErrCodeWebSocketUpgradeFailed, "websocket upgrade failed", err)
}

func ErrExecFailed(err error) *TerminalError {
	return NewTerminalError(ErrCodeExecFailed, "failed to execute command", err)
}

func ErrControlTimeout() *TerminalError {
	return NewTerminalError(ErrCodeControlTimeout, "timeout waiting for exec control", nil)
}

func ErrResizeFailed(err error) *TerminalError {
	return NewTerminalError(ErrCodeResizeFailed, "terminal resize failed", err)
}

// ============================================================================
// WEBSOCKET CONFIGURATION
// ============================================================================

var upgrader = websocket.Upgrader{
	ReadBufferSize:  8192,
	WriteBufferSize: 8192,
	CheckOrigin: func(r *http.Request) bool {
		return true // In production, validate origin properly
	},
	HandshakeTimeout: 10 * time.Second,
}

const (
	// Connection timeouts
	controlTimeout     = 10 * time.Second
	writeTimeout       = 10 * time.Second
	readTimeout        = 60 * time.Second
	pingInterval       = 30 * time.Second
	maxMessageSize     = 8192
	shutdownTimeout    = 5 * time.Second
	
	// Message types
	msgTypeResize = "resize"
	msgTypeData   = "data"
	msgTypePing   = "ping"
	msgTypePong   = "pong"
)

// ============================================================================
// METRICS
// ============================================================================

type TerminalMetrics struct {
	sessionsTotal     atomic.Uint64
	sessionsActive    atomic.Int64
	messagesSent      atomic.Uint64
	messagesReceived  atomic.Uint64
	resizeCommands    atomic.Uint64
	errors            atomic.Uint64
	authFailures      atomic.Uint64
	upgradeFailures   atomic.Uint64
}

var globalMetrics = &TerminalMetrics{}

func (m *TerminalMetrics) Snapshot() map[string]interface{} {
	return map[string]interface{}{
		"sessions_total":    m.sessionsTotal.Load(),
		"sessions_active":   m.sessionsActive.Load(),
		"messages_sent":     m.messagesSent.Load(),
		"messages_received": m.messagesReceived.Load(),
		"resize_commands":   m.resizeCommands.Load(),
		"errors":            m.errors.Load(),
		"auth_failures":     m.authFailures.Load(),
		"upgrade_failures":  m.upgradeFailures.Load(),
	}
}

// ============================================================================
// WEBSOCKET WRITER
// ============================================================================

type wsWriter struct {
	conn   *websocket.Conn
	mu     sync.Mutex
	closed atomic.Bool
}

func newWSWriter(conn *websocket.Conn) *wsWriter {
	return &wsWriter{conn: conn}
}

func (w *wsWriter) Write(p []byte) (n int, err error) {
	if w.closed.Load() {
		return 0, errors.New("writer closed")
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	w.conn.SetWriteDeadline(time.Now().Add(writeTimeout))
	
	if err := w.conn.WriteMessage(websocket.BinaryMessage, p); err != nil {
		globalMetrics.errors.Add(1)
		return 0, fmt.Errorf("write failed: %w", err)
	}

	globalMetrics.messagesSent.Add(1)
	return len(p), nil
}

func (w *wsWriter) Close() error {
	w.closed.Store(true)
	return nil
}

// ============================================================================
// MESSAGE TYPES
// ============================================================================

type ResizeMessage struct {
	Type string `json:"type"`
	Cols int    `json:"cols"`
	Rows int    `json:"rows"`
}

type LXDResizeCommand struct {
	Command string `json:"command"`
	Width   int    `json:"width"`
	Height  int    `json:"height"`
}

type ControlMessage struct {
	Type    string `json:"type"`
	Payload []byte `json:"payload,omitempty"`
}

// ============================================================================
// TERMINAL SESSION
// ============================================================================

type TerminalSession struct {
	instanceName    string
	conn            *websocket.Conn
	execControl     *websocket.Conn
	stdinReader     *io.PipeReader
	stdinWriter     *io.PipeWriter
	stdoutWriter    *wsWriter
	instanceService *lxc.InstanceService
	
	ctx             context.Context
	cancel          context.CancelFunc
	
	state           atomic.Uint32
	errCh           chan error
	controlCh       chan *websocket.Conn
	
	wg              sync.WaitGroup
	closeOnce       sync.Once
}

const (
	sessionStateCreated uint32 = 0
	sessionStateRunning uint32 = 1
	sessionStateClosing uint32 = 2
	sessionStateClosed  uint32 = 3
)

func NewTerminalSession(instanceName string, conn *websocket.Conn, instanceService *lxc.InstanceService) *TerminalSession {
	ctx, cancel := context.WithCancel(context.Background())
	stdinReader, stdinWriter := io.Pipe()
	
	session := &TerminalSession{
		instanceName:    instanceName,
		conn:            conn,
		stdinReader:     stdinReader,
		stdinWriter:     stdinWriter,
		stdoutWriter:    newWSWriter(conn),
		instanceService: instanceService,
		ctx:             ctx,
		cancel:          cancel,
		errCh:           make(chan error, 1),
		controlCh:       make(chan *websocket.Conn, 1),
	}
	
	session.state.Store(sessionStateCreated)
	return session
}

func (s *TerminalSession) Start() error {
	if !s.state.CompareAndSwap(sessionStateCreated, sessionStateRunning) {
		return errors.New("session already started")
	}

	globalMetrics.sessionsTotal.Add(1)
	globalMetrics.sessionsActive.Add(1)

	// Start exec goroutine
	s.wg.Add(1)
	go s.execLoop()

	// Wait for control channel with timeout
	select {
	case s.execControl = <-s.controlCh:
		log.Printf("[Session %s] Control channel established", s.instanceName)
	case <-time.After(controlTimeout):
		s.Close()
		globalMetrics.errors.Add(1)
		return ErrControlTimeout()
	case <-s.ctx.Done():
		return s.ctx.Err()
	}

	// Start message reader
	s.wg.Add(1)
	go s.readLoop()

	// Start ping/pong keeper
	s.wg.Add(1)
	go s.pingLoop()

	return nil
}

func (s *TerminalSession) execLoop() {
	defer s.wg.Done()
	defer log.Printf("[Session %s] Exec loop terminated", s.instanceName)

	controlFunc := func(control *websocket.Conn) {
		select {
		case s.controlCh <- control:
		case <-s.ctx.Done():
		}
	}

	err := s.instanceService.ExecInteractive(
		s.instanceName,
		[]string{"/bin/bash"},
		s.stdinReader,
		s.stdoutWriter,
		s.stdoutWriter,
		controlFunc,
	)

	if err != nil {
		log.Printf("[Session %s] Exec ended with error: %v", s.instanceName, err)
		globalMetrics.errors.Add(1)
		
		// Notify client
		s.writeErrorMessage(fmt.Sprintf("Session ended: %v", err))
	} else {
		log.Printf("[Session %s] Exec ended normally", s.instanceName)
	}

	s.Close()
}

func (s *TerminalSession) readLoop() {
	defer s.wg.Done()
	defer log.Printf("[Session %s] Read loop terminated", s.instanceName)
	defer s.Close()

	s.conn.SetReadLimit(maxMessageSize)
	s.conn.SetReadDeadline(time.Now().Add(readTimeout))
	s.conn.SetPongHandler(func(string) error {
		s.conn.SetReadDeadline(time.Now().Add(readTimeout))
		return nil
	})

	for {
		select {
		case <-s.ctx.Done():
			return
		default:
		}

		mt, message, err := s.conn.ReadMessage()
		if err != nil {
			if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				log.Printf("[Session %s] Client closed connection", s.instanceName)
			} else {
				log.Printf("[Session %s] Read error: %v", s.instanceName, err)
				globalMetrics.errors.Add(1)
			}
			return
		}

		globalMetrics.messagesReceived.Add(1)
		s.conn.SetReadDeadline(time.Now().Add(readTimeout))

		if err := s.handleMessage(mt, message); err != nil {
			log.Printf("[Session %s] Message handling error: %v", s.instanceName, err)
			globalMetrics.errors.Add(1)
		}
	}
}

func (s *TerminalSession) handleMessage(messageType int, message []byte) error {
	switch messageType {
	case websocket.TextMessage:
		return s.handleTextMessage(message)
	case websocket.BinaryMessage:
		return s.handleBinaryMessage(message)
	case websocket.PingMessage:
		return s.conn.WriteControl(websocket.PongMessage, nil, time.Now().Add(writeTimeout))
	default:
		return fmt.Errorf("unexpected message type: %d", messageType)
	}
}

func (s *TerminalSession) handleTextMessage(message []byte) error {
	var msg ResizeMessage
	
	if err := json.Unmarshal(message, &msg); err == nil && msg.Type == msgTypeResize {
		return s.handleResize(msg.Cols, msg.Rows)
	}

	// Not a resize command, treat as stdin data
	return s.writeToStdin(message)
}

func (s *TerminalSession) handleBinaryMessage(message []byte) error {
	return s.writeToStdin(message)
}

func (s *TerminalSession) handleResize(cols, rows int) error {
	if s.execControl == nil {
		return errors.New("control channel not available")
	}

	if cols <= 0 || rows <= 0 {
		return fmt.Errorf("invalid dimensions: cols=%d rows=%d", cols, rows)
	}

	lxdMsg := LXDResizeCommand{
		Command: "window-resize",
		Width:   cols,
		Height:  rows,
	}

	s.execControl.SetWriteDeadline(time.Now().Add(writeTimeout))
	if err := s.execControl.WriteJSON(lxdMsg); err != nil {
		globalMetrics.errors.Add(1)
		return ErrResizeFailed(err)
	}

	globalMetrics.resizeCommands.Add(1)
	log.Printf("[Session %s] Terminal resized to %dx%d", s.instanceName, cols, rows)
	return nil
}

func (s *TerminalSession) writeToStdin(data []byte) error {
	if len(data) == 0 {
		return nil
	}

	select {
	case <-s.ctx.Done():
		return s.ctx.Err()
	default:
	}

	_, err := s.stdinWriter.Write(data)
	if err != nil {
		globalMetrics.errors.Add(1)
		return fmt.Errorf("stdin write failed: %w", err)
	}

	return nil
}

func (s *TerminalSession) writeErrorMessage(msg string) {
	errorMsg := fmt.Sprintf("\r\n[ERROR] %s\r\n", msg)
	s.conn.SetWriteDeadline(time.Now().Add(writeTimeout))
	s.conn.WriteMessage(websocket.TextMessage, []byte(errorMsg))
}

func (s *TerminalSession) pingLoop() {
	defer s.wg.Done()
	ticker := time.NewTicker(pingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.conn.SetWriteDeadline(time.Now().Add(writeTimeout))
			if err := s.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				log.Printf("[Session %s] Ping failed: %v", s.instanceName, err)
				s.Close()
				return
			}
		case <-s.ctx.Done():
			return
		}
	}
}

func (s *TerminalSession) Close() {
	s.closeOnce.Do(func() {
		if !s.state.CompareAndSwap(sessionStateRunning, sessionStateClosing) {
			return
		}

		log.Printf("[Session %s] Initiating shutdown", s.instanceName)

		// Cancel context to signal all goroutines
		s.cancel()

		// Close pipes
		s.stdinWriter.Close()
		s.stdinReader.Close()
		s.stdoutWriter.Close()

		// Close exec control
		if s.execControl != nil {
			s.execControl.WriteMessage(
				websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""),
			)
			s.execControl.Close()
		}

		// Close main connection
		s.conn.WriteMessage(
			websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""),
		)
		s.conn.Close()

		// Wait for all goroutines with timeout
		done := make(chan struct{})
		go func() {
			s.wg.Wait()
			close(done)
		}()

		select {
		case <-done:
			log.Printf("[Session %s] Clean shutdown", s.instanceName)
		case <-time.After(shutdownTimeout):
			log.Printf("[Session %s] Shutdown timeout", s.instanceName)
		}

		s.state.Store(sessionStateClosed)
		globalMetrics.sessionsActive.Add(-1)
	})
}

// ============================================================================
// HTTP HANDLER
// ============================================================================

func TerminalHandler(c *gin.Context, instanceService *lxc.InstanceService) {
	instanceName := c.Param("name")
	token := c.Query("token")

	// Validate authentication
	if token == "" {
		globalMetrics.authFailures.Add(1)
		c.JSON(401, gin.H{
			"error": "authentication required",
			"code":  ErrCodeTokenMissing,
		})
		return
	}

	if _, err := auth.ValidateToken(token); err != nil {
		globalMetrics.authFailures.Add(1)
		log.Printf("[Terminal] Auth failed for instance %s: %v", instanceName, err)
		c.JSON(401, gin.H{
			"error": "invalid token",
			"code":  ErrCodeTokenInvalid,
		})
		return
	}

	// Upgrade to WebSocket
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		globalMetrics.upgradeFailures.Add(1)
		log.Printf("[Terminal] WebSocket upgrade failed for %s: %v", instanceName, err)
		return
	}

	log.Printf("[Terminal] New session for instance: %s", instanceName)

	// Create and start session
	session := NewTerminalSession(instanceName, conn, instanceService)
	
	if err := session.Start(); err != nil {
		log.Printf("[Terminal] Session start failed for %s: %v", instanceName, err)
		session.writeErrorMessage(err.Error())
		session.Close()
		return
	}

	// Block until session closes
	<-session.ctx.Done()
	log.Printf("[Terminal] Session completed for instance: %s", instanceName)
}

// ============================================================================
// METRICS ENDPOINT
// ============================================================================

func GetTerminalMetrics(c *gin.Context) {
	c.JSON(200, globalMetrics.Snapshot())
}

// ============================================================================
// GRACEFUL SHUTDOWN & SESSION TRACKING
// ============================================================================

var (
	activeSessions   = make(map[string]*TerminalSession)
	sessionsMutex    sync.RWMutex
)

func RegisterSession(id string, session *TerminalSession) {
	sessionsMutex.Lock()
	defer sessionsMutex.Unlock()
	activeSessions[id] = session
	log.Printf("[SessionTracker] Registered session: %s (total: %d)", id, len(activeSessions))
}

func UnregisterSession(id string) {
	sessionsMutex.Lock()
	defer sessionsMutex.Unlock()
	delete(activeSessions, id)
	log.Printf("[SessionTracker] Unregistered session: %s (remaining: %d)", id, len(activeSessions))
}

func GetActiveSessionCount() int {
	sessionsMutex.RLock()
	defer sessionsMutex.RUnlock()
	return len(activeSessions)
}

func GetActiveSessionIDs() []string {
	sessionsMutex.RLock()
	defer sessionsMutex.RUnlock()
	
	ids := make([]string, 0, len(activeSessions))
	for id := range activeSessions {
		ids = append(ids, id)
	}
	return ids
}

func ShutdownAllSessions(ctx context.Context) error {
	sessionsMutex.RLock()
	sessions := make([]*TerminalSession, 0, len(activeSessions))
	sessionIDs := make([]string, 0, len(activeSessions))
	for id, session := range activeSessions {
		sessions = append(sessions, session)
		sessionIDs = append(sessionIDs, id)
	}
	sessionsMutex.RUnlock()

	if len(sessions) == 0 {
		log.Println("[Terminal] No active sessions to shutdown")
		return nil
	}

	log.Printf("[Terminal] Initiating shutdown of %d active sessions: %v", len(sessions), sessionIDs)

	// Close all sessions
	for i, session := range sessions {
		log.Printf("[Terminal] Closing session %d/%d: %s", i+1, len(sessions), sessionIDs[i])
		session.Close()
	}

	// Wait for all sessions to close with timeout
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	startTime := time.Now()
	for {
		select {
		case <-ctx.Done():
			remaining := globalMetrics.sessionsActive.Load()
			if remaining > 0 {
				log.Printf("[Terminal] Shutdown timeout after %v: %d sessions still active", 
					time.Since(startTime), remaining)
				return fmt.Errorf("shutdown timeout: %d sessions remaining", remaining)
			}
			return nil
			
		case <-ticker.C:
			active := globalMetrics.sessionsActive.Load()
			if active == 0 {
				log.Printf("[Terminal] All sessions closed successfully in %v", time.Since(startTime))
				return nil
			}
			
			// Log progress every second
			if time.Since(startTime).Truncate(time.Second) == time.Since(startTime) {
				log.Printf("[Terminal] Waiting for %d sessions to close...", active)
			}
		}
	}
}

// ============================================================================
// HTTP HANDLER
// ============================================================================

func TerminalHandler(c *gin.Context, instanceService *lxc.InstanceService) {
	instanceName := c.Param("name")
	token := c.Query("token")

	// Validate authentication
	if token == "" {
		globalMetrics.authFailures.Add(1)
		c.JSON(401, gin.H{
			"error": "authentication required",
			"code":  ErrCodeTokenMissing,
		})
		return
	}

	if _, err := auth.ValidateToken(token); err != nil {
		globalMetrics.authFailures.Add(1)
		log.Printf("[Terminal] Auth failed for instance %s: %v", instanceName, err)
		c.JSON(401, gin.H{
			"error": "invalid token",
			"code":  ErrCodeTokenInvalid,
		})
		return
	}

	// Upgrade to WebSocket
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		globalMetrics.upgradeFailures.Add(1)
		log.Printf("[Terminal] WebSocket upgrade failed for %s: %v", instanceName, err)
		return
	}

	// Generate unique session ID
	sessionID := fmt.Sprintf("%s-%d", instanceName, time.Now().UnixNano())
	log.Printf("[Terminal] New session %s for instance: %s", sessionID, instanceName)

	// Create session
	session := NewTerminalSession(instanceName, conn, instanceService)
	
	// CRITICAL: Register session for graceful shutdown tracking
	RegisterSession(sessionID, session)
	defer UnregisterSession(sessionID)
	
	// Start session
	if err := session.Start(); err != nil {
		log.Printf("[Terminal] Session %s start failed: %v", sessionID, err)
		session.writeErrorMessage(err.Error())
		session.Close()
		return
	}

	// Block until session closes
	<-session.ctx.Done()
	log.Printf("[Terminal] Session %s completed for instance: %s", sessionID, instanceName)
}

// ============================================================================
// METRICS ENDPOINT
// ============================================================================

func GetTerminalMetrics(c *gin.Context) {
	metrics := globalMetrics.Snapshot()
	metrics["active_session_count"] = GetActiveSessionCount()
	metrics["active_session_ids"] = GetActiveSessionIDs()
	
	c.JSON(200, metrics)
}

// ============================================================================
// ROUTER REGISTRATION
// ============================================================================

// RegisterTerminalRoutes registers all terminal-related routes
// Call this in your main.go during router setup
func RegisterTerminalRoutes(r *gin.Engine, instanceService *lxc.InstanceService) {
	// WebSocket endpoint (no auth middleware - token in query param)
	r.GET("/ws/terminal/:name", func(c *gin.Context) {
		TerminalHandler(c, instanceService)
	})
	
	// Metrics endpoint (can be public or protected)
	r.GET("/terminal/metrics", GetTerminalMetrics)
	
	// Admin endpoints (should be protected with auth)
	admin := r.Group("/terminal/admin")
	// admin.Use(auth.AdminMiddleware()) // Uncomment if you have admin auth
	{
		admin.GET("/sessions", ListActiveSessionsHandler)
		admin.DELETE("/sessions/:id", CloseSessionHandler)
		admin.POST("/shutdown", ShutdownAllSessionsHandler)
	}
}

// ============================================================================
// INTEGRATION WITH MAIN APPLICATION
// ============================================================================

// Example integration in main.go:
/*
func main() {
	// ... initialize app ...
	
	lxcClient, err := lxc.NewClient()
	if err != nil {
		log.Fatal(err)
	}
	
	r := gin.Default()
	
	// Register terminal routes
	api.RegisterTerminalRoutes(r, lxcClient)
	
	// Start server
	srv := &http.Server{
		Addr:    ":8500",
		Handler: r,
	}
	
	go func() {
		if err := srv.ListenAndServe(); err != nil {
			log.Fatal(err)
		}
	}()
	
	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	
	log.Println("Shutting down server...")
	
	// Shutdown HTTP server
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("Server shutdown error: %v", err)
	}
	
	// CRITICAL: Shutdown all terminal sessions
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()
	
	if err := api.ShutdownAllSessions(shutdownCtx); err != nil {
		log.Printf("Terminal shutdown error: %v", err)
	}
	
	log.Println("Server stopped")
}
*/