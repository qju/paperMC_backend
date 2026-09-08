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

func TestSQLiteStoreAuditLogs(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test_audit.db")

	store, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to initialize SQLite store: %v", err)
	}
	defer store.Close()

	// 1. Initially empty
	logs, total, err := store.ListAuditLogs(50, 0, "", "")
	if err != nil {
		t.Fatalf("ListAuditLogs on empty table failed: %v", err)
	}
	if total != 0 || len(logs) != 0 {
		t.Errorf("Expected 0 logs initially, got total=%d len=%d", total, len(logs))
	}

	// 2. Insert records
	entries := []*AuditLog{
		{
			Username:   "admin",
			Action:     "server.start",
			Endpoint:   "/start",
			Method:     "POST",
			Details:    "Server initiated",
			IPAddress:  "127.0.0.1",
			StatusCode: 200,
		},
		{
			Username:   "admin",
			Action:     "server.stop",
			Endpoint:   "/stop",
			Method:     "POST",
			Details:    "Server halted",
			IPAddress:  "127.0.0.1",
			StatusCode: 200,
		},
		{
			Username:   "operator1",
			Action:     "player.whitelist_add",
			Endpoint:   "/api/players",
			Method:     "POST",
			Details:    "Player Steve whitelisted",
			IPAddress:  "192.168.1.50",
			StatusCode: 200,
		},
		{
			Username:   "unknown",
			Action:     "auth.login_failed",
			Endpoint:   "/login",
			Method:     "POST",
			Details:    "Invalid credentials",
			IPAddress:  "10.0.0.1",
			StatusCode: 401,
		},
	}

	for _, e := range entries {
		if err := store.RecordAuditLog(e); err != nil {
			t.Fatalf("RecordAuditLog failed for %s: %v", e.Action, err)
		}
		if e.ID <= 0 {
			t.Errorf("Expected positive ID assigned to recorded log, got %d", e.ID)
		}
	}

	// 3. List all logs with pagination
	allLogs, total, err := store.ListAuditLogs(10, 0, "", "")
	if err != nil {
		t.Fatalf("ListAuditLogs failed: %v", err)
	}
	if total != 4 || len(allLogs) != 4 {
		t.Fatalf("Expected 4 logs, got total=%d len=%d", total, len(allLogs))
	}
	// Verify DESC ordering by ID
	if allLogs[0].ID < allLogs[1].ID {
		t.Errorf("Expected descending ordering by ID: first=%d second=%d", allLogs[0].ID, allLogs[1].ID)
	}

	// 4. Test Limit & Offset
	page1, total, err := store.ListAuditLogs(2, 0, "", "")
	if err != nil || total != 4 || len(page1) != 2 {
		t.Errorf("Expected 2 logs for limit=2, got len=%d total=%d", len(page1), total)
	}
	page2, total, err := store.ListAuditLogs(2, 2, "", "")
	if err != nil || total != 4 || len(page2) != 2 {
		t.Errorf("Expected 2 logs for page 2, got len=%d total=%d", len(page2), total)
	}
	if page1[0].ID == page2[0].ID {
		t.Errorf("Page 1 and Page 2 should have distinct items")
	}

	// 5. Test Action Filter prefix
	serverLogs, count, err := store.ListAuditLogs(50, 0, "server", "")
	if err != nil {
		t.Fatalf("ListAuditLogs with server filter failed: %v", err)
	}
	if count != 2 || len(serverLogs) != 2 {
		t.Errorf("Expected 2 server logs, got count=%d len=%d", count, len(serverLogs))
	}

	// 6. Test Action Filter exact / wildcard
	exactLogs, count, err := store.ListAuditLogs(50, 0, "auth.login_failed", "")
	if err != nil || count != 1 || len(exactLogs) != 1 {
		t.Errorf("Expected 1 exact match for auth.login_failed, got count=%d len=%d", count, len(exactLogs))
	}

	wildcardLogs, count, err := store.ListAuditLogs(50, 0, "%whitelist%", "")
	if err != nil || count != 1 || len(wildcardLogs) != 1 {
		t.Errorf("Expected 1 wildcard match for %%whitelist%%, got count=%d len=%d", count, len(wildcardLogs))
	}

	// 7. Test User Filter
	userLogs, count, err := store.ListAuditLogs(50, 0, "", "operator1")
	if err != nil || count != 1 || len(userLogs) != 1 {
		t.Errorf("Expected 1 log for operator1, got count=%d len=%d", count, len(userLogs))
	}

	// 8. Test Clear Audit Logs
	if err := store.ClearAuditLogs(); err != nil {
		t.Fatalf("ClearAuditLogs failed: %v", err)
	}
	afterClear, totalAfter, err := store.ListAuditLogs(50, 0, "", "")
	if err != nil || totalAfter != 0 || len(afterClear) != 0 {
		t.Errorf("Expected 0 logs after clear, got total=%d len=%d", totalAfter, len(afterClear))
	}
}

