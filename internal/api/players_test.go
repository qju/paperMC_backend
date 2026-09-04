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

func TestPlayersExecutionEndpoints(t *testing.T) {
	tempDir := t.TempDir()
	store, _ := database.NewSQLiteStore(filepath.Join(tempDir, "test.db"))
	defer store.Close()

	mcServer := minecraft.NewServer(tempDir, "server.jar", "2G", store)
	handler := NewServerHandler(mcServer, store)

	// 1. Remove player stopped server
	reqRem := httptest.NewRequest(http.MethodDelete, "/api/players?username=Steve", nil)
	wRem := httptest.NewRecorder()
	handler.HandleRemovePlayer(wRem, reqRem)
	if wRem.Code != http.StatusInternalServerError {
		t.Errorf("Expected 500 removing player from stopped server, got %d", wRem.Code)
	}

	// 2. Ban player stopped server
	reqBan := httptest.NewRequest(http.MethodPost, "/api/players/banned", bytes.NewReader([]byte(`{"username":"Griefer","reason":"cheat"}`)))
	wBan := httptest.NewRecorder()
	handler.HandleBanPlayer(wBan, reqBan)
	if wBan.Code != http.StatusInternalServerError {
		t.Errorf("Expected 500 banning player on stopped server, got %d", wBan.Code)
	}

	// 3. Unban player stopped server
	reqUnban := httptest.NewRequest(http.MethodDelete, "/api/players/banned?username=Griefer", nil)
	wUnban := httptest.NewRecorder()
	handler.HandleUnbanPlayer(wUnban, reqUnban)
	if wUnban.Code != http.StatusInternalServerError {
		t.Errorf("Expected 500 unbanning player on stopped server, got %d", wUnban.Code)
	}

	// 4. Op player action=remove
	reqDeop := httptest.NewRequest(http.MethodPost, "/api/players/ops?action=remove", bytes.NewReader([]byte(`{"username":"Admin"}`)))
	wDeop := httptest.NewRecorder()
	handler.HandleOpPlayer(wDeop, reqDeop)
	if wDeop.Code != http.StatusInternalServerError {
		t.Errorf("Expected 500 deoping on stopped server, got %d", wDeop.Code)
	}

	// 5. Op player invalid action on stopped server returns 500
	reqBadAction := httptest.NewRequest(http.MethodPost, "/api/players/ops?action=invalid", bytes.NewReader([]byte(`{"username":"Admin"}`)))
	wBadAction := httptest.NewRecorder()
	handler.HandleOpPlayer(wBadAction, reqBadAction)
	if wBadAction.Code != http.StatusInternalServerError {
		t.Errorf("Expected 500 on stopped server, got %d", wBadAction.Code)
	}

	// 6. Op player empty username on stopped server returns 500
	reqNoUser := httptest.NewRequest(http.MethodPost, "/api/players/ops?action=add", bytes.NewReader([]byte(`{"username":""}`)))
	wNoUser := httptest.NewRecorder()
	handler.HandleOpPlayer(wNoUser, reqNoUser)
	if wNoUser.Code != http.StatusInternalServerError {
		t.Errorf("Expected 500 on stopped server, got %d", wNoUser.Code)
	}

	// 7. Add player running server with mock Mojang
	origMojang := minecraft.MojangBaseURL
	defer func() { minecraft.MojangBaseURL = origMojang }()

	origExec := minecraft.ExecCommandContext
	defer func() { minecraft.ExecCommandContext = origExec }()

	minecraft.ExecCommandContext = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "cat")
	}

	mockMojang := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"name":"Alex","id":"alex-uuid"}`))
	}))
	defer mockMojang.Close()
	minecraft.MojangBaseURL = mockMojang.URL

	if err := mcServer.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer mcServer.Kill()

	// Seed rejected player
	_ = store.UpsertRejectedPlayer("Alex")

	reqAdd := httptest.NewRequest(http.MethodPost, "/api/players", bytes.NewReader([]byte(`{"username":"Alex"}`)))
	wAdd := httptest.NewRecorder()
	handler.HandleAddPlayer(wAdd, reqAdd)

	if wAdd.Code != http.StatusOK {
		t.Errorf("Expected 200 adding player on running server, got %d: %s", wAdd.Code, wAdd.Body.String())
	}

	// Verify Alex was removed from rejected list
	rejections, _ := store.GetRejectedPlayers()
	for _, rej := range rejections {
		if rej.Username == "Alex" {
			t.Errorf("Expected Alex to be removed from rejected players")
		}
	}
}

