package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"paperMC_backend/internal/auth"
	"paperMC_backend/internal/database"
	"paperMC_backend/internal/minecraft"
)

func TestAuditLogAPI(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test_audit_api.db")
	store, err := database.NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to initialize store: %v", err)
	}
	defer store.Close()

	mcServer := &minecraft.Server{
		WorkDir: tempDir,
		JarFile: "server.jar",
		RAM:     "2G",
	}
	handler := NewServerHandler(mcServer, store)

	// 1. Initial GET /api/audit should be empty
	reqEmpty := httptest.NewRequest(http.MethodGet, "/api/audit", nil)
	wEmpty := httptest.NewRecorder()
	handler.HandleGetAuditLogs(wEmpty, reqEmpty)

	if wEmpty.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK, got %d: %s", wEmpty.Code, wEmpty.Body.String())
	}
	var emptyResp AuditLogsResponse
	if err := json.NewDecoder(wEmpty.Body).Decode(&emptyResp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}
	if emptyResp.Total != 0 || len(emptyResp.Logs) != 0 || emptyResp.TotalPages != 0 {
		t.Errorf("Expected empty response, got %+v", emptyResp)
	}

	// 2. Seed audit entries using recordAudit and recordAuditWithUser
	reqWithForwarded := httptest.NewRequest(http.MethodPost, "/start", nil)
	reqWithForwarded.Header.Set("X-Forwarded-For", "203.0.113.195, 70.41.3.18")
	ctxAdmin := context.WithValue(reqWithForwarded.Context(), auth.UserKey, &auth.Claims{Username: "admin_tester", Role: "admin"})
	reqWithForwarded = reqWithForwarded.WithContext(ctxAdmin)

	handler.recordAudit(reqWithForwarded, "server.start", http.StatusOK, "Server booted up")

	reqWithRealIP := httptest.NewRequest(http.MethodPost, "/api/players", nil)
	reqWithRealIP.Header.Set("X-Real-IP", "198.51.100.42")
	ctxOp := context.WithValue(reqWithRealIP.Context(), auth.UserKey, &auth.Claims{Username: "moderator_dan", Role: "operator"})
	reqWithRealIP = reqWithRealIP.WithContext(ctxOp)

	handler.recordAudit(reqWithRealIP, "player.whitelist_add", http.StatusOK, "Whitelisted player Alex")

	// Unauthenticated request fallback
	reqUnauth := httptest.NewRequest(http.MethodPost, "/login", nil)
	reqUnauth.RemoteAddr = "192.0.2.1:54321"
	handler.recordAudit(reqUnauth, "auth.login_failed", http.StatusUnauthorized, "Invalid credentials")

	// Explicit user
	handler.recordAuditWithUser("custom_service", nil, "backup.create", http.StatusCreated, "Nightly world backup")

	// 3. Query all logs
	reqAll := httptest.NewRequest(http.MethodGet, "/api/audit?page=1&limit=10", nil)
	wAll := httptest.NewRecorder()
	handler.HandleGetAuditLogs(wAll, reqAll)

	if wAll.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK, got %d", wAll.Code)
	}
	var allResp AuditLogsResponse
	if err := json.NewDecoder(wAll.Body).Decode(&allResp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}
	if allResp.Total != 4 || len(allResp.Logs) != 4 || allResp.TotalPages != 1 {
		t.Fatalf("Expected 4 logs and 1 total page, got %+v", allResp)
	}

	// Verify IP parsing in recorded logs
	var foundForwarded, foundRealIP, foundRemoteAddr bool
	for _, l := range allResp.Logs {
		if l.Action == "server.start" && l.IPAddress == "203.0.113.195" {
			foundForwarded = true
		}
		if l.Action == "player.whitelist_add" && l.IPAddress == "198.51.100.42" {
			foundRealIP = true
		}
		if l.Action == "auth.login_failed" && l.IPAddress == "192.0.2.1" {
			foundRemoteAddr = true
		}
	}
	if !foundForwarded {
		t.Errorf("Expected X-Forwarded-For IP '203.0.113.195' to be recorded")
	}
	if !foundRealIP {
		t.Errorf("Expected X-Real-IP '198.51.100.42' to be recorded")
	}
	if !foundRemoteAddr {
		t.Errorf("Expected stripped RemoteAddr IP '192.0.2.1' to be recorded")
	}

	// 4. Test pagination limits and capping
	reqCapped := httptest.NewRequest(http.MethodGet, "/api/audit?limit=250", nil)
	wCapped := httptest.NewRecorder()
	handler.HandleGetAuditLogs(wCapped, reqCapped)
	var cappedResp AuditLogsResponse
	_ = json.NewDecoder(wCapped.Body).Decode(&cappedResp)
	if cappedResp.Limit != 100 {
		t.Errorf("Expected limit capped at 100, got %d", cappedResp.Limit)
	}

	// 5. Test filter by action
	reqAct := httptest.NewRequest(http.MethodGet, "/api/audit?action=server", nil)
	wAct := httptest.NewRecorder()
	handler.HandleGetAuditLogs(wAct, reqAct)
	var actResp AuditLogsResponse
	_ = json.NewDecoder(wAct.Body).Decode(&actResp)
	if actResp.Total != 1 || actResp.Logs[0].Action != "server.start" {
		t.Errorf("Expected 1 server.start match, got %+v", actResp)
	}

	// 6. Test filter by username
	reqUser := httptest.NewRequest(http.MethodGet, "/api/audit?username=moderator", nil)
	wUser := httptest.NewRecorder()
	handler.HandleGetAuditLogs(wUser, reqUser)
	var userResp AuditLogsResponse
	_ = json.NewDecoder(wUser.Body).Decode(&userResp)
	if userResp.Total != 1 || userResp.Logs[0].Username != "moderator_dan" {
		t.Errorf("Expected 1 moderator_dan match, got %+v", userResp)
	}

	// 7. Test Clear Audit Logs - Method not allowed
	reqBadClear := httptest.NewRequest(http.MethodPost, "/api/audit", nil)
	wBadClear := httptest.NewRecorder()
	handler.HandleClearAuditLogs(wBadClear, reqBadClear)
	if wBadClear.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected 405 Method Not Allowed, got %d", wBadClear.Code)
	}

	// 8. Test Clear Audit Logs - Success
	reqClear := httptest.NewRequest(http.MethodDelete, "/api/audit", nil)
	ctxAdminClear := context.WithValue(reqClear.Context(), auth.UserKey, &auth.Claims{Username: "admin_tester", Role: "admin"})
	reqClear = reqClear.WithContext(ctxAdminClear)
	wClear := httptest.NewRecorder()
	handler.HandleClearAuditLogs(wClear, reqClear)

	if wClear.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK on clear, got %d: %s", wClear.Code, wClear.Body.String())
	}

	// After clearing, only the audit.clear event itself should exist in the log
	reqAfterClear := httptest.NewRequest(http.MethodGet, "/api/audit", nil)
	wAfterClear := httptest.NewRecorder()
	handler.HandleGetAuditLogs(wAfterClear, reqAfterClear)
	var afterClearResp AuditLogsResponse
	_ = json.NewDecoder(wAfterClear.Body).Decode(&afterClearResp)
	if afterClearResp.Total != 1 || afterClearResp.Logs[0].Action != "audit.clear" {
		t.Errorf("Expected only the audit.clear record to remain, got %+v", afterClearResp)
	}
}

