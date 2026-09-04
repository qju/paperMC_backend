package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"testing"

	"paperMC_backend/internal/database"
	"paperMC_backend/internal/minecraft"
)

func setupTestScheduleEnvironment(t *testing.T) (database.Store, *Handler, func()) {
	t.Helper()
	tempDir := t.TempDir()
	setupTestWorldForAPI(t, tempDir, "world")
	dbPath := filepath.Join(tempDir, "test.db")
	store, err := database.NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create sqlite store: %v", err)
	}

	mcServer := minecraft.NewServer(tempDir, "server.jar", "1G", nil)
	handler := NewServerHandler(mcServer, store)
	// Scheduler is automatically initialized by NewServerHandler
	if err := handler.StartScheduler(); err != nil {
		t.Fatalf("Failed to start scheduler: %v", err)
	}

	cleanup := func() {
		handler.StopScheduler()
		_ = store.Close()
	}

	return store, handler, cleanup
}

func TestSchedulesAPI_StoreUnavailable(t *testing.T) {
	handler := NewServerHandler(nil, nil)

	// List
	req := httptest.NewRequest(http.MethodGet, "/api/schedules", nil)
	w := httptest.NewRecorder()
	handler.HandleListSchedules(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected 500 when store is nil, got %d", w.Code)
	}

	// Create
	req = httptest.NewRequest(http.MethodPost, "/api/schedules", bytes.NewBufferString("{}"))
	w = httptest.NewRecorder()
	handler.HandleCreateSchedule(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected 500 when store is nil, got %d", w.Code)
	}

	// Update
	req = httptest.NewRequest(http.MethodPut, "/api/schedules", bytes.NewBufferString("{}"))
	w = httptest.NewRecorder()
	handler.HandleUpdateSchedule(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected 500 when store is nil, got %d", w.Code)
	}

	// Toggle
	req = httptest.NewRequest(http.MethodPost, "/api/schedules/toggle?id=1", nil)
	w = httptest.NewRecorder()
	handler.HandleToggleSchedule(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected 500 when store is nil, got %d", w.Code)
	}

	// Run
	req = httptest.NewRequest(http.MethodPost, "/api/schedules/run?id=1", nil)
	w = httptest.NewRecorder()
	handler.HandleRunSchedule(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected 500 when store is nil, got %d", w.Code)
	}

	// Delete
	req = httptest.NewRequest(http.MethodDelete, "/api/schedules?id=1", nil)
	w = httptest.NewRecorder()
	handler.HandleDeleteSchedule(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected 500 when store is nil, got %d", w.Code)
	}

	// Get Logs
	req = httptest.NewRequest(http.MethodGet, "/api/schedules/logs", nil)
	w = httptest.NewRecorder()
	handler.HandleGetScheduleLogs(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected 500 when store is nil, got %d", w.Code)
	}

	// Clear Logs
	req = httptest.NewRequest(http.MethodDelete, "/api/schedules/logs", nil)
	w = httptest.NewRecorder()
	handler.HandleClearScheduleLogs(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected 500 when store is nil, got %d", w.Code)
	}
}

