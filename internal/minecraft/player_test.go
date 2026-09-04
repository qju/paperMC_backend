package minecraft

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestPlayerFileOperations(t *testing.T) {
	tmpDir := t.TempDir()
	server := NewServer(tmpDir, "server.jar", "4G", nil)

	// 1. Non-existent files return empty slice without error
	wl, err := server.GetWhiteList()
	if err != nil || len(wl) != 0 {
		t.Errorf("Expected empty whitelist from non-existent file, got err: %v, len: %d", err, len(wl))
	}
	banned, err := server.GetBanned()
	if err != nil || len(banned) != 0 {
		t.Errorf("Expected empty banned list from non-existent file, got err: %v, len: %d", err, len(banned))
	}
	ops, err := server.GetOps()
	if err != nil || len(ops) != 0 {
		t.Errorf("Expected empty ops list from non-existent file, got err: %v, len: %d", err, len(ops))
	}

	// 2. Write valid player files and verify
	whitelistData := []Player{
		{UUID: "11111111-1111-1111-1111-111111111111", UserName: "PlayerOne"},
		{UUID: "22222222-2222-2222-2222-222222222222", UserName: "PlayerTwo"},
	}
	bannedData := []Player{
		{UUID: "33333333-3333-3333-3333-333333333333", UserName: "Griefer", Reason: "Griefing"},
	}
	opsData := []Player{
		{UUID: "44444444-4444-4444-4444-444444444444", UserName: "AdminUser", Level: 4},
	}

	wb, _ := json.Marshal(whitelistData)
	bb, _ := json.Marshal(bannedData)
	ob, _ := json.Marshal(opsData)

	_ = os.WriteFile(filepath.Join(tmpDir, "whitelist.json"), wb, 0644)
	_ = os.WriteFile(filepath.Join(tmpDir, "banned-players.json"), bb, 0644)
	_ = os.WriteFile(filepath.Join(tmpDir, "ops.json"), ob, 0644)

	wl, err = server.GetWhiteList()
	if err != nil || len(wl) != 2 || wl[0].UserName != "PlayerOne" {
		t.Errorf("Failed to read whitelist.json: %v, got %+v", err, wl)
	}

	banned, err = server.GetBanned()
	if err != nil || len(banned) != 1 || banned[0].Reason != "Griefing" {
		t.Errorf("Failed to read banned-players.json: %v, got %+v", err, banned)
	}

	ops, err = server.GetOps()
	if err != nil || len(ops) != 1 || ops[0].Level != 4 {
		t.Errorf("Failed to read ops.json: %v, got %+v", err, ops)
	}

	// 3. Corrupted JSON file returns error
	_ = os.WriteFile(filepath.Join(tmpDir, "whitelist.json"), []byte("invalid-json"), 0644)
	_, err = server.GetWhiteList()
	if err == nil {
		t.Errorf("Expected error when reading malformed whitelist.json, got nil")
	}
}

func TestPlayerCommandMethodsWhenStopped(t *testing.T) {
	tmpDir := t.TempDir()
	server := NewServer(tmpDir, "server.jar", "4G", nil)

	// Since server is stopped, all command methods return "server is already stopped" error
	if err := server.RemoveWhitelist("testuser"); err == nil {
		t.Errorf("Expected error sending whitelist remove when stopped, got nil")
	}
	if err := server.BanUser("testuser", "reason"); err == nil {
		t.Errorf("Expected error sending ban when stopped, got nil")
	}
	if err := server.UnbanUser("testuser"); err == nil {
		t.Errorf("Expected error sending unban when stopped, got nil")
	}
	if err := server.OpUser("testuser"); err == nil {
		t.Errorf("Expected error sending op when stopped, got nil")
	}
	if err := server.DeopUser("testuser"); err == nil {
		t.Errorf("Expected error sending deop when stopped, got nil")
	}
}

func TestWhiteListUser(t *testing.T) {
	origMojang := MojangBaseURL
	origGeyser := GeyserBaseURL
	defer func() {
		MojangBaseURL = origMojang
		GeyserBaseURL = origGeyser
	}()

	tmpDir := t.TempDir()
	server := NewServer(tmpDir, "server.jar", "4G", nil)

	// Mock Mojang & Geyser APIs
	mockMojang := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/users/profiles/minecraft/JavaUser" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"name":"JavaUser","id":"java-uuid-123"}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer mockMojang.Close()

	mockGeyser := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v2/xbox/xuid/BedrockUser" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"xuid":9988776655}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer mockGeyser.Close()

	MojangBaseURL = mockMojang.URL
	GeyserBaseURL = mockGeyser.URL

	// 1. Java User: Mojang matches -> sends whitelist add (returns stopped error)
	errJava := server.WhiteListUser("JavaUser")
	if errJava == nil || errJava.Error() != "server is already stopped" {
		t.Errorf("Expected stopped error for JavaUser, got: %v", errJava)
	}

	// 2. Bedrock User: Geyser matches -> sends fwhitelist add (returns stopped error)
	errBedrock := server.WhiteListUser("BedrockUser")
	if errBedrock == nil || errBedrock.Error() != "server is already stopped" {
		t.Errorf("Expected stopped error for BedrockUser, got: %v", errBedrock)
	}

	// 3. Bedrock User with prefix
	errBedrockPrefix := server.WhiteListUser("*BedrockUser")
	if errBedrockPrefix == nil || errBedrockPrefix.Error() != "server is already stopped" {
		t.Errorf("Expected stopped error for *BedrockUser, got: %v", errBedrockPrefix)
	}

	// 4. Unknown User: Neither matches -> returns user not found
	errUnknown := server.WhiteListUser("GhostUser")
	if errUnknown == nil || errUnknown.Error() != "user not found on Mojang or Xbox Live" {
		t.Errorf("Expected user not found error for GhostUser, got: %v", errUnknown)
	}
}

func TestBanUserDefaultReason(t *testing.T) {
	tmpDir := t.TempDir()
	server := NewServer(tmpDir, "server.jar", "4G", nil)

	// Ban with empty reason
	err := server.BanUser("spammer", "")
	if err == nil || err.Error() != "server is already stopped" {
		t.Errorf("Expected stopped error, got: %v", err)
	}
}