func TestAuditNilStoreAndHelpers(t *testing.T) {
	nilHandler := &Handler{store: nil}

	// Nil store should respond with 500 cleanly
	w1 := httptest.NewRecorder()
	r1 := httptest.NewRequest(http.MethodGet, "/api/audit", nil)
	nilHandler.HandleGetAuditLogs(w1, r1)
	if w1.Code != http.StatusInternalServerError {
		t.Errorf("Expected 500 for nil store, got %d", w1.Code)
	}

	w2 := httptest.NewRecorder()
	r2 := httptest.NewRequest(http.MethodDelete, "/api/audit", nil)
	nilHandler.HandleClearAuditLogs(w2, r2)
	if w2.Code != http.StatusInternalServerError {
		t.Errorf("Expected 500 for nil store clear, got %d", w2.Code)
	}

	// recordAudit should not panic on nil handler or store
	nilHandler.recordAudit(nil, "test", 200, "details")
	var completelyNilHandler *Handler
	completelyNilHandler.recordAudit(nil, "test", 200, "details")

	// Test getClientIP helper edge cases
	if ip := getClientIP(nil); ip != "127.0.0.1" {
		t.Errorf("Expected fallback 127.0.0.1 for nil request, got %s", ip)
	}

	emptyReq := httptest.NewRequest("GET", "/", nil)
	emptyReq.RemoteAddr = ""
	if ip := getClientIP(emptyReq); ip != "127.0.0.1" {
		t.Errorf("Expected fallback 127.0.0.1 for empty RemoteAddr, got %s", ip)
	}

	noPortReq := httptest.NewRequest("GET", "/", nil)
	noPortReq.RemoteAddr = "10.10.10.10"
	if ip := getClientIP(noPortReq); ip != "10.10.10.10" {
		t.Errorf("Expected '10.10.10.10' preserved when port missing, got %s", ip)
	}
}

