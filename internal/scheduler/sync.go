package scheduler

import (
	"database/sql"
	"log"

	"aexon/internal/db"
	"aexon/internal/provider/lxc"
	"aexon/internal/types"
)

// RunStartupSync synchronizes instances from the LXD provider to the database.
func RunStartupSync(dbConn *sql.DB, lxd *lxc.InstanceService) {
	log.Println("[Sync] Starting LXD to DB synchronization...")

	lxdInstances, err := lxd.ListInstances()
	if err != nil {
		log.Printf("[Sync] ERROR: Failed to list instances from LXD: %v", err)
		return
	}

	for _, lxdInstance := range lxdInstances {
		_, err := db.GetInstance(lxdInstance.Name)
		if err != nil {
			if err == sql.ErrNoRows {
				// Instance does not exist in DB, let's import it.
				log.Printf("[Sync] Importing new instance '%s' from LXD to database...", lxdInstance.Name)

				newInstance := &types.Instance{
					Name:            lxdInstance.Name,
					Image:           lxdInstance.Config["volatile.base_image"],
					Limits:          lxdInstance.Config,
					Type:            lxdInstance.Type,
					BackupSchedule:  "@daily", // Default value
					BackupRetention: 7,        // Default value
					BackupEnabled:   false,    // Default value
				}

				if err := db.CreateInstance(newInstance); err != nil {
					log.Printf("[Sync] ERROR: Failed to import instance '%s': %v", lxdInstance.Name, err)
				} else {
					log.Printf("[Sync] Imported instance '%s' successfully.", lxdInstance.Name)
				}
			} else {
				log.Printf("[Sync] ERROR: Failed to query instance '%s' from DB: %v", lxdInstance.Name, err)
			}
		}
	}
	log.Println("[Sync] Synchronization finished.")
}
