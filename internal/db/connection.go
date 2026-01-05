// database/connection.go
package db

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"sync"
	"sync/atomic"
	"time"

	_ "github.com/lib/pq"
)

// ============================================================================
// ERROR TAXONOMY
// ============================================================================

type DBErrorCode int

const (
	ErrCodeConnectionFailed DBErrorCode = iota + 7000
	ErrCodePingFailed
	ErrCodeMigrationFailed
	ErrCodeTransactionFailed
	ErrCodeQueryFailed
	ErrCodePoolExhausted
	ErrCodeContextCanceled
	ErrCodeInvalidConfig
)

type DBError struct {
	Code      DBErrorCode
	Message   string
	Err       error
	Query     string
	Timestamp int64
}

func (e *DBError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("[%d] %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("[%d] %s", e.Code, e.Message)
}

func NewDBError(code DBErrorCode, msg string, err error) *DBError {
	return &DBError{
		Code:      code,
		Message:   msg,
		Err:       err,
		Timestamp: time.Now().UnixNano(),
	}
}

// ============================================================================
// CONFIGURATION
// ============================================================================

type Config struct {
	Host            string
	Port            int
	User            string
	Password        string
	Database        string
	SSLMode         string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
	ConnectTimeout  time.Duration
	QueryTimeout    time.Duration
}

func DefaultConfig() *Config {
	return &Config{
		Host:            getEnv("DB_HOST", "localhost"),
		Port:            getEnvInt("DB_PORT", 5432),
		User:            getEnv("DB_USER", "axion"),
		Password:        getEnv("DB_PASSWORD", "axion_password"),
		Database:        getEnv("DB_NAME", "axion_db"),
		SSLMode:         getEnv("DB_SSLMODE", "disable"),
		MaxOpenConns:    getEnvInt("DB_MAX_OPEN_CONNS", 25),
		MaxIdleConns:    getEnvInt("DB_MAX_IDLE_CONNS", 5),
		ConnMaxLifetime: getEnvDuration("DB_CONN_MAX_LIFETIME", 5*time.Minute),
		ConnMaxIdleTime: getEnvDuration("DB_CONN_MAX_IDLE_TIME", 10*time.Minute),
		ConnectTimeout:  getEnvDuration("DB_CONNECT_TIMEOUT", 10*time.Second),
		QueryTimeout:    getEnvDuration("DB_QUERY_TIMEOUT", 30*time.Second),
	}
}

func (c *Config) Validate() error {
	if c.Host == "" {
		return NewDBError(ErrCodeInvalidConfig, "host is required", nil)
	}
	if c.User == "" {
		return NewDBError(ErrCodeInvalidConfig, "user is required", nil)
	}
	if c.Database == "" {
		return NewDBError(ErrCodeInvalidConfig, "database is required", nil)
	}
	if c.MaxOpenConns < c.MaxIdleConns {
		return NewDBError(ErrCodeInvalidConfig, "max_open_conns must be >= max_idle_conns", nil)
	}
	return nil
}

func (c *Config) DSN() string {
	return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s connect_timeout=%d",
		c.Host, c.Port, c.User, c.Password, c.Database, c.SSLMode, int(c.ConnectTimeout.Seconds()))
}

// ============================================================================
// METRICS
// ============================================================================

type DBMetrics struct {
	queriesTotal       atomic.Uint64
	queriesSuccess     atomic.Uint64
	queriesFailed      atomic.Uint64
	transactionsTotal  atomic.Uint64
	transactionsFailed atomic.Uint64
	slowQueries        atomic.Uint64
	contextCanceled    atomic.Uint64
	totalQueryTime     atomic.Int64 // nanoseconds
}

func (m *DBMetrics) RecordQuery(duration time.Duration, success bool) {
	m.queriesTotal.Add(1)
	m.totalQueryTime.Add(int64(duration))
	
	if success {
		m.queriesSuccess.Add(1)
	} else {
		m.queriesFailed.Add(1)
	}
	
	if duration > 1*time.Second {
		m.slowQueries.Add(1)
	}
}

func (m *DBMetrics) RecordTransaction(success bool) {
	m.transactionsTotal.Add(1)
	if !success {
		m.transactionsFailed.Add(1)
	}
}

func (m *DBMetrics) RecordContextCanceled() {
	m.contextCanceled.Add(1)
}

func (m *DBMetrics) Snapshot() map[string]interface{} {
	total := m.queriesTotal.Load()
	avgQueryTime := int64(0)
	if total > 0 {
		avgQueryTime = m.totalQueryTime.Load() / int64(total)
	}

	return map[string]interface{}{
		"queries_total":        total,
		"queries_success":      m.queriesSuccess.Load(),
		"queries_failed":       m.queriesFailed.Load(),
		"transactions_total":   m.transactionsTotal.Load(),
		"transactions_failed":  m.transactionsFailed.Load(),
		"slow_queries":         m.slowQueries.Load(),
		"context_canceled":     m.contextCanceled.Load(),
		"avg_query_time_ns":    avgQueryTime,
		"avg_query_time_ms":    float64(avgQueryTime) / 1e6,
	}
}

