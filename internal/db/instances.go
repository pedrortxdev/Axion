// database/instances.go
package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"

	"aexon/internal/types"
)

// ============================================================================
// INSTANCE REPOSITORY
// ============================================================================

type InstanceRepository struct {
	db *Service
}

func NewInstanceRepository(db *Service) *InstanceRepository {
	return &InstanceRepository{db: db}
}

// ============================================================================
// CRUD OPERATIONS
// ============================================================================

func (r *InstanceRepository) Create(ctx context.Context, instance *types.Instance) error {
	limitsJSON, err := json.Marshal(instance.Limits)
	if err != nil {
		return fmt.Errorf("marshal limits: %w", err)
	}

	query := `
		INSERT INTO instances (
			name, image, limits, user_data, type,
			backup_schedule, backup_retention, backup_enabled
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`

	_, err = r.db.ExecContext(ctx, query,
		instance.Name,
		instance.Image,
		string(limitsJSON),
		instance.UserData,
		instance.Type,
		instance.BackupSchedule,
		instance.BackupRetention,
		instance.BackupEnabled,
	)

	return err
}

func (r *InstanceRepository) Get(ctx context.Context, name string) (*types.Instance, error) {
	query := `
		SELECT name, image, limits, user_data, type,
		       backup_schedule, backup_retention, backup_enabled
		FROM instances
		WHERE name = $1
	`

	row := r.db.QueryRowContext(ctx, query, name)

	var instance types.Instance
	var limitsJSON string

	err := row.Scan(
		&instance.Name,
		&instance.Image,
		&limitsJSON,
		&instance.UserData,
		&instance.Type,
		&instance.BackupSchedule,
		&instance.BackupRetention,
		&instance.BackupEnabled,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("instance not found: %s", name)
		}
		return nil, err
	}

	if err := json.Unmarshal([]byte(limitsJSON), &instance.Limits); err != nil {
		return nil, fmt.Errorf("unmarshal limits: %w", err)
	}

	// Set default retention if zero
	if instance.BackupRetention == 0 {
		instance.BackupRetention = 7
	}

	return &instance, nil
}

func (r *InstanceRepository) List(ctx context.Context) ([]types.Instance, error) {
	query := `
		SELECT name, image, limits, user_data, type,
		       backup_schedule, backup_retention, backup_enabled
		FROM instances
		ORDER BY name
	`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var instances []types.Instance

	for rows.Next() {
		var instance types.Instance
		var limitsJSON string

		err := rows.Scan(
			&instance.Name,
			&instance.Image,
			&limitsJSON,
			&instance.UserData,
			&instance.Type,
			&instance.BackupSchedule,
			&instance.BackupRetention,
			&instance.BackupEnabled,
		)

		if err != nil {
			return nil, err
		}

		if err := json.Unmarshal([]byte(limitsJSON), &instance.Limits); err != nil {
			log.Printf("[Instances] Failed to unmarshal limits for %s: %v", instance.Name, err)
			instance.Limits = make(map[string]string)
		}

		// Set default retention
		if instance.BackupRetention == 0 {
			instance.BackupRetention = 7
		}

		instances = append(instances, instance)
	}

	return instances, rows.Err()
}

func (r *InstanceRepository) Update(ctx context.Context, instance *types.Instance) error {
	limitsJSON, err := json.Marshal(instance.Limits)
	if err != nil {
		return fmt.Errorf("marshal limits: %w", err)
	}

	query := `
		UPDATE instances
		SET image = $2,
		    limits = $3,
		    user_data = $4,
		    type = $5,
		    backup_schedule = $6,
		    backup_retention = $7,
		    backup_enabled = $8
		WHERE name = $1
	`

	result, err := r.db.ExecContext(ctx, query,
		instance.Name,
		instance.Image,
		string(limitsJSON),
		instance.UserData,
		instance.Type,
		instance.BackupSchedule,
		instance.BackupRetention,
		instance.BackupEnabled,
	)

	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rows == 0 {
		return fmt.Errorf("instance not found: %s", instance.Name)
	}

	return nil
}