func TestSchedulesAPI_CreateAndList(t *testing.T) {
	_, handler, cleanup := setupTestScheduleEnvironment(t)
	defer cleanup()

	// 1. Initial list should be empty
	req := httptest.NewRequest(http.MethodGet, "/api/schedules", nil)
	w := httptest.NewRecorder()
	handler.HandleListSchedules(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", w.Code)
	}
	var listResp struct {
		Schedules []database.Schedule `json:"schedules"`
	}
	_ = json.NewDecoder(w.Body).Decode(&listResp)
	if len(listResp.Schedules) != 0 {
		t.Errorf("Expected 0 schedules initially, got %d", len(listResp.Schedules))
	}

	// 2. Bad JSON
	req = httptest.NewRequest(http.MethodPost, "/api/schedules", bytes.NewBufferString("invalid-json"))
	w = httptest.NewRecorder()
	handler.HandleCreateSchedule(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 on bad JSON, got %d", w.Code)
	}

	// 3. Empty name
	reqBody, _ := json.Marshal(CreateScheduleRequest{
		Name:       "",
		CronExpr:   "0 4 * * *",
		ActionType: "backup",
	})
	req = httptest.NewRequest(http.MethodPost, "/api/schedules", bytes.NewBuffer(reqBody))
	w = httptest.NewRecorder()
	handler.HandleCreateSchedule(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 on empty name, got %d", w.Code)
	}

	// 4. Invalid cron
	reqBody, _ = json.Marshal(CreateScheduleRequest{
		Name:       "Nightly Backup",
		CronExpr:   "invalid-cron-expr",
		ActionType: "backup",
	})
	req = httptest.NewRequest(http.MethodPost, "/api/schedules", bytes.NewBuffer(reqBody))
	w = httptest.NewRecorder()
	handler.HandleCreateSchedule(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 on invalid cron, got %d", w.Code)
	}

	// 5. Invalid action type
	reqBody, _ = json.Marshal(CreateScheduleRequest{
		Name:       "Nightly Backup",
		CronExpr:   "0 4 * * *",
		ActionType: "not_a_real_action",
	})
	req = httptest.NewRequest(http.MethodPost, "/api/schedules", bytes.NewBuffer(reqBody))
	w = httptest.NewRecorder()
	handler.HandleCreateSchedule(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 on invalid action, got %d", w.Code)
	}

	// 6. Successful creation
	enabled := true
	reqBody, _ = json.Marshal(CreateScheduleRequest{
		Name:       "Nightly World Backup",
		CronExpr:   "0 4 * * *",
		ActionType: "backup",
		Payload:    "world",
		IsEnabled:  &enabled,
	})
	req = httptest.NewRequest(http.MethodPost, "/api/schedules", bytes.NewBuffer(reqBody))
	w = httptest.NewRecorder()
	handler.HandleCreateSchedule(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("Expected 201 on valid schedule create, got %d: %s", w.Code, w.Body.String())
	}

	var created database.Schedule
	_ = json.NewDecoder(w.Body).Decode(&created)
	if created.ID == 0 || created.Name != "Nightly World Backup" || !created.IsEnabled {
		t.Fatalf("Created schedule fields invalid: %+v", created)
	}
	if created.NextRunAt == nil {
		t.Errorf("Expected NextRunAt to be populated for enabled schedule")
	}

	// 7. Verify schedule shows up in HandleListSchedules
	req = httptest.NewRequest(http.MethodGet, "/api/schedules", nil)
	w = httptest.NewRecorder()
	handler.HandleListSchedules(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", w.Code)
	}
	_ = json.NewDecoder(w.Body).Decode(&listResp)
	if len(listResp.Schedules) != 1 {
		t.Fatalf("Expected 1 schedule in list, got %d", len(listResp.Schedules))
	}
	if listResp.Schedules[0].NextRunAt == nil {
		t.Errorf("Expected NextRunAt populated in list response")
	}
}

