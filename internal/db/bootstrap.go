// database/bootstrap.go
package db

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	_ "github.com/lib/pq"
)

// ============================================================================
// BOOTSTRAP CONFIGURATION
// ============================================================================

type BootstrapConfig struct {
	SuperUser     string
	SuperPassword string
	DatabaseName  string
	AppUser       string
	AppPassword   string
	Timeout       time.Duration
	MaxRetries    int
	RetryDelay    time.Duration
}

func DefaultBootstrapConfig() *BootstrapConfig {
	return &BootstrapConfig{
		SuperUser:     getEnv("BOOTSTRAP_SUPER_USER", "postgres"),
		SuperPassword: os.Getenv("PGPASSWORD"),
		DatabaseName:  getEnv("DB_NAME", "axion_db"),
		AppUser:       getEnv("DB_USER", "axion"),
		AppPassword:   getEnv("DB_PASSWORD", "axion_password"),
		Timeout:       30 * time.Second,
		MaxRetries:    3,
		RetryDelay:    2 * time.Second,
	}
}

// ============================================================================
// BOOTSTRAP EXECUTION
// ============================================================================

var (
	bootstrapOnce sync.Once
	bootstrapErr  error
)

func EnsureDBSetup() error {
	bootstrapOnce.Do(func() {
		bootstrapErr = runBootstrap(DefaultBootstrapConfig())
	})
	return bootstrapErr
}

func EnsureDBSetupWithConfig(cfg *BootstrapConfig) error {
	return runBootstrap(cfg)
}

func runBootstrap(cfg *BootstrapConfig) error {
	log.Println("[Bootstrap] Starting database bootstrap process...")

	ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
	defer cancel()

	// Try to connect as superuser
	db, err := connectAsSuperUser(ctx, cfg)
	if err != nil {
		return fmt.Errorf("failed to connect as superuser: %w", err)
	}
	defer db.Close()

	// Ensure user exists
	if err := ensureUser(ctx, db, cfg); err != nil {
		return fmt.Errorf("failed to ensure user: %w", err)
	}

	// Ensure database exists
	if err := ensureDatabase(ctx, db, cfg); err != nil {
		return fmt.Errorf("failed to ensure database: %w", err)
	}

	// Grant privileges
	if err := grantPrivileges(ctx, db, cfg); err != nil {
		return fmt.Errorf("failed to grant privileges: %w", err)
	}

	// Verify connection as app user
	if err := verifyAppUserConnection(ctx, cfg); err != nil {
		return fmt.Errorf("failed to verify app user connection: %w", err)
	}

	log.Println("[Bootstrap] Database bootstrap completed successfully")
	return nil
}

// ============================================================================
// CONNECTION STRATEGIES
// ============================================================================

func connectAsSuperUser(ctx context.Context, cfg *BootstrapConfig) (*sql.DB, error) {
	strategies := buildConnectionStrategies(cfg)

	var lastErr error
	for i, connStr := range strategies {
		log.Printf("[Bootstrap] Attempting connection strategy %d/%d...", i+1, len(strategies))

		db, err := sql.Open("postgres", connStr)
		if err != nil {
			lastErr = err
			continue
		}

		// Test connection
		pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		err = db.PingContext(pingCtx)
		cancel()

		if err == nil {
			log.Printf("[Bootstrap] Successfully connected using strategy %d", i+1)
			return db, nil
		}

		lastErr = err
		db.Close()
	}

	return nil, fmt.Errorf("all connection strategies failed. Last error: %w", lastErr)
}

