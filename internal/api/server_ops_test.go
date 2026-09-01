package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

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