func (r *InstanceRepository) Delete(ctx context.Context, name string) error {
	query := `DELETE FROM instances WHERE name = $1`

	result, err := r.db.ExecContext(ctx, query, name)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rows == 0 {
		return fmt.Errorf("instance not found: %s", name)
	}

	return nil
}

// ============================================================================
// BACKUP OPERATIONS
// ============================================================================

func (r *InstanceRepository) UpdateBackupConfig(ctx context.Context, name string, enabled bool, schedule string, retention int) error {
	query := `
		UPDATE instances
		SET backup_enabled = $1,
		    backup_schedule = $2,
		    backup_retention = $3
		WHERE name = $4
	`

	result, err := r.db.ExecContext(ctx, query, enabled, schedule, retention, name)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rows == 0 {
		return fmt.Errorf("instance not found: %s", name)
	}

	return nil
}

func (r *InstanceRepository) GetWithBackupInfo(ctx context.Context, name string, jobRepo *JobRepository) (*types.Instance, error) {
	instance, err := r.Get(ctx, name)
	if err != nil {
		return nil, err
	}

	// Create backup info
	backupInfo := &types.InstanceBackupInfo{
		Enabled:  instance.BackupEnabled,
		Schedule: instance.BackupSchedule,
	}

	// Get next run time if backup enabled
	if instance.BackupEnabled {
		nextRun, err := GetNextRunTime(instance.BackupSchedule)
		if err != nil {
			log.Printf("[Instances] Error calculating next backup for %s: %v", name, err)
		} else {
			backupInfo.NextRun = nextRun
		}
	}

	// Get last backup job
	if jobRepo != nil {
		lastJob, err := jobRepo.GetLastBackupJob(ctx, name)
		if err != nil {
			log.Printf("[Instances] Error getting last backup job for %s: %v", name, err)
		} else if lastJob != nil {
			backupInfo.LastRun = lastJob.FinishedAt
			backupInfo.LastStatus = string(lastJob.Status)
		}
	}

	instance.BackupInfo = backupInfo
	return instance, nil
}

// ============================================================================
// HARDWARE INFO ENRICHMENT
// ============================================================================

// ============================================================================
// LIMITS UPDATE
// ============================================================================

func (r *InstanceRepository) UpdateLimits(ctx context.Context, name string, limits map[string]string) error {
	limitsJSON, err := json.Marshal(limits)
	if err != nil {
		return fmt.Errorf("marshal limits: %w", err)
	}

	query := `UPDATE instances SET limits = $1 WHERE name = $2`

	result, err := r.db.ExecContext(ctx, query, string(limitsJSON), name)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rows == 0 {
		return fmt.Errorf("instance not found: %s", name)
	}

	return nil
}

// ============================================================================
// BATCH OPERATIONS
// ============================================================================

