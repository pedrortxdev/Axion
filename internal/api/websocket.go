package api

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"aexon/internal/events"
	"aexon/internal/monitor"
	"aexon/internal/provider/lxc"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// ============================================================================
// ERROR TAXONOMY
// ============================================================================

type TelemetryErrorCode int

const (
	ErrCodeTelemetryUpgradeFailed TelemetryErrorCode = iota + 5000
	ErrCodeTelemetryWriteFailed
	ErrCodeTelemetryMetricsFailed
	ErrCodeTelemetryConnectionClosed
	ErrCodeTelemetryBufferOverflow
	ErrCodeTelemetryTimeout
)

type TelemetryError struct {
	Code      TelemetryErrorCode
	Message   string
	Err       error
	Timestamp int64
}

func (e *TelemetryError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("[%d] %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("[%d] %s", e.Code, e.Message)
}

func NewTelemetryError(code TelemetryErrorCode, msg string, err error) *TelemetryError {
	return &TelemetryError{
		Code:      code,
		Message:   msg,
		Err:       err,
		Timestamp: time.Now().UnixNano(),
	}
}

// ============================================================================
// CONFIGURATION
// ============================================================================

var telemetryUpgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 8192,
	CheckOrigin: func(r *http.Request) bool {
		// In production, validate origin properly
		// return r.Header.Get("Origin") == "https://yourdomain.com"
		return true
	},
	HandshakeTimeout: 10 * time.Second,
}

const (
	telemetryMetricsInterval     = 1 * time.Second
	telemetryWriteTimeout        = 5 * time.Second
	telemetryPingInterval        = 30 * time.Second
	telemetryPongTimeout         = 60 * time.Second
	telemetryChannelBufferSize   = 256
	telemetryMaxReconnectAttempts = 3
	telemetryShutdownTimeout     = 10 * time.Second
)

// ============================================================================
// METRICS
// ============================================================================

type TelemetryMetrics struct {
	connectionsTotal      atomic.Uint64
	connectionsActive     atomic.Int64
	messagesSent          atomic.Uint64
	messagesDropped       atomic.Uint64
	metricsCollected      atomic.Uint64
	hostStatsCollected    atomic.Uint64
	eventsDispatched      atomic.Uint64
	errors                atomic.Uint64
	upgradeFailures       atomic.Uint64
	writeFailures         atomic.Uint64
	bufferOverflows       atomic.Uint64
}

var globalTelemetryMetrics = &TelemetryMetrics{}

func (m *TelemetryMetrics) Snapshot() map[string]interface{} {
	return map[string]interface{}{
		"connections_total":    m.connectionsTotal.Load(),
		"connections_active":   m.connectionsActive.Load(),
		"messages_sent":        m.messagesSent.Load(),
		"messages_dropped":     m.messagesDropped.Load(),
		"metrics_collected":    m.metricsCollected.Load(),
		"host_stats_collected": m.hostStatsCollected.Load(),
		"events_dispatched":    m.eventsDispatched.Load(),
		"errors":               m.errors.Load(),
		"upgrade_failures":     m.upgradeFailures.Load(),
		"write_failures":       m.writeFailures.Load(),
		"buffer_overflows":     m.bufferOverflows.Load(),
	}
}

// ============================================================================
// MESSAGE TYPES
// ============================================================================

type MessageType string

const (
	MessageTypeInstanceMetrics MessageType = "instance_metrics"
	MessageTypeHostTelemetry   MessageType = "host_telemetry"
	MessageTypeEvent           MessageType = "event"
	MessageTypePing            MessageType = "ping"
	MessageTypePong            MessageType = "pong"
	MessageTypeError           MessageType = "error"
)

type TelemetryMessage struct {
	Type      MessageType `json:"type"`
	Data      interface{} `json:"data"`
	Timestamp int64       `json:"timestamp"`
}

func NewMessage(msgType MessageType, data interface{}) *TelemetryMessage {
	return &TelemetryMessage{
		Type:      msgType,
		Data:      data,
		Timestamp: time.Now().UnixNano(),
	}
}

// ============================================================================
// TELEMETRY CLIENT
// ============================================================================

