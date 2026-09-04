package scheduler

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/robfig/cron/v3"

	"paperMC_backend/internal/backup"
	"paperMC_backend/internal/database"
	"paperMC_backend/internal/minecraft"
)

// ServerController defines the server interface needed by the Scheduler service.
type ServerController interface {
	GetStatus() minecraft.Status
	SendCommand(cmd string) error
	Broadcast(msg string)
	Start() error
	Stop() error
}

// Service manages cron scheduling, task execution, and run log persistence.
type Service struct {
	store    database.Store
	server   ServerController
	workDir  string
	cron     *cron.Cron
	entryIDs map[int]cron.EntryID
	mu       sync.Mutex
	execMu   sync.Mutex
}

// NewService instantiates a new Scheduler service.
func NewService(store database.Store, server ServerController, workDir string) *Service {
	// Standard 5-field cron parser supporting @daily, @hourly, etc.
	c := cron.New(cron.WithParser(cron.NewParser(
		cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor,
	)))

	return &Service{
		store:    store,
		server:   server,
		workDir:  workDir,
		cron:     c,
		entryIDs: make(map[int]cron.EntryID),
	}
}

// ValidateCronExpression checks whether a cron string is valid syntax.
func ValidateCronExpression(spec string) error {
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)
	_, err := parser.Parse(spec)
	return err
}

// Start loads all active schedules from the database and begins cron scheduling.
func (s *Service) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.loadSchedulesLocked(); err != nil {
		return fmt.Errorf("failed to load initial schedules: %w", err)
	}

	s.cron.Start()
	log.Printf("[Scheduler] Background scheduler started with %d active jobs.", len(s.entryIDs))
	return nil
}

// Stop stops the background cron runner.
func (s *Service) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	ctx := s.cron.Stop()
	<-ctx.Done()
	log.Println("[Scheduler] Background scheduler stopped.")
}

// Reload clears all running cron jobs and re-registers enabled schedules from the database.
func (s *Service) Reload() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Clear existing entries from cron
	for _, entryID := range s.entryIDs {
		s.cron.Remove(entryID)
	}
	s.entryIDs = make(map[int]cron.EntryID)

	return s.loadSchedulesLocked()
}

func (s *Service) loadSchedulesLocked() error {
	if s.store == nil {
		return nil
	}

	schedules, err := s.store.ListSchedules()
	if err != nil {
		return err
	}

	for _, sched := range schedules {
		if !sched.IsEnabled {
			continue
		}

		schedCopy := sched
		entryID, err := s.cron.AddFunc(sched.CronExpr, func() {
			_ = s.ExecuteJob(schedCopy.ID)
		})
		if err != nil {
			log.Printf("[Scheduler] Warning: Failed to register job #%d (%s): %v", sched.ID, sched.Name, err)
			continue
		}

		s.entryIDs[sched.ID] = entryID
	}

	return nil
}

// RegisterSchedule adds or updates an active schedule in the cron runner.
func (s *Service) RegisterSchedule(sched database.Schedule) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Remove old entry if registered
	if oldID, exists := s.entryIDs[sched.ID]; exists {
		s.cron.Remove(oldID)
		delete(s.entryIDs, sched.ID)
	}

	if !sched.IsEnabled {
		return nil
	}

	schedCopy := sched
	entryID, err := s.cron.AddFunc(sched.CronExpr, func() {
		_ = s.ExecuteJob(schedCopy.ID)
	})
	if err != nil {
		return fmt.Errorf("invalid cron expression '%s': %w", sched.CronExpr, err)
	}

	s.entryIDs[sched.ID] = entryID
	return nil
}

// UnregisterSchedule removes a schedule from the cron runner.
func (s *Service) UnregisterSchedule(scheduleID int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if entryID, exists := s.entryIDs[scheduleID]; exists {
		s.cron.Remove(entryID)
		delete(s.entryIDs, scheduleID)
	}
}

// GetNextRun returns the calculated next execution time for a schedule if active.
func (s *Service) GetNextRun(scheduleID int) *time.Time {
	s.mu.Lock()
	defer s.mu.Unlock()

	entryID, exists := s.entryIDs[scheduleID]
	if !exists {
		return nil
	}

	entry := s.cron.Entry(entryID)
	if entry.Next.IsZero() {
		return nil
	}
	next := entry.Next
	return &next
}