func buildConnectionStrategies(cfg *BootstrapConfig) []string {
	strategies := []string{
		// Strategy 1: Unix socket without password
		fmt.Sprintf("user=%s host=/var/run/postgresql sslmode=disable", cfg.SuperUser),

		// Strategy 2: Localhost without password (peer auth)
		fmt.Sprintf("user=%s host=localhost sslmode=disable", cfg.SuperUser),

		// Strategy 3: TCP with password from env
		fmt.Sprintf("user=%s host=localhost sslmode=disable password=%s",
			cfg.SuperUser, cfg.SuperPassword),

		// Strategy 4: TCP to 127.0.0.1 (sometimes different from localhost)
		fmt.Sprintf("user=%s host=127.0.0.1 sslmode=disable password=%s",
			cfg.SuperUser, cfg.SuperPassword),
	}

	// Filter out strategies with empty passwords where password is required
	var validStrategies []string
	for _, s := range strategies {
		// Skip TCP strategies if no password provided
		if cfg.SuperPassword == "" && (containsString(s, "localhost") || containsString(s, "127.0.0.1")) {
			continue
		}
		validStrategies = append(validStrategies, s)
	}

	return validStrategies
}

// ============================================================================
// USER MANAGEMENT
// ============================================================================

func ensureUser(ctx context.Context, db *sql.DB, cfg *BootstrapConfig) error {
	log.Printf("[Bootstrap] Checking if user '%s' exists...", cfg.AppUser)

	var exists bool
	query := "SELECT EXISTS(SELECT 1 FROM pg_roles WHERE rolname = $1)"

	err := db.QueryRowContext(ctx, query, cfg.AppUser).Scan(&exists)
	if err != nil {
		return fmt.Errorf("failed to check user existence: %w", err)
	}

	if exists {
		log.Printf("[Bootstrap] User '%s' already exists", cfg.AppUser)
		
		// Update password in case it changed
		if err := updateUserPassword(ctx, db, cfg); err != nil {
			log.Printf("[Bootstrap] Warning: failed to update user password: %v", err)
		}
		
		return nil
	}

	log.Printf("[Bootstrap] Creating user '%s'...", cfg.AppUser)

	// Create user with limited privileges (not SUPERUSER)
	createUserSQL := fmt.Sprintf(
		"CREATE USER %s WITH PASSWORD '%s' CREATEDB",
		cfg.AppUser,
		cfg.AppPassword,
	)

	if _, err := db.ExecContext(ctx, createUserSQL); err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}

	log.Printf("[Bootstrap] User '%s' created successfully", cfg.AppUser)
	return nil
}

func updateUserPassword(ctx context.Context, db *sql.DB, cfg *BootstrapConfig) error {
	query := fmt.Sprintf("ALTER USER %s WITH PASSWORD '%s'", cfg.AppUser, cfg.AppPassword)
	_, err := db.ExecContext(ctx, query)
	return err
}

// ============================================================================
// DATABASE MANAGEMENT
// ============================================================================

func ensureDatabase(ctx context.Context, db *sql.DB, cfg *BootstrapConfig) error {
	log.Printf("[Bootstrap] Checking if database '%s' exists...", cfg.DatabaseName)

	var exists bool
	query := "SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname = $1)"

	err := db.QueryRowContext(ctx, query, cfg.DatabaseName).Scan(&exists)
	if err != nil {
		return fmt.Errorf("failed to check database existence: %w", err)
	}

	if exists {
		log.Printf("[Bootstrap] Database '%s' already exists", cfg.DatabaseName)
		return nil
	}

	log.Printf("[Bootstrap] Creating database '%s'...", cfg.DatabaseName)

	createDBSQL := fmt.Sprintf(
		"CREATE DATABASE %s OWNER %s ENCODING 'UTF8' LC_COLLATE 'en_US.UTF-8' LC_CTYPE 'en_US.UTF-8'",
		cfg.DatabaseName,
		cfg.AppUser,
	)

	if _, err := db.ExecContext(ctx, createDBSQL); err != nil {
		return fmt.Errorf("failed to create database: %w", err)
	}

	log.Printf("[Bootstrap] Database '%s' created successfully", cfg.DatabaseName)
	return nil
}

// ============================================================================
// PRIVILEGE MANAGEMENT
// ============================================================================