type TelemetryClient struct {
	id              string
	conn            *websocket.Conn
	sendChan        chan *TelemetryMessage
	instanceService *lxc.InstanceService
	metricProcessor func([]lxc.InstanceMetric)
	
	ctx             context.Context
	cancel          context.CancelFunc
	
	state           atomic.Uint32
	wg              sync.WaitGroup
	closeOnce       sync.Once
	
	lastPong        atomic.Int64
}

const (
	clientStateCreated uint32 = 0
	clientStateRunning uint32 = 1
	clientStateClosing uint32 = 2
	clientStateClosed  uint32 = 3
)

func NewTelemetryClient(
	id string,
	conn *websocket.Conn,
	instanceService *lxc.InstanceService,
	metricProcessor func([]lxc.InstanceMetric),
) *TelemetryClient {
	ctx, cancel := context.WithCancel(context.Background())
	
	client := &TelemetryClient{
		id:              id,
		conn:            conn,
		sendChan:        make(chan *TelemetryMessage, telemetryChannelBufferSize),
		instanceService: instanceService,
		metricProcessor: metricProcessor,
		ctx:             ctx,
		cancel:          cancel,
	}
	
	client.state.Store(clientStateCreated)
	client.lastPong.Store(time.Now().UnixNano())
	
	return client
}

func (c *TelemetryClient) Start() error {
	if !c.state.CompareAndSwap(clientStateCreated, clientStateRunning) {
		return errors.New("client already started")
	}

	globalTelemetryMetrics.connectionsTotal.Add(1)
	globalTelemetryMetrics.connectionsActive.Add(1)

	log.Printf("[Telemetry] Client %s connected", c.id)

	// Register with broadcaster
	registerTelemetryClient(c)

	// Start goroutines
	c.wg.Add(3)
	go c.metricsPoller()
	go c.writeLoop()
	go c.pingLoop()

	// Setup pong handler
	c.conn.SetPongHandler(func(string) error {
		c.lastPong.Store(time.Now().UnixNano())
		return nil
	})

	return nil
}

func (c *TelemetryClient) metricsPoller() {
	defer c.wg.Done()
	defer log.Printf("[Telemetry] Client %s metrics poller stopped", c.id)

	ticker := time.NewTicker(telemetryMetricsInterval)
	defer ticker.Stop()

	for {
		select {
		case <-c.ctx.Done():
			return
			
		case <-ticker.C:
			// Collect instance metrics
			if err := c.collectInstanceMetrics(); err != nil {
				log.Printf("[Telemetry] Client %s: metrics collection error: %v", c.id, err)
				globalTelemetryMetrics.errors.Add(1)
			}

			// Collect host stats
			if err := c.collectHostStats(); err != nil {
				log.Printf("[Telemetry] Client %s: host stats error: %v", c.id, err)
				globalTelemetryMetrics.errors.Add(1)
			}
		}
	}
}

func (c *TelemetryClient) collectInstanceMetrics() error {
	metrics, err := c.instanceService.ListInstances()
	if err != nil {
		return fmt.Errorf("ListInstances failed: %w", err)
	}

	// Process metrics if handler provided
	if c.metricProcessor != nil {
		c.metricProcessor(metrics)
	}

	// Send to client
	msg := NewMessage(MessageTypeInstanceMetrics, metrics)
	
	select {
	case c.sendChan <- msg:
		globalTelemetryMetrics.metricsCollected.Add(1)
		return nil
	case <-c.ctx.Done():
		return c.ctx.Err()
	default:
		globalTelemetryMetrics.bufferOverflows.Add(1)
		globalTelemetryMetrics.messagesDropped.Add(1)
		return fmt.Errorf("buffer full, metrics dropped")
	}
}

func (c *TelemetryClient) collectHostStats() error {
	hostStats, err := monitor.GetHostStats()
	if err != nil {
		return fmt.Errorf("GetHostStats failed: %w", err)
	}

	msg := NewMessage(MessageTypeHostTelemetry, hostStats)
	
	select {
	case c.sendChan <- msg:
		globalTelemetryMetrics.hostStatsCollected.Add(1)
		return nil
	case <-c.ctx.Done():
		return c.ctx.Err()
	default:
		globalTelemetryMetrics.bufferOverflows.Add(1)
		globalTelemetryMetrics.messagesDropped.Add(1)
		return fmt.Errorf("buffer full, host stats dropped")
	}
}

