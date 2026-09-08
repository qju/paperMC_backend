package database

import (
	"database/sql"
	"path/filepath"
	"testing"
)

func TestMigrationsFreshDatabase(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "fresh_mig.db")

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("Failed to open test db: %v", err)
	}
	defer db.Close()

	// Initial version before migrations must be 0
	initialVer, err := GetSchemaVersion(db)
	if err != nil {
		t.Fatalf("GetSchemaVersion failed: %v", err)
	}
	if initialVer != 0 {
		t.Errorf("Expected initial version 0, got %d", initialVer)
	}

	// Run migrations
	if err := RunMigrations(db); err != nil {
		t.Fatalf("RunMigrations failed: %v", err)
	}

	// Schema version must match total migrations
	expectedVer := len(migrations)
	verAfter, err := GetSchemaVersion(db)
	if err != nil {
		t.Fatalf("GetSchemaVersion failed: %v", err)
	}
	if verAfter != expectedVer {
		t.Errorf("Expected schema version %d after migrations, got %d", expectedVer, verAfter)
	}

	// Verify schedules, schedule_logs, server_flags, audit_logs, crash_reports, ai_settings, profiler_reports, user_mfa, and user_sessions tables exist
	var schedCount, logCount, flagCount, auditCount, crashCount, aiCount, profilerCount, mfaCount, sessCount int
	_ = db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='schedules';").Scan(&schedCount)
	_ = db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='schedule_logs';").Scan(&logCount)
	_ = db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='server_flags';").Scan(&flagCount)
	_ = db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='audit_logs';").Scan(&auditCount)
	_ = db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='crash_reports';").Scan(&crashCount)
	_ = db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='ai_settings';").Scan(&aiCount)
	_ = db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='profiler_reports';").Scan(&profilerCount)
	_ = db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='user_mfa';").Scan(&mfaCount)
	_ = db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='user_sessions';").Scan(&sessCount)
	if schedCount != 1 || logCount != 1 || flagCount != 1 || auditCount != 1 || crashCount != 1 || aiCount != 1 || profilerCount != 1 || mfaCount != 1 || sessCount != 1 {
		t.Errorf("Expected tables to exist: sched=%d, log=%d, flag=%d, audit=%d, crash=%d, ai=%d, profiler=%d, mfa=%d, sess=%d", schedCount, logCount, flagCount, auditCount, crashCount, aiCount, profilerCount, mfaCount, sessCount)
	}

	// Re-running migrations must be idempotent
	if err := RunMigrations(db); err != nil {
		t.Fatalf("Second RunMigrations call failed: %v", err)
	}
}

func TestMigrationsLegacyUnversionedDatabase(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "legacy_mig.db")

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("Failed to open test db: %v", err)
	}
	defer db.Close()

	// Create legacy unversioned tables directly without user_version
	_, err = db.Exec(`
		CREATE TABLE users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT NOT NULL,
			password TEXT NOT NULL,
			role TEXT NOT NULL
		);
	`)
	if err != nil {
		t.Fatalf("Failed to create legacy table: %v", err)
	}

	// Version is 0
	ver, err := GetSchemaVersion(db)
	if err != nil || ver != 0 {
		t.Fatalf("Expected version 0 for un-stamped db, got %d", ver)
	}

	// Run migration engine: should detect existing tables, stamp version 1, then apply migration 2
	if err := RunMigrations(db); err != nil {
		t.Fatalf("RunMigrations failed on legacy db: %v", err)
	}

	verAfter, err := GetSchemaVersion(db)
	if err != nil || verAfter != len(migrations) {
		t.Errorf("Expected schema version %d after legacy upgrade, got %d", len(migrations), verAfter)
	}
}
