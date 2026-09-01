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

func TestPlayersAPI(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test_players.db")
	store, err := database.NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to init store: %v", err)
	}
	defer store.Close()

	// Seed files
	wlData := []minecraft.Player{{UUID: "uuid-1", UserName: "WhitelistedOne"}}
	bannedData := []minecraft.Player{{UUID: "uuid-2", UserName: "BannedOne", Reason: "Rule Violation"}}
	opsData := []minecraft.Player{{UUID: "uuid-3", UserName: "OpOne", Level: 4}}

	wb, _ := json.Marshal(wlData)
	bb, _ := json.Marshal(bannedData)
	ob, _ := json.Marshal(opsData)

	_ = os.WriteFile(filepath.Join(tempDir, "whitelist.json"), wb, 0644)
	_ = os.WriteFile(filepath.Join(tempDir, "banned-players.json"), bb, 0644)
	_ = os.WriteFile(filepath.Join(tempDir, "ops.json"), ob, 0644)

	// Seed rejected players in DB
	_ = store.UpsertRejectedPlayer("RejectedPlayer1")

	mcServer := minecraft.NewServer(tempDir, "server.jar", "2G", store)
	handler := NewServerHandler(mcServer, store)

	// 1. GET Whitelist
	t.Run("GET /api/players", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/players", nil)
		w := httptest.NewRecorder()
		handler.HandleGetPlayers(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("Expected 200 OK, got %d", w.Code)
		}
		var list []minecraft.Player
		_ = json.NewDecoder(w.Body).Decode(&list)
		if len(list) != 1 || list[0].UserName != "WhitelistedOne" {
			t.Errorf("Unexpected whitelist: %+v", list)
		}
	})

	// 2. GET Banned
	t.Run("GET /api/players/banned", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/players/banned", nil)
		w := httptest.NewRecorder()
		handler.HandleGetBanned(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("Expected 200 OK, got %d", w.Code)
		}
		var list []minecraft.Player
		_ = json.NewDecoder(w.Body).Decode(&list)
		if len(list) != 1 || list[0].UserName != "BannedOne" {
			t.Errorf("Unexpected banned list: %+v", list)
		}
	})

	// 3. GET Ops
	t.Run("GET /api/players/ops", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/players/ops", nil)
		w := httptest.NewRecorder()
		handler.HandleGetOps(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("Expected 200 OK, got %d", w.Code)
		}
		var list []minecraft.Player
		_ = json.NewDecoder(w.Body).Decode(&list)
		if len(list) != 1 || list[0].UserName != "OpOne" {
			t.Errorf("Unexpected ops list: %+v", list)
		}
	})

	// 4. GET Rejected
	t.Run("GET /api/players/rejected", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/players/rejected", nil)
		w := httptest.NewRecorder()
		handler.HandleGetRejected(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("Expected 200 OK, got %d", w.Code)
		}
		var list []database.RejectedPlayer
		_ = json.NewDecoder(w.Body).Decode(&list)
		if len(list) != 1 || list[0].Username != "RejectedPlayer1" {
			t.Errorf("Unexpected rejected list: %+v", list)
		}
	})

	// 5. DELETE Rejected
	t.Run("DELETE /api/players/rejected", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/api/players/rejected?username=RejectedPlayer1", nil)
		w := httptest.NewRecorder()
		handler.HandleDeleteRejected(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("Expected 200 OK, got %d", w.Code)
		}

		// Verify deletion
		list, _ := store.GetRejectedPlayers()
		if len(list) != 0 {
			t.Errorf("Expected rejected player list to be empty after deletion")
		}

		// Empty username fails with 400
		reqBad := httptest.NewRequest(http.MethodDelete, "/api/players/rejected", nil)
		wBad := httptest.NewRecorder()
		handler.HandleDeleteRejected(wBad, reqBad)
		if wBad.Code != http.StatusBadRequest {
			t.Errorf("Expected 400 when missing username, got %d", wBad.Code)
		}
	})

	// 6. Request validation errors on modifying players
	t.Run("Validation Errors", func(t *testing.T) {
		// Empty username in AddPlayer
		reqAddEmpty := httptest.NewRequest(http.MethodPost, "/api/players", bytes.NewReader([]byte(`{"username":""}`)))
		wAddEmpty := httptest.NewRecorder()
		handler.HandleAddPlayer(wAddEmpty, reqAddEmpty)
		if wAddEmpty.Code != http.StatusBadRequest {
			t.Errorf("Expected 400 on empty username add, got %d", wAddEmpty.Code)
		}

		// Missing username query param in RemovePlayer
		reqRemEmpty := httptest.NewRequest(http.MethodDelete, "/api/players", nil)
		wRemEmpty := httptest.NewRecorder()
		handler.HandleRemovePlayer(wRemEmpty, reqRemEmpty)
		if wRemEmpty.Code != http.StatusBadRequest {
			t.Errorf("Expected 400 on empty username remove, got %d", wRemEmpty.Code)
		}

		// Empty username in BanPlayer
		reqBanEmpty := httptest.NewRequest(http.MethodPost, "/api/players/banned", bytes.NewReader([]byte(`{"username":""}`)))
		wBanEmpty := httptest.NewRecorder()
		handler.HandleBanPlayer(wBanEmpty, reqBanEmpty)
		if wBanEmpty.Code != http.StatusBadRequest {
			t.Errorf("Expected 400 on empty username ban, got %d", wBanEmpty.Code)
		}

		// Missing username in UnbanPlayer
		reqUnbanEmpty := httptest.NewRequest(http.MethodDelete, "/api/players/banned", nil)
		wUnbanEmpty := httptest.NewRecorder()
		handler.HandleUnbanPlayer(wUnbanEmpty, reqUnbanEmpty)
		if wUnbanEmpty.Code != http.StatusBadRequest {
			t.Errorf("Expected 400 on empty username unban, got %d", wUnbanEmpty.Code)
		}

		// Bad JSON bodies
		badReq := httptest.NewRequest(http.MethodPost, "/api/players", bytes.NewReader([]byte(`invalid`)))
		badRec := httptest.NewRecorder()
		handler.HandleAddPlayer(badRec, badReq)
		if badRec.Code != http.StatusBadRequest {
			t.Errorf("Expected 400 on bad JSON, got %d", badRec.Code)
		}

		// HandleOpPlayer invalid JSON
		badOp := httptest.NewRequest(http.MethodPost, "/api/players/ops", bytes.NewReader([]byte(`invalid`)))
		badOpRec := httptest.NewRecorder()
		handler.HandleOpPlayer(badOpRec, badOp)
		if badOpRec.Code != http.StatusBadRequest {
			t.Errorf("Expected 400 on bad Op JSON, got %d", badOpRec.Code)
		}

		// HandleOpPlayer stopped server
		opReq := httptest.NewRequest(http.MethodPost, "/api/players/ops?action=add", bytes.NewReader([]byte(`{"username":"Player1"}`)))
		opRec := httptest.NewRecorder()
		handler.HandleOpPlayer(opRec, opReq)
		if opRec.Code != http.StatusInternalServerError {
			t.Errorf("Expected 500 when oping on stopped server, got %d", opRec.Code)
		}
	})
}

