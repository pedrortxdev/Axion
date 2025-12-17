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
	Node            string             `json:"node"`        // Ex: "pve-01" ou "lxd-node-1"
	CPUCount        int                `json:"cpu_count"`   // Quantidade de vCPUs
	DiskUsage       int64              `json:"disk_usage"`  // Bytes usados
	DiskLimit       int64              `json:"disk_limit"`  // Bytes totais (tamanho do disco)
}
