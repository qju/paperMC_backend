package database

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

type SQLiteStore struct {
	db *sql.DB
}

func NewSQLiteStore(storePath string) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite", storePath)
	if err != nil {
		return nil, err
	}
	// Ping, make sure it is alive

	if err := db.Ping(); err != nil {
		return nil, err
	}

	// Enable Foreign Keys for this connection
	if _, err := db.Exec("PRAGMA foreign_keys = ON;"); err != nil {
		return nil, err
	}

	store := &SQLiteStore{db: db}
	// Migrate
	if err := store.Migrate(); err != nil {
		return nil, err
	}
	return store, nil
}

func (s *SQLiteStore) Migrate() error {
	return RunMigrations(s.db)
}

func (s *SQLiteStore) GetUser(username string) (*User, error) {
	SQL := `SELECT id, username, password, role FROM users WHERE username = ?`
	row := s.db.QueryRow(SQL, username)

	var user User
	err := row.Scan(&user.ID, &user.Username, &user.Password, &user.Role)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (s *SQLiteStore) ListUsers() ([]User, error) {
	SQL := `SELECT id, username, role FROM users ORDER BY id ASC`
	rows, err := s.db.Query(SQL)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	users := []User{}
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.Username, &u.Role); err != nil {
			continue
		}
		users = append(users, u)
	}
	return users, nil
}

func (s *SQLiteStore) CreateUser(user *User) error {
	SQL := `INSERT INTO users (username, password, role) VALUES (?, ?, ?)`
	_, err := s.db.Exec(SQL, user.Username, user.Password, user.Role)
	return err
}

func (s *SQLiteStore) UpdateUserPassword(username, passwordHash string) error {
	SQL := `UPDATE users SET password = ? WHERE username = ?`
	_, err := s.db.Exec(SQL, passwordHash, username)
	return err
}

func (s *SQLiteStore) DeleteUser(username string) error {
	SQL := `DELETE FROM users WHERE username = ?`
	_, err := s.db.Exec(SQL, username)
	return err
}

func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

// --- Players Inteligence ---

func (s *SQLiteStore) UpsertRejectedPlayer(username string) error {
	// If exist, update count and time. If not INSERT
	SQL := `INSERT INTO rejected_players (username, count, last_seen)
			VALUES (?, 1, CURRENT_TIMESTAMP)
			ON CONFLICT(username) DO UPDATE SET
				count = count + 1,
				last_seen = CURRENT_TIMESTAMP;`
	_, err := s.db.Exec(SQL, username)
	return err
}

func parseSQLiteTime(t string) time.Time {
	layouts := []string{
		"2006-01-02 15:04:05",
		time.RFC3339,
		"2006-01-02T15:04:05Z",
		"2006-01-02 15:04:05.999999999-07:00",
		"2006-01-02T15:04:05.999999999Z07:00",
		"2006-01-02",
	}
	for _, layout := range layouts {
		if parsed, err := time.Parse(layout, t); err == nil {
			return parsed
		}
	}
	return time.Time{}
}

func (s *SQLiteStore) GetRejectedPlayers() ([]RejectedPlayer, error) {
	SQL := `SELECT username, count, last_seen FROM rejected_players ORDER BY last_seen DESC LIMIT 50`
	rows, err := s.db.Query(SQL)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := []RejectedPlayer{}
	for rows.Next() {
		var p RejectedPlayer
		var t string
		if err := rows.Scan(&p.Username, &p.Count, &t); err != nil {
			continue
		}
		p.LastSeen = parseSQLiteTime(t)
		list = append(list, p)
	}
	return list, nil
}

func (s *SQLiteStore) DeleteRejectedPlayer(username string) error {
	SQL := `DELETE FROM rejected_players WHERE username = ?`
	_, err := s.db.Exec(SQL, username)
	return err
}

// --- Scheduler Operations ---

