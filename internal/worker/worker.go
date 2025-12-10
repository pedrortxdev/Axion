package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"time"

	"aexon/internal/db"
	"aexon/internal/events"
	"aexon/internal/provider/lxc"
	"aexon/internal/types"
)

var JobQueue chan string

// Timeout aumentado para suportar criações (download de imagem)
const JobTimeout = 5 * time.Minute

func Init(numWorkers int, lxcClient *lxc.InstanceService) {
	JobQueue = make(chan string, 100)

	if err := db.RecoverStuckJobs(); err != nil {
		log.Printf("[Worker System] Erro ao recuperar jobs: %v", err)
	}

	for i := 0; i < numWorkers; i++ {
		go worker(i, lxcClient)
	}
	log.Printf("[Worker System] Iniciados %d workers", numWorkers)
}

func DispatchJob(jobID string) {
	JobQueue <- jobID
}

func worker(id int, lxcClient *lxc.InstanceService) {
	log.Printf("[Worker %d] Pronto", id)
	for jobID := range JobQueue {
		processJob(id, jobID, lxcClient)
	}
}

func processJob(workerID int, jobID string, lxcClient *lxc.InstanceService) {
	if err := db.MarkJobStarted(jobID); err != nil {
		log.Printf("[Worker %d] Erro ao iniciar job %s: %v", workerID, jobID, err)
		return
	}

	job, err := db.GetJob(jobID)
	if err != nil {
		log.Printf("[Worker %d] Erro ao ler job %s: %v", workerID, jobID, err)
		return
	}

	events.Publish(events.Event{
		Type:      events.JobUpdate,
		JobID:     job.ID,
		Target:    job.Target,
		Payload:   job,
		Timestamp: time.Now().Unix(),
	})

	log.Printf("[Worker %d] Executando Job %s (%s em %s) - Tentativa %d/%d",
		workerID, job.ID, job.Type, job.Target, job.AttemptCount, types.MaxRetries)

	ctx, cancel := context.WithTimeout(context.Background(), JobTimeout)
	defer cancel()

	execErr := executeLogic(ctx, job, lxcClient)

	if execErr != nil {
		log.Printf("[Worker %d] Job %s FALHOU: %v", workerID, job.ID, execErr)

		isFatal := job.AttemptCount >= types.MaxRetries
		if err := db.MarkJobFailed(job.ID, execErr.Error(), isFatal); err != nil {
			log.Printf("[Worker %d] Erro ao atualizar status de falha: %v", workerID, err)
		}

		updatedJob, _ := db.GetJob(jobID)
		events.Publish(events.Event{
			Type:      events.JobUpdate,
			JobID:     updatedJob.ID,
			Target:    updatedJob.Target,
			Payload:   updatedJob,
			Timestamp: time.Now().Unix(),
		})

		if !isFatal {
			backoffSeconds := float64(types.BaseDelay) * math.Pow(2, float64(job.AttemptCount-1))
			delay := time.Duration(backoffSeconds) * time.Second

			log.Printf("[Worker %d] Agendando retry para job %s em %v", workerID, job.ID, delay)

			go func() {
				time.Sleep(delay)
				JobQueue <- jobID
			}()
		}

	} else {
		log.Printf("[Worker %d] Job %s CONCLUÍDO", workerID, job.ID)
		if err := db.MarkJobCompleted(job.ID); err != nil {
			log.Printf("[Worker %d] Erro ao concluir job: %v", workerID, err)
		}

		updatedJob, _ := db.GetJob(jobID)
		events.Publish(events.Event{
			Type:      events.JobUpdate,
			JobID:     updatedJob.ID,
			Target:    updatedJob.Target,
			Payload:   updatedJob,
			Timestamp: time.Now().Unix(),
		})
	}
}

