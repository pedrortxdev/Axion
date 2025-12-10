package monitor

import (
	"fmt"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/net"
)

// HostStats holds the host system resource statistics
type HostStats struct {
	CPUPercent    float64 `json:"cpu_percent"`
	MemoryUsedMB  uint64  `json:"memory_used_mb"`
	MemoryTotalMB uint64  `json:"memory_total_mb"`
	DiskUsedGB    uint64  `json:"disk_used_gb"`
	DiskTotalGB   uint64  `json:"disk_total_gb"`
	NetRxKB       uint64  `json:"net_rx_kb"`
	NetTxKB       uint64  `json:"net_tx_kb"`
}

// netStats stores network statistics for calculating deltas
type netStats struct {
	rxBytes uint64
	txBytes uint64
	time    time.Time
}

// Global variable to store the last network stats for calculating deltas
var lastNetStats netStats
var netStatsMutex sync.RWMutex

// GetHostStats retrieves the current host system resource statistics
func GetHostStats() (*HostStats, error) {
	stats := &HostStats{}

	// Get CPU usage percentage
	cpuPercent, err := cpu.Percent(0, false)
	if err != nil {
		return nil, fmt.Errorf("failed to get CPU stats: %w", err)
	}
	if len(cpuPercent) > 0 {
		stats.CPUPercent = cpuPercent[0]
	}

	// Get memory usage
	memInfo, err := mem.VirtualMemory()
	if err != nil {
		return nil, fmt.Errorf("failed to get memory stats: %w", err)
	}
	stats.MemoryUsedMB = memInfo.Used / 1024 / 1024 // Convert bytes to MB
	stats.MemoryTotalMB = memInfo.Total / 1024 / 1024

	// Get disk usage for root path
	diskUsage, err := disk.Usage("/")
	if err != nil {
		return nil, fmt.Errorf("failed to get disk stats: %w", err)
	}
	stats.DiskUsedGB = diskUsage.Used / 1024 / 1024 / 1024 // Convert bytes to GB
	stats.DiskTotalGB = diskUsage.Total / 1024 / 1024 / 1024

	// Get network I/O stats
	netIO, err := net.IOCounters(false)
	if err != nil {
		return nil, fmt.Errorf("failed to get network stats: %w", err)
	}

	var currentRx, currentTx uint64
	if len(netIO) > 0 {
		currentRx = netIO[0].BytesRecv
		currentTx = netIO[0].BytesSent
	}

	// Calculate network deltas if we have previous stats
	netStatsMutex.Lock()
	defer netStatsMutex.Unlock()

	now := time.Now()
	if lastNetStats.time.IsZero() {
		// First call, just store the values
		lastNetStats.rxBytes = currentRx
		lastNetStats.txBytes = currentTx
		lastNetStats.time = now
		stats.NetRxKB = 0
		stats.NetTxKB = 0
	} else {
		// Calculate the time difference in seconds
		timeDiff := now.Sub(lastNetStats.time).Seconds()
		if timeDiff > 0 {
			rxDiff := currentRx - lastNetStats.rxBytes
			txDiff := currentTx - lastNetStats.txBytes

			// Convert bytes to KB and calculate rate per second
			stats.NetRxKB = uint64(float64(rxDiff) / 1024 / timeDiff)
			stats.NetTxKB = uint64(float64(txDiff) / 1024 / timeDiff)
		} else {
			stats.NetRxKB = 0
			stats.NetTxKB = 0
		}

		// Update the stored values for next calculation
		lastNetStats.rxBytes = currentRx
		lastNetStats.txBytes = currentTx
		lastNetStats.time = now
	}

	return stats, nil
}