func (c *TelemetryClient) writeLoop() {
	defer c.wg.Done()
	defer log.Printf("[Telemetry] Client %s write loop stopped", c.id)
	defer c.Close()

	for {
		select {
		case msg, ok := <-c.sendChan:
			if !ok {
				return
			}

			c.conn.SetWriteDeadline(time.Now().Add(telemetryWriteTimeout))
			if err := c.conn.WriteJSON(msg); err != nil {
				if !websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
					log.Printf("[Telemetry] Client %s write error: %v", c.id, err)
					globalTelemetryMetrics.writeFailures.Add(1)
				}
				return
			}

			globalTelemetryMetrics.messagesSent.Add(1)

		case <-c.ctx.Done():
			return
		}
	}
}

func (c *TelemetryClient) pingLoop() {
	defer c.wg.Done()

	pingTicker := time.NewTicker(telemetryPingInterval)
	defer pingTicker.Stop()

	checkTicker := time.NewTicker(5 * time.Second)
	defer checkTicker.Stop()

	for {
		select {
		case <-pingTicker.C:
			c.conn.SetWriteDeadline(time.Now().Add(telemetryWriteTimeout))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				log.Printf("[Telemetry] Client %s ping failed: %v", c.id, err)
				c.Close()
				return
			}

		case <-checkTicker.C:
			lastPong := time.Unix(0, c.lastPong.Load())
			if time.Since(lastPong) > telemetryPongTimeout {
				log.Printf("[Telemetry] Client %s pong timeout (no response for %v)", c.id, time.Since(lastPong))
				c.Close()
				return
			}

		case <-c.ctx.Done():
			return
		}
	}
}

func (c *TelemetryClient) SendEvent(event interface{}) {
	if c.state.Load() != clientStateRunning {
		return
	}

	msg := NewMessage(MessageTypeEvent, event)
	
	select {
	case c.sendChan <- msg:
		globalTelemetryMetrics.eventsDispatched.Add(1)
	case <-c.ctx.Done():
	default:
		globalTelemetryMetrics.messagesDropped.Add(1)
		globalTelemetryMetrics.bufferOverflows.Add(1)
	}
}

func (c *TelemetryClient) Close() {
	c.closeOnce.Do(func() {
		if !c.state.CompareAndSwap(clientStateRunning, clientStateClosing) {
			return
		}

		log.Printf("[Telemetry] Client %s initiating shutdown", c.id)

		// Cancel context to stop all goroutines
		c.cancel()

		// Unregister from broadcaster
		unregisterTelemetryClient(c)

		// Close send channel
		close(c.sendChan)

		// Wait for goroutines with timeout
		done := make(chan struct{})
		go func() {
			c.wg.Wait()
			close(done)
		}()

		select {
		case <-done:
			log.Printf("[Telemetry] Client %s clean shutdown", c.id)
		case <-time.After(telemetryShutdownTimeout):
			log.Printf("[Telemetry] Client %s shutdown timeout", c.id)
		}

		// Close WebSocket connection
		c.conn.WriteMessage(
			websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""),
		)
		c.conn.Close()

		c.state.Store(clientStateClosed)
		globalTelemetryMetrics.connectionsActive.Add(-1)
	})
}

// ============================================================================
// CLIENT REGISTRY
// ============================================================================

var (
	telemetryClients = make(map[string]*TelemetryClient)
	telemetryMutex   sync.RWMutex
)

func registerTelemetryClient(client *TelemetryClient) {
	telemetryMutex.Lock()
	defer telemetryMutex.Unlock()
	telemetryClients[client.id] = client
	log.Printf("[Telemetry] Registered client: %s (total: %d)", client.id, len(telemetryClients))
}

func unregisterTelemetryClient(client *TelemetryClient) {
	telemetryMutex.Lock()
	defer telemetryMutex.Unlock()
	delete(telemetryClients, client.id)
	log.Printf("[Telemetry] Unregistered client: %s (remaining: %d)", client.id, len(telemetryClients))
}

