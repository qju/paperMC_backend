package database

import (
	"database/sql"
	"errors"
	"fmt"
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
