package database

import (
	"time"
)

type User struct {
	ID       int    `json:"id"`
	Username string `json:"username"`
	Password string `json:"-"` // hash, not serialized in JSON
	Role     string `json:"role"`
}

type RejectedPlayer struct {
	Username string    `json:"username"`
	Count    int       `json:"count"`
	LastSeen time.Time `json:"last_seen"`
}

type Schedule struct {
	ID            int        `json:"id"`
	Name          string     `json:"name"`
	CronExpr      string     `json:"cron_expr"`
	ActionType    string     `json:"action_type"` // "backup", "restart", "command", "broadcast", "start", "stop"
	Payload       string     `json:"payload"`
	IsEnabled     bool       `json:"is_enabled"`
	LastRunAt     *time.Time `json:"last_run_at,omitempty"`
	LastRunStatus string     `json:"last_run_status,omitempty"` // "success", "failed"
	LastRunError  string     `json:"last_run_error,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	NextRunAt     *time.Time `json:"next_run_at,omitempty"`
}

type ScheduleLog struct {
	ID           int       `json:"id"`
	ScheduleID   int       `json:"schedule_id"`
	ScheduleName string    `json:"schedule_name"`
	ActionType   string    `json:"action_type"`
	Status       string    `json:"status"` // "success" or "failed"
	DurationMs   int64     `json:"duration_ms"`
	ErrorMessage string    `json:"error_message,omitempty"`
	ExecutedAt   time.Time `json:"executed_at"`
}

type Store interface {
	Migrate() error
	Close() error

	// User Auth
	GetUser(username string) (*User, error)
	ListUsers() ([]User, error)
	CreateUser(user *User) error
	UpdateUserPassword(username, passwordHash string) error
	DeleteUser(username string) error

	// Player Intelligence
	UpsertRejectedPlayer(username string) error
	GetRejectedPlayers() ([]RejectedPlayer, error)
	DeleteRejectedPlayer(username string) error

	// Scheduler & Execution Logs
	ListSchedules() ([]Schedule, error)
	GetSchedule(id int) (*Schedule, error)
	CreateSchedule(s *Schedule) error
	UpdateSchedule(s *Schedule) error
	ToggleSchedule(id int, isEnabled bool) error
	DeleteSchedule(id int) error
	RecordScheduleExecution(scheduleID int, status string, durationMs int64, errorMessage string) error
	ListScheduleLogs(scheduleID int, limit int) ([]ScheduleLog, error)
	ClearScheduleLogs(scheduleID int) error
}
