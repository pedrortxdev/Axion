package scheduler

import (
	"database/sql"
	"log"
	"sort"
	"strings"
	"time"
	"aexon/internal/db"
	"aexon/internal/provider/lxc"
	"aexon/internal/types"
	"github.com/robfig/cron/v3"
	"github.com/canonical/lxd/shared/api"
)

type BackupScheduler struct {
	cron      *cron.Cron
	db        *sql.DB
	lxcClient *lxc.InstanceService
}

func NewBackupScheduler(db *sql.DB, lxcClient *lxc.InstanceService) *BackupScheduler {
	return &BackupScheduler{
		cron:      cron.New(),
		db:        db,
		lxcClient: lxcClient,
	}
}

func (s *BackupScheduler) Start() {
	s.cron.Start()
}

func (s *BackupScheduler) Stop() {
	s.cron.Stop()
}

func (s *BackupScheduler) SyncJobs() {
	log.Println("Syncing backup jobs...")
	s.cron.Stop()
	for _, entry := range s.cron.Entries() {
		s.cron.Remove(entry.ID)
	}

	instances, err := db.ListInstances()
	if err != nil {
		log.Printf("Error listing instances for backup scheduling: %v", err)
		return
	}

	for _, instance := range instances {
		if instance.BackupEnabled {
			s.AddInstanceJob(instance)
		}
	}
	s.cron.Start()
}

func (s *BackupScheduler) AddInstanceJob(instance types.Instance) {
	log.Printf("Scheduling backup for instance %s with schedule %s", instance.Name, instance.BackupSchedule)
	_, err := s.cron.AddFunc(instance.BackupSchedule, func() {
		log.Printf("Running backup for instance %s", instance.Name)
		snapshotName := "auto-backup-" + time.Now().UTC().Format("2006-01-02-15-04-05")
		if err := s.lxcClient.CreateSnapshot(instance.Name, snapshotName); err != nil {
			log.Printf("Error creating snapshot for instance %s: %v", instance.Name, err)
			return
		}

		snapshots, err := s.lxcClient.ListSnapshots(instance.Name)
		if err != nil {
			log.Printf("Error listing snapshots for instance %s: %v", instance.Name, err)
			return
		}

		var autoBackups []api.InstanceSnapshot
		for _, snap := range snapshots {
			if strings.HasPrefix(snap.Name, "auto-backup-") {
				autoBackups = append(autoBackups, snap)
			}
		}

		if len(autoBackups) > instance.BackupRetention {
			sort.Slice(autoBackups, func(i, j int) bool {
				return autoBackups[i].CreatedAt.Before(autoBackups[j].CreatedAt)
			})

			for i := 0; i < len(autoBackups)-instance.BackupRetention; i++ {
				log.Printf("Deleting old backup %s for instance %s", autoBackups[i].Name, instance.Name)
				if err := s.lxcClient.DeleteSnapshot(instance.Name, autoBackups[i].Name); err != nil {
					log.Printf("Error deleting snapshot %s for instance %s: %v", autoBackups[i].Name, instance.Name, err)
				}
			}
		}
	})
	if err != nil {
		log.Printf("Error scheduling backup for instance %s: %v", instance.Name, err)
	}
}

func (s *BackupScheduler) ReloadInstance(name string) {
	log.Printf("Reloading backup job for instance %s", name)
	for _, entry := range s.cron.Entries() {
		s.cron.Remove(entry.ID)
	}
	s.SyncJobs()
}