func (s *SQLiteStore) ListSchedules() ([]Schedule, error) {
	SQL := `SELECT id, name, cron_expr, action_type, payload, is_enabled, last_run_at, last_run_status, last_run_error, created_at FROM schedules ORDER BY id ASC`
	rows, err := s.db.Query(SQL)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []Schedule
	for rows.Next() {
		var item Schedule
		var isEnabledInt int
		var lastRunStr sql.NullString
		var createdStr string
		if err := rows.Scan(&item.ID, &item.Name, &item.CronExpr, &item.ActionType, &item.Payload, &isEnabledInt, &lastRunStr, &item.LastRunStatus, &item.LastRunError, &createdStr); err != nil {
			continue
		}
		item.IsEnabled = isEnabledInt == 1
		item.CreatedAt = parseSQLiteTime(createdStr)
		if lastRunStr.Valid && lastRunStr.String != "" {
			t := parseSQLiteTime(lastRunStr.String)
			item.LastRunAt = &t
		}
		list = append(list, item)
	}
	return list, nil
}

func (s *SQLiteStore) GetSchedule(id int) (*Schedule, error) {
	SQL := `SELECT id, name, cron_expr, action_type, payload, is_enabled, last_run_at, last_run_status, last_run_error, created_at FROM schedules WHERE id = ?`
	row := s.db.QueryRow(SQL, id)

	var item Schedule
	var isEnabledInt int
	var lastRunStr sql.NullString
	var createdStr string
	if err := row.Scan(&item.ID, &item.Name, &item.CronExpr, &item.ActionType, &item.Payload, &isEnabledInt, &lastRunStr, &item.LastRunStatus, &item.LastRunError, &createdStr); err != nil {
		return nil, err
	}
	item.IsEnabled = isEnabledInt == 1
	item.CreatedAt = parseSQLiteTime(createdStr)
	if lastRunStr.Valid && lastRunStr.String != "" {
		t := parseSQLiteTime(lastRunStr.String)
		item.LastRunAt = &t
	}
	return &item, nil
}

func (s *SQLiteStore) CreateSchedule(sched *Schedule) error {
	SQL := `INSERT INTO schedules (name, cron_expr, action_type, payload, is_enabled) VALUES (?, ?, ?, ?, ?)`
	enabledInt := 0
	if sched.IsEnabled {
		enabledInt = 1
	}
	res, err := s.db.Exec(SQL, sched.Name, sched.CronExpr, sched.ActionType, sched.Payload, enabledInt)
	if err != nil {
		return err
	}
	id, err := res.LastInsertId()
	if err == nil {
		sched.ID = int(id)
	}
	return nil
}

func (s *SQLiteStore) UpdateSchedule(sched *Schedule) error {
	SQL := `UPDATE schedules SET name = ?, cron_expr = ?, action_type = ?, payload = ?, is_enabled = ? WHERE id = ?`
	enabledInt := 0
	if sched.IsEnabled {
		enabledInt = 1
	}
	_, err := s.db.Exec(SQL, sched.Name, sched.CronExpr, sched.ActionType, sched.Payload, enabledInt, sched.ID)
	return err
}

func (s *SQLiteStore) ToggleSchedule(id int, isEnabled bool) error {
	SQL := `UPDATE schedules SET is_enabled = ? WHERE id = ?`
	enabledInt := 0
	if isEnabled {
		enabledInt = 1
	}
	_, err := s.db.Exec(SQL, enabledInt, id)
	return err
}

func (s *SQLiteStore) DeleteSchedule(id int) error {
	SQL := `DELETE FROM schedules WHERE id = ?`
	_, err := s.db.Exec(SQL, id)
	return err
}

func (s *SQLiteStore) RecordScheduleExecution(scheduleID int, status string, durationMs int64, errorMessage string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// 1. Fetch schedule name and action type
	var name, actionType string
	err = tx.QueryRow("SELECT name, action_type FROM schedules WHERE id = ?", scheduleID).Scan(&name, &actionType)
	if err != nil {
		name = fmt.Sprintf("Schedule #%d", scheduleID)
		actionType = "unknown"
	}

	// 2. Insert into schedule_logs
	logSQL := `INSERT INTO schedule_logs (schedule_id, schedule_name, action_type, status, duration_ms, error_message, executed_at) VALUES (?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)`
	if _, err := tx.Exec(logSQL, scheduleID, name, actionType, status, durationMs, errorMessage); err != nil {
		return err
	}

	// 3. Update schedules table with last_run stats
	updateSQL := `UPDATE schedules SET last_run_at = CURRENT_TIMESTAMP, last_run_status = ?, last_run_error = ? WHERE id = ?`
	if _, err := tx.Exec(updateSQL, status, errorMessage, scheduleID); err != nil {
		return err
	}

	return tx.Commit()
}

