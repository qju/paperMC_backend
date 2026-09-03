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
)

func TestConfigAPI(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")
	store, err := database.NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to init store: %v", err)
	}
	defer store.Close()

	// Create initial server.properties
	initialProps := `difficulty=normal
max-players=20
motd=A Minecraft Server
`
	_ = os.WriteFile(filepath.Join(tempDir, "server.properties"), []byte(initialProps), 0644)

	mcServer := minecraft.NewServer(tempDir, "server.jar", "2G", store)
	handler := NewServerHandler(mcServer, store)

	// 1. Test GET /config
	reqGet := httptest.NewRequest(http.MethodGet, "/config", nil)
	wGet := httptest.NewRecorder()
	handler.GetConfig(wGet, reqGet)

	if wGet.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK, got %d: %s", wGet.Code, wGet.Body.String())
	}

	var props map[string]string
	if err := json.NewDecoder(wGet.Body).Decode(&props); err != nil {
		t.Fatalf("Failed to decode config response: %v", err)
	}
	if props["difficulty"] != "normal" || props["max-players"] != "20" {
		t.Errorf("Unexpected properties returned: %+v", props)
	}

	// 2. Test POST /config
	updatedProps := map[string]string{
		"difficulty":  "hard",
		"max-players": "50",
	}
	body, _ := json.Marshal(updatedProps)
	reqPost := httptest.NewRequest(http.MethodPost, "/config", bytes.NewReader(body))
	wPost := httptest.NewRecorder()
	handler.PostConfig(wPost, reqPost)

	if wPost.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK on PostConfig, got %d: %s", wPost.Code, wPost.Body.String())
	}

	// Verify updated properties via GET
	wGet2 := httptest.NewRecorder()
	handler.GetConfig(wGet2, reqGet)
	var props2 map[string]string
	_ = json.NewDecoder(wGet2.Body).Decode(&props2)
	if props2["difficulty"] != "hard" || props2["max-players"] != "50" {
		t.Errorf("Updated properties mismatch: %+v", props2)
	}

	// 3. Test POST /config with invalid JSON
	reqBad := httptest.NewRequest(http.MethodPost, "/config", bytes.NewReader([]byte("invalid")))
	wBad := httptest.NewRecorder()
	handler.PostConfig(wBad, reqBad)

	if wBad.Code != http.StatusBadRequest {
		t.Fatalf("Expected 400 Bad Request on invalid JSON, got %d", wBad.Code)
	}
}
