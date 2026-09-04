package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"paperMC_backend/internal/config"
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

	// 6. Test POST /api/worlds/create
	t.Run("Create World", func(t *testing.T) {
		createBody, _ := json.Marshal(SetActiveWorldRequest{
			WorldName: "new_creative_world",
		})
		reqCreate := httptest.NewRequest(http.MethodPost, "/api/worlds/create", bytes.NewReader(createBody))
		wCreate := httptest.NewRecorder()
		handler.HandleCreateWorld(wCreate, reqCreate)

		if wCreate.Code != http.StatusOK {
			t.Errorf("Expected 200 OK creating world, got %d: %s", wCreate.Code, wCreate.Body.String())
		}

		// Verify server.properties has updated level-name
		props, err := config.LoadProperties(tempDir)
		if err != nil || props["level-name"] != "new_creative_world" {
			t.Errorf("Expected server.properties level-name to be 'new_creative_world', got: %v, %+v", err, props)
		}

		// Missing name fails
		badCreate := httptest.NewRequest(http.MethodPost, "/api/worlds/create", bytes.NewReader([]byte(`{"world_name":""}`)))
		wBadCreate := httptest.NewRecorder()
		handler.HandleCreateWorld(wBadCreate, badCreate)
		if wBadCreate.Code != http.StatusBadRequest {
			t.Errorf("Expected 400 on empty world name, got %d", wBadCreate.Code)
		}
	})

	// 7. Test POST /api/worlds/active
	t.Run("Set Active World", func(t *testing.T) {
		// Empty name fails
		badActive := httptest.NewRequest(http.MethodPost, "/api/worlds/active", bytes.NewReader([]byte(`{"world_name":""}`)))
		wBadActive := httptest.NewRecorder()
		handler.HandleSetActiveWorld(wBadActive, badActive)
		if wBadActive.Code != http.StatusBadRequest {
			t.Errorf("Expected 400 on empty active world name, got %d", wBadActive.Code)
		}

		// Valid name switches server.properties when server is stopped
		goodActive := httptest.NewRequest(http.MethodPost, "/api/worlds/active", bytes.NewReader([]byte(`{"world_name":"new_creative_world"}`)))
		wGoodActive := httptest.NewRecorder()
		handler.HandleSetActiveWorld(wGoodActive, goodActive)
		if wGoodActive.Code != http.StatusOK {
			t.Errorf("Expected 200 setting active world, got %d: %s", wGoodActive.Code, wGoodActive.Body.String())
		}
	})

	// 8. Delete world validation
	t.Run("Delete World Validation", func(t *testing.T) {
		reqEmptyDel := httptest.NewRequest(http.MethodDelete, "/api/worlds", nil)
		wEmptyDel := httptest.NewRecorder()
		handler.HandleDeleteWorld(wEmptyDel, reqEmptyDel)
		if wEmptyDel.Code != http.StatusBadRequest {
			t.Errorf("Expected 400 on empty delete name, got %d", wEmptyDel.Code)
		}
	})
}

func TestWorldsAdditionalValidation(t *testing.T) {
	tempDir := t.TempDir()
	_ = os.WriteFile(filepath.Join(tempDir, "server.properties"), []byte("level-name=world\n"), 0644)

	mcServer := &minecraft.Server{
		WorkDir: tempDir,
		JarFile: "server.jar",
		RAM:     "2G",
	}
	handler := NewServerHandler(mcServer, nil)

	// 1. SetActiveWorld invalid JSON
	badJSON := httptest.NewRequest(http.MethodPost, "/api/worlds/active", bytes.NewReader([]byte(`not-json`)))
	wBadJSON := httptest.NewRecorder()
	handler.HandleSetActiveWorld(wBadJSON, badJSON)
	if wBadJSON.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 on bad JSON in SetActiveWorld, got %d", wBadJSON.Code)
	}

	// 2. SetActiveWorld with seed
	seed := "987654321"
	seedBody, _ := json.Marshal(SetActiveWorldRequest{
		WorldName: "world_with_seed",
		Seed:      &seed,
	})
	reqSeed := httptest.NewRequest(http.MethodPost, "/api/worlds/active", bytes.NewReader(seedBody))
	wSeed := httptest.NewRecorder()
	handler.HandleSetActiveWorld(wSeed, reqSeed)
	if wSeed.Code != http.StatusOK {
		t.Errorf("Expected 200 on SetActiveWorld with seed, got %d", wSeed.Code)
	}

	// 3. DuplicateWorld invalid JSON
	badDupJSON := httptest.NewRequest(http.MethodPost, "/api/worlds/duplicate", bytes.NewReader([]byte(`bad`)))
	wBadDupJSON := httptest.NewRecorder()
	handler.HandleDuplicateWorld(wBadDupJSON, badDupJSON)
	if wBadDupJSON.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 on bad JSON in DuplicateWorld, got %d", wBadDupJSON.Code)
	}

	// 4. DuplicateWorld empty source or target
	dupNoSource, _ := json.Marshal(DuplicateWorldRequest{SourceWorld: "", TargetWorld: "target"})
	reqNoSource := httptest.NewRequest(http.MethodPost, "/api/worlds/duplicate", bytes.NewReader(dupNoSource))
	wNoSource := httptest.NewRecorder()
	handler.HandleDuplicateWorld(wNoSource, reqNoSource)
	if wNoSource.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 on empty source in DuplicateWorld, got %d", wNoSource.Code)
	}

	dupNoTarget, _ := json.Marshal(DuplicateWorldRequest{SourceWorld: "source", TargetWorld: ""})
	reqNoTarget := httptest.NewRequest(http.MethodPost, "/api/worlds/duplicate", bytes.NewReader(dupNoTarget))
	wNoTarget := httptest.NewRecorder()
	handler.HandleDuplicateWorld(wNoTarget, reqNoTarget)
	if wNoTarget.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 on empty target in DuplicateWorld, got %d", wNoTarget.Code)
	}

	// 5. CreateWorld invalid JSON
	badCreateJSON := httptest.NewRequest(http.MethodPost, "/api/worlds/create", bytes.NewReader([]byte(`bad`)))
	wBadCreateJSON := httptest.NewRecorder()
	handler.HandleCreateWorld(wBadCreateJSON, badCreateJSON)
	if wBadCreateJSON.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 on bad JSON in CreateWorld, got %d", wBadCreateJSON.Code)
	}
}

func TestSetActiveWorldRunningServer(t *testing.T) {
	origExec := minecraft.ExecCommandContext
	defer func() { minecraft.ExecCommandContext = origExec }()

	minecraft.ExecCommandContext = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "cat")
	}

	tempDir := t.TempDir()
	_ = os.WriteFile(filepath.Join(tempDir, "server.properties"), []byte("level-name=world\n"), 0644)

	mcServer := minecraft.NewServer(tempDir, "server.jar", "2G", nil)
	handler := NewServerHandler(mcServer, nil)

	// Start server
	if err := mcServer.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer mcServer.Kill()

	// Call HandleSetActiveWorld while running
	req := httptest.NewRequest(http.MethodPost, "/api/worlds/active", bytes.NewReader([]byte(`{"world_name":"switched_running_world"}`)))
	w := httptest.NewRecorder()
	handler.HandleSetActiveWorld(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200 OK switching active world on running server, got %d: %s", w.Code, w.Body.String())
	}

	props, err := config.LoadProperties(tempDir)
	if err != nil || props["level-name"] != "switched_running_world" {
		t.Errorf("Expected level-name to be switched_running_world, got: %v, %v", err, props["level-name"])
	}
}

