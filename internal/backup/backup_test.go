package backup

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"paperMC_backend/internal/minecraft"
)

type mockServerController struct {
	status        minecraft.Status
	commandsSent  []string
	broadcasts    []string
	stopCalled    bool
	startCalled   bool
	stopErr       error
	startErr      error
	stopTransitionsToStopped bool
}

func (m *mockServerController) GetStatus() minecraft.Status {
	return m.status
}

func (m *mockServerController) SendCommand(cmd string) error {
	m.commandsSent = append(m.commandsSent, cmd)
	return nil
}

func (m *mockServerController) Broadcast(msg string) {
	m.broadcasts = append(m.broadcasts, msg)
}

func (m *mockServerController) Stop() error {
	m.stopCalled = true
	if m.stopTransitionsToStopped {
		m.status = minecraft.StatusStopped
	}
	return m.stopErr
}

func (m *mockServerController) Start() error {
	m.startCalled = true
	m.status = minecraft.StatusRunning
	return m.startErr
}

func setupTestWorld(t *testing.T, workDir, worldName string) {
	worldDir := filepath.Join(workDir, worldName)
	if err := os.MkdirAll(worldDir, 0755); err != nil {
		t.Fatalf("Failed to create world dir: %v", err)
	}

	if err := os.WriteFile(filepath.Join(worldDir, "level.dat"), []byte("mock-level-data"), 0644); err != nil {
		t.Fatalf("Failed to create level.dat: %v", err)
	}

	regionDir := filepath.Join(worldDir, "region")
	if err := os.MkdirAll(regionDir, 0755); err != nil {
		t.Fatalf("Failed to create region dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(regionDir, "r.0.0.mca"), []byte("chunk-data"), 0644); err != nil {
		t.Fatalf("Failed to create chunk: %v", err)
	}

	// Sibling dimension
	netherDir := filepath.Join(workDir, worldName+"_nether")
	if err := os.MkdirAll(netherDir, 0755); err != nil {
		t.Fatalf("Failed to create nether dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(netherDir, "dim-1.dat"), []byte("nether-data"), 0644); err != nil {
		t.Fatalf("Failed to create nether file: %v", err)
	}
}

func TestCreateAndListWorldBackup(t *testing.T) {
	workDir := t.TempDir()
	setupTestWorld(t, workDir, "world")

	mock := &mockServerController{
		status: minecraft.StatusRunning,
	}

	req := CreateBackupRequest{
		Type:      "world",
		WorldName: "world",
	}

	info, err := CreateBackup(workDir, req, mock)
	if err != nil {
		t.Fatalf("CreateBackup failed: %v", err)
	}

	if info.BackupType != "world" {
		t.Errorf("Expected backup type 'world', got %s", info.BackupType)
	}
	if info.WorldName != "world" {
		t.Errorf("Expected world name 'world', got %s", info.WorldName)
	}
	if info.SizeBytes <= 0 {
		t.Errorf("Expected positive size, got %d", info.SizeBytes)
	}
	if info.ChecksumSHA256 == "" {
		t.Error("Expected non-empty checksum")
	}

	// Verify server snapshot synchronization commands
	if len(mock.commandsSent) < 3 {
		t.Fatalf("Expected at least 3 commands sent, got %d: %v", len(mock.commandsSent), mock.commandsSent)
	}
	if mock.commandsSent[0] != "save-off" || mock.commandsSent[1] != "save-all flush" || mock.commandsSent[2] != "save-on" {
		t.Errorf("Unexpected command sequence: %v", mock.commandsSent)
	}

	// List backups
	list, err := ListBackups(workDir)
	if err != nil {
		t.Fatalf("ListBackups failed: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("Expected 1 backup in list, got %d", len(list))
	}
	if list[0].Filename != info.Filename {
		t.Errorf("Expected filename %s, got %s", info.Filename, list[0].Filename)
	}
	if list[0].WorldName != "world" {
		t.Errorf("Expected world name 'world', got %s", list[0].WorldName)
	}
}

func TestCreateFullBackup(t *testing.T) {
	workDir := t.TempDir()
	setupTestWorld(t, workDir, "world")

	// Add server jar and config
	_ = os.WriteFile(filepath.Join(workDir, "server.jar"), []byte("mock-jar"), 0644)
	_ = os.WriteFile(filepath.Join(workDir, "server.properties"), []byte("difficulty=hard"), 0644)

	mock := &mockServerController{
		status: minecraft.StatusStopped,
	}

	req := CreateBackupRequest{
		Type: "full",
	}

	info, err := CreateBackup(workDir, req, mock)
	if err != nil {
		t.Fatalf("CreateBackup full failed: %v", err)
	}

	if info.BackupType != "full" {
		t.Errorf("Expected type 'full', got %s", info.BackupType)
	}

	// Validate path and checksum
	path, err := GetBackupPath(workDir, info.Filename)
	if err != nil {
		t.Fatalf("GetBackupPath failed: %v", err)
	}

	calcChecksum, err := CalculateFileSHA256(path)
	if err != nil {
		t.Fatalf("CalculateFileSHA256 failed: %v", err)
	}
	if calcChecksum != info.ChecksumSHA256 {
		t.Errorf("Checksum mismatch: got %s, want %s", calcChecksum, info.ChecksumSHA256)
	}
}

func TestCreateBackupErrors(t *testing.T) {
	workDir := t.TempDir()

	// 1. Invalid backup type
	_, err := CreateBackup(workDir, CreateBackupRequest{Type: "invalid"}, nil)
	if err == nil || !strings.Contains(err.Error(), "invalid backup type") {
		t.Errorf("Expected invalid backup type error, got %v", err)
	}

	// 2. Non-existent world
	_, err = CreateBackup(workDir, CreateBackupRequest{Type: "world", WorldName: "nonexistent"}, nil)
	if err == nil || !strings.Contains(err.Error(), "does not exist") {
		t.Errorf("Expected non-existent world error, got %v", err)
	}
}

func TestGetBackupPathValidation(t *testing.T) {
	workDir := t.TempDir()
	_, _ = EnsureBackupDir(workDir)

	// Path traversal attempts
	traversalFiles := []string{
		"../etc/passwd",
		"backup_test.zip/../../etc",
		"backup_foo;rm -rf.zip",
		"other.zip",
		"backup_.zip",
		"/tmp/backup_123.zip",
	}

	for _, bad := range traversalFiles {
		if err := ValidateBackupFilename(bad); err == nil {
			t.Errorf("Expected validation failure for dangerous filename: %s", bad)
		}
	}

	// Valid filenames
	validFiles := []string{
		"backup_world_world_20260904_120000.zip",
		"backup_full_20260904_120000.zip",
		"backup_custom-123.zip",
	}
	for _, good := range validFiles {
		if err := ValidateBackupFilename(good); err != nil {
			t.Errorf("Expected valid filename for %s, got error: %v", good, err)
		}
	}

	// Non-existent file lookup
	_, err := GetBackupPath(workDir, "backup_nonexistent.zip")
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Errorf("Expected not found error, got: %v", err)
	}
}

func TestDeleteBackup(t *testing.T) {
	workDir := t.TempDir()
	setupTestWorld(t, workDir, "world")

	info, err := CreateBackup(workDir, CreateBackupRequest{Type: "world", WorldName: "world"}, nil)
	if err != nil {
		t.Fatalf("CreateBackup failed: %v", err)
	}

	if err := DeleteBackup(workDir, info.Filename); err != nil {
		t.Fatalf("DeleteBackup failed: %v", err)
	}

	// Subsequent delete should fail
	if err := DeleteBackup(workDir, info.Filename); err == nil {
		t.Error("Expected error deleting already deleted backup")
	}
}

func TestSafeExtractZipAndZipSlipDefense(t *testing.T) {
	workDir := t.TempDir()
	destDir := filepath.Join(workDir, "extracted")

	// 1. Create a zip with normal file
	goodZipPath := filepath.Join(workDir, "good.zip")
	zf, err := os.Create(goodZipPath)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(zf)
	w, _ := zw.Create("hello.txt")
	_, _ = w.Write([]byte("hello world"))
	_ = zw.Close()
	_ = zf.Close()

	if err := SafeExtractZip(goodZipPath, destDir); err != nil {
		t.Fatalf("SafeExtractZip failed on valid zip: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(destDir, "hello.txt"))
	if err != nil || string(content) != "hello world" {
		t.Errorf("Expected extracted content 'hello world', got: %s", string(content))
	}

	// 2. Create malicious ZipSlip archive
	badZipPath := filepath.Join(workDir, "slip.zip")
	bzf, err := os.Create(badZipPath)
	if err != nil {
		t.Fatal(err)
	}
	bzw := zip.NewWriter(bzf)
	// Malicious relative path targeting parent directory
	bw, _ := bzw.Create("../evil.txt")
	_, _ = bw.Write([]byte("malicious"))
	_ = bzw.Close()
	_ = bzf.Close()

	err = SafeExtractZip(badZipPath, destDir)
	if err == nil || !strings.Contains(err.Error(), "illegal file path") {
		t.Errorf("Expected ZipSlip detection error, got: %v", err)
	}
}

func TestRestoreBackup(t *testing.T) {
	workDir := t.TempDir()
	setupTestWorld(t, workDir, "world")

	// Create backup
	info, err := CreateBackup(workDir, CreateBackupRequest{Type: "world", WorldName: "world"}, nil)
	if err != nil {
		t.Fatalf("CreateBackup failed: %v", err)
	}

	// Modify world file
	levelDat := filepath.Join(workDir, "world", "level.dat")
	_ = os.WriteFile(levelDat, []byte("tampered-data"), 0644)

	mock := &mockServerController{
		status:                   minecraft.StatusRunning,
		stopTransitionsToStopped: true,
	}

	// Restore backup
	if err := RestoreBackup(workDir, info.Filename, mock); err != nil {
		t.Fatalf("RestoreBackup failed: %v", err)
	}

	if !mock.stopCalled {
		t.Error("Expected Stop() to be called on running server")
	}
	if !mock.startCalled {
		t.Error("Expected Start() to be called after restore")
	}

	restoredContent, err := os.ReadFile(levelDat)
	if err != nil || string(restoredContent) != "mock-level-data" {
		t.Errorf("Expected restored level.dat content 'mock-level-data', got: %s", string(restoredContent))
	}
}

func TestListBackupsEmptyDir(t *testing.T) {
	workDir := t.TempDir()
	list, err := ListBackups(workDir)
	if err != nil {
		t.Fatalf("Expected no error on missing backup dir, got: %v", err)
	}
	if len(list) != 0 {
		t.Errorf("Expected 0 backups, got %d", len(list))
	}
}

func TestRestoreBackupStoppedServer(t *testing.T) {
	workDir := t.TempDir()
	setupTestWorld(t, workDir, "world")

	info, err := CreateBackup(workDir, CreateBackupRequest{Type: "world", WorldName: "world"}, nil)
	if err != nil {
		t.Fatalf("CreateBackup failed: %v", err)
	}

	mock := &mockServerController{
		status: minecraft.StatusStopped,
	}

	if err := RestoreBackup(workDir, info.Filename, mock); err != nil {
		t.Fatalf("RestoreBackup failed on stopped server: %v", err)
	}

	if mock.stopCalled {
		t.Error("Stop() should not be called if server was already stopped")
	}
	if mock.startCalled {
		t.Error("Start() should not be called if server was stopped")
	}
}

func TestRestoreBackupNonExistent(t *testing.T) {
	workDir := t.TempDir()
	err := RestoreBackup(workDir, "backup_world_missing_20260904-120000.zip", nil)
	if err == nil {
		t.Fatal("Expected error restoring non-existent backup")
	}
}

func TestSafeExtractZipInvalidFile(t *testing.T) {
	workDir := t.TempDir()
	corruptZip := filepath.Join(workDir, "corrupted.zip")
	_ = os.WriteFile(corruptZip, []byte("not-a-zip"), 0644)

	err := SafeExtractZip(corruptZip, filepath.Join(workDir, "out"))
	if err == nil {
		t.Fatal("Expected error extracting corrupted zip")
	}
}

