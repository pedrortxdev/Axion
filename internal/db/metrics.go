// database/metrics.go
package db

import (
	"context"
	"database/sql"
	"log"
	"time"
)

// ============================================================================
// METRIC TYPES
// ============================================================================

type Metric struct {
	ID           int       `json:"id"`
	InstanceName string    `json:"instance_name"`
	Timestamp    time.Time `json:"timestamp"`
	CPUPercent   float64   `json:"cpu_percent"`
	MemoryUsage  int64     `json:"memory_usage"`
	DiskUsage    int64     `json:"disk_usage"`
}

type AggregatedMetric struct {
	Timestamp        time.Time `json:"timestamp"`
	AvgCPUPercent    float64   `json:"avg_cpu_percent"`
	MaxCPUPercent    float64   `json:"max_cpu_percent"`
	AvgMemoryUsage   int64     `json:"avg_memory_usage"`
	MaxMemoryUsage   int64     `json:"max_memory_usage"`
	AvgDiskUsage     int64     `json:"avg_disk_usage"`
	MaxDiskUsage     int64     `json:"max_disk_usage"`
	SampleCount      int       `json:"sample_count"`
}

// ============================================================================
// METRICS REPOSITORY
// ============================================================================

type MetricsRepository struct {
	db *Service
}

func NewMetricsRepository(db *Service) *MetricsRepository {
	return &MetricsRepository{db: db}
}

// ============================================================================
// INSERT OPERATIONS
// ============================================================================

func (r *MetricsRepository) Insert(ctx context.Context, metric *Metric) error {
	query := `
		INSERT INTO metrics (
			instance_name, timestamp,
			cpu_percent, memory_usage, disk_usage
		) VALUES ($1, $2, $3, $4, $5)
	`

	// Always use UTC for timestamps
	timestamp := time.Now().UTC()
	if !metric.Timestamp.IsZero() {
		timestamp = metric.Timestamp.UTC()
	}

	_, err := r.db.ExecContext(ctx, query,
		metric.InstanceName,
		timestamp,
		metric.CPUPercent,
		metric.MemoryUsage,
		metric.DiskUsage,
	)

	return err
}

