// database/jobs.go
package db

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	"aexon/internal/types"
)

// ============================================================================
// JOB REPOSITORY
// ============================================================================

type Job struct {
	ID           string          `json:"id"`
	Type         types.JobType   `json:"type"`
	Target       string          `json:"target"`
	Payload      string          `json:"payload"`
	Status       types.JobStatus `json:"status"`
	Error        *string         `json:"error,omitempty"`
	CreatedAt    time.Time       `json:"created_at"`
	StartedAt    *time.Time      `json:"started_at,omitempty"`
	FinishedAt   *time.Time      `json:"finished_at,omitempty"`
	AttemptCount int             `json:"attempt_count"`
	RequestedBy  *string         `json:"requested_by,omitempty"`
}

type JobRepository struct {
	db *Service
}

func NewJobRepository(db *Service) *JobRepository {
	return &JobRepository{db: db}
}

// ============================================================================
// CRUD OPERATIONS
// ============================================================================

func (r *JobRepository) Create(ctx context.Context, job *Job) error {
	query := `
		INSERT INTO jobs (
			id, type, target, payload, status,
			created_at, attempt_count, requested_by
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`

	job.CreatedAt = time.Now().UTC()
	job.Status = types.JobPending
	job.AttemptCount = 0

	_, err := r.db.ExecContext(ctx, query,
		job.ID,
		job.Type,
		job.Target,
		job.Payload,
		job.Status,
		job.CreatedAt,
		job.AttemptCount,
		job.RequestedBy,
	)

	return err
}

func (r *JobRepository) Get(ctx context.Context, id string) (*Job, error) {
	query := `
		SELECT id, type, target, payload, status, error,
		       created_at, started_at, finished_at,
		       attempt_count, requested_by
		FROM jobs
		WHERE id = $1
	`

	row := r.db.QueryRowContext(ctx, query, id)

	var job Job
	var errStr sql.NullString
	var reqByStr sql.NullString
	var startedAt sql.NullTime
	var finishedAt sql.NullTime

	err := row.Scan(
		&job.ID,
		&job.Type,
		&job.Target,
		&job.Payload,
		&job.Status,
		&errStr,
		&job.CreatedAt,
		&startedAt,
		&finishedAt,
		&job.AttemptCount,
		&reqByStr,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("job not found: %s", id)
		}
		return nil, err
	}

	// Handle nullable fields
	if errStr.Valid {
		s := errStr.String
		job.Error = &s
	}
	if reqByStr.Valid {
		s := reqByStr.String
		job.RequestedBy = &s
	}
	if startedAt.Valid {
		job.StartedAt = &startedAt.Time
	}
	if finishedAt.Valid {
		job.FinishedAt = &finishedAt.Time
	}

	return &job, nil
}

func (r *JobRepository) List(ctx context.Context, limit int) ([]Job, error) {
	query := `
		SELECT id, type, target, payload, status, error,
		       created_at, started_at, finished_at,
		       attempt_count, requested_by
		FROM jobs
		ORDER BY created_at DESC
		LIMIT $1
	`

	rows, err := r.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobs []Job

	for rows.Next() {
		var job Job
		var errStr sql.NullString
		var reqByStr sql.NullString
		var startedAt sql.NullTime
		var finishedAt sql.NullTime

		err := rows.Scan(
			&job.ID,
			&job.Type,
			&job.Target,
			&job.Payload,
			&job.Status,
			&errStr,
			&job.CreatedAt,
			&startedAt,
			&finishedAt,
			&job.AttemptCount,
			&reqByStr,
		)

		if err != nil {
			return nil, err
		}

		if errStr.Valid {
			s := errStr.String
			job.Error = &s
		}
		if reqByStr.Valid {
			s := reqByStr.String
			job.RequestedBy = &s
		}
		if startedAt.Valid {
			job.StartedAt = &startedAt.Time
		}
		if finishedAt.Valid {
			job.FinishedAt = &finishedAt.Time
		}

		jobs = append(jobs, job)
	}

	return jobs, rows.Err()
}

// ============================================================================
// STATUS UPDATES
// ============================================================================