func GetActiveTelemetryCount() int {
	telemetryMutex.RLock()
	defer telemetryMutex.RUnlock()
	return len(telemetryClients)
}

func GetActiveTelemetryIDs() []string {
	telemetryMutex.RLock()
	defer telemetryMutex.RUnlock()
	
	ids := make([]string, 0, len(telemetryClients))
	for id := range telemetryClients {
		ids = append(ids, id)
	}
	return ids
}

// ============================================================================
// EVENT BROADCASTER
// ============================================================================

var (
	broadcasterRunning atomic.Bool
	broadcasterWg      sync.WaitGroup
	broadcasterCtx     context.Context
	broadcasterCancel  context.CancelFunc
)

func InitBroadcaster() {
	if !broadcasterRunning.CompareAndSwap(false, true) {
		log.Println("[Broadcaster] Already running")
		return
	}

	broadcasterCtx, broadcasterCancel = context.WithCancel(context.Background())

	broadcasterWg.Add(1)
	go func() {
		defer broadcasterWg.Done()
		defer log.Println("[Broadcaster] Stopped")

		log.Println("[Broadcaster] Started")

		for {
			select {
			case evt, ok := <-events.GlobalBus:
				if !ok {
					log.Println("[Broadcaster] GlobalBus closed")
					return
				}

				broadcastEvent(evt)

			case <-broadcasterCtx.Done():
				return
			}
		}
	}()
}

func broadcastEvent(event interface{}) {
	telemetryMutex.RLock()
	clients := make([]*TelemetryClient, 0, len(telemetryClients))
	for _, client := range telemetryClients {
		clients = append(clients, client)
	}
	telemetryMutex.RUnlock()

	for _, client := range clients {
		client.SendEvent(event)
	}
}

func ShutdownBroadcaster(ctx context.Context) error {
	if !broadcasterRunning.CompareAndSwap(true, false) {
		return nil
	}

	log.Println("[Broadcaster] Initiating shutdown")

	if broadcasterCancel != nil {
		broadcasterCancel()
	}

	done := make(chan struct{})
	go func() {
		broadcasterWg.Wait()
		close(done)
	}()

	select {
	case <-done:
		log.Println("[Broadcaster] Shutdown completed")
		return nil
	case <-ctx.Done():
		log.Println("[Broadcaster] Shutdown timeout")
		return fmt.Errorf("broadcaster shutdown timeout")
	}
}

func ShutdownAllTelemetryClients(ctx context.Context) error {
	telemetryMutex.RLock()
	clients := make([]*TelemetryClient, 0, len(telemetryClients))
	clientIDs := make([]string, 0, len(telemetryClients))
	for id, client := range telemetryClients {
		clients = append(clients, client)
		clientIDs = append(clientIDs, id)
	}
	telemetryMutex.RUnlock()

	if len(clients) == 0 {
		log.Println("[Telemetry] No active clients to shutdown")
		return nil
	}

	log.Printf("[Telemetry] Shutting down %d clients: %v", len(clients), clientIDs)

	for i, client := range clients {
		log.Printf("[Telemetry] Closing client %d/%d: %s", i+1, len(clients), clientIDs[i])
		client.Close()
	}

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	startTime := time.Now()

	for {
		select {
		case <-ctx.Done():
			remaining := globalTelemetryMetrics.connectionsActive.Load()
			if remaining > 0 {
				return fmt.Errorf("timeout: %d clients remaining", remaining)
			}
			return nil

		case <-ticker.C:
			active := globalTelemetryMetrics.connectionsActive.Load()
			if active == 0 {
				log.Printf("[Telemetry] All clients closed in %v", time.Since(startTime))
				return nil
			}

			if time.Since(startTime).Truncate(time.Second) == time.Since(startTime) {
				log.Printf("[Telemetry] Waiting: %d clients remaining...", active)
			}
		}
	}
}

// ============================================================================
// HTTP HANDLER
// ============================================================================