func (r *MetricsRepository) InsertBatch(ctx context.Context, metrics []Metric) error {
	if len(metrics) == 0 {
		return nil
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	query := `
		INSERT INTO metrics (
			instance_name, timestamp,
			cpu_percent, memory_usage, disk_usage
		) VALUES ($1, $2, $3, $4, $5)
	`

	for _, metric := range metrics {
		timestamp := time.Now().UTC()
		if !metric.Timestamp.IsZero() {
			timestamp = metric.Timestamp.UTC()
		}

		_, err := tx.ExecContext(ctx, query,
			metric.InstanceName,
			timestamp,
			metric.CPUPercent,
			metric.MemoryUsage,
			metric.DiskUsage,
		)

		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

// ============================================================================
// QUERY OPERATIONS
// ============================================================================

func (r *MetricsRepository) GetByInstance(ctx context.Context, instanceName string, interval string) ([]Metric, error) {
	query := `
		SELECT id, instance_name, timestamp,
		       cpu_percent, memory_usage, disk_usage
		FROM metrics
		WHERE instance_name = $1
		  AND timestamp > NOW() - $2::interval
		ORDER BY timestamp ASC
	`

	rows, err := r.db.QueryContext(ctx, query, instanceName, interval)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var metrics []Metric

	for rows.Next() {
		var m Metric

		err := rows.Scan(
			&m.ID,
			&m.InstanceName,
			&m.Timestamp,
			&m.CPUPercent,
			&m.MemoryUsage,
			&m.DiskUsage,
		)

		if err != nil {
			return nil, err
		}

		metrics = append(metrics, m)
	}

	return metrics, rows.Err()
}

func (r *MetricsRepository) GetByTimeRange(ctx context.Context, instanceName string, start, end time.Time) ([]Metric, error) {
	query := `
		SELECT id, instance_name, timestamp,
		       cpu_percent, memory_usage, disk_usage
		FROM metrics
		WHERE instance_name = $1
		  AND timestamp BETWEEN $2 AND $3
		ORDER BY timestamp ASC
	`

	rows, err := r.db.QueryContext(ctx, query, instanceName, start.UTC(), end.UTC())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var metrics []Metric

	for rows.Next() {
		var m Metric

		err := rows.Scan(
			&m.ID,
			&m.InstanceName,
			&m.Timestamp,
			&m.CPUPercent,
			&m.MemoryUsage,
			&m.DiskUsage,
		)

		if err != nil {
			return nil, err
		}

		metrics = append(metrics, m)
	}

	return metrics, rows.Err()
}

func (r *MetricsRepository) GetLatest(ctx context.Context, instanceName string, limit int) ([]Metric, error) {
	query := `
		SELECT id, instance_name, timestamp,
		       cpu_percent, memory_usage, disk_usage
		FROM metrics
		WHERE instance_name = $1
		ORDER BY timestamp DESC
		LIMIT $2
	`

	rows, err := r.db.QueryContext(ctx, query, instanceName, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var metrics []Metric

	for rows.Next() {
		var m Metric

		err := rows.Scan(
			&m.ID,
			&m.InstanceName,
			&m.Timestamp,
			&m.CPUPercent,
			&m.MemoryUsage,
			&m.DiskUsage,
		)

		if err != nil {
			return nil, err
		}

		metrics = append(metrics, m)
	}

	// Reverse to get chronological order
	for i := 0; i < len(metrics)/2; i++ {
		j := len(metrics) - 1 - i
		metrics[i], metrics[j] = metrics[j], metrics[i]
	}

	return metrics, rows.Err()
}

// ============================================================================
// AGGREGATION OPERATIONS
// ============================================================================

func (r *MetricsRepository) GetAggregatedByInterval(
	ctx context.Context,
	instanceName string,
	interval string,
	bucketSize string,
) ([]AggregatedMetric, error) {
	query := `
		SELECT 
			time_bucket($1::interval, timestamp) AS bucket,
			AVG(cpu_percent) AS avg_cpu,
			MAX(cpu_percent) AS max_cpu,
			AVG(memory_usage) AS avg_memory,
			MAX(memory_usage) AS max_memory,
			AVG(disk_usage) AS avg_disk,
			MAX(disk_usage) AS max_disk,
			COUNT(*) AS sample_count
		FROM metrics
		WHERE instance_name = $2
		  AND timestamp > NOW() - $3::interval
		GROUP BY bucket
		ORDER BY bucket ASC
	`

	rows, err := r.db.QueryContext(ctx, query, bucketSize, instanceName, interval)
	if err != nil {
		// Fallback to simple query if time_bucket is not available
		return r.getAggregatedFallback(ctx, instanceName, interval)
	}
	defer rows.Close()

	var metrics []AggregatedMetric

	for rows.Next() {
		var m AggregatedMetric

		err := rows.Scan(
			&m.Timestamp,
			&m.AvgCPUPercent,
			&m.MaxCPUPercent,
			&m.AvgMemoryUsage,
			&m.MaxMemoryUsage,
			&m.AvgDiskUsage,
			&m.MaxDiskUsage,
			&m.SampleCount,
		)

		if err != nil {
			return nil, err
		}

		metrics = append(metrics, m)
	}

	return metrics, rows.Err()
}

func (r *MetricsRepository) getAggregatedFallback(ctx context.Context, instanceName string, interval string) ([]AggregatedMetric, error) {
	query := `
		SELECT 
			AVG(cpu_percent) AS avg_cpu,
			MAX(cpu_percent) AS max_cpu,
			AVG(memory_usage) AS avg_memory,
			MAX(memory_usage) AS max_memory,
			AVG(disk_usage) AS avg_disk,
			MAX(disk_usage) AS max_disk,
			COUNT(*) AS sample_count
		FROM metrics
		WHERE instance_name = $1
		  AND timestamp > NOW() - $2::interval
	`

	row := r.db.QueryRowContext(ctx, query, instanceName, interval)

	var m AggregatedMetric
	m.Timestamp = time.Now().UTC()

	err := row.Scan(
		&m.AvgCPUPercent,
		&m.MaxCPUPercent,
		&m.AvgMemoryUsage,
		&m.MaxMemoryUsage,
		&m.AvgDiskUsage,
		&m.MaxDiskUsage,
		&m.SampleCount,
	)

	if err != nil {
		return nil, err
	}

	return []AggregatedMetric{m}, nil
}

func (r *MetricsRepository) GetPeakUsage(ctx context.Context, instanceName string, interval string) (*Metric, error) {
	query := `
		SELECT id, instance_name, timestamp,
		       cpu_percent, memory_usage, disk_usage
		FROM metrics
		WHERE instance_name = $1
		  AND timestamp > NOW() - $2::interval
		ORDER BY cpu_percent DESC, memory_usage DESC
		LIMIT 1
	`

	row := r.db.QueryRowContext(ctx, query, instanceName, interval)

	var m Metric

	err := row.Scan(
		&m.ID,
		&m.InstanceName,
		&m.Timestamp,
		&m.CPUPercent,
		&m.MemoryUsage,
		&m.DiskUsage,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	return &m, nil
}

// ============================================================================
// CLEANUP OPERATIONS
// ============================================================================

func (r *MetricsRepository) DeleteOlderThan(ctx context.Context, age time.Duration) (int, error) {
	query := `
		DELETE FROM metrics
		WHERE timestamp < $1
		RETURNING id
	`

	cutoff := time.Now().UTC().Add(-age)

	rows, err := r.db.QueryContext(ctx, query, cutoff)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return count, err
		}
		count++
	}

	if count > 0 {
		log.Printf("[Metrics] Deleted %d old metrics (older than %v)", count, age)
	}

	return count, rows.Err()
}

func (r *MetricsRepository) DeleteByInstance(ctx context.Context, instanceName string) (int, error) {
	query := `DELETE FROM metrics WHERE instance_name = $1 RETURNING id`

	rows, err := r.db.QueryContext(ctx, query, instanceName)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return count, err
		}
		count++
	}

	return count, rows.Err()
}

// ============================================================================
// STATISTICS
// ============================================================================

func (r *MetricsRepository) GetStatistics(ctx context.Context) (map[string]interface{}, error) {
	query := `
		SELECT 
			COUNT(*) AS total_metrics,
			COUNT(DISTINCT instance_name) AS instance_count,
			MIN(timestamp) AS oldest_metric,
			MAX(timestamp) AS newest_metric
		FROM metrics
	`

	row := r.db.QueryRowContext(ctx, query)

	var totalMetrics int
	var instanceCount int
	var oldestMetric, newestMetric time.Time

	err := row.Scan(&totalMetrics, &instanceCount, &oldestMetric, &newestMetric)
	if err != nil {
		return nil, err
	}

	stats := map[string]interface{}{
		"total_metrics":  totalMetrics,
		"instance_count": instanceCount,
		"oldest_metric":  oldestMetric,
		"newest_metric":  newestMetric,
		"time_range":     newestMetric.Sub(oldestMetric).String(),
	}

	return stats, nil
}

func (r *MetricsRepository) GetInstanceMetricCounts(ctx context.Context) (map[string]int, error) {
	query := `
		SELECT instance_name, COUNT(*) AS count
		FROM metrics
		GROUP BY instance_name
		ORDER BY count DESC
	`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	counts := make(map[string]int)

	for rows.Next() {
		var instanceName string
		var count int

		if err := rows.Scan(&instanceName, &count); err != nil {
			return nil, err
		}

		counts[instanceName] = count
	}

	return counts, rows.Err()
}

// ============================================================================
// DOWNSAMPLING (for long-term storage optimization)
// ============================================================================

func (r *MetricsRepository) DownsampleOldMetrics(ctx context.Context, olderThan time.Duration, sampleInterval time.Duration) (int, error) {
	// This creates hourly averages for old data
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	cutoff := time.Now().UTC().Add(-olderThan)

	// Insert downsampled data into a temporary aggregation table
	query := `
		CREATE TEMP TABLE IF NOT EXISTS metrics_downsampled AS
		SELECT 
			instance_name,
			date_trunc('hour', timestamp) AS hour,
			AVG(cpu_percent) AS avg_cpu,
			AVG(memory_usage) AS avg_memory,
			AVG(disk_usage) AS avg_disk
		FROM metrics
		WHERE timestamp < $1
		GROUP BY instance_name, hour
	`

	_, err = tx.ExecContext(ctx, query, cutoff)
	if err != nil {
		return 0, err
	}

	// Delete old raw metrics
	deleteQuery := `DELETE FROM metrics WHERE timestamp < $1 RETURNING id`
	rows, err := tx.QueryContext(ctx, deleteQuery, cutoff)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return count, err
		}
		count++
	}

	if err := tx.Commit(); err != nil {
		return count, err
	}

	log.Printf("[Metrics] Downsampled %d old metrics (older than %v)", count, olderThan)
	return count, nil
}

// ============================================================================
// COMPATIBILITY FUNCTIONS
// ============================================================================

func InsertMetric(m *Metric) error {
	ctx := context.Background()
	repo := NewMetricsRepository(GetService())
	return repo.Insert(ctx, m)
}

func GetInstanceMetrics(instanceName string, interval string) ([]Metric, error) {
	ctx := context.Background()
	repo := NewMetricsRepository(GetService())
	return repo.GetByInstance(ctx, instanceName, interval)
}