func TestAuditOperationsIntegration(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test_audit_hooks.db")
	store, err := database.NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to init store: %v", err)
	}
	defer store.Close()

	// Seed user
	pwdHash, _ := auth.HashPassword("securepass")
	_ = store.CreateUser(&database.User{
		Username: "admin_user",
		Password: pwdHash,
		Role:     "admin",
	})

	mcServer := &minecraft.Server{
		WorkDir: tempDir,
		JarFile: "server.jar",
		RAM:     "2G",
	}
	handler := NewServerHandler(mcServer, store)

	// 1. Failed login (wrong password)
	badLoginBody := `{"username": "admin_user", "password": "wrongpassword"}`
	req1 := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(badLoginBody))
	w1 := httptest.NewRecorder()
	handler.Login(w1, req1)
	if w1.Code != http.StatusUnauthorized {
		t.Fatalf("Expected 401 Unauthorized, got %d", w1.Code)
	}

	// 2. Failed login (user not found)
	noUserBody := `{"username": "non_existent", "password": "secret"}`
	req2 := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(noUserBody))
	w2 := httptest.NewRecorder()
	handler.Login(w2, req2)
	if w2.Code != http.StatusNotFound {
		t.Fatalf("Expected 404 NotFound, got %d", w2.Code)
	}

	// 3. Successful login
	goodLoginBody := `{"username": "admin_user", "password": "securepass"}`
	req3 := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(goodLoginBody))
	w3 := httptest.NewRecorder()
	handler.Login(w3, req3)
	if w3.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK, got %d", w3.Code)
	}

	// 4. Command execution
	cmdReqBody := `{"command": "/say Hello World"}`
	req4 := httptest.NewRequest(http.MethodPost, "/command", strings.NewReader(cmdReqBody))
	ctx := context.WithValue(req4.Context(), auth.UserKey, &auth.Claims{Username: "admin_user", Role: "admin"})
	req4 = req4.WithContext(ctx)
	w4 := httptest.NewRecorder()
	handler.SendCommand(w4, req4)

	// 5. Config save
	cfgBody := `{"motd": "Welcome to Lodestone!"}`
	req5 := httptest.NewRequest(http.MethodPost, "/config", strings.NewReader(cfgBody))
	req5 = req5.WithContext(ctx)
	w5 := httptest.NewRecorder()
	handler.PostConfig(w5, req5)
	if w5.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK, got %d", w5.Code)
	}

	// 6. Verify audit logs recorded
	logs, total, err := store.ListAuditLogs(50, 0, "", "")
	if err != nil {
		t.Fatalf("Failed to list audit logs: %v", err)
	}
	if total < 5 {
		t.Fatalf("Expected at least 5 audit logs, got %d", total)
	}

	actionCounts := map[string]int{}
	for _, l := range logs {
		actionCounts[l.Action]++
	}

	if actionCounts["auth.login_failed"] != 2 {
		t.Errorf("Expected 2 auth.login_failed events, got %d", actionCounts["auth.login_failed"])
	}
	if actionCounts["auth.login"] != 1 {
		t.Errorf("Expected 1 auth.login event, got %d", actionCounts["auth.login"])
	}
	if actionCounts["config.save"] != 1 {
		t.Errorf("Expected 1 config.save event, got %d", actionCounts["config.save"])
	}
}

