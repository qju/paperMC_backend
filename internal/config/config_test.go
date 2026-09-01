package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigLoad(t *testing.T) {
	// Test Default Values
	cfg := Load()
	if cfg.Port != "8080" {
		t.Errorf("Expected default Port '8080', got '%s'", cfg.Port)
	}
	if cfg.WorkDir != "./paperMS" {
		t.Errorf("Expected default WorkDir './paperMS', got '%s'", cfg.WorkDir)
	}
	if cfg.JarFile != "server.jar" {
		t.Errorf("Expected default JarFile 'server.jar', got '%s'", cfg.JarFile)
	}
	if cfg.RAM != "8G" {
		t.Errorf("Expected default RAM '8G', got '%s'", cfg.RAM)
	}
	if cfg.DBName != "paper.db" {
		t.Errorf("Expected default DBName 'paper.db', got '%s'", cfg.DBName)
	}
	if cfg.AdminUser != "admin" {
		t.Errorf("Expected default AdminUser 'admin', got '%s'", cfg.AdminUser)
	}

	// Test Environment Overrides
	_ = os.Setenv("PORT", "9090")
	_ = os.Setenv("MC_WORKDIR", "/custom/path")
	_ = os.Setenv("JAR_FILE", "paper-custom.jar")
	_ = os.Setenv("RAM", "16G")
	_ = os.Setenv("DBNAME", "test.db")
	_ = os.Setenv("ADMIN_USER", "superadmin")
	_ = os.Setenv("ADMIN_PASS", "supersecret")

	defer func() {
		_ = os.Unsetenv("PORT")
		_ = os.Unsetenv("MC_WORKDIR")
		_ = os.Unsetenv("JAR_FILE")
		_ = os.Unsetenv("RAM")
		_ = os.Unsetenv("DBNAME")
		_ = os.Unsetenv("ADMIN_USER")
		_ = os.Unsetenv("ADMIN_PASS")
	}()

	customCfg := Load()
	if customCfg.Port != "9090" || customCfg.WorkDir != "/custom/path" || customCfg.JarFile != "paper-custom.jar" ||
		customCfg.RAM != "16G" || customCfg.DBName != "test.db" || customCfg.AdminUser != "superadmin" || customCfg.AdminPass != "supersecret" {
		t.Errorf("Custom config values not loaded correctly: %+v", customCfg)
	}
}

func TestSaveProperties_PreserveCommentsAndAppend(t *testing.T) {
	tmpDir := t.TempDir()
	initialContent := `# Minecraft Server Properties
# Custom Comment
difficulty=easy
pvp=true
motd=Original MOTD
`
	err := os.WriteFile(filepath.Join(tmpDir, "server.properties"), []byte(initialContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create initial file: %v", err)
	}

	changes := map[string]string{
		"difficulty":  "hard",
		"max-players": "50",
	}

	err = SaveProperties(tmpDir, changes)
	if err != nil {
		t.Fatalf("SaveProperties failed: %v", err)
	}

	contentBytes, err := os.ReadFile(filepath.Join(tmpDir, "server.properties"))
	if err != nil {
		t.Fatalf("Failed to read updated file: %v", err)
	}
	content := string(contentBytes)

	if !strings.Contains(content, "# Minecraft Server Properties") {
		t.Errorf("Expected comments to be preserved in file")
	}
	if !strings.Contains(content, "difficulty=hard") {
		t.Errorf("Expected difficulty to be updated to hard")
	}
	if !strings.Contains(content, "pvp=true") {
		t.Errorf("Expected untouched property pvp=true to remain")
	}
	if !strings.Contains(content, "max-players=50") {
		t.Errorf("Expected new property max-players=50 to be appended")
	}
}

func TestLoadProperties_NonExistentFile(t *testing.T) {
	tmpDir := t.TempDir()
	_, err := LoadProperties(filepath.Join(tmpDir, "non_existent"))
	if err == nil {
		t.Errorf("Expected error loading properties from non-existent directory, got nil")
	}
}