func (r *InstanceRepository) CreateBatch(ctx context.Context, instances []*types.Instance) error {
	if len(instances) == 0 {
		return nil
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	query := `
		INSERT INTO instances (
			name, image, limits, user_data, type,
			backup_schedule, backup_retention, backup_enabled
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`

	for _, instance := range instances {
		limitsJSON, err := json.Marshal(instance.Limits)
		if err != nil {
			return fmt.Errorf("marshal limits for %s: %w", instance.Name, err)
		}

		_, err = tx.ExecContext(ctx, query,
			instance.Name,
			instance.Image,
			string(limitsJSON),
			instance.UserData,
			instance.Type,
			instance.BackupSchedule,
			instance.BackupRetention,
			instance.BackupEnabled,
		)

		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (r *InstanceRepository) DeleteBatch(ctx context.Context, names []string) error {
	if len(names) == 0 {
		return nil
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	query := `DELETE FROM instances WHERE name = $1`

	for _, name := range names {
		_, err := tx.ExecContext(ctx, query, name)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

// ============================================================================
// QUERY HELPERS
// ============================================================================

func (r *InstanceRepository) Exists(ctx context.Context, name string) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM instances WHERE name = $1)`

	var exists bool
	err := r.db.QueryRowContext(ctx, query, name).Scan(&exists)
	return exists, err
}

func (r *InstanceRepository) Count(ctx context.Context) (int, error) {
	query := `SELECT COUNT(*) FROM instances`

	var count int
	err := r.db.QueryRowContext(ctx, query).Scan(&count)
	return count, err
}

func (r *InstanceRepository) ListByType(ctx context.Context, instanceType string) ([]types.Instance, error) {
	query := `
		SELECT name, image, limits, user_data, type,
		       backup_schedule, backup_retention, backup_enabled
		FROM instances
		WHERE type = $1
		ORDER BY name
	`

	rows, err := r.db.QueryContext(ctx, query, instanceType)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var instances []types.Instance

	for rows.Next() {
		var instance types.Instance
		var limitsJSON string

		err := rows.Scan(
			&instance.Name,
			&instance.Image,
			&limitsJSON,
			&instance.UserData,
			&instance.Type,
			&instance.BackupSchedule,
			&instance.BackupRetention,
			&instance.BackupEnabled,
		)

		if err != nil {
			return nil, err
		}

		if err := json.Unmarshal([]byte(limitsJSON), &instance.Limits); err != nil {
			log.Printf("[Instances] Failed to unmarshal limits for %s: %v", instance.Name, err)
			instance.Limits = make(map[string]string)
		}

		if instance.BackupRetention == 0 {
			instance.BackupRetention = 7
		}

		instances = append(instances, instance)
	}

	return instances, rows.Err()
}

func (r *InstanceRepository) ListWithBackupEnabled(ctx context.Context) ([]types.Instance, error) {
	query := `
		SELECT name, image, limits, user_data, type,
		       backup_schedule, backup_retention, backup_enabled
		FROM instances
		WHERE backup_enabled = true
		ORDER BY name
	`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var instances []types.Instance

	for rows.Next() {
		var instance types.Instance
		var limitsJSON string

		err := rows.Scan(
			&instance.Name,
			&instance.Image,
			&limitsJSON,
			&instance.UserData,
			&instance.Type,
			&instance.BackupSchedule,
			&instance.BackupRetention,
			&instance.BackupEnabled,
		)

		if err != nil {
			return nil, err
		}

		if err := json.Unmarshal([]byte(limitsJSON), &instance.Limits); err != nil {
			log.Printf("[Instances] Failed to unmarshal limits for %s: %v", instance.Name, err)
			instance.Limits = make(map[string]string)
		}

		if instance.BackupRetention == 0 {
			instance.BackupRetention = 7
		}

		instances = append(instances, instance)
	}

	return instances, rows.Err()
}

// ============================================================================
// COMPATIBILITY FUNCTIONS (for existing code)
// ============================================================================

func CreateInstance(instance *types.Instance) error {
	ctx := context.Background()
	repo := NewInstanceRepository(GetService())
	return repo.Create(ctx, instance)
}

func GetInstance(name string) (*types.Instance, error) {
	ctx := context.Background()
	repo := NewInstanceRepository(GetService())
	return repo.Get(ctx, name)
}

func ListInstances() ([]types.Instance, error) {
	ctx := context.Background()
	repo := NewInstanceRepository(GetService())
	return repo.List(ctx)
}

func DeleteInstance(name string) error {
	ctx := context.Background()
	repo := NewInstanceRepository(GetService())
	return repo.Delete(ctx, name)
}

func UpdateInstanceBackupConfig(name string, enabled bool, schedule string, retention int) error {
	ctx := context.Background()
	repo := NewInstanceRepository(GetService())
	return repo.UpdateBackupConfig(ctx, name, enabled, schedule, retention)
}

func UpdateInstanceStatusAndLimits(name string, limits map[string]string) error {
	ctx := context.Background()
	repo := NewInstanceRepository(GetService())
	return repo.UpdateLimits(ctx, name, limits)
}