func (r *JobRepository) MarkStarted(ctx context.Context, id string) error {
	query := `
		UPDATE jobs
		SET status = $1,
		    started_at = $2,
		    attempt_count = attempt_count + 1
		WHERE id = $3
	`

	result, err := r.db.ExecContext(ctx, query,
		types.JobInProgress,
		time.Now().UTC(),
		id,
	)

	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rows == 0 {
		return fmt.Errorf("job not found: %s", id)
	}

	return nil
}

func (r *JobRepository) MarkCompleted(ctx context.Context, id string) error {
	query := `
		UPDATE jobs
		SET status = $1,
		    finished_at = $2,
		    error = NULL
		WHERE id = $3
	`

	result, err := r.db.ExecContext(ctx, query,
		types.JobCompleted,
		time.Now().UTC(),
		id,
	)

	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rows == 0 {
		return fmt.Errorf("job not found: %s", id)
	}

	return nil
}

func (r *JobRepository) MarkFailed(ctx context.Context, id string, errorMsg string, isFatal bool) error {
	status := types.JobPending
	if isFatal {
		status = types.JobFailed
	}

	var query string
	var args []interface{}

	if isFatal {
		query = `
			UPDATE jobs
			SET status = $1,
			    error = $2,
			    finished_at = $3
			WHERE id = $4
		`
		args = []interface{}{status, errorMsg, time.Now().UTC(), id}
	} else {
		query = `
			UPDATE jobs
			SET status = $1,
			    error = $2
			WHERE id = $3
		`
		args = []interface{}{status, errorMsg, id}
	}

	result, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rows == 0 {
		return fmt.Errorf("job not found: %s", id)
	}

	return nil
}

func (r *JobRepository) MarkCanceled(ctx context.Context, id string, reason string) error {
	query := `
		UPDATE jobs
		SET status = $1,
		    error = $2,
		    finished_at = $3
		WHERE id = $4
	`

	result, err := r.db.ExecContext(ctx, query,
		types.JobCanceled,
		reason,
		time.Now().UTC(),
		id,
	)

	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rows == 0 {
		return fmt.Errorf("job not found: %s", id)
	}

	return nil
}

// ============================================================================
// RECOVERY OPERATIONS
// ============================================================================

func (r *JobRepository) RecoverStuckJobs(ctx context.Context, timeout time.Duration) (int, error) {
	query := `
		UPDATE jobs
		SET status = $1,
		    attempt_count = attempt_count + 1
		WHERE status = $2
		  AND started_at < $3
		RETURNING id
	`

	cutoff := time.Now().UTC().Add(-timeout)

	rows, err := r.db.QueryContext(ctx, query,
		types.JobPending,
		types.JobInProgress,
		cutoff,
	)

	if err != nil {
		return 0, err
	}
	defer rows.Close()

	var recoveredIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return 0, err
		}
		recoveredIDs = append(recoveredIDs, id)
	}

	if len(recoveredIDs) > 0 {
		log.Printf("[Jobs] Recovered %d stuck jobs: %v", len(recoveredIDs), recoveredIDs)
	}

	return len(recoveredIDs), rows.Err()
}

func (r *JobRepository) GetStuckJobs(ctx context.Context, timeout time.Duration) ([]Job, error) {
	query := `
		SELECT id, type, target, payload, status, error,
		       created_at, started_at, finished_at,
		       attempt_count, requested_by
		FROM jobs
		WHERE status = $1
		  AND started_at < $2
		ORDER BY started_at ASC
	`

	cutoff := time.Now().UTC().Add(-timeout)

	rows, err := r.db.QueryContext(ctx, query, types.JobInProgress, cutoff)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobs []Job

	for rows.Next() {
		var job Job
		var errStr sql.NullString
		var reqByStr sql.NullString
		var startedAt sql.NullTime
		var finishedAt sql.NullTime

		err := rows.Scan(
			&job.ID,
			&job.Type,
			&job.Target,
			&job.Payload,
			&job.Status,
			&errStr,
			&job.CreatedAt,
			&startedAt,
			&finishedAt,
			&job.AttemptCount,
			&reqByStr,
		)

		if err != nil {
			return nil, err
		}

		if errStr.Valid {
			s := errStr.String
			job.Error = &s
		}
		if reqByStr.Valid {
			s := reqByStr.String
			job.RequestedBy = &s
		}
		if startedAt.Valid {
			job.StartedAt = &startedAt.Time
		}
		if finishedAt.Valid {
			job.FinishedAt = &finishedAt.Time
		}

		jobs = append(jobs, job)
	}

	return jobs, rows.Err()
}

