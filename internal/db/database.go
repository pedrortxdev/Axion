package db

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	"aexon/internal/types"

	_ "github.com/mattn/go-sqlite3"
)

var DB *sql.DB

// Job representa uma tarefa assíncrona no sistema.
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

// Init inicializa a conexão com o SQLite e cria as tabelas.
func Init(dbPath string) error {
	var err error
	dsn := fmt.Sprintf("%s?_busy_timeout=5000&_foreign_keys=1", dbPath)

	DB, err = sql.Open("sqlite3", dsn)
	if err != nil {
		return fmt.Errorf("erro ao abrir banco de dados: %w", err)
	}

	if err = DB.Ping(); err != nil {
		return fmt.Errorf("erro ao conectar ao banco de dados: %w", err)
	}

	if _, err := DB.Exec("PRAGMA journal_mode=WAL;"); err != nil {
		return fmt.Errorf("erro ao ativar WAL mode: %w", err)
	}
	if _, err := DB.Exec("PRAGMA synchronous=NORMAL;"); err != nil {
		return fmt.Errorf("erro ao ativar synchronous=NORMAL: %w", err)
	}

	return createTables()
}

func createTables() error {
	query := `
	CREATE TABLE IF NOT EXISTS jobs (
		id TEXT PRIMARY KEY,
		type TEXT NOT NULL,
		target TEXT,
		payload TEXT NOT NULL,
		status TEXT NOT NULL,
		error TEXT,
		created_at DATETIME NOT NULL DEFAULT (datetime('now')),
		started_at DATETIME,
		finished_at DATETIME,
		attempt_count INTEGER DEFAULT 0,
		requested_by TEXT
	);
	CREATE INDEX IF NOT EXISTS idx_jobs_status ON jobs(status);
	CREATE INDEX IF NOT EXISTS idx_jobs_target ON jobs(target);
	CREATE INDEX IF NOT EXISTS idx_jobs_created_at ON jobs(created_at);
	`
	_, err := DB.Exec(query)
	return err
}

func RecoverStuckJobs() error {
	// Resetar IN_PROGRESS antigos para PENDING
	query := `
	UPDATE jobs
	SET status = ?, attempt_count = attempt_count + 1
	WHERE status = ?
	  AND started_at < datetime('now', '-5 minutes')
	`
	res, err := DB.Exec(query, types.JobPending, types.JobInProgress)
	if err != nil {
		return err
	}

	rows, _ := res.RowsAffected()
	if rows > 0 {
		log.Printf("[DB Recovery] %d jobs presos foram reiniciados para PENDING", rows)
	}
	return nil
}

func CreateJob(job *Job) error {
	query := `
	INSERT INTO jobs (id, type, target, payload, status, created_at, attempt_count, requested_by)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`
	job.CreatedAt = time.Now()
	job.Status = types.JobPending
	job.AttemptCount = 0

	_, err := DB.Exec(query, job.ID, job.Type, job.Target, job.Payload, job.Status, job.CreatedAt, job.AttemptCount, job.RequestedBy)
	return err
}

func MarkJobStarted(id string) error {
	query := `
	UPDATE jobs 
	SET status = ?, started_at = ?, attempt_count = attempt_count + 1
	WHERE id = ?
	`
	_, err := DB.Exec(query, types.JobInProgress, time.Now(), id)
	return err
}

func MarkJobCompleted(id string) error {
	query := `
	UPDATE jobs 
	SET status = ?, finished_at = ?, error = NULL
	WHERE id = ?
	`
	_, err := DB.Exec(query, types.JobCompleted, time.Now(), id)
	return err
}

func MarkJobFailed(id string, errorMsg string, isFatal bool) error {
	status := types.JobPending
	if isFatal {
		status = types.JobFailed
	}

	if isFatal {
		query := `
		UPDATE jobs 
		SET status = ?, error = ?, finished_at = ?
		WHERE id = ?
		`
		_, err := DB.Exec(query, status, errorMsg, time.Now(), id)
		return err
	} else {
		query := `
		UPDATE jobs 
		SET status = ?, error = ?
		WHERE id = ?
		`
		_, err := DB.Exec(query, status, errorMsg, id)
		return err
	}
}

func GetJob(id string) (*Job, error) {
	query := `SELECT id, type, target, payload, status, error, created_at, started_at, finished_at, attempt_count, requested_by FROM jobs WHERE id = ?`
	row := DB.QueryRow(query, id)

	var job Job
	var errStr sql.NullString
	var reqByStr sql.NullString

	err := row.Scan(&job.ID, &job.Type, &job.Target, &job.Payload, &job.Status, &errStr, &job.CreatedAt, &job.StartedAt, &job.FinishedAt, &job.AttemptCount, &reqByStr)
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

	return &job, nil
}

func ListRecentJobs(limit int) ([]Job, error) {
	query := `
	SELECT id, type, target, payload, status, error, created_at, started_at, finished_at, attempt_count, requested_by 
	FROM jobs 
	ORDER BY created_at DESC 
	LIMIT ?
	`
	rows, err := DB.Query(query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobs []Job
	for rows.Next() {
		var job Job
		var errStr sql.NullString
		var reqByStr sql.NullString

		if err := rows.Scan(&job.ID, &job.Type, &job.Target, &job.Payload, &job.Status, &errStr, &job.CreatedAt, &job.StartedAt, &job.FinishedAt, &job.AttemptCount, &reqByStr); err != nil {
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

		jobs = append(jobs, job)
	}
	return jobs, nil
}
