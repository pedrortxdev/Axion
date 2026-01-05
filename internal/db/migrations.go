// database/migrations.go
package db

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/robfig/cron/v3"
)

// ============================================================================
// SCHEMA MIGRATIONS
// ============================================================================

type Migration struct {
	Version     int
	Description string
	Up          string
	Down        string
}

var migrations = []Migration{
	{
		Version:     1,
		Description: "Initial schema",
		Up: `
			CREATE TABLE IF NOT EXISTS jobs (
				id TEXT PRIMARY KEY,
				type TEXT NOT NULL,
				target TEXT,
				payload TEXT NOT NULL,
				status TEXT NOT NULL,
				error TEXT,
				created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
				started_at TIMESTAMP,
				finished_at TIMESTAMP,
				attempt_count INTEGER DEFAULT 0,
				requested_by TEXT
			);
			
			CREATE INDEX IF NOT EXISTS idx_jobs_status ON jobs(status);
			CREATE INDEX IF NOT EXISTS idx_jobs_target ON jobs(target);
			CREATE INDEX IF NOT EXISTS idx_jobs_created_at ON jobs(created_at DESC);
			CREATE INDEX IF NOT EXISTS idx_jobs_type_target ON jobs(type, target);
		`,
		Down: `DROP TABLE IF EXISTS jobs CASCADE;`,
	},
	{
		Version:     2,
		Description: "Create instances table",
		Up: `
			CREATE TABLE IF NOT EXISTS instances (
				name TEXT PRIMARY KEY,
				image TEXT NOT NULL,
				limits JSONB DEFAULT '{}'::jsonb,
				user_data TEXT,
				type TEXT DEFAULT 'container',
				backup_schedule TEXT,
				backup_retention INTEGER DEFAULT 7 CHECK (backup_retention > 0),
				backup_enabled BOOLEAN DEFAULT FALSE,
				created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
				updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
			);
			
			CREATE INDEX IF NOT EXISTS idx_instances_type ON instances(type);
			CREATE INDEX IF NOT EXISTS idx_instances_backup_enabled ON instances(backup_enabled) WHERE backup_enabled = true;
		`,
		Down: `DROP TABLE IF EXISTS instances CASCADE;`,
	},
	{
		Version:     3,
		Description: "Create metrics table with partitioning support",
		Up: `
			CREATE TABLE IF NOT EXISTS metrics (
				id BIGSERIAL PRIMARY KEY,
				instance_name TEXT NOT NULL,
				timestamp TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
				cpu_percent DOUBLE PRECISION CHECK (cpu_percent >= 0),
				memory_usage BIGINT CHECK (memory_usage >= 0),
				disk_usage BIGINT CHECK (disk_usage >= 0)
			);
			
			CREATE INDEX IF NOT EXISTS idx_metrics_instance_time ON metrics(instance_name, timestamp DESC);
			CREATE INDEX IF NOT EXISTS idx_metrics_timestamp ON metrics(timestamp DESC);
		`,
		Down: `DROP TABLE IF EXISTS metrics CASCADE;`,
	},
	{
		Version:     4,
		Description: "Add schema_migrations tracking table",
		Up: `
			CREATE TABLE IF NOT EXISTS schema_migrations (
				version INTEGER PRIMARY KEY,
				description TEXT NOT NULL,
				applied_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
			);
		`,
		Down: `DROP TABLE IF EXISTS schema_migrations CASCADE;`,
	},
	{
		Version:     5,
		Description: "Create branding settings table",
		Up: `
			CREATE TABLE IF NOT EXISTS branding_settings (
				id SERIAL PRIMARY KEY,
				user_id INTEGER UNIQUE NOT NULL,
				logo_url VARCHAR(500),
				primary_color VARCHAR(7) DEFAULT '#3B82F6',
				hide_powered_by BOOLEAN DEFAULT FALSE,
				created_at TIMESTAMP DEFAULT NOW(),
				updated_at TIMESTAMP DEFAULT NOW()
			);

			CREATE INDEX IF NOT EXISTS idx_branding_user_id ON branding_settings(user_id);
		`,
		Down: `DROP TABLE IF EXISTS branding_settings CASCADE;`,
	},
	{
		Version:     6,
		Description: "Create and populate IPAM leases table",
		Up: `
			CREATE TABLE IF NOT EXISTS ip_leases (
				ip VARCHAR(15) PRIMARY KEY,
				instance_name TEXT,
				allocated_at TIMESTAMP,
				CONSTRAINT fk_instance FOREIGN KEY (instance_name) REFERENCES instances(name) ON DELETE SET NULL
			);

			CREATE INDEX IF NOT EXISTS idx_ip_leases_instance ON ip_leases(instance_name);

			-- Populate with 172.16.0.2 to 172.16.0.254
			DO $$
			BEGIN
				FOR i IN 2..254 LOOP
					INSERT INTO ip_leases (ip) VALUES ('172.16.0.' || i) ON CONFLICT DO NOTHING;
				END LOOP;
			END $$;
		`,
		Down: `DROP TABLE IF EXISTS ip_leases CASCADE;`,
	},
	{
		Version:     7,
		Description: "Create users table",
		Up: `
			CREATE TABLE IF NOT EXISTS users (
				id SERIAL PRIMARY KEY,
				email TEXT UNIQUE NOT NULL,
				password_hash TEXT NOT NULL,
				role TEXT DEFAULT 'user',
				created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
				updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
			);
			CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);
		`,
		Down: `DROP TABLE IF EXISTS users CASCADE;`,
	},
	{
		Version:     8,
		Description: "Create networks table and link to ip_leases",
		Up: `
			CREATE TABLE IF NOT EXISTS networks (
				id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
				name VARCHAR(50) NOT NULL,
				cidr VARCHAR(18) NOT NULL,
				gateway VARCHAR(15) NOT NULL,
				dns1 VARCHAR(15) DEFAULT '1.1.1.1',
				vlan_id INT DEFAULT 0,
				is_public BOOLEAN DEFAULT FALSE,
				created_at TIMESTAMP DEFAULT NOW()
			);

			-- Enable UUID extension if not exists
			CREATE EXTENSION IF NOT EXISTS "pgcrypto";

			-- Seed default network
			INSERT INTO networks (name, cidr, gateway, is_public) 
			VALUES ('Default Private', '172.16.0.0/24', '172.16.0.1', false);

			-- Add network_id to ip_leases
			ALTER TABLE ip_leases ADD COLUMN IF NOT EXISTS network_id UUID REFERENCES networks(id);

			-- Link existing IPs to the default network
			DO $$
			DECLARE
				net_id UUID;
			BEGIN
				SELECT id INTO net_id FROM networks WHERE cidr = '172.16.0.0/24' LIMIT 1;
				IF net_id IS NOT NULL THEN
					UPDATE ip_leases SET network_id = net_id WHERE network_id IS NULL;
				END IF;
			END $$;
		`,
		Down: `
			ALTER TABLE ip_leases DROP COLUMN IF EXISTS network_id;
			DROP TABLE IF EXISTS networks CASCADE;
		`,
	},
	{
		Version:     9,
		Description: "Seed extra network pools",
		Up: `
			-- 1. Pool Padrão (O que o AxHV usa nativamente)
			INSERT INTO networks (name, cidr, gateway, dns1, is_public, vlan_id)
			VALUES ('Default AxHV NAT', '172.16.0.0/24', '172.16.0.1', '1.1.1.1', false, 0);

			-- 2. Pool Extra (Corporativo - 10.x)
			INSERT INTO networks (name, cidr, gateway, dns1, is_public, vlan_id)
			VALUES ('Corporate Pool A', '10.0.0.0/24', '10.0.0.1', '8.8.8.8', false, 0);

			-- 3. Pool Home Lab (192.x)
			INSERT INTO networks (name, cidr, gateway, dns1, is_public, vlan_id)
			VALUES ('Home Pool B', '192.168.100.0/24', '192.168.100.1', '8.8.4.4', false, 0);
		`,
		Down: `
			DELETE FROM networks WHERE cidr IN ('172.16.0.0/24', '10.0.0.0/24', '192.168.100.0/24');
		`,
	},
}

