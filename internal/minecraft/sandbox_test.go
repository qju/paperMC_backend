package minecraft

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultIsolationConfig(t *testing.T) {
	// Set test environment variables
	t.Setenv("ISOLATION_MODE", "user")
	t.Setenv("MINECRAFT_USER", "nobody")
	t.Setenv("MINECRAFT_UID", "65534")
	t.Setenv("MINECRAFT_GID", "65534")

	cfg := DefaultIsolationConfig()
	if cfg.Mode != IsolationUser {
		t.Errorf("Expected IsolationUser, got %s", cfg.Mode)
	}
	if cfg.MinecraftUser != "nobody" {
		t.Errorf("Expected nobody, got %s", cfg.MinecraftUser)
	}
	if cfg.UID != 65534 || cfg.GID != 65534 {
		t.Errorf("Expected UID=65534, GID=65534, got %d, %d", cfg.UID, cfg.GID)
	}
}

func TestBuildSandboxCommand_None(t *testing.T) {
	ctx := context.Background()
	cfg := IsolationConfig{Mode: IsolationNone}
	workDir := t.TempDir()
	javaArgs := []string{"-Xmx2G", "-jar", "server.jar"}

	cmd, err := BuildSandboxCommand(ctx, cfg, workDir, javaArgs)
	if err != nil {
		t.Fatalf("BuildSandboxCommand failed: %v", err)
	}

	if cmd.Path != "java" && !strings.HasSuffix(cmd.Path, "/java") {
		t.Errorf("Expected java binary, got %s", cmd.Path)
	}
	absWorkDir, _ := filepath.Abs(workDir)
	if cmd.Dir != absWorkDir {
		t.Errorf("Expected dir %s, got %s", absWorkDir, cmd.Dir)
	}
	if cmd.SysProcAttr != nil && cmd.SysProcAttr.Credential != nil {
		t.Errorf("SysProcAttr Credential should not be set in IsolationNone")
	}
}

func TestBuildSandboxCommand_User(t *testing.T) {
	ctx := context.Background()
	cfg := IsolationConfig{
		Mode: IsolationUser,
		UID:  1001,
		GID:  1001,
	}
	workDir := t.TempDir()
	javaArgs := []string{"-jar", "server.jar"}

	cmd, err := BuildSandboxCommand(ctx, cfg, workDir, javaArgs)
	if err != nil {
		t.Fatalf("BuildSandboxCommand failed: %v", err)
	}

	if cmd.SysProcAttr == nil || cmd.SysProcAttr.Credential == nil {
		t.Fatalf("Expected SysProcAttr.Credential to be set")
	}
	if cmd.SysProcAttr.Credential.Uid != 1001 || cmd.SysProcAttr.Credential.Gid != 1001 {
		t.Errorf("Expected UID 1001, GID 1001, got %d, %d", cmd.SysProcAttr.Credential.Uid, cmd.SysProcAttr.Credential.Gid)
	}
}

func TestBuildSandboxCommand_Bwrap(t *testing.T) {
	ctx := context.Background()
	workDir := t.TempDir()

	// Create a dummy paper.db inside workDir to test masking
	dbPath := filepath.Join(workDir, "paper.db")
	_ = os.WriteFile(dbPath, []byte("sensitive data"), 0600)

	// Create a mock bwrap executable in a temp directory
	mockBinDir := t.TempDir()
	mockBwrap := filepath.Join(mockBinDir, "mock-bwrap")
	_ = os.WriteFile(mockBwrap, []byte("#!/bin/sh\nexit 0\n"), 0755)

	cfg := IsolationConfig{
		Mode:             IsolationBwrap,
		BwrapPath:        mockBwrap,
		MaskPaths:        []string{"paper.db"},
		ReadOnlyRoot:     true,
		IsolatedTmp:      true,
		UnsharePID:       true,
		UnshareUTS:       true,
		UnshareIPC:       true,
		DropCapabilities: true,
	}

	javaArgs := []string{"-Xmx4G", "-jar", "server.jar"}
	cmd, err := BuildSandboxCommand(ctx, cfg, workDir, javaArgs)
	if err != nil {
		t.Fatalf("BuildSandboxCommand failed: %v", err)
	}

	argsStr := strings.Join(cmd.Args, " ")
	if !strings.Contains(argsStr, "--ro-bind / /") {
		t.Errorf("Expected --ro-bind / / in bwrap args: %s", argsStr)
	}
	if !strings.Contains(argsStr, "--tmpfs /tmp") {
		t.Errorf("Expected --tmpfs /tmp in bwrap args: %s", argsStr)
	}
	if !strings.Contains(argsStr, "--unshare-pid") {
		t.Errorf("Expected --unshare-pid in bwrap args: %s", argsStr)
	}
	if !strings.Contains(argsStr, "--cap-drop ALL") {
		t.Errorf("Expected --cap-drop ALL in bwrap args: %s", argsStr)
	}
	if !strings.Contains(argsStr, "--tmpfs "+dbPath) {
		t.Errorf("Expected masked paper.db via --tmpfs in bwrap args: %s", argsStr)
	}
	if !strings.Contains(argsStr, "java -Xmx4G -jar server.jar") {
		t.Errorf("Expected target java invocation at the end: %s", argsStr)
	}
}

func TestBuildSandboxCommand_Errors(t *testing.T) {
	ctx := context.Background()
	workDir := t.TempDir()

	// Unsupported mode
	_, err := BuildSandboxCommand(ctx, IsolationConfig{Mode: "unsupported_mode"}, workDir, nil)
	if err == nil {
		t.Errorf("Expected error on unsupported mode")
	}

	// Missing bwrap binary
	_, err = BuildSandboxCommand(ctx, IsolationConfig{Mode: IsolationBwrap, BwrapPath: "/nonexistent/path/bwrap"}, workDir, nil)
	if err == nil {
		t.Errorf("Expected error for non-existent bwrap executable")
	}
}

func TestServerIsolationConfigGetSet(t *testing.T) {
	srv := NewServer(t.TempDir(), "server.jar", "2G", nil)
	initCfg := srv.GetIsolationConfig()
	if initCfg.Mode == "" {
		t.Errorf("Expected default mode to be set")
	}

	customCfg := IsolationConfig{Mode: IsolationUser, UID: 5000}
	srv.SetIsolationConfig(customCfg)
	if srv.GetIsolationConfig().UID != 5000 {
		t.Errorf("Expected UID 5000 after SetIsolationConfig")
	}
}