func (s *SQLiteStore) ListScheduleLogs(scheduleID int, limit int) ([]ScheduleLog, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}

	var query string
	var args []interface{}

	if scheduleID > 0 {
		query = `SELECT id, schedule_id, schedule_name, action_type, status, duration_ms, error_message, executed_at FROM schedule_logs WHERE schedule_id = ? ORDER BY executed_at DESC, id DESC LIMIT ?`
		args = append(args, scheduleID, limit)
	} else {
		query = `SELECT id, schedule_id, schedule_name, action_type, status, duration_ms, error_message, executed_at FROM schedule_logs ORDER BY executed_at DESC, id DESC LIMIT ?`
		args = append(args, limit)
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []ScheduleLog
	for rows.Next() {
		var l ScheduleLog
		var execStr string
		if err := rows.Scan(&l.ID, &l.ScheduleID, &l.ScheduleName, &l.ActionType, &l.Status, &l.DurationMs, &l.ErrorMessage, &execStr); err != nil {
			continue
		}
		l.ExecutedAt = parseSQLiteTime(execStr)
		logs = append(logs, l)
	}
	return logs, nil
}

func (s *SQLiteStore) ClearScheduleLogs(scheduleID int) error {
	if scheduleID > 0 {
		_, err := s.db.Exec("DELETE FROM schedule_logs WHERE schedule_id = ?", scheduleID)
		return err
	}
	_, err := s.db.Exec("DELETE FROM schedule_logs")
	return err
}

func (s *SQLiteStore) GetServerFlags() (*ServerFlags, error) {
	row := s.db.QueryRow("SELECT ram, preset, custom_flags, updated_at FROM server_flags WHERE id = 1")
	var f ServerFlags
	var updatedStr string
	if err := row.Scan(&f.RAM, &f.Preset, &f.CustomFlags, &updatedStr); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return &ServerFlags{
				RAM:         "8G",
				Preset:      "aikar",
				CustomFlags: "",
				UpdatedAt:   time.Now(),
			}, nil
		}
		return nil, err
	}
	f.UpdatedAt = parseSQLiteTime(updatedStr)
	return &f, nil
}

func (s *SQLiteStore) SaveServerFlags(flags *ServerFlags) error {
	query := `
	INSERT INTO server_flags (id, ram, preset, custom_flags, updated_at) 
	VALUES (1, ?, ?, ?, CURRENT_TIMESTAMP)
	ON CONFLICT(id) DO UPDATE SET 
		ram = excluded.ram,
		preset = excluded.preset,
		custom_flags = excluded.custom_flags,
		updated_at = CURRENT_TIMESTAMP
	`
	_, err := s.db.Exec(query, flags.RAM, flags.Preset, flags.CustomFlags)
	return err
}

// --- Audit Logging ---

func (s *SQLiteStore) RecordAuditLog(entry *AuditLog) error {
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = time.Now()
	}
	query := `INSERT INTO audit_logs (username, action, endpoint, method, details, ip_address, status_code, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`
	res, err := s.db.Exec(query, entry.Username, entry.Action, entry.Endpoint, entry.Method, entry.Details, entry.IPAddress, entry.StatusCode, entry.CreatedAt.Format("2006-01-02 15:04:05"))
	if err != nil {
		return err
	}
	id, err := res.LastInsertId()
	if err == nil {
		entry.ID = int(id)
	}
	return nil
}