func TestSchedulesAPI_UpdateAndToggle(t *testing.T) {
	_, handler, cleanup := setupTestScheduleEnvironment(t)
	defer cleanup()

	// Create initial schedule
	reqBody, _ := json.Marshal(CreateScheduleRequest{
		Name:       "Daily Restart",
		CronExpr:   "0 6 * * *",
		ActionType: "restart",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/schedules", bytes.NewBuffer(reqBody))
	w := httptest.NewRecorder()
	handler.HandleCreateSchedule(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("Expected 201, got %d", w.Code)
	}
	var created database.Schedule
	_ = json.NewDecoder(w.Body).Decode(&created)

	// Update with invalid JSON
	req = httptest.NewRequest(http.MethodPut, "/api/schedules", bytes.NewBufferString("{bad"))
	w = httptest.NewRecorder()
	handler.HandleUpdateSchedule(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 on bad JSON update, got %d", w.Code)
	}

	// Update with invalid ID
	updateBadID, _ := json.Marshal(UpdateScheduleRequest{
		ID:         0,
		Name:       "Test",
		CronExpr:   "0 0 * * *",
		ActionType: "restart",
	})
	req = httptest.NewRequest(http.MethodPut, "/api/schedules", bytes.NewBuffer(updateBadID))
	w = httptest.NewRecorder()
	handler.HandleUpdateSchedule(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 on 0 ID, got %d", w.Code)
	}

	// Update with empty name
	updateEmptyName, _ := json.Marshal(UpdateScheduleRequest{
		ID:         created.ID,
		Name:       "",
		CronExpr:   "0 0 * * *",
		ActionType: "restart",
	})
	req = httptest.NewRequest(http.MethodPut, "/api/schedules", bytes.NewBuffer(updateEmptyName))
	w = httptest.NewRecorder()
	handler.HandleUpdateSchedule(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 on empty name, got %d", w.Code)
	}

	// Update with invalid cron
	updateBadCron, _ := json.Marshal(UpdateScheduleRequest{
		ID:         created.ID,
		Name:       "Test",
		CronExpr:   "bad-cron",
		ActionType: "restart",
	})
	req = httptest.NewRequest(http.MethodPut, "/api/schedules", bytes.NewBuffer(updateBadCron))
	w = httptest.NewRecorder()
	handler.HandleUpdateSchedule(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 on bad cron, got %d", w.Code)
	}

	// Update with invalid action
	updateBadAction, _ := json.Marshal(UpdateScheduleRequest{
		ID:         created.ID,
		Name:       "Test",
		CronExpr:   "0 0 * * *",
		ActionType: "bad-action",
	})
	req = httptest.NewRequest(http.MethodPut, "/api/schedules", bytes.NewBuffer(updateBadAction))
	w = httptest.NewRecorder()
	handler.HandleUpdateSchedule(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 on bad action, got %d", w.Code)
	}

	// Valid update: change cron and set is_enabled = false
	updateValid, _ := json.Marshal(UpdateScheduleRequest{
		ID:         created.ID,
		Name:       "Daily Morning Restart",
		CronExpr:   "30 5 * * *",
		ActionType: "restart",
		Payload:    "10",
		IsEnabled:  false,
	})
	req = httptest.NewRequest(http.MethodPut, "/api/schedules", bytes.NewBuffer(updateValid))
	w = httptest.NewRecorder()
	handler.HandleUpdateSchedule(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200 on valid update, got %d: %s", w.Code, w.Body.String())
	}

	var updated database.Schedule
	_ = json.NewDecoder(w.Body).Decode(&updated)
	if updated.Name != "Daily Morning Restart" || updated.IsEnabled != false {
		t.Errorf("Schedule update mismatch: %+v", updated)
	}

	// Toggle back to enabled using JSON body
	toggleBody, _ := json.Marshal(ScheduleActionRequest{ID: created.ID})
	req = httptest.NewRequest(http.MethodPost, "/api/schedules/toggle", bytes.NewBuffer(toggleBody))
	w = httptest.NewRecorder()
	handler.HandleToggleSchedule(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200 on toggle, got %d: %s", w.Code, w.Body.String())
	}
	var toggled database.Schedule
	_ = json.NewDecoder(w.Body).Decode(&toggled)
	if !toggled.IsEnabled {
		t.Errorf("Expected schedule to be enabled after toggle")
	}
	if toggled.NextRunAt == nil {
		t.Errorf("Expected NextRunAt to be populated after toggling to enabled")
	}

	// Toggle using query parameter back to disabled
	req = httptest.NewRequest(http.MethodPost, "/api/schedules/toggle?id="+strconv.Itoa(created.ID), nil)
	w = httptest.NewRecorder()
	handler.HandleToggleSchedule(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200 on query param toggle, got %d", w.Code)
	}
	toggled = database.Schedule{}
	_ = json.NewDecoder(w.Body).Decode(&toggled)
	if toggled.IsEnabled {
		t.Errorf("Expected schedule to be disabled after second toggle")
	}
	if toggled.NextRunAt != nil {
		t.Errorf("Expected NextRunAt to be nil when disabled")
	}

	// Toggle invalid ID
	req = httptest.NewRequest(http.MethodPost, "/api/schedules/toggle?id=99999", nil)
	w = httptest.NewRecorder()
	handler.HandleToggleSchedule(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("Expected 404 for non-existent schedule, got %d", w.Code)
	}

	// Toggle without ID
	req = httptest.NewRequest(http.MethodPost, "/api/schedules/toggle", nil)
	w = httptest.NewRecorder()
	handler.HandleToggleSchedule(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 for missing ID, got %d", w.Code)
	}
}

func TestSchedulesAPI_RunNowAndDelete(t *testing.T) {
	_, handler, cleanup := setupTestScheduleEnvironment(t)
	defer cleanup()

	// Create a backup schedule
	reqBody, _ := json.Marshal(CreateScheduleRequest{
		Name:       "World Backup",
		CronExpr:   "*/10 * * * *",
		ActionType: "backup",
		Payload:    "world",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/schedules", bytes.NewBuffer(reqBody))
	w := httptest.NewRecorder()
	handler.HandleCreateSchedule(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("Expected 201, got %d", w.Code)
	}
	var created database.Schedule
	_ = json.NewDecoder(w.Body).Decode(&created)

	// Run without ID
	req = httptest.NewRequest(http.MethodPost, "/api/schedules/run", nil)
	w = httptest.NewRecorder()
	handler.HandleRunSchedule(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 for Run without ID, got %d", w.Code)
	}

	// Run with non-existent ID
	req = httptest.NewRequest(http.MethodPost, "/api/schedules/run?id=99999", nil)
	w = httptest.NewRecorder()
	handler.HandleRunSchedule(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected 500 when running non-existent task, got %d", w.Code)
	}

	// Run valid schedule
	req = httptest.NewRequest(http.MethodPost, "/api/schedules/run?id="+strconv.Itoa(created.ID), nil)
	w = httptest.NewRecorder()
	handler.HandleRunSchedule(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200 on manual run, got %d: %s", w.Code, w.Body.String())
	}

	// Verify log was created
	req = httptest.NewRequest(http.MethodGet, "/api/schedules/logs?schedule_id="+strconv.Itoa(created.ID), nil)
	w = httptest.NewRecorder()
	handler.HandleGetScheduleLogs(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200 on logs query, got %d", w.Code)
	}
	var logsResp struct {
		Logs []database.ScheduleLog `json:"logs"`
	}
	_ = json.NewDecoder(w.Body).Decode(&logsResp)
	if len(logsResp.Logs) != 1 {
		t.Fatalf("Expected 1 log after run, got %d", len(logsResp.Logs))
	}
	if logsResp.Logs[0].Status != "success" {
		t.Errorf("Expected log status 'success', got %s", logsResp.Logs[0].Status)
	}

	// Delete without ID
	req = httptest.NewRequest(http.MethodDelete, "/api/schedules", nil)
	w = httptest.NewRecorder()
	handler.HandleDeleteSchedule(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 on delete without ID, got %d", w.Code)
	}

	// Delete valid schedule
	req = httptest.NewRequest(http.MethodDelete, "/api/schedules?id="+strconv.Itoa(created.ID), nil)
	w = httptest.NewRecorder()
	handler.HandleDeleteSchedule(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200 on valid delete, got %d", w.Code)
	}

	// Verify list is now empty
	req = httptest.NewRequest(http.MethodGet, "/api/schedules", nil)
	w = httptest.NewRecorder()
	handler.HandleListSchedules(w, req)
	var listResp struct {
		Schedules []database.Schedule `json:"schedules"`
	}
	_ = json.NewDecoder(w.Body).Decode(&listResp)
	if len(listResp.Schedules) != 0 {
		t.Errorf("Expected 0 schedules after delete, got %d", len(listResp.Schedules))
	}

	// Cascading delete should also remove schedule logs
	req = httptest.NewRequest(http.MethodGet, "/api/schedules/logs", nil)
	w = httptest.NewRecorder()
	handler.HandleGetScheduleLogs(w, req)
	_ = json.NewDecoder(w.Body).Decode(&logsResp)
	if len(logsResp.Logs) != 0 {
		t.Errorf("Expected 0 logs after cascade delete, got %d", len(logsResp.Logs))
	}
}

func TestSchedulesAPI_LogsAndPurge(t *testing.T) {
	store, handler, cleanup := setupTestScheduleEnvironment(t)
	defer cleanup()

	// Insert test schedule and logs directly
	sched := database.Schedule{
		Name:       "Test Sched",
		CronExpr:   "0 0 * * *",
		ActionType: "backup",
		IsEnabled:  true,
	}
	if err := store.CreateSchedule(&sched); err != nil {
		t.Fatalf("Failed to create schedule: %v", err)
	}

	// Insert 2 logs
	_ = store.RecordScheduleExecution(sched.ID, "success", 120, "")
	_ = store.RecordScheduleExecution(sched.ID, "failed", 45, "Disk full error")

	// Get logs with default limit
	req := httptest.NewRequest(http.MethodGet, "/api/schedules/logs", nil)
	w := httptest.NewRecorder()
	handler.HandleGetScheduleLogs(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200 on logs query, got %d", w.Code)
	}
	var logsResp struct {
		Logs []database.ScheduleLog `json:"logs"`
	}
	_ = json.NewDecoder(w.Body).Decode(&logsResp)
	if len(logsResp.Logs) != 2 {
		t.Fatalf("Expected 2 logs, got %d", len(logsResp.Logs))
	}

	// Get logs with custom limit
	req = httptest.NewRequest(http.MethodGet, "/api/schedules/logs?limit=1", nil)
	w = httptest.NewRecorder()
	handler.HandleGetScheduleLogs(w, req)
	_ = json.NewDecoder(w.Body).Decode(&logsResp)
	if len(logsResp.Logs) != 1 {
		t.Errorf("Expected 1 log with limit=1, got %d", len(logsResp.Logs))
	}

	// Clear logs for specific schedule
	req = httptest.NewRequest(http.MethodDelete, "/api/schedules/logs?schedule_id="+strconv.Itoa(sched.ID), nil)
	w = httptest.NewRecorder()
	handler.HandleClearScheduleLogs(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200 on clear logs, got %d", w.Code)
	}

	// Verify logs are cleared
	req = httptest.NewRequest(http.MethodGet, "/api/schedules/logs", nil)
	w = httptest.NewRecorder()
	handler.HandleGetScheduleLogs(w, req)
	_ = json.NewDecoder(w.Body).Decode(&logsResp)
	if len(logsResp.Logs) != 0 {
		t.Errorf("Expected 0 logs after clear, got %d", len(logsResp.Logs))
	}
}
