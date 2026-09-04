package api

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"paperMC_backend/internal/database"
	"paperMC_backend/internal/minecraft"
	"paperMC_backend/internal/updater"
)

func TestServerOperationsAPI(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")
	store, err := database.NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to init store: %v", err)
	}
	defer store.Close()

	// Write server.properties for level-name check
	_ = os.WriteFile(filepath.Join(tempDir, "server.properties"), []byte("level-name=custom_world\n"), 0644)

	mcServer := minecraft.NewServer(tempDir, "server.jar", "2G", store)
	handler := NewServerHandler(mcServer, store)

	// 1. GET /status
	t.Run("GET /status", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/status", nil)
		w := httptest.NewRecorder()
		handler.HandleStatus(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("Expected 200 OK, got %d", w.Code)
		}

		var vitals struct {
			Status      string             `json:"status"`
			ActiveWorld string             `json:"active_world"`
			PlayerCount int                `json:"player_count"`
			PlayerList  []minecraft.Player `json:"player_list"`
		}
		if err := json.NewDecoder(w.Body).Decode(&vitals); err != nil {
			t.Fatalf("Failed to decode vitals: %v", err)
		}
		if vitals.ActiveWorld != "custom_world" {
			t.Errorf("Expected ActiveWorld 'custom_world', got '%s'", vitals.ActiveWorld)
		}
		if vitals.Status != "Stopped" {
			t.Errorf("Expected Status 'Stopped', got '%s'", vitals.Status)
		}
	})

	// 2. POST /command
	t.Run("POST /command validation", func(t *testing.T) {
		// Empty command
		reqEmpty := httptest.NewRequest(http.MethodPost, "/command", bytes.NewReader([]byte(`{"command":""}`)))
		wEmpty := httptest.NewRecorder()
		handler.SendCommand(wEmpty, reqEmpty)
		if wEmpty.Code != http.StatusBadRequest {
			t.Errorf("Expected 400 on empty command, got %d", wEmpty.Code)
		}

		// Invalid JSON
		reqBad := httptest.NewRequest(http.MethodPost, "/command", bytes.NewReader([]byte(`invalid`)))
		wBad := httptest.NewRecorder()
		handler.SendCommand(wBad, reqBad)
		if wBad.Code != http.StatusBadRequest {
			t.Errorf("Expected 400 on invalid JSON, got %d", wBad.Code)
		}

		// Command to stopped server fails
		reqValid := httptest.NewRequest(http.MethodPost, "/command", bytes.NewReader([]byte(`{"command":"say hi"}`)))
		wValid := httptest.NewRecorder()
		handler.SendCommand(wValid, reqValid)
		if wValid.Code != http.StatusBadRequest {
			t.Errorf("Expected 400 on command to stopped server, got %d", wValid.Code)
		}
	})

	// 3. POST /whitelist_add validation
	t.Run("POST /whitelist_add validation", func(t *testing.T) {
		reqEmpty := httptest.NewRequest(http.MethodPost, "/whitelist_add", bytes.NewReader([]byte(`{"command":""}`)))
		wEmpty := httptest.NewRecorder()
		handler.WhiteListing(wEmpty, reqEmpty)
		if wEmpty.Code != http.StatusBadRequest {
			t.Errorf("Expected 400 on empty username, got %d", wEmpty.Code)
		}

		reqBad := httptest.NewRequest(http.MethodPost, "/whitelist_add", bytes.NewReader([]byte(`invalid`)))
		wBad := httptest.NewRecorder()
		handler.WhiteListing(wBad, reqBad)
		if wBad.Code != http.StatusBadRequest {
			t.Errorf("Expected 400 on bad JSON, got %d", wBad.Code)
		}
	})

	// 4. POST /stop and POST /kill on stopped server
	t.Run("POST /stop and /kill when stopped", func(t *testing.T) {
		reqStop := httptest.NewRequest(http.MethodPost, "/stop", nil)
		wStop := httptest.NewRecorder()
		handler.Stop(wStop, reqStop)
		if wStop.Code != http.StatusBadRequest {
			t.Errorf("Expected 400 stopping stopped server, got %d", wStop.Code)
		}

		reqKill := httptest.NewRequest(http.MethodPost, "/kill", nil)
		wKill := httptest.NewRecorder()
		handler.Kill(wKill, reqKill)
		if wKill.Code != http.StatusBadRequest {
			t.Errorf("Expected 400 killing stopped server, got %d", wKill.Code)
		}
	})

	// 5. BasicAuth Helper
	t.Run("BasicAuth Middleware", func(t *testing.T) {
		dummyHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})
		authMiddleware := handler.BasicAuth(dummyHandler, "admin", "secret")

		// Valid basic auth
		reqValid := httptest.NewRequest(http.MethodGet, "/", nil)
		reqValid.SetBasicAuth("admin", "secret")
		wValid := httptest.NewRecorder()
		authMiddleware.ServeHTTP(wValid, reqValid)
		if wValid.Code != http.StatusOK {
			t.Errorf("Expected 200 with valid Basic Auth, got %d", wValid.Code)
		}

		// Invalid basic auth
		reqInvalid := httptest.NewRequest(http.MethodGet, "/", nil)
		reqInvalid.SetBasicAuth("admin", "wrong")
		wInvalid := httptest.NewRecorder()
		authMiddleware.ServeHTTP(wInvalid, reqInvalid)
		if wInvalid.Code != http.StatusUnauthorized {
			t.Errorf("Expected 401 with invalid Basic Auth, got %d", wInvalid.Code)
		}
	})

	// 6. GET /api/updater/check validation
	t.Run("GET /api/updater/check missing version", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/updater/check", nil)
		w := httptest.NewRecorder()
		handler.HandleCheckUpdate(w, req)
		if w.Code != http.StatusBadRequest {
			t.Errorf("Expected 400 when 'version' query parameter is missing, got %d", w.Code)
		}
	})

	// 7. POST /api/updater/apply validation
	t.Run("POST /api/updater/apply validation", func(t *testing.T) {
		// Empty body
		reqEmpty := httptest.NewRequest(http.MethodPost, "/api/updater/apply", bytes.NewReader([]byte{}))
		wEmpty := httptest.NewRecorder()
		handler.HandleUpdate(wEmpty, reqEmpty)
		if wEmpty.Code != http.StatusBadRequest {
			t.Errorf("Expected 400 on empty update body, got %d", wEmpty.Code)
		}

		// Invalid JSON
		reqBad := httptest.NewRequest(http.MethodPost, "/api/updater/apply", bytes.NewReader([]byte(`not-json`)))
		wBad := httptest.NewRecorder()
		handler.HandleUpdate(wBad, reqBad)
		if wBad.Code != http.StatusBadRequest {
			t.Errorf("Expected 400 on invalid JSON in update, got %d", wBad.Code)
		}

		// Empty version
		reqNoVer := httptest.NewRequest(http.MethodPost, "/api/updater/apply", bytes.NewReader([]byte(`{"version":""}`)))
		wNoVer := httptest.NewRecorder()
		handler.HandleUpdate(wNoVer, reqNoVer)
		if wNoVer.Code != http.StatusBadRequest {
			t.Errorf("Expected 400 on empty version in update, got %d", wNoVer.Code)
		}
	})

	// 8. GET /api/updater/versions
	t.Run("GET /api/updater/versions", func(t *testing.T) {
		mockJSON := `{
			"project": {"id": "paper", "name": "Paper"},
			"versions": {
				"26.2": ["26.2.0"]
			}
		}`
		mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(mockJSON))
		}))
		defer mockServer.Close()

		origURL := updater.FillAPIBaseURL
		updater.FillAPIBaseURL = mockServer.URL
		defer func() { updater.FillAPIBaseURL = origURL }()

		req := httptest.NewRequest(http.MethodGet, "/api/updater/versions?project=paper", nil)
		w := httptest.NewRecorder()
		handler.HandleGetVersions(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected 200 OK fetching versions, got %d: %s", w.Code, w.Body.String())
		}
	})
}