// ============================================================================
// QUERY HELPERS
// ============================================================================

func (r *JobRepository) GetByStatus(ctx context.Context, status types.JobStatus, limit int) ([]Job, error) {
	query := `
		SELECT id, type, target, payload, status, error,
		       created_at, started_at, finished_at,
		       attempt_count, requested_by
		FROM jobs
		WHERE status = $1
		ORDER BY created_at DESC
		LIMIT $2
	`

	rows, err := r.db.QueryContext(ctx, query, status, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobs []Job

	for rows.Next() {
		var job Job
		var errStr sql.NullString
		var reqByStr sql.NullString
		var startedAt sql.NullTime
		var finishedAt sql.NullTime

		err := rows.Scan(
			&job.ID,
			&job.Type,
			&job.Target,
			&job.Payload,
			&job.Status,
			&errStr,
			&job.CreatedAt,
			&startedAt,
			&finishedAt,
			&job.AttemptCount,
			&reqByStr,
		)

		if err != nil {
			return nil, err
		}

		if errStr.Valid {
			s := errStr.String
			job.Error = &s
		}
		if reqByStr.Valid {
			s := reqByStr.String
			job.RequestedBy = &s
		}
		if startedAt.Valid {
			job.StartedAt = &startedAt.Time
		}
		if finishedAt.Valid {
			job.FinishedAt = &finishedAt.Time
		}

		jobs = append(jobs, job)
	}

	return jobs, rows.Err()
}

func (r *JobRepository) GetByTarget(ctx context.Context, target string, limit int) ([]Job, error) {
	query := `
		SELECT id, type, target, payload, status, error,
		       created_at, started_at, finished_at,
		       attempt_count, requested_by
		FROM jobs
		WHERE target = $1
		ORDER BY created_at DESC
		LIMIT $2
	`

	rows, err := r.db.QueryContext(ctx, query, target, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobs []Job

	for rows.Next() {
		var job Job
		var errStr sql.NullString
		var reqByStr sql.NullString
		var startedAt sql.NullTime
		var finishedAt sql.NullTime

		err := rows.Scan(
			&job.ID,
			&job.Type,
			&job.Target,
			&job.Payload,
			&job.Status,
			&errStr,
			&job.CreatedAt,
			&startedAt,
			&finishedAt,
			&job.AttemptCount,
			&reqByStr,
		)

		if err != nil {
			return nil, err
		}

		if errStr.Valid {
			s := errStr.String
			job.Error = &s
		}
		if reqByStr.Valid {
			s := reqByStr.String
			job.RequestedBy = &s
		}
		if startedAt.Valid {
			job.StartedAt = &startedAt.Time
		}
		if finishedAt.Valid {
			job.FinishedAt = &finishedAt.Time
		}

		jobs = append(jobs, job)
	}

	return jobs, rows.Err()
}

func (r *JobRepository) GetLastBackupJob(ctx context.Context, instanceName string) (*Job, error) {
	// Try by target first
	query := `
		SELECT id, type, target, payload, status, error,
		       created_at, started_at, finished_at,
		       attempt_count, requested_by
		FROM jobs
		WHERE type = $1 AND target = $2
		ORDER BY created_at DESC
		LIMIT 1
	`

	row := r.db.QueryRowContext(ctx, query, types.JobTypeCreateSnapshot, instanceName)

	var job Job
	var errStr sql.NullString
	var reqByStr sql.NullString
	var startedAt sql.NullTime
	var finishedAt sql.NullTime

	err := row.Scan(
		&job.ID,
		&job.Type,
		&job.Target,
		&job.Payload,
		&job.Status,
		&errStr,
		&job.CreatedAt,
		&startedAt,
		&finishedAt,
		&job.AttemptCount,
		&reqByStr,
	)

	if err == nil {
		if errStr.Valid {
			s := errStr.String
			job.Error = &s
		}
		if reqByStr.Valid {
			s := reqByStr.String
			job.RequestedBy = &s
		}
		if startedAt.Valid {
			job.StartedAt = &startedAt.Time
		}
		if finishedAt.Valid {
			job.FinishedAt = &finishedAt.Time
		}
		return &job, nil
	}

	if err != sql.ErrNoRows {
		return nil, err
	}

	// Fallback: search in payload (less efficient)
	query = `
		SELECT id, type, target, payload, status, error,
		       created_at, started_at, finished_at,
		       attempt_count, requested_by
		FROM jobs
		WHERE type = $1 AND payload LIKE $2
		ORDER BY created_at DESC
		LIMIT 1
	`

	row = r.db.QueryRowContext(ctx, query, types.JobTypeCreateSnapshot, "%"+instanceName+"%")

	err = row.Scan(
		&job.ID,
		&job.Type,
		&job.Target,
		&job.Payload,
		&job.Status,
		&errStr,
		&job.CreatedAt,
		&startedAt,
		&finishedAt,
		&job.AttemptCount,
		&reqByStr,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // No backup found
		}
		return nil, err
	}

	if errStr.Valid {
		s := errStr.String
		job.Error = &s
	}
	if reqByStr.Valid {
		s := reqByStr.String
		job.RequestedBy = &s
	}
	if startedAt.Valid {
		job.StartedAt = &startedAt.Time
	}
	if finishedAt.Valid {
		job.FinishedAt = &finishedAt.Time
	}

	return &job, nil
}

func (r *JobRepository) CountByStatus(ctx context.Context, status types.JobStatus) (int, error) {
	query := `SELECT COUNT(*) FROM jobs WHERE status = $1`

	var count int
	err := r.db.QueryRowContext(ctx, query, status).Scan(&count)
	return count, err
}

func (r *JobRepository) GetStatistics(ctx context.Context) (map[string]int, error) {
	query := `
		SELECT status, COUNT(*) as count
		FROM jobs
		GROUP BY status
	`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	stats := make(map[string]int)

	for rows.Next() {
		var status string
		var count int

		if err := rows.Scan(&status, &count); err != nil {
			return nil, err
		}

		stats[status] = count
	}

	return stats, rows.Err()
}

// ============================================================================
// CLEANUP OPERATIONS
// ============================================================================

func (r *JobRepository) DeleteOldJobs(ctx context.Context, olderThan time.Duration) (int, error) {
	query := `
		DELETE FROM jobs
		WHERE finished_at IS NOT NULL
		  AND finished_at < $1
		RETURNING id
	`

	cutoff := time.Now().UTC().Add(-olderThan)

	rows, err := r.db.QueryContext(ctx, query, cutoff)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return count, err
		}
		count++
	}

	if count > 0 {
		log.Printf("[Jobs] Deleted %d old jobs (older than %v)", count, olderThan)
	}

	return count, rows.Err()
}

