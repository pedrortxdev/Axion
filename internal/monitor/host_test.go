package monitor

import (
	"fmt"
	"testing"
	"time"
)

func TestGetHostStats(t *testing.T) {
	// Test the first call to GetHostStats
	stats, err := GetHostStats()
	if err != nil {
		t.Fatalf("Failed to get host stats: %v", err)
	}

	// Verify that the stats are populated with reasonable values
	if stats.CPUPercent < 0 || stats.CPUPercent > 100 {
		t.Errorf("CPU percent out of range: %f", stats.CPUPercent)
	}

	if stats.MemoryTotalMB == 0 {
		t.Error("MemoryTotalMB should not be zero")
	}

	if stats.MemoryUsedMB > stats.MemoryTotalMB {
		t.Error("MemoryUsedMB should not exceed MemoryTotalMB")
	}

	if stats.DiskTotalGB == 0 {
		t.Error("DiskTotalGB should not be zero")
	}

	if stats.DiskUsedGB > stats.DiskTotalGB {
		t.Error("DiskUsedGB should not exceed DiskTotalGB")
	}

	// Print the stats for verification
	fmt.Printf("Host Stats: %+v\n", stats)

	// Wait a bit and call again to test network rate calculation
	time.Sleep(2 * time.Second)

	stats2, err := GetHostStats()
	if err != nil {
		t.Fatalf("Failed to get host stats on second call: %v", err)
	}

	// Verify that the second call also works
	if stats2.CPUPercent < 0 || stats2.CPUPercent > 100 {
		t.Errorf("CPU percent out of range on second call: %f", stats2.CPUPercent)
	}

	fmt.Printf("Second call Host Stats: %+v\n", stats2)
}
