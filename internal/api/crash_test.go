package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"paperMC_backend/internal/database"
	"paperMC_backend/internal/minecraft"
)

func setupTestCrashHandler(t *testing.T) (*Handler, database.Store, string) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test_crash_api.db")
	store, err := database.NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to initialize test DB: %v", err)
	}

	mcServer := minecraft.NewServer(tempDir, "paper.jar", "4G", store)
	handler := NewServerHandler(mcServer, store)

	return handler, store, tempDir
}

func TestHandleListCrashReports(t *testing.T) {
	handler, store, workDir := setupTestCrashHandler(t)

	// 1. Create a physical crash dump in workDir/crash-reports
	crashDir := filepath.Join(workDir, "crash-reports")
	_ = os.MkdirAll(crashDir, 0755)
	_ = os.WriteFile(filepath.Join(crashDir, "crash-2026-09-08-server.txt"), []byte("java.lang.OutOfMemoryError: Java heap space"), 0644)

	// Record an existing report directly in DB
	_ = store.RecordCrashReport(&database.CrashReport{
		Source:         "runtime",
		Category:       "PortConflict",
		Title:          "Port in use",
		Summary:        "Port 25565 occupied",
		Recommendation: "Change port",
		RawLog:         "FAILED TO BIND TO PORT",
	})

	// Request GET /api/crash
	req := httptest.NewRequest("GET", "/api/crash?limit=10&offset=0", nil)
	w := httptest.NewRecorder()
	handler.HandleListCrashReports(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Reports []database.CrashReport `json:"reports"`
		Total   int                    `json:"total"`
		Limit   int                    `json:"limit"`
		Offset  int                    `json:"offset"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if resp.Total < 2 || len(resp.Reports) < 2 {
		t.Errorf("Expected at least 2 reports (1 DB + 1 auto-ingested from disk), got total=%d, len=%d", resp.Total, len(resp.Reports))
	}
}

func TestHandleGetCrashReport(t *testing.T) {
	handler, store, _ := setupTestCrashHandler(t)

	report := &database.CrashReport{
		Source:         "runtime",
		Category:       "WatchdogTimeout",
		Title:          "Server froze",
		Summary:        "Watchdog triggered",
		Recommendation: "Check laggy entities",
		RawLog:         "Server Hang Watchdog",
	}
	_ = store.RecordCrashReport(report)

	// Existing report
	req := httptest.NewRequest("GET", "/api/crash/"+strconv.Itoa(report.ID), nil)
	req.SetPathValue("id", strconv.Itoa(report.ID))
	w := httptest.NewRecorder()
	handler.HandleGetCrashReport(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK, got %d: %s", w.Code, w.Body.String())
	}

	var fetched database.CrashReport
	_ = json.NewDecoder(w.Body).Decode(&fetched)
	if fetched.ID != report.ID || fetched.Category != "WatchdogTimeout" {
		t.Errorf("Unexpected report data: %+v", fetched)
	}

	// 404 non-existent
	req404 := httptest.NewRequest("GET", "/api/crash/99999", nil)
	req404.SetPathValue("id", "99999")
	w404 := httptest.NewRecorder()
	handler.HandleGetCrashReport(w404, req404)

	if w404.Code != http.StatusNotFound {
		t.Errorf("Expected 404, got %d", w404.Code)
	}

	// Invalid ID
	reqBad := httptest.NewRequest("GET", "/api/crash/invalid", nil)
	reqBad.SetPathValue("id", "invalid")
	wBad := httptest.NewRecorder()
	handler.HandleGetCrashReport(wBad, reqBad)

	if wBad.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 for bad ID, got %d", wBad.Code)
	}
}

func TestHandleAnalyzeCrash(t *testing.T) {
	handler, store, _ := setupTestCrashHandler(t)

	// 1. Analyze explicit log and save
	payload := AnalyzeRequest{
		Log:  "**** FAILED TO BIND TO PORT!\nAddress already in use: bind to port 25565",
		Save: true,
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/api/crash/analyze", bytes.NewReader(body))
	w := httptest.NewRecorder()
	handler.HandleAnalyzeCrash(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Analysis struct {
			Category string `json:"category"`
			Title    string `json:"title"`
			Culprit  string `json:"culprit"`
		} `json:"analysis"`
		Report database.CrashReport `json:"report"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if resp.Analysis.Category != "PortConflict" {
		t.Errorf("Expected PortConflict category, got %s", resp.Analysis.Category)
	}
	if resp.Report.ID <= 0 {
		t.Errorf("Expected positive saved report ID, got %d", resp.Report.ID)
	}

	// 2. Empty log fallback to server history
	handler.mc.Broadcast("java.lang.OutOfMemoryError: Java heap space")
	emptyPayload := AnalyzeRequest{Log: "", Save: false}
	emptyBody, _ := json.Marshal(emptyPayload)
	reqEmpty := httptest.NewRequest("POST", "/api/crash/analyze", bytes.NewReader(emptyBody))
	wEmpty := httptest.NewRecorder()
	handler.HandleAnalyzeCrash(wEmpty, reqEmpty)

	if wEmpty.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK for history fallback, got %d", wEmpty.Code)
	}

	// 3. Both empty log and empty history returns 400
	handler.mc.LogHistory = nil
	reqZero := httptest.NewRequest("POST", "/api/crash/analyze", bytes.NewReader([]byte("{}")))
	wZero := httptest.NewRecorder()
	handler.HandleAnalyzeCrash(wZero, reqZero)

	if wZero.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 when log is completely empty, got %d", wZero.Code)
	}

	_ = store
}

