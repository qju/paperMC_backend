package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"paperMC_backend/internal/database"
	"paperMC_backend/internal/minecraft"
	"paperMC_backend/internal/profiler"
)

func setupTestProfilerHandler(t *testing.T) (*Handler, database.Store, *minecraft.Server) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test_profiler_api.db")
	store, err := database.NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to initialize test DB: %v", err)
	}

	mcServer := minecraft.NewServer(tempDir, "paper.jar", "4G", store)
	handler := NewServerHandler(mcServer, store)

	return handler, store, mcServer
}

type nopWriteCloser struct {
	*bytes.Buffer
}

func (n *nopWriteCloser) Close() error {
	return nil
}

func TestHandleListProfilerReports(t *testing.T) {
	handler, store, _ := setupTestProfilerHandler(t)

	// Seed reports
	r1 := &database.ProfilerReport{
		ReportType: "spark_profile",
		Title:      "Spark Session 1",
		URL:        "https://spark.lucko.me/abc111",
		Summary:    "Summary 1",
		RawOutput:  "Raw 1",
		CreatedAt:  time.Now().UTC(),
	}
	r2 := &database.ProfilerReport{
		ReportType: "timings",
		Title:      "Timings Session 2",
		URL:        "https://timings.aikar.co/?id=def222",
		Summary:    "Summary 2",
		RawOutput:  "Raw 2",
		CreatedAt:  time.Now().UTC(),
	}
	_ = store.RecordProfilerReport(r1)
	_ = store.RecordProfilerReport(r2)

	// 1. List all
	req := httptest.NewRequest("GET", "/api/profiler/reports?limit=10&offset=0", nil)
	w := httptest.NewRecorder()
	handler.HandleListProfilerReports(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Reports []database.ProfilerReport `json:"reports"`
		Total   int                       `json:"total"`
		Limit   int                       `json:"limit"`
		Offset  int                       `json:"offset"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode: %v", err)
	}
	if resp.Total != 2 || len(resp.Reports) != 2 {
		t.Errorf("Expected 2 reports, got total=%d, len=%d", resp.Total, len(resp.Reports))
	}

	// 2. Filter by type
	reqFilter := httptest.NewRequest("GET", "/api/profiler/reports?type=timings", nil)
	wFilter := httptest.NewRecorder()
	handler.HandleListProfilerReports(wFilter, reqFilter)

	var respFilter struct {
		Reports []database.ProfilerReport `json:"reports"`
		Total   int                       `json:"total"`
	}
	_ = json.NewDecoder(wFilter.Body).Decode(&respFilter)
	if respFilter.Total != 1 || respFilter.Reports[0].ReportType != "timings" {
		t.Errorf("Expected 1 timings report, got %d", respFilter.Total)
	}

	// 3. Nil store error
	nilHandler := &Handler{}
	wNil := httptest.NewRecorder()
	nilHandler.HandleListProfilerReports(wNil, req)
	if wNil.Code != http.StatusInternalServerError {
		t.Errorf("Expected 500 for nil store, got %d", wNil.Code)
	}
}

func TestHandleGetProfilerReport(t *testing.T) {
	handler, store, _ := setupTestProfilerHandler(t)

	report := &database.ProfilerReport{
		ReportType: "spark_profile",
		Title:      "Spark Session",
		URL:        "https://spark.lucko.me/get123",
		Summary:    "Captured spark",
		RawOutput:  "test output",
		CreatedAt:  time.Now().UTC(),
	}
	if err := store.RecordProfilerReport(report); err != nil {
		t.Fatalf("Failed to record: %v", err)
	}

	// 1. Success
	req := httptest.NewRequest("GET", "/api/profiler/reports/"+strconv.Itoa(report.ID), nil)
	req.SetPathValue("id", strconv.Itoa(report.ID))
	w := httptest.NewRecorder()
	handler.HandleGetProfilerReport(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK, got %d: %s", w.Code, w.Body.String())
	}
	var fetched database.ProfilerReport
	_ = json.NewDecoder(w.Body).Decode(&fetched)
	if fetched.ID != report.ID || fetched.URL != report.URL {
		t.Errorf("Report mismatch: %+v", fetched)
	}

	// 2. Not Found
	reqNF := httptest.NewRequest("GET", "/api/profiler/reports/99999", nil)
	reqNF.SetPathValue("id", "99999")
	wNF := httptest.NewRecorder()
	handler.HandleGetProfilerReport(wNF, reqNF)
	if wNF.Code != http.StatusNotFound {
		t.Errorf("Expected 404 for missing report, got %d", wNF.Code)
	}

	// 3. Invalid ID
	reqBad := httptest.NewRequest("GET", "/api/profiler/reports/abc", nil)
	reqBad.SetPathValue("id", "abc")
	wBad := httptest.NewRecorder()
	handler.HandleGetProfilerReport(wBad, reqBad)
	if wBad.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 for bad ID, got %d", wBad.Code)
	}

	// 4. Nil store
	nilHandler := &Handler{}
	wNil := httptest.NewRecorder()
	nilHandler.HandleGetProfilerReport(wNil, req)
	if wNil.Code != http.StatusInternalServerError {
		t.Errorf("Expected 500 for nil store, got %d", wNil.Code)
	}
}

func TestHandleDeleteProfilerReport(t *testing.T) {
	handler, store, _ := setupTestProfilerHandler(t)

	r1 := &database.ProfilerReport{
		ReportType: "spark_profile",
		Title:      "R1",
		URL:        "https://spark.lucko.me/del1",
		Summary:    "S1",
		RawOutput:  "O1",
		CreatedAt:  time.Now().UTC(),
	}
	r2 := &database.ProfilerReport{
		ReportType: "timings",
		Title:      "R2",
		URL:        "https://timings.aikar.co/?id=del2",
		Summary:    "S2",
		RawOutput:  "O2",
		CreatedAt:  time.Now().UTC(),
	}
	_ = store.RecordProfilerReport(r1)
	_ = store.RecordProfilerReport(r2)

	// 1. Delete single report
	reqDel := httptest.NewRequest("DELETE", "/api/profiler/reports/"+strconv.Itoa(r1.ID), nil)
	reqDel.SetPathValue("id", strconv.Itoa(r1.ID))
	wDel := httptest.NewRecorder()
	handler.HandleDeleteProfilerReport(wDel, reqDel)

	if wDel.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK, got %d: %s", wDel.Code, wDel.Body.String())
	}

	fetched, _ := store.GetProfilerReport(r1.ID)
	if fetched != nil {
		t.Errorf("Expected report 1 to be deleted, but still exists")
	}

	// 2. Clear all reports
	reqClear := httptest.NewRequest("DELETE", "/api/profiler/reports?id=all", nil)
	wClear := httptest.NewRecorder()
	handler.HandleDeleteProfilerReport(wClear, reqClear)

	if wClear.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK on clear, got %d", wClear.Code)
	}
	_, total, _ := store.ListProfilerReports(10, 0, "")
	if total != 0 {
		t.Errorf("Expected 0 reports after clear, got %d", total)
	}

	// 3. Bad ID
	reqBad := httptest.NewRequest("DELETE", "/api/profiler/reports/invalid", nil)
	reqBad.SetPathValue("id", "invalid")
	wBad := httptest.NewRecorder()
	handler.HandleDeleteProfilerReport(wBad, reqBad)
	if wBad.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 for invalid ID, got %d", wBad.Code)
	}

	// 4. Nil store
	nilHandler := &Handler{}
	wNil := httptest.NewRecorder()
	nilHandler.HandleDeleteProfilerReport(wNil, reqBad)
	if wNil.Code != http.StatusInternalServerError {
		t.Errorf("Expected 500 for nil store, got %d", wNil.Code)
	}
}

func TestHandleProfilerHealth(t *testing.T) {
	handler, store, mcServer := setupTestProfilerHandler(t)

	// 1. Test parsing custom log input
	customLog := `TPS from last 1m, 5m, 15m: 19.9, 19.8, 19.7
Tick durations (min/med/95%ile/max ms): 10.2/15.4/22.1/35.0
CPU usage (process/system): 35.5% / 45.0%
Memory: Heap 2048 / 4096 MB (50.0%)
GC: G1 Young Generation - 12 collections (120ms total)
Disk: 15.0 / 50.0 GB (30.0%)
`
	body := ProfilerHealthRequest{
		Log:  customLog,
		Save: true,
	}
	bodyBytes, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/api/profiler/health", bytes.NewReader(bodyBytes))
	w := httptest.NewRecorder()
	handler.HandleProfilerHealth(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Health profiler.HealthSnapshot `json:"health"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode: %v", err)
	}

	if resp.Health.TPS != "19.9, 19.8, 19.7" {
		t.Errorf("Expected TPS '19.9, 19.8, 19.7', got %s", resp.Health.TPS)
	}
	if resp.Health.CPUProcess != "35.5%" {
		t.Errorf("Expected CPUProcess '35.5%%', got %s", resp.Health.CPUProcess)
	}

	// Verify report was saved in DB
	reports, total, _ := store.ListProfilerReports(10, 0, "spark_health")
	if total != 1 || len(reports) != 1 {
		t.Fatalf("Expected 1 saved health report, got %d", total)
	}
	if !strings.Contains(reports[0].Summary, "19.9") {
		t.Errorf("Expected summary to contain TPS, got: %s", reports[0].Summary)
	}

	// 2. Test fallback to server vitals / history when Log is empty
	mcServer.Broadcast("TPS from last 1m, 5m, 15m: 20.0, 20.0, 20.0")
	mcServer.Broadcast("Tick durations: 12.0/15.0/18.0/22.0")
	emptyReq := httptest.NewRequest("POST", "/api/profiler/health", strings.NewReader(`{}`))
	wEmpty := httptest.NewRecorder()
	handler.HandleProfilerHealth(wEmpty, emptyReq)
	if wEmpty.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK on empty log, got %d", wEmpty.Code)
	}

	// 3. Test nil server
	nilHandler := &Handler{}
	wNil := httptest.NewRecorder()
	nilHandler.HandleProfilerHealth(wNil, emptyReq)
	if wNil.Code != http.StatusOK {
		t.Errorf("Expected 200 OK even for nil server, got %d", wNil.Code)
	}
}

