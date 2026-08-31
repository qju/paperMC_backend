package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"paperMC_backend/internal/minecraft"
)

func TestWorldsAPIHandlers(t *testing.T) {
	tempDir := t.TempDir()

	// 1. Setup mock world
	worldDir := filepath.Join(tempDir, "world")
	_ = os.MkdirAll(worldDir, 0755)
	_ = os.WriteFile(filepath.Join(worldDir, "level.dat"), []byte("mock-data"), 0644)

	// Mock server.properties
	_ = os.WriteFile(filepath.Join(tempDir, "server.properties"), []byte("level-name=world\n"), 0644)

	mcServer := &minecraft.Server{
		WorkDir: tempDir,
		JarFile: "server.jar",
		RAM:     "2G",
	}
	handler := NewServerHandler(mcServer, nil)

	// 2. Test GET /api/worlds
	req := httptest.NewRequest(http.MethodGet, "/api/worlds", nil)
	w := httptest.NewRecorder()
	handler.HandleGetWorlds(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("HandleGetWorlds returned status %d, expected 200", w.Code)
	}

	var worldsResp GetWorldsResponse
	if err := json.NewDecoder(w.Body).Decode(&worldsResp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}
	if worldsResp.ActiveWorld != "world" {
		t.Errorf("Expected ActiveWorld 'world', got '%s'", worldsResp.ActiveWorld)
	}
	if len(worldsResp.Worlds) != 1 {
		t.Fatalf("Expected 1 world, got %d", len(worldsResp.Worlds))
	}

	// 3. Test POST /api/worlds/duplicate
	dupBody, _ := json.Marshal(DuplicateWorldRequest{
		SourceWorld: "world",
		TargetWorld: "world_copy",
	})
	reqDup := httptest.NewRequest(http.MethodPost, "/api/worlds/duplicate", bytes.NewReader(dupBody))
	wDup := httptest.NewRecorder()
	handler.HandleDuplicateWorld(wDup, reqDup)

	if wDup.Code != http.StatusOK {
		t.Fatalf("HandleDuplicateWorld returned status %d, expected 200. Body: %s", wDup.Code, wDup.Body.String())
	}

	// 4. Test DELETE /api/worlds (Active World -> Rejection)
	reqDelActive := httptest.NewRequest(http.MethodDelete, "/api/worlds?name=world", nil)
	wDelActive := httptest.NewRecorder()
	handler.HandleDeleteWorld(wDelActive, reqDelActive)

	if wDelActive.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400 when deleting active world, got %d", wDelActive.Code)
	}

	// 5. Test DELETE /api/worlds (Inactive Copy -> Success)
	reqDelCopy := httptest.NewRequest(http.MethodDelete, "/api/worlds?name=world_copy", nil)
	wDelCopy := httptest.NewRecorder()
	handler.HandleDeleteWorld(wDelCopy, reqDelCopy)

	if wDelCopy.Code != http.StatusOK {
		t.Errorf("Expected status 200 when deleting inactive world, got %d", wDelCopy.Code)
	}
}