func TestHandleDeleteCrashReport(t *testing.T) {
	handler, store, _ := setupTestCrashHandler(t)

	r1 := &database.CrashReport{Category: "OutOfMemory", RawLog: "OOM"}
	r2 := &database.CrashReport{Category: "PortConflict", RawLog: "Port"}
	_ = store.RecordCrashReport(r1)
	_ = store.RecordCrashReport(r2)

	// Delete specific report
	reqDel := httptest.NewRequest("DELETE", "/api/crash?id="+strconv.Itoa(r1.ID), nil)
	wDel := httptest.NewRecorder()
	handler.HandleDeleteCrashReport(wDel, reqDel)

	if wDel.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK, got %d", wDel.Code)
	}

	rem, total, _ := store.ListCrashReports(10, 0)
	if total != 1 || rem[0].ID != r2.ID {
		t.Errorf("Expected only r2 remaining, got total=%d", total)
	}

	// Clear all reports
	reqClear := httptest.NewRequest("DELETE", "/api/crash?id=all", nil)
	wClear := httptest.NewRecorder()
	handler.HandleDeleteCrashReport(wClear, reqClear)

	if wClear.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK on clear all, got %d", wClear.Code)
	}

	_, totalAfter, _ := store.ListCrashReports(10, 0)
	if totalAfter != 0 {
		t.Errorf("Expected 0 reports after clear, got %d", totalAfter)
	}
}

func TestHandleAISettingsEndpoints(t *testing.T) {
	handler, _, _ := setupTestCrashHandler(t)

	// 1. Get initial default AI settings
	reqGet := httptest.NewRequest("GET", "/api/crash/ai-config", nil)
	wGet := httptest.NewRecorder()
	handler.HandleGetAISettings(wGet, reqGet)

	if wGet.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK, got %d", wGet.Code)
	}

	var initResp AISettingsResponse
	_ = json.NewDecoder(wGet.Body).Decode(&initResp)
	if initResp.Provider != "openai" || initResp.HasAPIKey {
		t.Errorf("Default AI settings mismatch: %+v", initResp)
	}

	// 2. Save new AI settings with real API key
	savePayload := database.AISettings{
		Provider:  "openai",
		APIKey:    "sk-proj-supersecret1234567890",
		Model:     "gpt-4o-mini",
		BaseURL:   "https://api.openai.com",
		IsEnabled: true,
	}
	saveBody, _ := json.Marshal(savePayload)
	reqSave := httptest.NewRequest("POST", "/api/crash/ai-config", bytes.NewReader(saveBody))
	wSave := httptest.NewRecorder()
	handler.HandleSaveAISettings(wSave, reqSave)

	if wSave.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK on save, got %d: %s", wSave.Code, wSave.Body.String())
	}

	var savedResp AISettingsResponse
	_ = json.NewDecoder(wSave.Body).Decode(&savedResp)
	if !savedResp.HasAPIKey || !strings.Contains(savedResp.APIKey, "****") {
		t.Errorf("Expected masked API key in response, got: %s", savedResp.APIKey)
	}

	// 3. Save again with masked key (should preserve previous secret key)
	preservePayload := database.AISettings{
		Provider:  "openai",
		APIKey:    "sk-****890",
		Model:     "gpt-4o",
		IsEnabled: true,
	}
	presBody, _ := json.Marshal(preservePayload)
	reqPres := httptest.NewRequest("POST", "/api/crash/ai-config", bytes.NewReader(presBody))
	wPres := httptest.NewRecorder()
	handler.HandleSaveAISettings(wPres, reqPres)

	if wPres.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK on preserve save, got %d", wPres.Code)
	}

	// Verify in DB that original key was preserved
	dbSettings, _ := handler.store.GetAISettings()
	if dbSettings.APIKey != "sk-proj-supersecret1234567890" {
		t.Errorf("Expected original secret key preserved, got: %s", dbSettings.APIKey)
	}
	if dbSettings.Model != "gpt-4o" {
		t.Errorf("Expected model updated to gpt-4o, got: %s", dbSettings.Model)
	}
}