// ============================================================================
// MIGRATION MANAGEMENT
// ============================================================================

func RunMigrations(ctx context.Context, db *Service) error {
	log.Println("[Migrations] Starting database migrations...")

	// Ensure schema_migrations table exists
	if err := ensureSchemaMigrationsTable(ctx, db); err != nil {
		return fmt.Errorf("failed to create schema_migrations table: %w", err)
	}

	// Get current version
	currentVersion, err := getCurrentVersion(ctx, db)
	if err != nil {
		return fmt.Errorf("failed to get current version: %w", err)
	}

	log.Printf("[Migrations] Current schema version: %d", currentVersion)

	// Apply pending migrations
	appliedCount := 0
	for _, migration := range migrations {
		if migration.Version <= currentVersion {
			continue
		}

		log.Printf("[Migrations] Applying migration %d: %s", migration.Version, migration.Description)

		if err := applyMigration(ctx, db, migration); err != nil {
			return fmt.Errorf("failed to apply migration %d: %w", migration.Version, err)
		}

		appliedCount++
	}

	if appliedCount > 0 {
		log.Printf("[Migrations] Successfully applied %d migration(s)", appliedCount)
	} else {
		log.Println("[Migrations] Database schema is up to date")
	}

	return nil
}

func ensureSchemaMigrationsTable(ctx context.Context, db *Service) error {
	query := `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			description TEXT NOT NULL,
			applied_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`
	_, err := db.ExecContext(ctx, query)
	return err
}

