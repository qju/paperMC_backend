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

	// 3b. Test List Users
	users, err := store.ListUsers()
	if err != nil {
		t.Fatalf("ListUsers failed: %v", err)
	}
	if len(users) != 1 || users[0].Username != "admin_test" {
		t.Errorf("Unexpected user list: %+v", users)
	}

	// 3c. Test Update Password
	if err := store.UpdateUserPassword("admin_test", "new_hashed_pwd"); err != nil {
		t.Fatalf("UpdateUserPassword failed: %v", err)
	}
	updatedUser, _ := store.GetUser("admin_test")
	if updatedUser.Password != "new_hashed_pwd" {
		t.Errorf("Password was not updated, got %s", updatedUser.Password)
	}

	// 3d. Test Delete User
	if err := store.DeleteUser("admin_test"); err != nil {
		t.Fatalf("DeleteUser failed: %v", err)
	}
	usersAfterDel, _ := store.ListUsers()
	if len(usersAfterDel) != 0 {
		t.Errorf("Expected 0 users after deletion, got %d", len(usersAfterDel))
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

func TestSQLiteStoreSchedulesAndLogs(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test_sched.db")

	store, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to initialize SQLite store: %v", err)
	}
	defer store.Close()

	// 1. Create Schedule
	sched := &Schedule{
		Name:       "Daily World Backup",
		CronExpr:   "0 4 * * *",
		ActionType: "backup",
		Payload:    `{"type":"world","world_name":"world"}`,
		IsEnabled:  true,
	}

	if err := store.CreateSchedule(sched); err != nil {
		t.Fatalf("CreateSchedule failed: %v", err)
	}
	if sched.ID <= 0 {
		t.Fatalf("Expected positive schedule ID, got %d", sched.ID)
	}

	// 2. Get Schedule
	fetched, err := store.GetSchedule(sched.ID)
	if err != nil {
		t.Fatalf("GetSchedule failed: %v", err)
	}
	if fetched.Name != sched.Name || fetched.CronExpr != sched.CronExpr || !fetched.IsEnabled {
		t.Errorf("Fetched schedule mismatch: %+v", fetched)
	}

	// 3. List Schedules
	list, err := store.ListSchedules()
	if err != nil {
		t.Fatalf("ListSchedules failed: %v", err)
	}
	if len(list) != 1 || list[0].ID != sched.ID {
		t.Fatalf("Expected 1 schedule in list, got %d", len(list))
	}

	// 4. Update Schedule
	sched.Name = "Updated Nightly Backup"
	sched.CronExpr = "0 3 * * *"
	if err := store.UpdateSchedule(sched); err != nil {
		t.Fatalf("UpdateSchedule failed: %v", err)
	}

	updated, _ := store.GetSchedule(sched.ID)
	if updated.Name != "Updated Nightly Backup" || updated.CronExpr != "0 3 * * *" {
		t.Errorf("Schedule was not updated correctly: %+v", updated)
	}

	// 5. Toggle Schedule
	if err := store.ToggleSchedule(sched.ID, false); err != nil {
		t.Fatalf("ToggleSchedule failed: %v", err)
	}
	toggled, _ := store.GetSchedule(sched.ID)
	if toggled.IsEnabled {
		t.Errorf("Expected schedule to be disabled")
	}

	// 6. Record Execution Logs (Success & Failure)
	if err := store.RecordScheduleExecution(sched.ID, "success", 1250, ""); err != nil {
		t.Fatalf("RecordScheduleExecution success failed: %v", err)
	}
	if err := store.RecordScheduleExecution(sched.ID, "failed", 350, "Disk write error"); err != nil {
		t.Fatalf("RecordScheduleExecution failed attempt failed: %v", err)
	}

	// Verify schedule record has updated last_run stats
	afterRun, _ := store.GetSchedule(sched.ID)
	if afterRun.LastRunAt == nil || afterRun.LastRunStatus != "failed" || afterRun.LastRunError != "Disk write error" {
		t.Errorf("Unexpected last run metadata: %+v", afterRun)
	}

	// 7. List Logs
	logs, err := store.ListScheduleLogs(sched.ID, 50)
	if err != nil {
		t.Fatalf("ListScheduleLogs failed: %v", err)
	}
	if len(logs) != 2 {
		t.Fatalf("Expected 2 execution logs, got %d", len(logs))
	}
	if logs[0].Status != "failed" || logs[1].Status != "success" {
		t.Errorf("Logs ordering or status unexpected: %+v", logs)
	}

	// Global log query without scheduleID filter
	globalLogs, err := store.ListScheduleLogs(0, 10)
	if err != nil || len(globalLogs) != 2 {
		t.Fatalf("Global ListScheduleLogs failed: %v, len=%d", err, len(globalLogs))
	}

	// 8. Clear Logs
	if err := store.ClearScheduleLogs(sched.ID); err != nil {
		t.Fatalf("ClearScheduleLogs failed: %v", err)
	}
	logsAfterClear, _ := store.ListScheduleLogs(sched.ID, 50)
	if len(logsAfterClear) != 0 {
		t.Errorf("Expected 0 logs after clear, got %d", len(logsAfterClear))
	}

	// 9. Delete Schedule
	if err := store.DeleteSchedule(sched.ID); err != nil {
		t.Fatalf("DeleteSchedule failed: %v", err)
	}
	_, err = store.GetSchedule(sched.ID)
	if !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("Expected sql.ErrNoRows after deletion, got %v", err)
	}
}

func TestSQLiteStoreServerFlags(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test_flags.db")

	store, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to initialize SQLite store: %v", err)
	}
	defer store.Close()

	// 1. Initial seeded server flags from migration v3
	flags, err := store.GetServerFlags()
	if err != nil {
		t.Fatalf("GetServerFlags failed: %v", err)
	}
	if flags.RAM != "8G" || flags.Preset != "aikar" {
		t.Errorf("Expected default 8G and aikar, got %+v", flags)
	}

	// 2. Save new server flags
	newFlags := &ServerFlags{
		RAM:         "16G",
		Preset:      "aikar",
		CustomFlags: "-Dmy.custom.flag=true",
	}
	if err := store.SaveServerFlags(newFlags); err != nil {
		t.Fatalf("SaveServerFlags failed: %v", err)
	}

	// 3. Fetch updated flags
	updated, err := store.GetServerFlags()
	if err != nil {
		t.Fatalf("GetServerFlags after save failed: %v", err)
	}
	if updated.RAM != "16G" || updated.Preset != "aikar" || updated.CustomFlags != "-Dmy.custom.flag=true" {
		t.Errorf("Updated flags mismatch: %+v", updated)
	}
}