func TestServerLifecycleEndpoints(t *testing.T) {
	origExec := minecraft.ExecCommandContext
	defer func() { minecraft.ExecCommandContext = origExec }()

	minecraft.ExecCommandContext = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "cat")
	}

	tempDir := t.TempDir()
	store, _ := database.NewSQLiteStore(filepath.Join(tempDir, "test.db"))
	defer store.Close()

	mcServer := minecraft.NewServer(tempDir, "server.jar", "2G", store)
	handler := NewServerHandler(mcServer, store)

	// 1. POST /start
	reqStart := httptest.NewRequest(http.MethodPost, "/start", nil)
	wStart := httptest.NewRecorder()
	handler.Start(wStart, reqStart)

	if wStart.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK starting server, got %d: %s", wStart.Code, wStart.Body.String())
	}

	// 2. POST /start while already running fails with 400
	wStartAgain := httptest.NewRecorder()
	handler.Start(wStartAgain, reqStart)
	if wStartAgain.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 starting already running server, got %d", wStartAgain.Code)
	}

	// 3. POST /command while running succeeds with 200
	reqCmd := httptest.NewRequest(http.MethodPost, "/command", bytes.NewReader([]byte(`{"command":"say hello"}`)))
	wCmd := httptest.NewRecorder()
	handler.SendCommand(wCmd, reqCmd)
	if wCmd.Code != http.StatusOK {
		t.Errorf("Expected 200 sending command to running server, got %d", wCmd.Code)
	}

	// 4. POST /stop while running
	reqStop := httptest.NewRequest(http.MethodPost, "/stop", nil)
	wStop := httptest.NewRecorder()
	handler.Stop(wStop, reqStop)
	if wStop.Code != http.StatusOK {
		t.Errorf("Expected 200 OK stopping running server, got %d: %s", wStop.Code, wStop.Body.String())
	}

	// 5. Test POST /kill while running/stopping
	reqKill := httptest.NewRequest(http.MethodPost, "/kill", nil)
	wKill := httptest.NewRecorder()
	handler.Kill(wKill, reqKill)
	if wKill.Code != http.StatusOK {
		t.Errorf("Expected 200 killing running server, got %d: %s", wKill.Code, wKill.Body.String())
	}

	// Wait for process monitor to reach stopped
	for i := 0; i < 20; i++ {
		if mcServer.GetStatus() == minecraft.StatusStopped {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	// 6. Test POST /kill when already stopped
	wKillStopped := httptest.NewRecorder()
	handler.Kill(wKillStopped, reqKill)
	if wKillStopped.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 killing stopped server, got %d", wKillStopped.Code)
	}

	// 7. POST /whitelist_add with mock Mojang on running server
	origMojang := minecraft.MojangBaseURL
	defer func() { minecraft.MojangBaseURL = origMojang }()
	mockMojang := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"name":"Alex","id":"alex-uuid"}`))
	}))
	defer mockMojang.Close()
	minecraft.MojangBaseURL = mockMojang.URL

	wStart3 := httptest.NewRecorder()
	handler.Start(wStart3, reqStart)
	if wStart3.Code != http.StatusOK {
		t.Fatalf("Expected 200 starting server for whitelist test, got %d", wStart3.Code)
	}
	defer mcServer.Kill()

	reqWl := httptest.NewRequest(http.MethodPost, "/whitelist_add", bytes.NewReader([]byte(`{"command":"Alex"}`)))
	wWl := httptest.NewRecorder()
	handler.WhiteListing(wWl, reqWl)
	if wWl.Code != http.StatusOK {
		t.Errorf("Expected 200 on WhiteListing, got %d: %s", wWl.Code, wWl.Body.String())
	}
}

func TestHandleLogsSSE(t *testing.T) {
	tempDir := t.TempDir()
	mcServer := minecraft.NewServer(tempDir, "server.jar", "2G", nil)
	handler := NewServerHandler(mcServer, nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := httptest.NewRequest(http.MethodGet, "/logs", nil).WithContext(ctx)
	w := httptest.NewRecorder()

	// Seed log message
	go func() {
		time.Sleep(20 * time.Millisecond)
		select {
		case mcServer.LogChan <- "Sample log stream line":
		default:
		}
		time.Sleep(30 * time.Millisecond)
		cancel()
	}()

	handler.HandleLogs(w, req)

	if w.Header().Get("Content-Type") != "text/event-stream" {
		t.Errorf("Expected Content-Type text/event-stream, got %s", w.Header().Get("Content-Type"))
	}
}

func TestUpdaterAPIExecution(t *testing.T) {
	tempDir := t.TempDir()
	store, _ := database.NewSQLiteStore(filepath.Join(tempDir, "test.db"))
	defer store.Close()

	mcServer := minecraft.NewServer(tempDir, "server.jar", "2G", store)
	handler := NewServerHandler(mcServer, store)

	// Mock file content and hash
	mockJarBytes := []byte("mock-jar-content-12345")
	h := sha256.Sum256(mockJarBytes)
	validHash := hex.EncodeToString(h[:])

	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/builds/latest") {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			mockBuildJSON := fmt.Sprintf(`{
				"id": 100,
				"channel": "STABLE",
				"version": "26.2",
				"downloads": {
					"server:default": {
						"name": "paper-26.2-100.jar",
						"url": "http://%s/download",
						"checksums": {
							"sha256": "%s"
						},
						"size": %d
					}
				}
			}`, r.Host, validHash, len(mockJarBytes))
			_, _ = w.Write([]byte(mockBuildJSON))
			return
		}
		if strings.Contains(r.URL.Path, "/download") {
			w.Header().Set("Content-Type", "application/java-archive")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(mockJarBytes)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer mockServer.Close()

	origURL := updater.FillAPIBaseURL
	updater.FillAPIBaseURL = mockServer.URL
	defer func() { updater.FillAPIBaseURL = origURL }()

	// 1. GET /api/updater/check
	reqCheck := httptest.NewRequest(http.MethodGet, "/api/updater/check?version=26.2", nil)
	wCheck := httptest.NewRecorder()
	handler.HandleCheckUpdate(wCheck, reqCheck)

	if wCheck.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK on CheckUpdate, got %d: %s", wCheck.Code, wCheck.Body.String())
	}
	var checkResp CheckUpdateResponse
	_ = json.NewDecoder(wCheck.Body).Decode(&checkResp)
	if checkResp.LatestBuild != 100 || checkResp.LatestHash != validHash {
		t.Errorf("Unexpected check response: %+v", checkResp)
	}

	// 2. POST /api/updater/apply (Success download)
	reqApply := httptest.NewRequest(http.MethodPost, "/api/updater/apply", bytes.NewReader([]byte(`{"version":"26.2"}`)))
	wApply := httptest.NewRecorder()
	handler.HandleUpdate(wApply, reqApply)

	if wApply.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK on HandleUpdate, got %d: %s", wApply.Code, wApply.Body.String())
	}

	// 3. POST /api/updater/apply (Already up to date)
	wApplyAgain := httptest.NewRecorder()
	reqApplyAgain := httptest.NewRequest(http.MethodPost, "/api/updater/apply", bytes.NewReader([]byte(`{"version":"26.2"}`)))
	handler.HandleUpdate(wApplyAgain, reqApplyAgain)

	if wApplyAgain.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK on already installed build, got %d: %s", wApplyAgain.Code, wApplyAgain.Body.String())
	}
	var upToDateResp map[string]interface{}
	_ = json.NewDecoder(wApplyAgain.Body).Decode(&upToDateResp)
	if upToDateResp["status"] != "up_to_date" {
		t.Errorf("Expected status up_to_date, got %v", upToDateResp["status"])
	}

	// 4. Concurrent update conflict (mutex lock)
	handler.updateMu.Lock()
	wConflict := httptest.NewRecorder()
	handler.HandleUpdate(wConflict, reqApply)
	handler.updateMu.Unlock()
	if wConflict.Code != http.StatusConflict {
		t.Errorf("Expected 409 Conflict when update is in progress, got %d", wConflict.Code)
	}

	// 5. Check error path when updater returns 500
	errorServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer errorServer.Close()
	updater.FillAPIBaseURL = errorServer.URL

	wCheckErr := httptest.NewRecorder()
	handler.HandleCheckUpdate(wCheckErr, reqCheck)
	if wCheckErr.Code != http.StatusInternalServerError {
		t.Errorf("Expected 500 when remote updater errors, got %d", wCheckErr.Code)
	}
}