func getCurrentVersion(ctx context.Context, db *Service) (int, error) {
	query := `SELECT COALESCE(MAX(version), 0) FROM schema_migrations`

	var version int
	err := db.QueryRowContext(ctx, query).Scan(&version)
	if err != nil {
		return 0, err
	}

	return version, nil
}

func applyMigration(ctx context.Context, db *Service, migration Migration) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Execute migration
	if _, err := tx.ExecContext(ctx, migration.Up); err != nil {
		return fmt.Errorf("failed to execute migration SQL: %w", err)
	}

	// Record migration
	query := `INSERT INTO schema_migrations (version, description) VALUES ($1, $2)`
	if _, err := tx.ExecContext(ctx, query, migration.Version, migration.Description); err != nil {
		return fmt.Errorf("failed to record migration: %w", err)
	}

	return tx.Commit()
}

func RollbackMigration(ctx context.Context, db *Service, targetVersion int) error {
	currentVersion, err := getCurrentVersion(ctx, db)
	if err != nil {
		return err
	}

	if targetVersion >= currentVersion {
		return fmt.Errorf("target version must be less than current version")
	}

	log.Printf("[Migrations] Rolling back from version %d to %d", currentVersion, targetVersion)

	// Apply rollbacks in reverse order
	for i := len(migrations) - 1; i >= 0; i-- {
		migration := migrations[i]

		if migration.Version <= targetVersion || migration.Version > currentVersion {
			continue
		}

		log.Printf("[Migrations] Rolling back migration %d: %s", migration.Version, migration.Description)

		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}

		// Execute rollback
		if _, err := tx.ExecContext(ctx, migration.Down); err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to rollback migration %d: %w", migration.Version, err)
		}

		// Remove migration record
		query := `DELETE FROM schema_migrations WHERE version = $1`
		if _, err := tx.ExecContext(ctx, query, migration.Version); err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to delete migration record: %w", err)
		}

		if err := tx.Commit(); err != nil {
			return err
		}
	}

	log.Printf("[Migrations] Successfully rolled back to version %d", targetVersion)
	return nil
}

// ============================================================================
// CRON HELPERS
// ============================================================================