func (s *SQLiteStore) ListAuditLogs(limit, offset int, actionFilter, userFilter string) ([]AuditLog, int, error) {
	var conditions []string
	var args []interface{}

	if strings.TrimSpace(actionFilter) != "" {
		act := strings.TrimSpace(actionFilter)
		if strings.Contains(act, "%") {
			conditions = append(conditions, "action LIKE ?")
			args = append(args, act)
		} else {
			conditions = append(conditions, "(action = ? OR action LIKE ?)")
			args = append(args, act, act+".%")
		}
	}

	if strings.TrimSpace(userFilter) != "" {
		usr := strings.TrimSpace(userFilter)
		conditions = append(conditions, "username LIKE ?")
		args = append(args, "%"+usr+"%")
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = " WHERE " + strings.Join(conditions, " AND ")
	}

	countSQL := "SELECT COUNT(*) FROM audit_logs" + whereClause
	var total int
	if err := s.db.QueryRow(countSQL, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	if limit <= 0 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	dataSQL := "SELECT id, username, action, endpoint, method, details, ip_address, status_code, created_at FROM audit_logs" +
		whereClause + " ORDER BY id DESC LIMIT ? OFFSET ?"
	dataArgs := append(args, limit, offset)

	rows, err := s.db.Query(dataSQL, dataArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	logs := []AuditLog{}
	for rows.Next() {
		var l AuditLog
		var createdStr string
		if err := rows.Scan(&l.ID, &l.Username, &l.Action, &l.Endpoint, &l.Method, &l.Details, &l.IPAddress, &l.StatusCode, &createdStr); err != nil {
			continue
		}
		l.CreatedAt = parseSQLiteTime(createdStr)
		logs = append(logs, l)
	}

	return logs, total, nil
}

func (s *SQLiteStore) ClearAuditLogs() error {
	_, err := s.db.Exec("DELETE FROM audit_logs")
	return err
}

func (s *SQLiteStore) RecordCrashReport(report *CrashReport) error {
	query := `INSERT INTO crash_reports (source, category, title, culprit, summary, recommendation, raw_log, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`
	now := time.Now().UTC()
	if report.CreatedAt.IsZero() {
		report.CreatedAt = now
	}
	res, err := s.db.Exec(query, report.Source, report.Category, report.Title, report.Culprit, report.Summary, report.Recommendation, report.RawLog, report.CreatedAt.Format("2006-01-02 15:04:05"))
	if err != nil {
		return err
	}
	id, err := res.LastInsertId()
	if err == nil {
		report.ID = int(id)
	}
	return nil
}

func (s *SQLiteStore) ListCrashReports(limit, offset int) ([]CrashReport, int, error) {
	var total int
	err := s.db.QueryRow("SELECT COUNT(*) FROM crash_reports").Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	if limit <= 0 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	query := `SELECT id, source, category, title, culprit, summary, recommendation, raw_log, created_at
		FROM crash_reports ORDER BY id DESC LIMIT ? OFFSET ?`
	rows, err := s.db.Query(query, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	reports := []CrashReport{}
	for rows.Next() {
		var r CrashReport
		var createdStr string
		if err := rows.Scan(&r.ID, &r.Source, &r.Category, &r.Title, &r.Culprit, &r.Summary, &r.Recommendation, &r.RawLog, &createdStr); err != nil {
			continue
		}
		r.CreatedAt = parseSQLiteTime(createdStr)
		reports = append(reports, r)
	}
	return reports, total, nil
}

func (s *SQLiteStore) GetCrashReport(id int) (*CrashReport, error) {
	query := `SELECT id, source, category, title, culprit, summary, recommendation, raw_log, created_at
		FROM crash_reports WHERE id = ?`
	var r CrashReport
	var createdStr string
	err := s.db.QueryRow(query, id).Scan(&r.ID, &r.Source, &r.Category, &r.Title, &r.Culprit, &r.Summary, &r.Recommendation, &r.RawLog, &createdStr)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	r.CreatedAt = parseSQLiteTime(createdStr)
	return &r, nil
}

func (s *SQLiteStore) DeleteCrashReport(id int) error {
	_, err := s.db.Exec("DELETE FROM crash_reports WHERE id = ?", id)
	return err
}

func (s *SQLiteStore) ClearCrashReports() error {
	_, err := s.db.Exec("DELETE FROM crash_reports")
	return err
}

func (s *SQLiteStore) GetAISettings() (*AISettings, error) {
	query := `SELECT provider, api_key, model, base_url, is_enabled, updated_at FROM ai_settings WHERE id = 1`
	var settings AISettings
	var isEnabledInt int
	var updatedStr string
	err := s.db.QueryRow(query).Scan(&settings.Provider, &settings.APIKey, &settings.Model, &settings.BaseURL, &isEnabledInt, &updatedStr)
	if err == sql.ErrNoRows {
		return &AISettings{
			Provider:  "openai",
			Model:     "gpt-4o-mini",
			IsEnabled: false,
			UpdatedAt: time.Now().UTC(),
		}, nil
	}
	if err != nil {
		return nil, err
	}
	settings.IsEnabled = isEnabledInt == 1
	settings.UpdatedAt = parseSQLiteTime(updatedStr)
	return &settings, nil
}

func (s *SQLiteStore) SaveAISettings(settings *AISettings) error {
	isEnabledInt := 0
	if settings.IsEnabled {
		isEnabledInt = 1
	}
	now := time.Now().UTC().Format("2006-01-02 15:04:05")
	query := `INSERT INTO ai_settings (id, provider, api_key, model, base_url, is_enabled, updated_at)
		VALUES (1, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			provider = excluded.provider,
			api_key = excluded.api_key,
			model = excluded.model,
			base_url = excluded.base_url,
			is_enabled = excluded.is_enabled,
			updated_at = excluded.updated_at`
	_, err := s.db.Exec(query, settings.Provider, settings.APIKey, settings.Model, settings.BaseURL, isEnabledInt, now)
	return err
}

func (s *SQLiteStore) RecordProfilerReport(report *ProfilerReport) error {
	query := `INSERT INTO profiler_reports (report_type, title, url, tps, mspt, cpu_usage, memory_usage, gc_metrics, summary, raw_output, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	now := time.Now().UTC()
	if report.CreatedAt.IsZero() {
		report.CreatedAt = now
	}
	res, err := s.db.Exec(query, report.ReportType, report.Title, report.URL, report.TPS, report.MSPT, report.CPUUsage, report.MemoryUsage, report.GCMetrics, report.Summary, report.RawOutput, report.CreatedAt.Format("2006-01-02 15:04:05"))
	if err != nil {
		return err
	}
	id, err := res.LastInsertId()
	if err == nil {
		report.ID = int(id)
	}
	return nil
}

func (s *SQLiteStore) ListProfilerReports(limit, offset int, reportType string) ([]ProfilerReport, int, error) {
	whereClause := ""
	var args []interface{}
	if strings.TrimSpace(reportType) != "" {
		whereClause = " WHERE report_type = ?"
		args = append(args, strings.TrimSpace(reportType))
	}

	countSQL := "SELECT COUNT(*) FROM profiler_reports" + whereClause
	var total int
	if err := s.db.QueryRow(countSQL, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	if limit <= 0 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	query := `SELECT id, report_type, title, url, tps, mspt, cpu_usage, memory_usage, gc_metrics, summary, raw_output, created_at
		FROM profiler_reports` + whereClause + ` ORDER BY id DESC LIMIT ? OFFSET ?`
	dataArgs := append(args, limit, offset)
	rows, err := s.db.Query(query, dataArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	reports := []ProfilerReport{}
	for rows.Next() {
		var r ProfilerReport
		var createdStr string
		if err := rows.Scan(&r.ID, &r.ReportType, &r.Title, &r.URL, &r.TPS, &r.MSPT, &r.CPUUsage, &r.MemoryUsage, &r.GCMetrics, &r.Summary, &r.RawOutput, &createdStr); err != nil {
			continue
		}
		r.CreatedAt = parseSQLiteTime(createdStr)
		reports = append(reports, r)
	}
	return reports, total, nil
}

func (s *SQLiteStore) GetProfilerReport(id int) (*ProfilerReport, error) {
	query := `SELECT id, report_type, title, url, tps, mspt, cpu_usage, memory_usage, gc_metrics, summary, raw_output, created_at
		FROM profiler_reports WHERE id = ?`
	var r ProfilerReport
	var createdStr string
	err := s.db.QueryRow(query, id).Scan(&r.ID, &r.ReportType, &r.Title, &r.URL, &r.TPS, &r.MSPT, &r.CPUUsage, &r.MemoryUsage, &r.GCMetrics, &r.Summary, &r.RawOutput, &createdStr)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	r.CreatedAt = parseSQLiteTime(createdStr)
	return &r, nil
}

func (s *SQLiteStore) DeleteProfilerReport(id int) error {
	_, err := s.db.Exec("DELETE FROM profiler_reports WHERE id = ?", id)
	return err
}

func (s *SQLiteStore) ClearProfilerReports() error {
	_, err := s.db.Exec("DELETE FROM profiler_reports")
	return err
}



