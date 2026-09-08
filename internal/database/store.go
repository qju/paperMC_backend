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

type ServerFlags struct {
	RAM         string    `json:"ram"`
	Preset      string    `json:"preset"`
	CustomFlags string    `json:"custom_flags"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type AuditLog struct {
	ID         int       `json:"id"`
	Username   string    `json:"username"`
	Action     string    `json:"action"`
	Endpoint   string    `json:"endpoint"`
	Method     string    `json:"method"`
	Details    string    `json:"details"`
	IPAddress  string    `json:"ip_address"`
	StatusCode int       `json:"status_code"`
	CreatedAt  time.Time `json:"created_at"`
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

	// Server Flags & JVM Tuning
	GetServerFlags() (*ServerFlags, error)
	SaveServerFlags(flags *ServerFlags) error

	// Audit Logging
	RecordAuditLog(log *AuditLog) error
	ListAuditLogs(limit, offset int, actionFilter, userFilter string) ([]AuditLog, int, error)
	ClearAuditLogs() error

	// Crash Reports & AI Settings
	RecordCrashReport(report *CrashReport) error
	ListCrashReports(limit, offset int) ([]CrashReport, int, error)
	GetCrashReport(id int) (*CrashReport, error)
	DeleteCrashReport(id int) error
	ClearCrashReports() error
	GetAISettings() (*AISettings, error)
	SaveAISettings(settings *AISettings) error
}

type CrashReport struct {
	ID             int       `json:"id"`
	Source         string    `json:"source"` // "runtime", "crash_file", "manual"
	Category       string    `json:"category"`
	Title          string    `json:"title"`
	Culprit        string    `json:"culprit,omitempty"`
	Summary        string    `json:"summary"`
	Recommendation string    `json:"recommendation"`
	RawLog         string    `json:"raw_log"`
	CreatedAt      time.Time `json:"created_at"`
}

type AISettings struct {
	Provider  string    `json:"provider"` // "openai", "gemini"
	APIKey    string    `json:"api_key"`
	Model     string    `json:"model"`
	BaseURL   string    `json:"base_url"`
	IsEnabled bool      `json:"is_enabled"`
	UpdatedAt time.Time `json:"updated_at"`
}

