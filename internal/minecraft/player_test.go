package minecraft

import (
	"encoding/json"
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