func TestSQLiteStoreCrashReportsAndAI(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test_crash_ai.db")

	store, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to initialize SQLite store: %v", err)
	}
	defer store.Close()

	// 1. Initially empty crash reports
	reports, total, err := store.ListCrashReports(50, 0)
	if err != nil {
		t.Fatalf("ListCrashReports failed: %v", err)
	}
	if total != 0 || len(reports) != 0 {
		t.Errorf("Expected 0 crash reports initially, got total=%d, len=%d", total, len(reports))
	}

	// 2. Insert crash reports
	r1 := &CrashReport{
		Source:         "runtime",
		Category:       "OutOfMemory",
		Title:          "Server ran out of memory (Java heap space)",
		Culprit:        "JVM Heap Exhaustion",
		Summary:        "The JVM crashed with java.lang.OutOfMemoryError",
		Recommendation: "Increase allocated RAM in Server Flags to at least 4GB.",
		RawLog:         "java.lang.OutOfMemoryError: Java heap space\n\tat net.minecraft.world...",
	}
	if err := store.RecordCrashReport(r1); err != nil {
		t.Fatalf("RecordCrashReport r1 failed: %v", err)
	}
	if r1.ID <= 0 {
		t.Errorf("Expected valid ID for r1, got %d", r1.ID)
	}

	r2 := &CrashReport{
		Source:         "crash_file",
		Category:       "PortConflict",
		Title:          "Port already in use",
		Culprit:        "Bind failed on 25565",
		Summary:        "Failed to bind to port 25565",
		Recommendation: "Check if another server is running or change server-port in server.properties.",
		RawLog:         "FAILED TO BIND TO PORT! Address already in use: bind",
	}
	if err := store.RecordCrashReport(r2); err != nil {
		t.Fatalf("RecordCrashReport r2 failed: %v", err)
	}

	// 3. List crash reports with pagination
	list, total, err := store.ListCrashReports(10, 0)
	if err != nil || total != 2 || len(list) != 2 {
		t.Fatalf("Expected 2 crash reports, got total=%d, len=%d, err=%v", total, len(list), err)
	}
	if list[0].ID != r2.ID {
		t.Errorf("Expected newest report first (r2 ID %d), got %d", r2.ID, list[0].ID)
	}

	// 4. Get individual crash report
	fetched, err := store.GetCrashReport(r1.ID)
	if err != nil {
		t.Fatalf("GetCrashReport failed: %v", err)
	}
	if fetched == nil || fetched.Category != "OutOfMemory" {
		t.Errorf("Fetched report unexpected: %+v", fetched)
	}

	// Non-existent crash report
	nonExistent, err := store.GetCrashReport(99999)
	if err != nil || nonExistent != nil {
		t.Errorf("Expected nil report for non-existent ID, got err=%v, report=%v", err, nonExistent)
	}

	// 5. Delete individual crash report
	if err := store.DeleteCrashReport(r1.ID); err != nil {
		t.Fatalf("DeleteCrashReport failed: %v", err)
	}
	afterDel, totalDel, err := store.ListCrashReports(10, 0)
	if err != nil || totalDel != 1 || len(afterDel) != 1 {
		t.Fatalf("Expected 1 report after delete, got total=%d, len=%d", totalDel, len(afterDel))
	}

	// 6. Clear all crash reports
	if err := store.ClearCrashReports(); err != nil {
		t.Fatalf("ClearCrashReports failed: %v", err)
	}
	afterClear, totalClear, err := store.ListCrashReports(10, 0)
	if err != nil || totalClear != 0 || len(afterClear) != 0 {
		t.Fatalf("Expected 0 reports after clear, got total=%d, len=%d", totalClear, len(afterClear))
	}

	// 7. AI Settings default
	aiSettings, err := store.GetAISettings()
	if err != nil {
		t.Fatalf("GetAISettings failed: %v", err)
	}
	if aiSettings.Provider != "openai" || aiSettings.Model != "gpt-4o-mini" || aiSettings.IsEnabled {
		t.Errorf("Default AI settings mismatch: %+v", aiSettings)
	}

	// 8. Save updated AI Settings
	aiSettings.Provider = "gemini"
	aiSettings.APIKey = "test-api-key-123"
	aiSettings.Model = "gemini-1.5-flash"
	aiSettings.BaseURL = "https://custom.endpoint"
	aiSettings.IsEnabled = true
	if err := store.SaveAISettings(aiSettings); err != nil {
		t.Fatalf("SaveAISettings failed: %v", err)
	}

	// 9. Fetch saved AI Settings
	savedAI, err := store.GetAISettings()
	if err != nil {
		t.Fatalf("GetAISettings after save failed: %v", err)
	}
	if savedAI.Provider != "gemini" || savedAI.APIKey != "test-api-key-123" || savedAI.Model != "gemini-1.5-flash" || savedAI.BaseURL != "https://custom.endpoint" || !savedAI.IsEnabled {
		t.Errorf("Saved AI settings mismatch: %+v", savedAI)
	}
}



