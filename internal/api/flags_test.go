package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"paperMC_backend/internal/database"
	"paperMC_backend/internal/flags"
	"paperMC_backend/internal/minecraft"
)

func setupFlagsTestHandler(t *testing.T) (*Handler, database.Store, *minecraft.Server) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "flags_test.db")
	store, err := database.NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to init store: %v", err)
	}

	server := minecraft.NewServer(tmpDir, "server.jar", "8G", store)
	handler := NewServerHandler(server, store)
	return handler, store, server
}

func TestHandleGetFlags_DefaultAndConfigured(t *testing.T) {
	handler, store, server := setupFlagsTestHandler(t)
	defer store.Close()

	// 1. Initial GET /api/flags (migration v3 seeded 8G aikar)
	req := httptest.NewRequest(http.MethodGet, "/api/flags", nil)
	w := httptest.NewRecorder()
	handler.HandleGetFlags(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp FlagsStatusResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if resp.Configured.RAM != "8G" || resp.Configured.Preset != "aikar" {
		t.Errorf("Expected default 8G aikar, got %+v", resp.Configured)
	}
	if resp.ServerRunning != false {
		t.Errorf("Expected ServerRunning to be false")
	}
	if resp.RestartRequired != false {
		t.Errorf("Expected RestartRequired to be false when stopped")
	}
	if len(resp.EffectiveFlags) == 0 {
		t.Errorf("Expected non-empty effective flags")
	}

	// 2. Simulate running server with different active args -> restart_required should be true
	server.RAM = "4G"
	reqRun := httptest.NewRequest(http.MethodGet, "/api/flags", nil)
	wRun := httptest.NewRecorder()
	handler.HandleGetFlags(wRun, reqRun)

	// Method not allowed check
	reqBad := httptest.NewRequest(http.MethodPost, "/api/flags", nil)
	wBad := httptest.NewRecorder()
	handler.HandleGetFlags(wBad, reqBad)
	if wBad.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected 405 for POST to HandleGetFlags, got %d", wBad.Code)
	}
}

func TestHandleSaveFlags_ValidationAndSuccess(t *testing.T) {
	handler, store, server := setupFlagsTestHandler(t)
	defer store.Close()

	// 1. Valid Save (12G Aikar)
	payload := SaveFlagsRequest{
		RAM:         "12G",
		Preset:      "aikar",
		CustomFlags: "",
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/flags", bytes.NewReader(body))
	w := httptest.NewRecorder()
	handler.HandleSaveFlags(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp FlagsStatusResponse
	_ = json.NewDecoder(w.Body).Decode(&resp)
	if resp.Configured.RAM != "12G" || resp.Configured.Preset != "aikar" {
		t.Errorf("Expected 12G aikar, got %+v", resp.Configured)
	}
	if server.RAM != "12G" {
		t.Errorf("Expected server.RAM to be updated to 12G, got %s", server.RAM)
	}

	// 2. Invalid RAM value
	badRAM := SaveFlagsRequest{RAM: "invalid_ram", Preset: "aikar"}
	bodyBadRAM, _ := json.Marshal(badRAM)
	reqBadRAM := httptest.NewRequest(http.MethodPost, "/api/flags", bytes.NewReader(bodyBadRAM))
	wBadRAM := httptest.NewRecorder()
	handler.HandleSaveFlags(wBadRAM, reqBadRAM)
	if wBadRAM.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 for bad RAM, got %d", wBadRAM.Code)
	}

	// 3. Invalid Preset
	badPreset := SaveFlagsRequest{RAM: "8G", Preset: "not_a_real_preset"}
	bodyBadPreset, _ := json.Marshal(badPreset)
	reqBadPreset := httptest.NewRequest(http.MethodPost, "/api/flags", bytes.NewReader(bodyBadPreset))
	wBadPreset := httptest.NewRecorder()
	handler.HandleSaveFlags(wBadPreset, reqBadPreset)
	if wBadPreset.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 for bad preset, got %d", wBadPreset.Code)
	}

	// 4. Malformed JSON
	reqBadJSON := httptest.NewRequest(http.MethodPost, "/api/flags", strings.NewReader("{invalid-json"))
	wBadJSON := httptest.NewRecorder()
	handler.HandleSaveFlags(wBadJSON, reqBadJSON)
	if wBadJSON.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 for malformed json, got %d", wBadJSON.Code)
	}

	// 5. Method not allowed
	reqBadMethod := httptest.NewRequest(http.MethodGet, "/api/flags", nil)
	wBadMethod := httptest.NewRecorder()
	handler.HandleSaveFlags(wBadMethod, reqBadMethod)
	if wBadMethod.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected 405 for GET to HandleSaveFlags, got %d", wBadMethod.Code)
	}
}

func TestHandleGetFlagPresets(t *testing.T) {
	handler, store, _ := setupFlagsTestHandler(t)
	defer store.Close()

	// 1. GET without param (default 8G)
	req := httptest.NewRequest(http.MethodGet, "/api/flags/presets", nil)
	w := httptest.NewRecorder()
	handler.HandleGetFlagPresets(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", w.Code)
	}

	var presets []flags.PresetInfo
	if err := json.NewDecoder(w.Body).Decode(&presets); err != nil {
		t.Fatalf("Failed to decode presets: %v", err)
	}
	if len(presets) != 4 {
		t.Fatalf("Expected 4 presets, got %d", len(presets))
	}

	// 2. GET with ?ram=16G
	req16G := httptest.NewRequest(http.MethodGet, "/api/flags/presets?ram=16G", nil)
	w16G := httptest.NewRecorder()
	handler.HandleGetFlagPresets(w16G, req16G)

	var presets16G []flags.PresetInfo
	_ = json.NewDecoder(w16G.Body).Decode(&presets16G)
	for _, p := range presets16G {
		if p.ID == flags.PresetAikar {
			if !strings.Contains(p.Description, ">=12GB") {
				t.Errorf("Expected 16G preset to mention >=12GB, got: %s", p.Description)
			}
		}
	}

	// 3. Method not allowed
	reqBad := httptest.NewRequest(http.MethodPost, "/api/flags/presets", nil)
	wBad := httptest.NewRecorder()
	handler.HandleGetFlagPresets(wBad, reqBad)
	if wBad.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected 405 for POST to HandleGetFlagPresets, got %d", wBad.Code)
	}
}