func TestHandleAIExplainCrash(t *testing.T) {
	// Mock external LLM server
	mockLLM := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"choices": []map[string]interface{}{
				{
					"message": map[string]string{
						"content": "### AI Crash Diagnosis\nMemory leak in entity ticking.",
					},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer mockLLM.Close()

	handler, store, _ := setupTestCrashHandler(t)

	report := &database.CrashReport{
		Category:       "OutOfMemory",
		Title:          "OOM",
		Summary:        "Exhausted memory",
		Recommendation: "Allocate RAM",
		RawLog:         "java.lang.OutOfMemoryError: Java heap space",
	}
	_ = store.RecordCrashReport(report)

	// 1. When AI disabled: returns 400
	reqExplain := httptest.NewRequest("POST", "/api/crash/ai-explain", strings.NewReader(`{"report_id": 1}`))
	wExplain := httptest.NewRecorder()
	handler.HandleAIExplainCrash(wExplain, reqExplain)

	if wExplain.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 when AI is disabled, got %d", wExplain.Code)
	}

	// 2. Enable AI with mock server
	_ = store.SaveAISettings(&database.AISettings{
		Provider:  "openai",
		APIKey:    "test-key",
		BaseURL:   mockLLM.URL,
		Model:     "gpt-4o-mini",
		IsEnabled: true,
	})

	// 3. Successful explanation with report_id
	explainPayload := AIExplainRequest{
		ReportID:     report.ID,
		CustomPrompt: "Can I optimize this without adding RAM?",
	}
	explainBytes, _ := json.Marshal(explainPayload)
	reqOK := httptest.NewRequest("POST", "/api/crash/ai-explain", bytes.NewReader(explainBytes))
	wOK := httptest.NewRecorder()
	handler.HandleAIExplainCrash(wOK, reqOK)

	if wOK.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK from AI explanation, got %d: %s", wOK.Code, wOK.Body.String())
	}

	var res map[string]string
	_ = json.NewDecoder(wOK.Body).Decode(&res)
	if !strings.Contains(res["explanation"], "Memory leak in entity ticking") {
		t.Errorf("Unexpected AI response: %v", res)
	}

	// 4. Successful explanation with on-the-fly raw_log
	rawPayload := AIExplainRequest{
		RawLog: "**** FAILED TO BIND TO PORT 25565",
	}
	rawBytes, _ := json.Marshal(rawPayload)
	reqRaw := httptest.NewRequest("POST", "/api/crash/ai-explain", bytes.NewReader(rawBytes))
	wRaw := httptest.NewRecorder()
	handler.HandleAIExplainCrash(wRaw, reqRaw)

	if wRaw.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK with raw log, got %d: %s", wRaw.Code, wRaw.Body.String())
	}
}

func TestCrashAPIErrorCases(t *testing.T) {
	// Handler with nil store
	handler := &Handler{mc: nil, store: nil}

	w := httptest.NewRecorder()
	handler.HandleListCrashReports(w, httptest.NewRequest("GET", "/api/crash", nil))
	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected 500 for nil store on list, got %d", w.Code)
	}

	w = httptest.NewRecorder()
	handler.HandleGetCrashReport(w, httptest.NewRequest("GET", "/api/crash/1", nil))
	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected 500 for nil store on get, got %d", w.Code)
	}

	w = httptest.NewRecorder()
	handler.HandleDeleteCrashReport(w, httptest.NewRequest("DELETE", "/api/crash?id=1", nil))
	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected 500 for nil store on delete, got %d", w.Code)
	}

	w = httptest.NewRecorder()
	handler.HandleGetAISettings(w, httptest.NewRequest("GET", "/api/crash/ai-config", nil))
	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected 500 for nil store on get AI, got %d", w.Code)
	}

	w = httptest.NewRecorder()
	handler.HandleSaveAISettings(w, httptest.NewRequest("POST", "/api/crash/ai-config", strings.NewReader("{}")))
	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected 500 for nil store on save AI, got %d", w.Code)
	}

	w = httptest.NewRecorder()
	handler.HandleAIExplainCrash(w, httptest.NewRequest("POST", "/api/crash/ai-explain", strings.NewReader("{}")))
	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected 500 for nil store on AI explain, got %d", w.Code)
	}
}
