package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"aexon/internal/provider/lxc"
	"aexon/internal/types"
	"aexon/internal/utils"
	"github.com/robfig/cron/v3"

	_ "github.com/lib/pq"
)

var DB *sql.DB

// Metric represents a single data point for instance monitoring.
type Metric struct {
	ID           int       `json:"id"`
	InstanceName string    `json:"instance_name"`
	Timestamp    time.Time `json:"timestamp"`
	CPUPercent   float64   `json:"cpu_percent"`
	MemoryUsage  int64     `json:"memory_usage"`
	DiskUsage    int64     `json:"disk_usage"`
}

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

// Init inicializa a conexão com o PostgreSQL e cria as tabelas.
func Init(dbPath string) error {
	var err error
	// Connection string for PostgreSQL
	connStr := "postgres://axion:axion_password@localhost/axion_db?sslmode=disable"

	DB, err = sql.Open("postgres", connStr)
	if err != nil {
		return fmt.Errorf("erro ao abrir banco de dados: %w", err)
	}

	if err = DB.Ping(); err != nil {
		// Check for specific errors that suggest the DB needs to be bootstrapped.
		if strings.Contains(err.Error(), "password authentication failed") || strings.Contains(err.Error(), "database \"axion_db\" does not exist") {
			log.Println("Database connection failed. Attempting to run bootstrap setup...")
			EnsureDBSetup()

			// Retry connection after bootstrap
			log.Println("Retrying database connection...")
			if err = DB.Ping(); err != nil {
				return fmt.Errorf("erro ao conectar ao banco de dados após bootstrap: %w", err)
			}
		} else {
			return fmt.Errorf("erro ao conectar ao banco de dados: %w", err)
		}
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
		created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		started_at TIMESTAMP,
		finished_at TIMESTAMP,
		attempt_count INTEGER DEFAULT 0,
		requested_by TEXT
	);
	CREATE INDEX IF NOT EXISTS idx_jobs_status ON jobs(status);
	CREATE INDEX IF NOT EXISTS idx_jobs_target ON jobs(target);
	CREATE INDEX IF NOT EXISTS idx_jobs_created_at ON jobs(created_at);

	CREATE TABLE IF NOT EXISTS instances (
		name TEXT PRIMARY KEY,
		image TEXT,
		limits TEXT,
		user_data TEXT,
		type TEXT,
		backup_schedule TEXT,
		backup_retention INTEGER DEFAULT 7,
		backup_enabled BOOLEAN DEFAULT FALSE
	);
	
	CREATE TABLE IF NOT EXISTS metrics (
		id SERIAL PRIMARY KEY,
		instance_name TEXT NOT NULL,
		timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		cpu_percent DOUBLE PRECISION,
		memory_usage BIGINT,
		disk_usage BIGINT
	);
	CREATE INDEX IF NOT EXISTS idx_metrics_instance_time ON metrics(instance_name, timestamp);
	`
	_, err := DB.Exec(query)
	return err
}

func InsertMetric(m *Metric) error {
	// Use current time in UTC for timestamp
	now := time.Now().UTC()
	query := `
	INSERT INTO metrics (instance_name, timestamp, cpu_percent, memory_usage, disk_usage)
	VALUES ($1, $2, $3, $4, $5)
	`
	_, err := DB.Exec(query, m.InstanceName, now, m.CPUPercent, m.MemoryUsage, m.DiskUsage)
	return err
}

func CreateInstance(instance *types.Instance) error {
	limits, err := json.Marshal(instance.Limits)
	if err != nil {
		return err
	}
	query := `
	INSERT INTO instances (name, image, limits, user_data, type, backup_schedule, backup_retention, backup_enabled)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`
	_, err = DB.Exec(query, instance.Name, instance.Image, string(limits), instance.UserData, instance.Type, instance.BackupSchedule, instance.BackupRetention, instance.BackupEnabled)
	return err
}

func GetInstance(name string) (*types.Instance, error) {
	query := `SELECT name, image, limits, user_data, type, backup_schedule, backup_retention, backup_enabled FROM instances WHERE name = $1`
	row := DB.QueryRow(query, name)

	var instance types.Instance
	var limits string
	err := row.Scan(&instance.Name, &instance.Image, &limits, &instance.UserData, &instance.Type, &instance.BackupSchedule, &instance.BackupRetention, &instance.BackupEnabled)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(limits), &instance.Limits); err != nil {
		return nil, err
	}
	return &instance, nil
}

func ListInstances() ([]types.Instance, error) {
	query := `SELECT name, image, limits, user_data, type, backup_schedule, backup_retention, backup_enabled FROM instances`
	rows, err := DB.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var instances []types.Instance
	for rows.Next() {
		var instance types.Instance
		var limits string
		if err := rows.Scan(&instance.Name, &instance.Image, &limits, &instance.UserData, &instance.Type, &instance.BackupSchedule, &instance.BackupRetention, &instance.BackupEnabled); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(limits), &instance.Limits); err != nil {
			log.Printf("failed to unmarshal limits for instance %s: %v", instance.Name, err)
		}
		instances = append(instances, instance)
	}
	return instances, nil
}

func UpdateInstanceBackupConfig(name string, enabled bool, schedule string, retention int) error {
	query := `
	UPDATE instances
	SET backup_enabled = $1, backup_schedule = $2, backup_retention = $3
	WHERE name = $4
	`
	_, err := DB.Exec(query, enabled, schedule, retention, name)
	return err
}

func DeleteInstance(name string) error {
	query := `DELETE FROM instances WHERE name = $1`
	_, err := DB.Exec(query, name)
	return err
}

// UpdateInstanceStatusAndLimits updates the limits (including IP addresses) of an instance in the database
func UpdateInstanceStatusAndLimits(name string, limits map[string]string) error {
	// Convert limits map back to JSON
	limitsJSON, err := json.Marshal(limits)
	if err != nil {
		return fmt.Errorf("failed to marshal limits: %w", err)
	}

	query := `
	UPDATE instances
	SET limits = $1
	WHERE name = $2
	`
	_, err = DB.Exec(query, string(limitsJSON), name)
	return err
}

func RecoverStuckJobs() error {
	// Resetar IN_PROGRESS antigos para PENDING
	query := `
	UPDATE jobs
	SET status = $1, attempt_count = attempt_count + 1
	WHERE status = $2
	  AND started_at < NOW() - INTERVAL '5 minutes'
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
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`
	job.CreatedAt = time.Now().UTC()
	job.Status = types.JobPending
	job.AttemptCount = 0

	_, err := DB.Exec(query, job.ID, job.Type, job.Target, job.Payload, job.Status, job.CreatedAt, job.AttemptCount, job.RequestedBy)
	return err
}

func MarkJobStarted(id string) error {
	query := `
	UPDATE jobs
	SET status = $1, started_at = $2, attempt_count = attempt_count + 1
	WHERE id = $3
	`
	_, err := DB.Exec(query, types.JobInProgress, time.Now().UTC(), id)
	return err
}

func MarkJobCompleted(id string) error {
	query := `
	UPDATE jobs
	SET status = $1, finished_at = $2, error = NULL
	WHERE id = $3
	`
	_, err := DB.Exec(query, types.JobCompleted, time.Now().UTC(), id)
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
		SET status = $1, error = $2, finished_at = $3
		WHERE id = $4
		`
		_, err := DB.Exec(query, status, errorMsg, time.Now().UTC(), id)
		return err
	} else {
		query := `
		UPDATE jobs
		SET status = $1, error = $2
		WHERE id = $3
		`
		_, err := DB.Exec(query, status, errorMsg, id)
		return err
	}
}