// ExecuteJob runs a schedule action immediately, records timing, logs the outcome, and broadcasts status.
func (s *Service) ExecuteJob(scheduleID int) error {
	// Guard against concurrent execution overlap
	s.execMu.Lock()
	defer s.execMu.Unlock()

	if s.store == nil {
		return errors.New("database store is nil")
	}

	sched, err := s.store.GetSchedule(scheduleID)
	if err != nil {
		return fmt.Errorf("schedule #%d not found: %w", scheduleID, err)
	}

	startTime := time.Now()
	if s.server != nil {
		s.server.Broadcast(fmt.Sprintf("[Scheduler] Executing task '%s' (Action: %s)...", sched.Name, sched.ActionType))
	}

	var actionErr error

	switch sched.ActionType {
	case "backup":
		actionErr = s.runBackup(sched.Payload)
	case "restart":
		actionErr = s.runRestart()
	case "command":
		actionErr = s.runCommand(sched.Payload)
	case "broadcast":
		actionErr = s.runBroadcast(sched.Payload)
	case "start":
		if s.server != nil {
			actionErr = s.server.Start()
		}
	case "stop":
		if s.server != nil {
			actionErr = s.server.Stop()
		}
	default:
		actionErr = fmt.Errorf("unknown schedule action type: '%s'", sched.ActionType)
	}

	durationMs := time.Since(startTime).Milliseconds()
	status := "success"
	errorMsg := ""

	if actionErr != nil {
		status = "failed"
		errorMsg = actionErr.Error()
		if s.server != nil {
			s.server.Broadcast(fmt.Sprintf("[Scheduler Error] Task '%s' failed in %dms: %s", sched.Name, durationMs, errorMsg))
		}
	} else {
		if s.server != nil {
			s.server.Broadcast(fmt.Sprintf("[Scheduler] Task '%s' completed successfully in %dms.", sched.Name, durationMs))
		}
	}

	// Persist execution log & update schedule stats
	if err := s.store.RecordScheduleExecution(sched.ID, status, durationMs, errorMsg); err != nil {
		log.Printf("[Scheduler] Failed to record execution log for schedule #%d: %v", sched.ID, err)
	}

	return actionErr
}

func (s *Service) runBackup(payload string) error {
	var req backup.CreateBackupRequest
	if payload != "" {
		_ = json.Unmarshal([]byte(payload), &req)
	}
	if req.Type == "" {
		req.Type = "world"
	}
	if req.Type == "world" && req.WorldName == "" {
		req.WorldName = "world"
	}

	_, err := backup.CreateBackup(s.workDir, req, s.server)
	return err
}

func (s *Service) runRestart() error {
	if s.server == nil {
		return errors.New("server controller is nil")
	}

	if s.server.GetStatus() == minecraft.StatusRunning || s.server.GetStatus() == minecraft.StatusStarting {
		_ = s.server.SendCommand("say [Server] Scheduled restart in progress...")
		if err := s.server.Stop(); err != nil {
			return fmt.Errorf("failed to stop server during scheduled restart: %w", err)
		}

		deadline := time.Now().Add(30 * time.Second)
		for {
			if s.server.GetStatus() == minecraft.StatusStopped {
				break
			}
			if time.Now().After(deadline) {
				return errors.New("timed out waiting for server to stop during scheduled restart")
			}
			time.Sleep(200 * time.Millisecond)
		}
	}

	return s.server.Start()
}

func (s *Service) runCommand(cmd string) error {
	if s.server == nil {
		return errors.New("server controller is nil")
	}
	if cmd == "" {
		return errors.New("command payload cannot be empty")
	}
	return s.server.SendCommand(cmd)
}

func (s *Service) runBroadcast(msg string) error {
	if s.server == nil {
		return errors.New("server controller is nil")
	}
	if msg == "" {
		return errors.New("broadcast message cannot be empty")
	}

	_ = s.server.SendCommand("say " + msg)
	s.server.Broadcast("[In-Game Announcement] " + msg)
	return nil
}