// ============================================================================
// DATABASE SERVICE
// ============================================================================

type Service struct {
	db      *sql.DB
	config  *Config
	metrics *DBMetrics
	
	state   atomic.Uint32
	mu      sync.RWMutex
}

const (
	stateDisconnected uint32 = 0
	stateConnected    uint32 = 1
	stateClosing      uint32 = 2
	stateClosed       uint32 = 3
)

var (
	globalService *Service
	globalOnce    sync.Once
)

func InitService(cfg *Config) (*Service, error) {
	var initErr error
	
	globalOnce.Do(func() {
		if cfg == nil {
			cfg = DefaultConfig()
		}

		if err := cfg.Validate(); err != nil {
			initErr = err
			return
		}

		globalService, initErr = connect(cfg)
		if initErr != nil {
			return
		}

		log.Printf("[DB] Connected: host=%s database=%s", cfg.Host, cfg.Database)
	})

	if initErr != nil {
		return nil, initErr
	}

	return globalService, nil
}

func GetService() *Service {
	if globalService == nil {
		log.Println("[DB] WARNING: GetService called before InitService()")
		db, err := InitService(nil)
		if err != nil {
			log.Fatalf("[DB] FATAL: Failed to initialize: %v", err)
		}
		return db
	}
	return globalService
}

func connect(cfg *Config) (*Service, error) {
	db, err := sql.Open("postgres", cfg.DSN())
	if err != nil {
		return nil, NewDBError(ErrCodeConnectionFailed, "sql.Open failed", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	db.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), cfg.ConnectTimeout)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, NewDBError(ErrCodePingFailed, "ping failed", err)
	}

	service := &Service{
		db:      db,
		config:  cfg,
		metrics: &DBMetrics{},
	}
	service.state.Store(stateConnected)

	return service, nil
}

func (s *Service) GetRawDB() *sql.DB {
	return s.db
}

func (s *Service) Ping(ctx context.Context) error {
	if s.state.Load() != stateConnected {
		return NewDBError(ErrCodeConnectionFailed, "not connected", nil)
	}

	if err := s.db.PingContext(ctx); err != nil {
		return NewDBError(ErrCodePingFailed, "ping failed", err)
	}

	return nil
}

func (s *Service) Stats() sql.DBStats {
	return s.db.Stats()
}

func (s *Service) Metrics() map[string]interface{} {
	metrics := s.metrics.Snapshot()
	
	// Add connection pool stats
	stats := s.db.Stats()
	metrics["pool_open_connections"] = stats.OpenConnections
	metrics["pool_in_use"] = stats.InUse
	metrics["pool_idle"] = stats.Idle
	metrics["pool_wait_count"] = stats.WaitCount
	metrics["pool_wait_duration_ms"] = float64(stats.WaitDuration.Milliseconds())
	metrics["pool_max_idle_closed"] = stats.MaxIdleClosed
	metrics["pool_max_lifetime_closed"] = stats.MaxLifetimeClosed
	
	return metrics
}

func (s *Service) Close() error {
	if !s.state.CompareAndSwap(stateConnected, stateClosing) {
		return nil // Already closing or closed
	}

	log.Println("[DB] Closing connection...")

	if err := s.db.Close(); err != nil {
		s.state.Store(stateClosed)
		return NewDBError(ErrCodeConnectionFailed, "close failed", err)
	}

	s.state.Store(stateClosed)
	log.Println("[DB] Connection closed")
	return nil
}

// ============================================================================
// QUERY HELPERS
// ============================================================================

func (s *Service) QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	start := time.Now()
	
	rows, err := s.db.QueryContext(ctx, query, args...)
	
	duration := time.Since(start)
	s.metrics.RecordQuery(duration, err == nil)
	
	if err != nil {
		if ctx.Err() == context.Canceled {
			s.metrics.RecordContextCanceled()
			return nil, NewDBError(ErrCodeContextCanceled, "query canceled", err)
		}
		
		dbErr := NewDBError(ErrCodeQueryFailed, "query failed", err)
		dbErr.Query = query
		return nil, dbErr
	}
	
	if duration > 1*time.Second {
		log.Printf("[DB] SLOW QUERY (%v): %s", duration, query)
	}
	
	return rows, nil
}

func (s *Service) QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	start := time.Now()
	
	row := s.db.QueryRowContext(ctx, query, args...)
	
	duration := time.Since(start)
	s.metrics.RecordQuery(duration, true) // Row errors are deferred
	
	if duration > 1*time.Second {
		log.Printf("[DB] SLOW QUERY (%v): %s", duration, query)
	}
	
	return row
}

