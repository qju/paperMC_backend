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

	// Schema version must now be at least 1
	verAfter, err := GetSchemaVersion(db)
	if err != nil {
		t.Fatalf("GetSchemaVersion failed: %v", err)
	}
	if verAfter != 1 {
		t.Errorf("Expected schema version 1 after migrations, got %d", verAfter)
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

	// Run migration engine: should detect existing tables and stamp version 1
	if err := RunMigrations(db); err != nil {
		t.Fatalf("RunMigrations failed on legacy db: %v", err)
	}

	verAfter, err := GetSchemaVersion(db)
	if err != nil || verAfter != 1 {
		t.Errorf("Expected schema version 1 after legacy upgrade, got %d", verAfter)
	}
}
