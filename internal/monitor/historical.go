package monitor

import (
	"database/sql"
	"fmt"
	"log"
	"strings"
	"time"

	"aexon/internal/provider/lxc"
)

// StartHistoricalCollector starts a ticker to collect and store instance metrics periodically.
func StartHistoricalCollector(dbConn *sql.DB, lxd *lxc.InstanceService) {
	log.Println("[Metrics] Starting historical metrics collector...")

	metricsTicker := time.NewTicker(1 * time.Minute)
	defer metricsTicker.Stop()

	retentionTicker := time.NewTicker(1 * time.Hour)
	defer retentionTicker.Stop()

	for {
		select {
		case <-metricsTicker.C:
			collectAndStoreMetrics(dbConn, lxd)
		case <-retentionTicker.C:
			applyRetentionPolicy(dbConn)
		}
	}
}

func collectAndStoreMetrics(dbConn *sql.DB, lxd *lxc.InstanceService) {
	instances, err := lxd.ListInstances()
	if err != nil {
		log.Printf("[Metrics] ERROR: Failed to list instances for metrics collection: %v", err)
		return
	}

	runningInstances := []lxc.InstanceMetric{}
	for _, inst := range instances {
		if inst.Status == "Running" {
			runningInstances = append(runningInstances, inst)
		}
	}

	if len(runningInstances) == 0 {
		return
	}

	// Bulk insert implementation
	valueStrings := make([]string, 0, len(runningInstances))
	valueArgs := make([]interface{}, 0, len(runningInstances)*4)
	i := 1
	for _, inst := range runningInstances {
		valueStrings = append(valueStrings, fmt.Sprintf("($%d, $%d, $%d, $%d)", i, i+1, i+2, i+3))
		// Note: CPU usage is cumulative seconds. To get a percentage, you'd need to compare deltas.
		// For simplicity here, we're storing a raw value that could represent load or usage over time.
		// A more advanced implementation would calculate the delta since the last collection.
		valueArgs = append(valueArgs, inst.Name, float64(inst.CPUUsageSeconds), inst.MemoryUsageBytes, inst.DiskUsageBytes)
		i += 4
	}

	stmt := fmt.Sprintf("INSERT INTO metrics (instance_name, cpu_percent, memory_usage, disk_usage) VALUES %s",
		strings.Join(valueStrings, ","))

	_, err = dbConn.Exec(stmt, valueArgs...)
	if err != nil {
		log.Printf("[Metrics] ERROR: Failed to bulk insert metrics: %v", err)
	} else {
		log.Printf("[Metrics] Stored metrics for %d running instances.", len(runningInstances))
	}
}

func applyRetentionPolicy(dbConn *sql.DB) {
	log.Println("[Metrics] Applying retention policy...")
	_, err := dbConn.Exec("DELETE FROM metrics WHERE timestamp < NOW() - INTERVAL '30 days'")
	if err != nil {
		log.Printf("[Metrics] ERROR: Failed to apply retention policy: %v", err)
	} else {
		log.Println("[Metrics] Retention policy applied successfully.")
	}
}