func executeLogic(ctx context.Context, job *db.Job, lxcClient *lxc.InstanceService) error {
	errChan := make(chan error, 1)

	go func() {
		var err error
		switch job.Type {
		case types.JobTypeStateChange:
			var payload struct {
				Action string `json:"action"`
			}
			if e := json.Unmarshal([]byte(job.Payload), &payload); e != nil {
				err = fmt.Errorf("payload inválido: %v", e)
			} else {
				err = lxcClient.UpdateInstanceState(job.Target, payload.Action)
			}

		case types.JobTypeUpdateLimits:
			var payload struct {
				Memory string `json:"memory"`
				CPU    string `json:"cpu"`
			}
			if e := json.Unmarshal([]byte(job.Payload), &payload); e != nil {
				err = fmt.Errorf("payload inválido: %v", e)
			} else {
				err = lxcClient.UpdateInstanceLimits(job.Target, payload.Memory, payload.CPU)
			}

		case types.JobTypeCreateInstance:
			var payload struct {
				Name     string            `json:"name"`
				Image    string            `json:"image"`
				Limits   map[string]string `json:"limits"`
				UserData string            `json:"user_data"` // Adicionado suporte a user_data
				Type     string            `json:"type"`      // Instance type: "container" or "virtual-machine"
			}
			if e := json.Unmarshal([]byte(job.Payload), &payload); e != nil {
				err = fmt.Errorf("payload inválido: %v", e)
			} else {
				// If Type is empty, default to "container"
				instanceType := payload.Type
				if instanceType == "" {
					instanceType = "container"
				}
				err = lxcClient.CreateInstance(payload.Name, payload.Image, instanceType, payload.Limits, payload.UserData)
			}

		case types.JobTypeDeleteInstance:
			err = lxcClient.DeleteInstance(job.Target)

		// --- Snapshot Operations ---
		case types.JobTypeCreateSnapshot:
			var payload struct {
				SnapshotName string `json:"snapshot_name"`
			}
			if e := json.Unmarshal([]byte(job.Payload), &payload); e != nil {
				err = fmt.Errorf("payload inválido: %v", e)
			} else {
				err = lxcClient.CreateSnapshot(job.Target, payload.SnapshotName)
			}

		case types.JobTypeRestoreSnapshot:
			var payload struct {
				SnapshotName string `json:"snapshot_name"`
			}
			if e := json.Unmarshal([]byte(job.Payload), &payload); e != nil {
				err = fmt.Errorf("payload inválido: %v", e)
			} else {
				err = lxcClient.RestoreSnapshot(job.Target, payload.SnapshotName)
			}

		case types.JobTypeDeleteSnapshot:
			var payload struct {
				SnapshotName string `json:"snapshot_name"`
			}
			if e := json.Unmarshal([]byte(job.Payload), &payload); e != nil {
				err = fmt.Errorf("payload inválido: %v", e)
			} else {
				err = lxcClient.DeleteSnapshot(job.Target, payload.SnapshotName)
			}

		// --- Port Forwarding ---
		case types.JobTypeAddPort:
			var payload struct {
				HostPort      int    `json:"host_port"`
				ContainerPort int    `json:"container_port"`
				Protocol      string `json:"protocol"`
			}
			if e := json.Unmarshal([]byte(job.Payload), &payload); e != nil {
				err = fmt.Errorf("payload inválido: %v", e)
			} else {
				err = lxcClient.AddProxyDevice(job.Target, payload.HostPort, payload.ContainerPort, payload.Protocol)
			}

		case types.JobTypeRemovePort:
			var payload struct {
				HostPort int `json:"host_port"`
			}
			if e := json.Unmarshal([]byte(job.Payload), &payload); e != nil {
				err = fmt.Errorf("payload inválido: %v", e)
			} else {
				err = lxcClient.RemoveProxyDevice(job.Target, payload.HostPort)
			}

		default:
			err = fmt.Errorf("tipo desconhecido: %s", job.Type)
		}
		errChan <- err
	}()

	select {
	case err := <-errChan:
		return err
	case <-ctx.Done():
		return fmt.Errorf("timeout de execução (%s)", JobTimeout)
	}
}
