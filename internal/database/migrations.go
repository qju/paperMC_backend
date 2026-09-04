package database

import (
	"database/sql"
	"fmt"
	"log"
)

// Migration represents a discrete, atomic database schema upgrade step.
type Migration struct {
	Version     int
	Description string
	Up          func(tx *sql.Tx) error
}

// migrations contains the strict chronological registry of database schema migrations.
// RULES:
// 1. Never modify or delete existing migrations that have been released to production.
// 2. Always append new migrations with strictly sequential version numbers (e.g. 2, 3, 4...).
// 3. Ensure all DDL and DML operations execute within the provided transaction.
var migrations = []Migration{
	{
		Version:     1,
		Description: "Initial schema: users and rejected_players tables",
		Up: func(tx *sql.Tx) error {
			schemaSQL := `
			CREATE TABLE IF NOT EXISTS users (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				username TEXT NOT NULL UNIQUE,
				password TEXT NOT NULL,
				role TEXT NOT NULL
			);

			CREATE TABLE IF NOT EXISTS rejected_players (
				username TEXT PRIMARY KEY,
				count INTEGER DEFAULT 1,
				last_seen DATETIME DEFAULT CURRENT_TIMESTAMP
			);
			`
			_, err := tx.Exec(schemaSQL)
			return err
		},
	},
	{
		Version:     2,
		Description: "Add schedules and schedule_logs tables for automated tasks",
		Up: func(tx *sql.Tx) error {
			schemaSQL := `
			CREATE TABLE IF NOT EXISTS schedules (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				name TEXT NOT NULL,
				cron_expr TEXT NOT NULL,
				action_type TEXT NOT NULL,
				payload TEXT DEFAULT '',
				is_enabled INTEGER DEFAULT 1,
				last_run_at DATETIME,
				last_run_status TEXT DEFAULT '',
				last_run_error TEXT DEFAULT '',
				created_at DATETIME DEFAULT CURRENT_TIMESTAMP
			);

			CREATE TABLE IF NOT EXISTS schedule_logs (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				schedule_id INTEGER NOT NULL,
				schedule_name TEXT NOT NULL,
				action_type TEXT NOT NULL,
				status TEXT NOT NULL,
				duration_ms INTEGER DEFAULT 0,
				error_message TEXT DEFAULT '',
				executed_at DATETIME DEFAULT CURRENT_TIMESTAMP,
				FOREIGN KEY(schedule_id) REFERENCES schedules(id) ON DELETE CASCADE
			);

			CREATE INDEX IF NOT EXISTS idx_schedule_logs_schedule_id ON schedule_logs(schedule_id);
			CREATE INDEX IF NOT EXISTS idx_schedule_logs_executed_at ON schedule_logs(executed_at);
			`
			_, err := tx.Exec(schemaSQL)
			return err
		},
	},
}

// GetSchemaVersion reads the current user_version from SQLite PRAGMA.
func GetSchemaVersion(db *sql.DB) (int, error) {
	var version int
	err := db.QueryRow("PRAGMA user_version;").Scan(&version)
	if err != nil {
		return 0, fmt.Errorf("failed to query schema version: %w", err)
	}
	return version, nil
}

// setSchemaVersion updates the user_version PRAGMA.
func setSchemaVersion(db *sql.DB, version int) error {
	query := fmt.Sprintf("PRAGMA user_version = %d;", version)
	_, err := db.Exec(query)
	if err != nil {
		return fmt.Errorf("failed to set schema version to %d: %w", version, err)
	}
	return nil
}

// RunMigrations executes all pending migrations sequentially within atomic transactions.
func RunMigrations(db *sql.DB) error {
	currentVersion, err := GetSchemaVersion(db)
	if err != nil {
		return err
	}

	// Backward-compatibility check: If user_version is 0, check if this is an existing
	// un-versioned production database that already contains the initial tables.
	if currentVersion == 0 {
		var userTableCount int
		err := db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='users';").Scan(&userTableCount)
		if err == nil && userTableCount > 0 {
			// Existing database from before the migration engine was introduced.
			// Mark as Version 1 baseline.
			if err := setSchemaVersion(db, 1); err != nil {
				return err
			}
			currentVersion = 1
			log.Printf("[DB] Existing database detected. Baseline stamped at schema version 1.")
		}
	}

	for _, m := range migrations {
		if m.Version > currentVersion {
			log.Printf("[DB] Applying migration v%d: %s...", m.Version, m.Description)

			tx, err := db.Begin()
			if err != nil {
				return fmt.Errorf("failed to begin transaction for migration v%d: %w", m.Version, err)
			}

			if err := m.Up(tx); err != nil {
				_ = tx.Rollback()
				return fmt.Errorf("migration v%d failed: %w", m.Version, err)
			}

			if err := tx.Commit(); err != nil {
				return fmt.Errorf("failed to commit migration v%d: %w", m.Version, err)
			}

			// Update SQLite user_version register
			if err := setSchemaVersion(db, m.Version); err != nil {
				return err
			}

			currentVersion = m.Version
			log.Printf("[DB] Migration v%d applied successfully.", m.Version)
		}
	}

	return nil
}
