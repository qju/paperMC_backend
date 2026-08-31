package database

import (
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func TestSQLiteStoreUsersAndRejectedPlayers(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test_paper.db")

	store, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to initialize SQLite store: %v", err)
	}
	defer store.Close()

	// 1. Test User Creation
	testUser := &User{
		Username: "admin_test",
		Password: "hashed_password_123",
		Role:     "admin",
	}
	if err := store.CreateUser(testUser); err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	// 2. Test Get User (Success)
	user, err := store.GetUser("admin_test")
	if err != nil {
		t.Fatalf("GetUser failed: %v", err)
	}
	if user.Username != testUser.Username || user.Role != testUser.Role {
		t.Errorf("GetUser returned %+v, expected %+v", user, testUser)
	}

	// 3. Test Get User (Not Found)
	_, err = store.GetUser("non_existent_user")
	if !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("Expected sql.ErrNoRows, got %v", err)
	}

	// 4. Test Upsert Rejected Player (Insert)
	if err := store.UpsertRejectedPlayer("Griefer123"); err != nil {
		t.Fatalf("UpsertRejectedPlayer failed on insert: %v", err)
	}

	rejectedList, err := store.GetRejectedPlayers()
	if err != nil {
		t.Fatalf("GetRejectedPlayers failed: %v", err)
	}
	if len(rejectedList) != 1 {
		t.Fatalf("Expected 1 rejected player, got %d", len(rejectedList))
	}
	if rejectedList[0].Username != "Griefer123" || rejectedList[0].Count != 1 {
		t.Errorf("Unexpected rejected player data: %+v", rejectedList[0])
	}
	if rejectedList[0].LastSeen.IsZero() {
		t.Errorf("Expected valid parsed timestamp, got zero value")
	}

	// 5. Test Upsert Rejected Player (Update Count)
	time.Sleep(10 * time.Millisecond)
	if err := store.UpsertRejectedPlayer("Griefer123"); err != nil {
		t.Fatalf("UpsertRejectedPlayer failed on update: %v", err)
	}

	rejectedList, err = store.GetRejectedPlayers()
	if err != nil {
		t.Fatalf("GetRejectedPlayers failed: %v", err)
	}
	if len(rejectedList) != 1 || rejectedList[0].Count != 2 {
		t.Errorf("Expected count 2 after second attempt, got %d", rejectedList[0].Count)
	}

	// 6. Test Delete Rejected Player
	if err := store.DeleteRejectedPlayer("Griefer123"); err != nil {
		t.Fatalf("DeleteRejectedPlayer failed: %v", err)
	}

	rejectedList, err = store.GetRejectedPlayers()
	if err != nil {
		t.Fatalf("GetRejectedPlayers failed after delete: %v", err)
	}
	if len(rejectedList) != 0 {
		t.Errorf("Expected empty rejected player list after delete, got %d items", len(rejectedList))
	}
}