func grantPrivileges(ctx context.Context, db *sql.DB, cfg *BootstrapConfig) error {
	log.Printf("[Bootstrap] Granting privileges to user '%s'...", cfg.AppUser)

	grants := []string{
		fmt.Sprintf("GRANT ALL PRIVILEGES ON DATABASE %s TO %s", cfg.DatabaseName, cfg.AppUser),
		fmt.Sprintf("ALTER DATABASE %s OWNER TO %s", cfg.DatabaseName, cfg.AppUser),
	}

	for _, grantSQL := range grants {
		if _, err := db.ExecContext(ctx, grantSQL); err != nil {
			// Log warning but don't fail - some privileges might already be granted
			log.Printf("[Bootstrap] Warning: %v", err)
		}
	}

	log.Printf("[Bootstrap] Privileges granted to user '%s'", cfg.AppUser)
	return nil
}

// ============================================================================
// VERIFICATION
// ============================================================================

func verifyAppUserConnection(ctx context.Context, cfg *BootstrapConfig) error {
	log.Printf("[Bootstrap] Verifying connection as user '%s'...", cfg.AppUser)

	connStr := fmt.Sprintf(
		"host=localhost port=5432 user=%s password=%s dbname=%s sslmode=disable",
		cfg.AppUser,
		cfg.AppPassword,
		cfg.DatabaseName,
	)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return fmt.Errorf("failed to open connection: %w", err)
	}
	defer db.Close()

	pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	if err := db.PingContext(pingCtx); err != nil {
		return fmt.Errorf("failed to ping as app user: %w", err)
	}

	log.Printf("[Bootstrap] Successfully verified connection as user '%s'", cfg.AppUser)
	return nil
}

// ============================================================================
// HEALTH CHECKS
// ============================================================================

func CheckPostgreSQLAvailability(ctx context.Context) error {
	cfg := DefaultBootstrapConfig()
	strategies := buildConnectionStrategies(cfg)

	var lastErr error
	for _, connStr := range strategies {
		db, err := sql.Open("postgres", connStr)
		if err != nil {
			lastErr = err
			continue
		}

		err = db.PingContext(ctx)
		db.Close()

		if err == nil {
			return nil
		}

		lastErr = err
	}

	return fmt.Errorf("PostgreSQL not available: %w", lastErr)
}

func GetPostgreSQLVersion(ctx context.Context) (string, error) {
	cfg := DefaultBootstrapConfig()
	db, err := connectAsSuperUser(ctx, cfg)
	if err != nil {
		return "", err
	}
	defer db.Close()

	var version string
	err = db.QueryRowContext(ctx, "SELECT version()").Scan(&version)
	return version, err
}

// ============================================================================
// CLEANUP & RESET
// ============================================================================

func DropDatabase(ctx context.Context, cfg *BootstrapConfig) error {
	log.Printf("[Bootstrap] WARNING: Dropping database '%s'...", cfg.DatabaseName)

	db, err := connectAsSuperUser(ctx, cfg)
	if err != nil {
		return err
	}
	defer db.Close()

	// Terminate existing connections
	terminateSQL := fmt.Sprintf(`
		SELECT pg_terminate_backend(pg_stat_activity.pid)
		FROM pg_stat_activity
		WHERE pg_stat_activity.datname = '%s'
		  AND pid <> pg_backend_pid()
	`, cfg.DatabaseName)

	if _, err := db.ExecContext(ctx, terminateSQL); err != nil {
		log.Printf("[Bootstrap] Warning: failed to terminate connections: %v", err)
	}

	// Drop database
	dropSQL := fmt.Sprintf("DROP DATABASE IF EXISTS %s", cfg.DatabaseName)
	if _, err := db.ExecContext(ctx, dropSQL); err != nil {
		return fmt.Errorf("failed to drop database: %w", err)
	}

	log.Printf("[Bootstrap] Database '%s' dropped successfully", cfg.DatabaseName)
	return nil
}

