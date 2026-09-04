package scheduler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"paperMC_backend/internal/database"
	"paperMC_backend/internal/minecraft"
)

type mockMCServer struct {
	status        minecraft.Status
	commandsSent  []string
	broadcasts    []string
	stopCalled    bool
	startCalled   bool
	stopErr       error
	startErr      error
}

func (m *mockMCServer) GetStatus() minecraft.Status {
	return m.status
}

func (m *mockMCServer) SendCommand(cmd string) error {
	m.commandsSent = append(m.commandsSent, cmd)
	return nil
}

func (m *mockMCServer) Broadcast(msg string) {
	m.broadcasts = append(m.broadcasts, msg)
}

func (m *mockMCServer) Stop() error {
	m.stopCalled = true
	m.status = minecraft.StatusStopped
	return m.stopErr
}

func (m *mockMCServer) Start() error {
	m.startCalled = true
	m.status = minecraft.StatusRunning
	return m.startErr
}

func TestValidateCronExpression(t *testing.T) {
	validSpecs := []string{
		"* * * * *",
		"0 4 * * *",
		"*/15 * * * *",
		"0 0 1 1 *",
		"@daily",
		"@hourly",
		"@weekly",
	}
	for _, spec := range validSpecs {
		if err := ValidateCronExpression(spec); err != nil {
			t.Errorf("Expected valid cron spec for '%s', got: %v", spec, err)
		}
	}

	invalidSpecs := []string{
		"invalid",
		"* * *",
		"60 * * * *",
		"* 25 * * *",
	}
	for _, spec := range invalidSpecs {
		if err := ValidateCronExpression(spec); err == nil {
			t.Errorf("Expected error for invalid cron spec '%s'", spec)
		}
	}
}

func TestSchedulerServiceLifecycleAndExecution(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test_sched.db")

	store, err := database.NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to init SQLite: %v", err)
	}
	defer store.Close()

	// Create mock world directory for backup testing
	worldDir := filepath.Join(tempDir, "world")
	_ = os.MkdirAll(worldDir, 0755)
	_ = os.WriteFile(filepath.Join(worldDir, "level.dat"), []byte("mock-data"), 0644)

	mockServer := &mockMCServer{
		status: minecraft.StatusRunning,
	}

	service := NewService(store, mockServer, tempDir)

	// 1. Register schedules in DB
	schedBackup := &database.Schedule{
		Name:       "Test Backup Job",
		CronExpr:   "0 4 * * *",
		ActionType: "backup",
		Payload:    `{"type":"world","world_name":"world"}`,
		IsEnabled:  true,
	}
	if err := store.CreateSchedule(schedBackup); err != nil {
		t.Fatalf("Failed to create backup schedule: %v", err)
	}

	schedCmd := &database.Schedule{
		Name:       "Clear Weather Job",
		CronExpr:   "*/30 * * * *",
		ActionType: "command",
		Payload:    "weather clear",
		IsEnabled:  true,
	}
	_ = store.CreateSchedule(schedCmd)

	schedBroadcast := &database.Schedule{
		Name:       "Announcement Job",
		CronExpr:   "@hourly",
		ActionType: "broadcast",
		Payload:    "Welcome to the server!",
		IsEnabled:  true,
	}
	_ = store.CreateSchedule(schedBroadcast)

	schedRestart := &database.Schedule{
		Name:       "Nightly Restart",
		CronExpr:   "0 5 * * *",
		ActionType: "restart",
		IsEnabled:  true,
	}
	_ = store.CreateSchedule(schedRestart)

	schedPower := &database.Schedule{
		Name:       "Off-peak Stop",
		CronExpr:   "0 1 * * *",
		ActionType: "stop",
		IsEnabled:  false, // disabled
	}
	_ = store.CreateSchedule(schedPower)

	// 2. Start Service
	if err := service.Start(); err != nil {
		t.Fatalf("Service Start failed: %v", err)
	}
	defer service.Stop()

	// Verify next run times
	nextRun := service.GetNextRun(schedBackup.ID)
	if nextRun == nil || nextRun.IsZero() {
		t.Error("Expected valid next run time for backup job")
	}

	// Disabled schedule should have no next run
	if service.GetNextRun(schedPower.ID) != nil {
		t.Error("Expected nil next run for disabled schedule")
	}

	// 3. Execute Job: Backup
	if err := service.ExecuteJob(schedBackup.ID); err != nil {
		t.Fatalf("ExecuteJob backup failed: %v", err)
	}

	// 4. Execute Job: Command
	if err := service.ExecuteJob(schedCmd.ID); err != nil {
		t.Fatalf("ExecuteJob command failed: %v", err)
	}

	// 5. Execute Job: Broadcast
	if err := service.ExecuteJob(schedBroadcast.ID); err != nil {
		t.Fatalf("ExecuteJob broadcast failed: %v", err)
	}

	// 6. Execute Job: Restart
	if err := service.ExecuteJob(schedRestart.ID); err != nil {
		t.Fatalf("ExecuteJob restart failed: %v", err)
	}
	if !mockServer.stopCalled || !mockServer.startCalled {
		t.Errorf("Expected stop and start called during restart job")
	}

	// 7. Verify Execution Logs in DB
	logs, err := store.ListScheduleLogs(0, 50)
	if err != nil {
		t.Fatalf("ListScheduleLogs failed: %v", err)
	}
	if len(logs) != 4 {
		t.Errorf("Expected 4 execution logs recorded, got %d", len(logs))
	}
	for _, l := range logs {
		if l.Status != "success" {
			t.Errorf("Expected success status, got %s: %s", l.Status, l.ErrorMessage)
		}
	}

	// 8. Test Execution Failure Handling
	schedFail := &database.Schedule{
		Name:       "Failing Action Job",
		CronExpr:   "0 0 * * *",
		ActionType: "command",
		Payload:    "", // empty command returns error
		IsEnabled:  true,
	}
	_ = store.CreateSchedule(schedFail)
	err = service.ExecuteJob(schedFail.ID)
	if err == nil {
		t.Fatal("Expected error for empty command payload")
	}

	failLogs, _ := store.ListScheduleLogs(schedFail.ID, 1)
	if len(failLogs) != 1 || failLogs[0].Status != "failed" || !strings.Contains(failLogs[0].ErrorMessage, "cannot be empty") {
		t.Errorf("Expected failure log recorded properly, got: %+v", failLogs)
	}

	// 9. Test Dynamic Register / Unregister / Reload
	schedDynamic := database.Schedule{
		ID:         999,
		Name:       "Dynamic Job",
		CronExpr:   "*/5 * * * *",
		ActionType: "broadcast",
		Payload:    "Test",
		IsEnabled:  true,
	}
	if err := service.RegisterSchedule(schedDynamic); err != nil {
		t.Fatalf("RegisterSchedule failed: %v", err)
	}
	if service.GetNextRun(999) == nil {
		t.Error("Expected active next run for registered schedule")
	}

	service.UnregisterSchedule(999)
	if service.GetNextRun(999) != nil {
		t.Error("Expected nil next run after UnregisterSchedule")
	}

	if err := service.Reload(); err != nil {
		t.Fatalf("Reload failed: %v", err)
	}
}