func GetNextRunTime(schedule string) (*time.Time, error) {
	if schedule == "" {
		return nil, nil
	}

	parser := cron.NewParser(
		cron.SecondOptional |
			cron.Minute |
			cron.Hour |
			cron.Dom |
			cron.Month |
			cron.Dow |
			cron.Descriptor,
	)

	sched, err := parser.Parse(schedule)
	if err != nil {
		return nil, fmt.Errorf("failed to parse cron schedule '%s': %w", schedule, err)
	}

	nextRun := sched.Next(time.Now().UTC())
	return &nextRun, nil
}

// ============================================================================
// MAINTENANCE TASKS
// ============================================================================

func RunMaintenance(ctx context.Context, db *Service) error {
	log.Println("[Maintenance] Starting database maintenance...")

	// Clean old metrics (older than 30 days)
	metricsRepo := NewMetricsRepository(db)
	deletedMetrics, err := metricsRepo.DeleteOlderThan(ctx, 30*24*time.Hour)
	if err != nil {
		log.Printf("[Maintenance] Error cleaning old metrics: %v", err)
	} else if deletedMetrics > 0 {
		log.Printf("[Maintenance] Deleted %d old metrics", deletedMetrics)
	}

	// Clean old jobs (completed/failed older than 7 days)
	jobsRepo := NewJobRepository(db)
	deletedJobs, err := jobsRepo.DeleteOldJobs(ctx, 7*24*time.Hour)
	if err != nil {
		log.Printf("[Maintenance] Error cleaning old jobs: %v", err)
	} else if deletedJobs > 0 {
		log.Printf("[Maintenance] Deleted %d old jobs", deletedJobs)
	}

	// Recover stuck jobs
	recoveredJobs, err := jobsRepo.RecoverStuckJobs(ctx, 5*time.Minute)
	if err != nil {
		log.Printf("[Maintenance] Error recovering stuck jobs: %v", err)
	} else if recoveredJobs > 0 {
		log.Printf("[Maintenance] Recovered %d stuck jobs", recoveredJobs)
	}

	// Vacuum analyze (PostgreSQL specific)
	if _, err := db.ExecContext(ctx, "VACUUM ANALYZE"); err != nil {
		log.Printf("[Maintenance] Error running VACUUM ANALYZE: %v", err)
	} else {
		log.Println("[Maintenance] VACUUM ANALYZE completed")
	}

	log.Println("[Maintenance] Database maintenance completed")
	return nil
}

func StartMaintenanceScheduler(ctx context.Context, db *Service, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	log.Printf("[Maintenance] Scheduler started (interval: %v)", interval)

	for {
		select {
		case <-ticker.C:
			if err := RunMaintenance(ctx, db); err != nil {
				log.Printf("[Maintenance] Error during maintenance: %v", err)
			}

		case <-ctx.Done():
			log.Println("[Maintenance] Scheduler stopped")
			return
		}
	}
}

// ============================================================================
// UTILITY FUNCTIONS
// ============================================================================

func GetDatabaseSize(ctx context.Context, db *Service, dbName string) (int64, error) {
	query := `SELECT pg_database_size($1)`

	var size int64
	err := db.QueryRowContext(ctx, query, dbName).Scan(&size)
	return size, err
}

func GetTableSizes(ctx context.Context, db *Service) (map[string]int64, error) {
	query := `
		SELECT
			tablename,
			pg_total_relation_size(schemaname||'.'||tablename) AS size
		FROM pg_tables
		WHERE schemaname = 'public'
		ORDER BY size DESC
	`

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	sizes := make(map[string]int64)

	for rows.Next() {
		var tableName string
		var size int64

		if err := rows.Scan(&tableName, &size); err != nil {
			return nil, err
		}

		sizes[tableName] = size
	}

	return sizes, rows.Err()
}

func GetConnectionCount(ctx context.Context, db *Service) (int, error) {
	query := `
		SELECT COUNT(*)
		FROM pg_stat_activity
		WHERE datname = current_database()
	`

	var count int
	err := db.QueryRowContext(ctx, query).Scan(&count)
	return count, err
}