func StreamTelemetry(
	c *gin.Context,
	instanceService *lxc.InstanceService,
	metricProcessor func([]lxc.InstanceMetric),
) {
	// Upgrade to WebSocket
	conn, err := telemetryUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		globalTelemetryMetrics.upgradeFailures.Add(1)
		log.Printf("[Telemetry] Upgrade failed: %v", err)
		return
	}

	// Generate unique client ID
	clientID := fmt.Sprintf("telemetry-%d", time.Now().UnixNano())
	log.Printf("[Telemetry] New client: %s", clientID)

	// Create client
	client := NewTelemetryClient(clientID, conn, instanceService, metricProcessor)

	if err := client.Start(); err != nil {
		log.Printf("[Telemetry] Client %s start failed: %v", clientID, err)
		client.Close()
		return
	}

	// Block until client closes
	<-client.ctx.Done()
	log.Printf("[Telemetry] Client %s disconnected", clientID)
}

// ============================================================================
// METRICS & ADMIN ENDPOINTS
// ============================================================================

func GetTelemetryMetrics(c *gin.Context) {
	metrics := globalTelemetryMetrics.Snapshot()
	metrics["active_client_count"] = GetActiveTelemetryCount()
	metrics["active_client_ids"] = GetActiveTelemetryIDs()
	
	c.JSON(200, metrics)
}

func ListTelemetryClientsHandler(c *gin.Context) {
	telemetryMutex.RLock()
	defer telemetryMutex.RUnlock()
	
	type ClientInfo struct {
		ID       string `json:"id"`
		State    uint32 `json:"state"`
		StateStr string `json:"state_str"`
		LastPong string `json:"last_pong"`
	}
	
	clients := make([]ClientInfo, 0, len(telemetryClients))
	for id, client := range telemetryClients {
		state := client.state.Load()
		stateStr := "unknown"
		switch state {
		case clientStateCreated:
			stateStr = "created"
		case clientStateRunning:
			stateStr = "running"
		case clientStateClosing:
			stateStr = "closing"
		case clientStateClosed:
			stateStr = "closed"
		}
		
		lastPong := time.Unix(0, client.lastPong.Load())
		
		clients = append(clients, ClientInfo{
			ID:       id,
			State:    state,
			StateStr: stateStr,
			LastPong: lastPong.Format(time.RFC3339),
		})
	}
	
	c.JSON(200, gin.H{
		"count":   len(clients),
		"clients": clients,
	})
}

func CloseTelemetryClientHandler(c *gin.Context) {
	clientID := c.Param("id")
	
	telemetryMutex.RLock()
	client, exists := telemetryClients[clientID]
	telemetryMutex.RUnlock()
	
	if !exists {
		c.JSON(404, gin.H{"error": "client not found"})
		return
	}
	
	log.Printf("[Admin] Force closing telemetry client: %s", clientID)
	client.Close()
	
	c.JSON(200, gin.H{
		"status":    "closed",
		"client_id": clientID,
	})
}

func ShutdownAllTelemetryHandler(c *gin.Context) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	if err := ShutdownAllTelemetryClients(ctx); err != nil {
		c.JSON(500, gin.H{
			"error":   "shutdown failed",
			"details": err.Error(),
		})
		return
	}
	
	c.JSON(200, gin.H{
		"status":  "success",
		"message": "all telemetry clients closed",
	})
}

// ============================================================================
// ROUTER REGISTRATION
// ============================================================================

func RegisterTelemetryRoutes(r *gin.Engine, instanceService *lxc.InstanceService) {
	// WebSocket endpoint
	r.GET("/ws/telemetry", func(c *gin.Context) {
		StreamTelemetry(c, instanceService, func(metrics []lxc.InstanceMetric) {
			// Process metrics (e.g., save to database)
			// This is called in the metrics poller goroutine
		})
	})
	
	// Metrics endpoint
	r.GET("/telemetry/metrics", GetTelemetryMetrics)
	
	// Admin endpoints
	admin := r.Group("/telemetry/admin")
	{
		admin.GET("/clients", ListTelemetryClientsHandler)
		admin.DELETE("/clients/:id", CloseTelemetryClientHandler)
		admin.POST("/shutdown", ShutdownAllTelemetryHandler)
	}
}