func GetJob(id string) (*Job, error) {
	query := `SELECT id, type, target, payload, status, error, created_at, started_at, finished_at, attempt_count, requested_by FROM jobs WHERE id = $1`
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
	LIMIT $1
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

func GetInstanceMetrics(instanceName string, interval string) ([]Metric, error) {
	query := `
	SELECT timestamp, cpu_percent, memory_usage, disk_usage
	FROM metrics
	WHERE instance_name = $1 AND timestamp > NOW() - $2::interval
	ORDER BY timestamp ASC
	`

	rows, err := DB.Query(query, instanceName, interval)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var metrics []Metric
	for rows.Next() {
		var m Metric
		if err := rows.Scan(&m.Timestamp, &m.CPUPercent, &m.MemoryUsage, &m.DiskUsage); err != nil {
			return nil, err
		}
		metrics = append(metrics, m)
	}
	return metrics, nil
}

func GetLastBackupJob(instanceName string) (*Job, error) {
	// First try to find by target field
	query := `
	SELECT id, type, target, payload, status, error, created_at, started_at, finished_at, attempt_count, requested_by
	FROM jobs
	WHERE type = $1 AND target = $2
	ORDER BY created_at DESC
	LIMIT 1
	`
	row := DB.QueryRow(query, types.JobTypeCreateSnapshot, instanceName)

	var job Job
	var errStr sql.NullString
	var reqByStr sql.NullString

	err := row.Scan(&job.ID, &job.Type, &job.Target, &job.Payload, &job.Status, &errStr, &job.CreatedAt, &job.StartedAt, &job.FinishedAt, &job.AttemptCount, &reqByStr)
	if err != nil {
		if err == sql.ErrNoRows {
			// If not found by target, try to find by instance name in payload
			query = `
			SELECT id, type, target, payload, status, error, created_at, started_at, finished_at, attempt_count, requested_by
			FROM jobs
			WHERE type = $1 AND payload LIKE $2
			ORDER BY created_at DESC
			LIMIT 1
			`
			row = DB.QueryRow(query, types.JobTypeCreateSnapshot, "%"+instanceName+"%")

			err = row.Scan(&job.ID, &job.Type, &job.Target, &job.Payload, &job.Status, &errStr, &job.CreatedAt, &job.StartedAt, &job.FinishedAt, &job.AttemptCount, &reqByStr)
			if err != nil {
				if err == sql.ErrNoRows {
					return nil, nil // No backup job found
				}
				return nil, err
			}
		} else {
			return nil, err
		}
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

func GetNextRunTime(schedule string) (*time.Time, error) {
	if schedule == "" {
		return nil, nil
	}

	parser := cron.NewParser(cron.SecondOptional | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)
	sched, err := parser.Parse(schedule)
	if err != nil {
		return nil, fmt.Errorf("failed to parse cron schedule: %w", err)
	}

	nextRun := sched.Next(time.Now())
	return &nextRun, nil
}

func GetInstanceWithBackupInfo(name string) (*types.Instance, error) {
	// Get the basic instance info
	instance, err := GetInstance(name)
	if err != nil {
		return nil, err
	}

	// Fix backup retention: if it's 0, set it to the default value of 7
	if instance.BackupRetention == 0 {
		instance.BackupRetention = 7
	}

	// Create and populate the backup info
	backupInfo := &types.InstanceBackupInfo{
		Enabled:  instance.BackupEnabled,
		Schedule: instance.BackupSchedule,
	}

	// Get next run time if backup is enabled
	if instance.BackupEnabled {
		nextRun, err := GetNextRunTime(instance.BackupSchedule)
		if err != nil {
			log.Printf("Error calculating next backup run time for %s: %v", name, err)
		} else {
			backupInfo.NextRun = nextRun
		}
	}

	// Get last backup job info
	lastJob, err := GetLastBackupJob(name)
	if err != nil {
		log.Printf("Error getting last backup job for %s: %v", name, err)
	} else if lastJob != nil {
		backupInfo.LastRun = lastJob.FinishedAt
		// Determine status based on job status
		switch lastJob.Status {
		case types.JobCompleted:
			backupInfo.LastStatus = "completed"
		case types.JobFailed:
			backupInfo.LastStatus = "failed"
		case types.JobCanceled:
			backupInfo.LastStatus = "canceled"
		case types.JobInProgress:
			backupInfo.LastStatus = "in_progress"
		default:
			backupInfo.LastStatus = string(lastJob.Status)
		}
	}

	// Set the backup info in the instance
	instance.BackupInfo = backupInfo

	return instance, nil
}

// GetInstanceWithHardwareInfo gets instance details with additional hardware information from LXD
func GetInstanceWithHardwareInfo(name string, lxdClient *lxc.InstanceService) (*types.Instance, error) {
	// Get the basic instance info
	instance, err := GetInstance(name)
	if err != nil {
		return nil, err
	}

	// Fix backup retention: if it's 0, set it to the default value of 7
	if instance.BackupRetention == 0 {
		instance.BackupRetention = 7
	}

	// Get hardware information from LXD
	inst, _, err := lxdClient.Server().GetInstance(name)
	if err != nil {
		log.Printf("Error getting instance from LXD for hardware info: %v", err)
		// Still return the instance with basic info, but hardware fields will be zero values
	} else {
		// Extract node location (use hostname if empty for single node setup)
		if inst.Location != "" {
			instance.Node = inst.Location
		} else {
			hostname, err := os.Hostname()
			if err != nil {
				instance.Node = "local" // fallback to 'local' if hostname fails
			} else {
				instance.Node = hostname
			}
		}

		// Extract CPU count from limits
		if cpuLimit, ok := inst.Config["limits.cpu"]; ok {
			instance.CPUCount = utils.ParseCpuCores(cpuLimit)
		} else {
			// Default to 1 if not specified
			instance.CPUCount = 1
		}

		// Extract disk limit from ExpandedDevices["root"]["size"] as requested
		if rootDevice, ok := inst.ExpandedDevices["root"]; ok {
			if size, exists := rootDevice["size"]; exists {
				instance.DiskLimit = utils.ParseMemoryToBytes(size)
			}
		} else if rootDevice, ok := inst.Devices["root"]; ok {
			// Fallback to Devices if ExpandedDevices isn't available
			if size, exists := rootDevice["size"]; exists {
				instance.DiskLimit = utils.ParseMemoryToBytes(size)
			}
		}

		// If still no size found, try from limits
		if instance.DiskLimit == 0 {
			if diskLimit, ok := inst.Config["limits.disk"]; ok {
				instance.DiskLimit = utils.ParseMemoryToBytes(diskLimit)
			}
		}

		// Get current disk usage and IP from state
		state, _, stateErr := lxdClient.Server().GetInstanceState(name)
		if stateErr == nil {
			if rootDisk, ok := state.Disk["root"]; ok {
				instance.DiskUsage = rootDisk.Usage
			}

			// Smart IP discovery: iterate through all network interfaces to find IPv4
			for _, networkInfo := range state.Network {
				if networkInfo.Type == "broadcast" { // ignore loopback
					for _, addr := range networkInfo.Addresses {
						if addr.Family == "inet" { // IPv4
							// Store IP in limits if instance has an IP
							if instance.Limits == nil {
								instance.Limits = make(map[string]string)
							}
							instance.Limits["volatile.ip_address"] = addr.Address
							break // Found primary IPv4 address, break inner loop
						}
					}
					if instance.Limits["volatile.ip_address"] != "" {
						break // Found IP, break outer loop
					}
				}
			}
		}
	}

	// Create and populate the backup info
	backupInfo := &types.InstanceBackupInfo{
		Enabled:  instance.BackupEnabled,
		Schedule: instance.BackupSchedule,
	}

	// Get next run time if backup is enabled
	if instance.BackupEnabled {
		nextRun, err := GetNextRunTime(instance.BackupSchedule)
		if err != nil {
			log.Printf("Error calculating next backup run time for %s: %v", name, err)
		} else {
			backupInfo.NextRun = nextRun
		}
	}

	// Get last backup job info
	lastJob, err := GetLastBackupJob(name)
	if err != nil {
		log.Printf("Error getting last backup job for %s: %v", name, err)
	} else if lastJob != nil {
		backupInfo.LastRun = lastJob.FinishedAt
		// Determine status based on job status
		switch lastJob.Status {
		case types.JobCompleted:
			backupInfo.LastStatus = "completed"
		case types.JobFailed:
			backupInfo.LastStatus = "failed"
		case types.JobCanceled:
			backupInfo.LastStatus = "canceled"
		case types.JobInProgress:
			backupInfo.LastStatus = "in_progress"
		default:
			backupInfo.LastStatus = string(lastJob.Status)
		}
	}

	// Set the backup info in the instance
	instance.BackupInfo = backupInfo

	return instance, nil
}