func TestHandleTriggerProfiler(t *testing.T) {
	handler, _, mcServer := setupTestProfilerHandler(t)

	// 1. Server stopped error
	triggerReq := TriggerProfilerRequest{Action: "health"}
	b, _ := json.Marshal(triggerReq)
	req := httptest.NewRequest("POST", "/api/profiler/trigger", bytes.NewReader(b))
	w := httptest.NewRecorder()
	handler.HandleTriggerProfiler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 Bad Request when server stopped, got %d", w.Code)
	}

	// 2. Set server running
	mcServer.SetReady(true)
	// Mock server running by setting status and an in-memory stdin pipe
	stdinPipe := &nopWriteCloser{Buffer: &bytes.Buffer{}}
	mcServer.SetRunningForTest(stdinPipe)

	// Test action: health
	wHealth := httptest.NewRecorder()
	reqHealth := httptest.NewRequest("POST", "/api/profiler/trigger", bytes.NewReader(b))
	handler.HandleTriggerProfiler(wHealth, reqHealth)
	if wHealth.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK for health trigger, got %d: %s", wHealth.Code, wHealth.Body.String())
	}
	if !strings.Contains(stdinPipe.String(), "spark health") {
		t.Errorf("Expected 'spark health' in stdin pipe, got: %s", stdinPipe.String())
	}

	// Test action: sampler_start
	stdinPipe.Reset()
	bStart, _ := json.Marshal(TriggerProfilerRequest{Action: "sampler_start"})
	wStart := httptest.NewRecorder()
	handler.HandleTriggerProfiler(wStart, httptest.NewRequest("POST", "/api/profiler/trigger", bytes.NewReader(bStart)))
	if wStart.Code != http.StatusOK {
		t.Errorf("Expected 200 OK for sampler_start, got %d", wStart.Code)
	}
	if !strings.Contains(stdinPipe.String(), "spark sampler start") {
		t.Errorf("Expected 'spark sampler start', got: %s", stdinPipe.String())
	}

	// Test action: sampler_stop
	stdinPipe.Reset()
	bStop, _ := json.Marshal(TriggerProfilerRequest{Action: "sampler_stop"})
	wStop := httptest.NewRecorder()
	handler.HandleTriggerProfiler(wStop, httptest.NewRequest("POST", "/api/profiler/trigger", bytes.NewReader(bStop)))
	if wStop.Code != http.StatusOK {
		t.Errorf("Expected 200 OK for sampler_stop, got %d", wStop.Code)
	}
	if !strings.Contains(stdinPipe.String(), "spark sampler stop") {
		t.Errorf("Expected 'spark sampler stop', got: %s", stdinPipe.String())
	}

	// Test action: timings_paste
	stdinPipe.Reset()
	bPaste, _ := json.Marshal(TriggerProfilerRequest{Action: "timings_paste"})
	wPaste := httptest.NewRecorder()
	handler.HandleTriggerProfiler(wPaste, httptest.NewRequest("POST", "/api/profiler/trigger", bytes.NewReader(bPaste)))
	if wPaste.Code != http.StatusOK {
		t.Errorf("Expected 200 OK for timings_paste, got %d", wPaste.Code)
	}
	if !strings.Contains(stdinPipe.String(), "timings paste") {
		t.Errorf("Expected 'timings paste', got: %s", stdinPipe.String())
	}

	// Test action: timings_reset
	stdinPipe.Reset()
	bReset, _ := json.Marshal(TriggerProfilerRequest{Action: "timings_reset"})
	wReset := httptest.NewRecorder()
	handler.HandleTriggerProfiler(wReset, httptest.NewRequest("POST", "/api/profiler/trigger", bytes.NewReader(bReset)))
	if wReset.Code != http.StatusOK {
		t.Errorf("Expected 200 OK for timings_reset, got %d", wReset.Code)
	}
	if !strings.Contains(stdinPipe.String(), "timings reset") {
		t.Errorf("Expected 'timings reset', got: %s", stdinPipe.String())
	}

	// Test action: custom valid
	stdinPipe.Reset()
	bCustom, _ := json.Marshal(TriggerProfilerRequest{Action: "custom", Command: "/spark tps"})
	wCustom := httptest.NewRecorder()
	handler.HandleTriggerProfiler(wCustom, httptest.NewRequest("POST", "/api/profiler/trigger", bytes.NewReader(bCustom)))
	if wCustom.Code != http.StatusOK {
		t.Errorf("Expected 200 OK for valid custom, got %d: %s", wCustom.Code, wCustom.Body.String())
	}
	if !strings.Contains(stdinPipe.String(), "spark tps") {
		t.Errorf("Expected 'spark tps', got: %s", stdinPipe.String())
	}

	// Test action: custom invalid prefix
	bInvalid, _ := json.Marshal(TriggerProfilerRequest{Action: "custom", Command: "op hacker"})
	wInvalid := httptest.NewRecorder()
	handler.HandleTriggerProfiler(wInvalid, httptest.NewRequest("POST", "/api/profiler/trigger", bytes.NewReader(bInvalid)))
	if wInvalid.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 for non-profiler custom command, got %d", wInvalid.Code)
	}

	// Test action: invalid action
	bBadAct, _ := json.Marshal(TriggerProfilerRequest{Action: "unknown_act"})
	wBadAct := httptest.NewRecorder()
	handler.HandleTriggerProfiler(wBadAct, httptest.NewRequest("POST", "/api/profiler/trigger", bytes.NewReader(bBadAct)))
	if wBadAct.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 for unknown action, got %d", wBadAct.Code)
	}

	// Test bad json body
	wBadJSON := httptest.NewRecorder()
	handler.HandleTriggerProfiler(wBadJSON, httptest.NewRequest("POST", "/api/profiler/trigger", strings.NewReader("bad json")))
	if wBadJSON.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 for bad json body, got %d", wBadJSON.Code)
	}

	// Test nil server
	nilHandler := &Handler{}
	wNil := httptest.NewRecorder()
	nilHandler.HandleTriggerProfiler(wNil, reqHealth)
	if wNil.Code != http.StatusInternalServerError {
		t.Errorf("Expected 500 for nil server, got %d", wNil.Code)
	}
}
