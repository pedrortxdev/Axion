package types

import (
	"time"
)

type InstanceBackupInfo struct {
	Enabled    bool      `json:"enabled"`
	Schedule   string    `json:"schedule"`
	NextRun    *time.Time `json:"next_run,omitempty"`
	LastRun    *time.Time `json:"last_run,omitempty"`
	LastStatus string    `json:"last_status,omitempty"` // "completed", "failed"
}

type Instance struct {
	Name            string             `json:"name"`
	Image           string             `json:"image"`
	Limits          map[string]string  `json:"limits"`
	UserData        string             `json:"user_data"`
	Type            string             `json:"type"`
	BackupSchedule  string             `json:"backup_schedule"`
	BackupRetention int                `json:"backup_retention"`
	BackupEnabled   bool               `json:"backup_enabled"`
	BackupInfo      *InstanceBackupInfo `json:"backup_info,omitempty"`
}