func DropUser(ctx context.Context, cfg *BootstrapConfig) error {
	log.Printf("[Bootstrap] WARNING: Dropping user '%s'...", cfg.AppUser)

	db, err := connectAsSuperUser(ctx, cfg)
	if err != nil {
		return err
	}
	defer db.Close()

	dropSQL := fmt.Sprintf("DROP USER IF EXISTS %s", cfg.AppUser)
	if _, err := db.ExecContext(ctx, dropSQL); err != nil {
		return fmt.Errorf("failed to drop user: %w", err)
	}

	log.Printf("[Bootstrap] User '%s' dropped successfully", cfg.AppUser)
	return nil
}

func ResetDatabase(ctx context.Context, cfg *BootstrapConfig) error {
	log.Println("[Bootstrap] WARNING: Resetting database (this will delete all data)...")

	if err := DropDatabase(ctx, cfg); err != nil {
		return err
	}

	if err := runBootstrap(cfg); err != nil {
		return err
	}

	log.Println("[Bootstrap] Database reset completed")
	return nil
}

// ============================================================================
// DIAGNOSTIC TOOLS
// ============================================================================

func DiagnoseConnectionIssues(ctx context.Context) {
	log.Println("[Bootstrap] Running connection diagnostics...")

	cfg := DefaultBootstrapConfig()
	strategies := buildConnectionStrategies(cfg)

	for i, connStr := range strategies {
		log.Printf("[Diagnostic] Testing strategy %d: %s", i+1, maskPassword(connStr))

		db, err := sql.Open("postgres", connStr)
		if err != nil {
			log.Printf("[Diagnostic]   ✗ Open failed: %v", err)
			continue
		}

		err = db.PingContext(ctx)
		db.Close()

		if err == nil {
			log.Printf("[Diagnostic]   ✓ Connection successful!")
		} else {
			log.Printf("[Diagnostic]   ✗ Ping failed: %v", err)
		}
	}

	// Check PostgreSQL version
	version, err := GetPostgreSQLVersion(ctx)
	if err != nil {
		log.Printf("[Diagnostic] PostgreSQL version: UNKNOWN (%v)", err)
	} else {
		log.Printf("[Diagnostic] PostgreSQL version: %s", version)
	}
}

// ============================================================================
// UTILITY FUNCTIONS
// ============================================================================

func containsString(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && 
		(s == substr || (len(s) > len(substr) && 
		(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr)))
}

func maskPassword(connStr string) string {
	// Simple password masking for logs
	if idx := findSubstring(connStr, "password="); idx >= 0 {
		start := idx + len("password=")
		end := start
		for end < len(connStr) && connStr[end] != ' ' {
			end++
		}
		if end > start {
			return connStr[:start] + "***MASKED***" + connStr[end:]
		}
	}
	return connStr
}

func findSubstring(s, substr string) int {
	if len(substr) > len(s) {
		return -1
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

// ============================================================================
// AUTO-RETRY WRAPPER
// ============================================================================

func EnsureDBSetupWithRetry() error {
	cfg := DefaultBootstrapConfig()

	var lastErr error
	for attempt := 1; attempt <= cfg.MaxRetries; attempt++ {
		log.Printf("[Bootstrap] Attempt %d/%d...", attempt, cfg.MaxRetries)

		err := runBootstrap(cfg)
		if err == nil {
			return nil
		}

		lastErr = err
		log.Printf("[Bootstrap] Attempt %d failed: %v", attempt, err)

		if attempt < cfg.MaxRetries {
			log.Printf("[Bootstrap] Retrying in %v...", cfg.RetryDelay)
			time.Sleep(cfg.RetryDelay)
		}
	}

	return fmt.Errorf("bootstrap failed after %d attempts: %w", cfg.MaxRetries, lastErr)
}

// ============================================================================
// COMPATIBILITY WRAPPER
// ============================================================================

// Wrapper for old code
func init() {
	// Auto-diagnose on import if DEBUG env is set
	if os.Getenv("DB_DEBUG") == "true" {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		DiagnoseConnectionIssues(ctx)
	}
}