func (s *Service) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	start := time.Now()
	
	result, err := s.db.ExecContext(ctx, query, args...)
	
	duration := time.Since(start)
	s.metrics.RecordQuery(duration, err == nil)
	
	if err != nil {
		if ctx.Err() == context.Canceled {
			s.metrics.RecordContextCanceled()
			return nil, NewDBError(ErrCodeContextCanceled, "exec canceled", err)
		}
		
		dbErr := NewDBError(ErrCodeQueryFailed, "exec failed", err)
		dbErr.Query = query
		return nil, dbErr
	}
	
	if duration > 1*time.Second {
		log.Printf("[DB] SLOW EXEC (%v): %s", duration, query)
	}
	
	return result, nil
}

// ============================================================================
// TRANSACTION SUPPORT
// ============================================================================

type Tx struct {
	tx      *sql.Tx
	service *Service
	ctx     context.Context
	started time.Time
}

func (s *Service) BeginTx(ctx context.Context, opts *sql.TxOptions) (*Tx, error) {
	tx, err := s.db.BeginTx(ctx, opts)
	if err != nil {
		s.metrics.RecordTransaction(false)
		return nil, NewDBError(ErrCodeTransactionFailed, "begin transaction failed", err)
	}

	return &Tx{
		tx:      tx,
		service: s,
		ctx:     ctx,
		started: time.Now(),
	}, nil
}

func (t *Tx) Commit() error {
	err := t.tx.Commit()
	duration := time.Since(t.started)
	
	t.service.metrics.RecordTransaction(err == nil)
	
	if err != nil {
		return NewDBError(ErrCodeTransactionFailed, "commit failed", err)
	}
	
	if duration > 5*time.Second {
		log.Printf("[DB] SLOW TRANSACTION (%v)", duration)
	}
	
	return nil
}

func (t *Tx) Rollback() error {
	err := t.tx.Rollback()
	t.service.metrics.RecordTransaction(false)
	
	if err != nil && err != sql.ErrTxDone {
		return NewDBError(ErrCodeTransactionFailed, "rollback failed", err)
	}
	
	return nil
}

func (t *Tx) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	start := time.Now()
	
	result, err := t.tx.ExecContext(ctx, query, args...)
	
	duration := time.Since(start)
	t.service.metrics.RecordQuery(duration, err == nil)
	
	if err != nil {
		dbErr := NewDBError(ErrCodeQueryFailed, "tx exec failed", err)
		dbErr.Query = query
		return nil, dbErr
	}
	
	return result, nil
}

func (t *Tx) QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	start := time.Now()
	
	rows, err := t.tx.QueryContext(ctx, query, args...)
	
	duration := time.Since(start)
	t.service.metrics.RecordQuery(duration, err == nil)
	
	if err != nil {
		dbErr := NewDBError(ErrCodeQueryFailed, "tx query failed", err)
		dbErr.Query = query
		return nil, dbErr
	}
	
	return rows, nil
}

func (t *Tx) QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	start := time.Now()
	
	row := t.tx.QueryRowContext(ctx, query, args...)
	
	duration := time.Since(start)
	t.service.metrics.RecordQuery(duration, true)
	
	return row
}

// ============================================================================
// UTILITY FUNCTIONS
// ============================================================================

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := fmt.Sscanf(value, "%d", new(int)); err == nil && intVal == 1 {
			var result int
			fmt.Sscanf(value, "%d", &result)
			return result
		}
	}
	return fallback
}

func getEnvDuration(key string, fallback time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return fallback
}

// ============================================================================
// HEALTH CHECK
// ============================================================================

func (s *Service) HealthCheck(ctx context.Context) error {
	if err := s.Ping(ctx); err != nil {
		return err
	}

	// Check pool stats
	stats := s.db.Stats()
	if stats.OpenConnections >= s.config.MaxOpenConns {
		return NewDBError(ErrCodePoolExhausted, 
			fmt.Sprintf("connection pool exhausted: %d/%d", stats.OpenConnections, s.config.MaxOpenConns), 
			nil)
	}

	return nil
}

// ============================================================================
// PREPARED STATEMENT CACHE
// ============================================================================

type PreparedStmtCache struct {
	stmts map[string]*sql.Stmt
	mu    sync.RWMutex
	db    *sql.DB
}

func NewPreparedStmtCache(db *sql.DB) *PreparedStmtCache {
	return &PreparedStmtCache{
		stmts: make(map[string]*sql.Stmt),
		db:    db,
	}
}

func (c *PreparedStmtCache) Get(ctx context.Context, query string) (*sql.Stmt, error) {
	c.mu.RLock()
	stmt, exists := c.stmts[query]
	c.mu.RUnlock()

	if exists {
		return stmt, nil
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// Double-check after acquiring write lock
	if stmt, exists := c.stmts[query]; exists {
		return stmt, nil
	}

	// Prepare new statement
	stmt, err := c.db.PrepareContext(ctx, query)
	if err != nil {
		return nil, NewDBError(ErrCodeQueryFailed, "prepare statement failed", err)
	}

	c.stmts[query] = stmt
	return stmt, nil
}

func (c *PreparedStmtCache) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	var errs []error
	for _, stmt := range c.stmts {
		if err := stmt.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("failed to close %d statements", len(errs))
	}

	return nil
}