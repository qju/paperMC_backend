package minecraft

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

// IsolationMode represents the sandboxing strategy applied to the Minecraft Java process.
type IsolationMode string

const (
	// IsolationNone runs Java directly as the parent process user (default/development).
	IsolationNone IsolationMode = "none"
	// IsolationBwrap uses Linux Bubblewrap containerized unprivileged sandbox namespaces.
	IsolationBwrap IsolationMode = "bwrap"
	// IsolationUser applies POSIX DAC user credential separation via SysProcAttr.
	IsolationUser IsolationMode = "user"
)

// IsolationConfig defines parameters for isolating the Minecraft server child process.
type IsolationConfig struct {
	Mode             IsolationMode `json:"mode"`
	BwrapPath        string        `json:"bwrap_path"`
	MinecraftUser    string        `json:"minecraft_user"`
	UID              int           `json:"uid"`
	GID              int           `json:"gid"`
	MaskPaths        []string      `json:"mask_paths"`
	ReadOnlyRoot     bool          `json:"read_only_root"`
	IsolatedTmp      bool          `json:"isolated_tmp"`
	UnsharePID       bool          `json:"unshare_pid"`
	UnshareUTS       bool          `json:"unshare_uts"`
	UnshareIPC       bool          `json:"unshare_ipc"`
	DropCapabilities bool          `json:"drop_capabilities"`
}

// DefaultIsolationConfig loads isolation settings from environment variables or returns safe defaults.
func DefaultIsolationConfig() IsolationConfig {
	modeStr := strings.ToLower(strings.TrimSpace(os.Getenv("ISOLATION_MODE")))
	var mode IsolationMode
	switch modeStr {
	case "bwrap":
		mode = IsolationBwrap
	case "user":
		mode = IsolationUser
	default:
		mode = IsolationNone
	}

	mcUser := strings.TrimSpace(os.Getenv("MINECRAFT_USER"))
	uid := 0
	gid := 0
	if uidStr := strings.TrimSpace(os.Getenv("MINECRAFT_UID")); uidStr != "" {
		if u, err := strconv.Atoi(uidStr); err == nil {
			uid = u
		}
	}
	if gidStr := strings.TrimSpace(os.Getenv("MINECRAFT_GID")); gidStr != "" {
		if g, err := strconv.Atoi(gidStr); err == nil {
			gid = g
		}
	}

	return IsolationConfig{
		Mode:             mode,
		BwrapPath:        "bwrap",
		MinecraftUser:    mcUser,
		UID:              uid,
		GID:              gid,
		MaskPaths:        []string{"paper.db", ".env", "paper.db-wal", "paper.db-shm"},
		ReadOnlyRoot:     true,
		IsolatedTmp:      true,
		UnsharePID:       true,
		UnshareUTS:       true,
		UnshareIPC:       true,
		DropCapabilities: true,
	}
}

// BuildSandboxCommand prepares an exec.Cmd wrapped according to the configured isolation engine.
func BuildSandboxCommand(ctx context.Context, cfg IsolationConfig, workDir string, javaArgs []string) (*exec.Cmd, error) {
	absWorkDir, err := filepath.Abs(workDir)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve absolute workdir: %w", err)
	}

	switch cfg.Mode {
	case IsolationBwrap:
		return buildBwrapCommand(ctx, cfg, absWorkDir, javaArgs)
	case IsolationUser:
		return buildUserCommand(ctx, cfg, absWorkDir, javaArgs)
	case IsolationNone, "":
		cmd := ExecCommandContext(ctx, "java", javaArgs...)
		cmd.Dir = absWorkDir
		return cmd, nil
	default:
		return nil, fmt.Errorf("unsupported isolation mode: %s", cfg.Mode)
	}
}

func buildBwrapCommand(ctx context.Context, cfg IsolationConfig, absWorkDir string, javaArgs []string) (*exec.Cmd, error) {
	bwrapPath := cfg.BwrapPath
	if bwrapPath == "" {
		bwrapPath = "bwrap"
	}
	if _, err := exec.LookPath(bwrapPath); err != nil {
		return nil, fmt.Errorf("bubblewrap executable (%s) not found in PATH: %w", bwrapPath, err)
	}

	args := []string{}

	// Read-only host filesystem hierarchy
	if cfg.ReadOnlyRoot {
		args = append(args, "--ro-bind", "/", "/")
	}

	// Essential virtual filesystems
	args = append(args, "--dev", "/dev")
	args = append(args, "--proc", "/proc")

	// Isolated temporary directory
	if cfg.IsolatedTmp {
		args = append(args, "--tmpfs", "/tmp")
	}

	// Read-write bind of Minecraft server working directory
	args = append(args, "--bind", absWorkDir, absWorkDir)

	// Mask sensitive database and credential files
	for _, mask := range cfg.MaskPaths {
		maskPath := mask
		if !filepath.IsAbs(maskPath) {
			maskPath = filepath.Join(absWorkDir, maskPath)
		}
		if _, err := os.Stat(maskPath); err == nil {
			args = append(args, "--tmpfs", maskPath)
		}
	}

	// Unshare namespaces to isolate process execution
	if cfg.UnsharePID {
		args = append(args, "--unshare-pid")
	}
	if cfg.UnshareUTS {
		args = append(args, "--unshare-uts")
	}
	if cfg.UnshareIPC {
		args = append(args, "--unshare-ipc")
	}

	args = append(args, "--die-with-parent")

	if cfg.DropCapabilities {
		args = append(args, "--cap-drop", "ALL")
	}

	args = append(args, "--chdir", absWorkDir)
	args = append(args, "java")
	args = append(args, javaArgs...)

	cmd := ExecCommandContext(ctx, bwrapPath, args...)
	cmd.Dir = absWorkDir
	return cmd, nil
}

func buildUserCommand(ctx context.Context, cfg IsolationConfig, absWorkDir string, javaArgs []string) (*exec.Cmd, error) {
	uid := cfg.UID
	gid := cfg.GID

	if cfg.MinecraftUser != "" && (uid == 0 || gid == 0) {
		u, err := user.Lookup(cfg.MinecraftUser)
		if err != nil {
			return nil, fmt.Errorf("failed to lookup isolation user %q: %w", cfg.MinecraftUser, err)
		}
		parsedUID, err := strconv.Atoi(u.Uid)
		if err != nil {
			return nil, fmt.Errorf("invalid uid %q for user %q", u.Uid, cfg.MinecraftUser)
		}
		parsedGID, err := strconv.Atoi(u.Gid)
		if err != nil {
			return nil, fmt.Errorf("invalid gid %q for user %q", u.Gid, cfg.MinecraftUser)
		}
		uid = parsedUID
		gid = parsedGID
	}

	cmd := ExecCommandContext(ctx, "java", javaArgs...)
	cmd.Dir = absWorkDir

	if uid > 0 || gid > 0 {
		cmd.SysProcAttr = &syscall.SysProcAttr{
			Credential: &syscall.Credential{
				Uid: uint32(uid),
				Gid: uint32(gid),
			},
		}
	}

	return cmd, nil
}
