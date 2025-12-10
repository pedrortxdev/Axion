package scheduler

import (
	"fmt"
	"log"
	"sync"
	"time"

	"aexon/internal/db"
	"aexon/internal/types"
	"aexon/internal/worker"

	"github.com/google/uuid"
	"github.com/robfig/cron/v3"
)

type Schedule struct {
	ID        string        `json:"id"`
	CronExpr  string        `json:"cron_expr"`
	Type      types.JobType `json:"type"`
	Target    string        `json:"target"`
	Payload   string        `json:"payload"`
	CreatedAt time.Time     `json:"created_at"`
}

type Service struct {
	cron    *cron.Cron
	entries map[string]cron.EntryID
	mu      sync.Mutex
}

// Init inicializa o serviço de scheduler, criando a tabela se necessário e carregando agendamentos.
func Init() (*Service, error) {
	// Create table
	query := `
	CREATE TABLE IF NOT EXISTS schedules (
		id TEXT PRIMARY KEY,
		cron_expr TEXT NOT NULL,
		task_type TEXT NOT NULL,
		target TEXT NOT NULL,
		payload TEXT NOT NULL,
		created_at DATETIME NOT NULL DEFAULT (datetime('now'))
	);`
	if _, err := db.DB.Exec(query); err != nil {
		return nil, fmt.Errorf("erro criando tabela schedules: %w", err)
	}

	svc := &Service{
		cron:    cron.New(),
		entries: make(map[string]cron.EntryID),
	}

	// Load existing schedules
	rows, err := db.DB.Query("SELECT id, cron_expr, task_type, target, payload, created_at FROM schedules")
	if err != nil {
		return nil, fmt.Errorf("erro carregando schedules: %w", err)
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var s Schedule
		if err := rows.Scan(&s.ID, &s.CronExpr, &s.Type, &s.Target, &s.Payload, &s.CreatedAt); err != nil {
			log.Printf("[Scheduler] Erro lendo schedule: %v", err)
			continue
		}
		if err := svc.addToCron(&s); err != nil {
			log.Printf("[Scheduler] Erro adicionando schedule %s ao cron: %v", s.ID, err)
		} else {
			count++
		}
	}

	svc.cron.Start()
	log.Printf("[Scheduler] Iniciado com %d agendamentos carregados", count)
	return svc, nil
}

func (s *Service) addToCron(sched *Schedule) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Wrap the execution logic
	entryID, err := s.cron.AddFunc(sched.CronExpr, func() {
		jobID := uuid.New().String()
		log.Printf("[Scheduler] Disparando job agendado %s (Schedule: %s)", jobID, sched.ID)

		job := &db.Job{
			ID:          jobID,
			Type:        sched.Type,
			Target:      sched.Target,
			Payload:     sched.Payload,
			RequestedBy: func() *string { s := "scheduler"; return &s }(),
		}

		if err := db.CreateJob(job); err != nil {
			log.Printf("[Scheduler] Erro criando job %s: %v", jobID, err)
			return
		}
		worker.DispatchJob(jobID)
	})

	if err != nil {
		return err
	}

	s.entries[sched.ID] = entryID
	return nil
}

func (s *Service) AddSchedule(cronExpr string, taskType types.JobType, target, payload string) (*Schedule, error) {
	// Validate cron expression
	if _, err := cron.ParseStandard(cronExpr); err != nil {
		return nil, fmt.Errorf("expressão cron inválida: %w", err)
	}

	id := uuid.New().String()
	sched := &Schedule{
		ID:        id,
		CronExpr:  cronExpr,
		Type:      taskType,
		Target:    target,
		Payload:   payload,
		CreatedAt: time.Now(),
	}

	query := `INSERT INTO schedules (id, cron_expr, task_type, target, payload, created_at) VALUES (?, ?, ?, ?, ?, ?)`
	_, err := db.DB.Exec(query, sched.ID, sched.CronExpr, sched.Type, sched.Target, sched.Payload, sched.CreatedAt)
	if err != nil {
		return nil, err
	}

	if err := s.addToCron(sched); err != nil {
		// Rollback DB if cron add fails
		db.DB.Exec("DELETE FROM schedules WHERE id = ?", sched.ID)
		return nil, fmt.Errorf("erro registrando no cron: %w", err)
	}

	return sched, nil
}

func (s *Service) RemoveSchedule(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	entryID, exists := s.entries[id]
	if !exists {
		// Check if it exists in DB but was missed in memory (shouldn't happen but for safety)
		var dbID string
		err := db.DB.QueryRow("SELECT id FROM schedules WHERE id = ?", id).Scan(&dbID)
		if err == nil {
			// Exist in DB, remove from DB only
			_, _ = db.DB.Exec("DELETE FROM schedules WHERE id = ?", id)
			return nil
		}
		return fmt.Errorf("agendamento não encontrado")
	}

	// Remove from DB
	if _, err := db.DB.Exec("DELETE FROM schedules WHERE id = ?", id); err != nil {
		return err
	}

	// Remove from Cron
	s.cron.Remove(entryID)
	delete(s.entries, id)
	return nil
}

func (s *Service) ListSchedules() ([]Schedule, error) {
	rows, err := db.DB.Query("SELECT id, cron_expr, task_type, target, payload, created_at FROM schedules ORDER BY created_at DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var schedules []Schedule
	for rows.Next() {
		var s Schedule
		if err := rows.Scan(&s.ID, &s.CronExpr, &s.Type, &s.Target, &s.Payload, &s.CreatedAt); err != nil {
			return nil, err
		}
		schedules = append(schedules, s)
	}
	return schedules, nil
}