func (r *JobRepository) DeleteByStatus(ctx context.Context, status types.JobStatus) (int, error) {
	query := `DELETE FROM jobs WHERE status = $1 RETURNING id`

	rows, err := r.db.QueryContext(ctx, query, status)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return count, err
		}
		count++
	}

	return count, rows.Err()
}

// ============================================================================
// COMPATIBILITY FUNCTIONS (for existing code)
// ============================================================================

func CreateJob(job *Job) error {
	ctx := context.Background()
	repo := NewJobRepository(GetService())
	return repo.Create(ctx, job)
}

func GetJob(id string) (*Job, error) {
	ctx := context.Background()
	repo := NewJobRepository(GetService())
	return repo.Get(ctx, id)
}

func ListRecentJobs(limit int) ([]Job, error) {
	ctx := context.Background()
	repo := NewJobRepository(GetService())
	return repo.List(ctx, limit)
}

func MarkJobStarted(id string) error {
	ctx := context.Background()
	repo := NewJobRepository(GetService())
	return repo.MarkStarted(ctx, id)
}

func MarkJobCompleted(id string) error {
	ctx := context.Background()
	repo := NewJobRepository(GetService())
	return repo.MarkCompleted(ctx, id)
}

func MarkJobFailed(id string, errorMsg string, isFatal bool) error {
	ctx := context.Background()
	repo := NewJobRepository(GetService())
	return repo.MarkFailed(ctx, id, errorMsg, isFatal)
}

func RecoverStuckJobs() error {
	ctx := context.Background()
	repo := NewJobRepository(GetService())
	_, err := repo.RecoverStuckJobs(ctx, 5*time.Minute)
	return err
}

func GetLastBackupJob(instanceName string) (*Job, error) {
	ctx := context.Background()
	repo := NewJobRepository(GetService())
	return repo.GetLastBackupJob(ctx, instanceName)
}