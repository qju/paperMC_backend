package backup

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"paperMC_backend/internal/minecraft"
)

var validBackupFilenameRegex = regexp.MustCompile(`^backup_[a-zA-Z0-9_\-]+\.zip$`)

// ServerController defines the subset of server management actions required for backup snapshotting.
type ServerController interface {
	GetStatus() minecraft.Status
	SendCommand(cmd string) error
	Broadcast(msg string)
	Stop() error
	Start() error
}

// BackupInfo contains comprehensive metadata about a backup archive.
type BackupInfo struct {
	Filename       string    `json:"filename"`
	SizeBytes      int64     `json:"size_bytes"`
	FormattedSize  string    `json:"formatted_size"`
	CreatedAt      time.Time `json:"created_at"`
	BackupType     string    `json:"backup_type"` // "world" or "full"
	WorldName      string    `json:"world_name,omitempty"`
	ChecksumSHA256 string    `json:"checksum_sha256,omitempty"`
}

// CreateBackupRequest contains parameters for creating a new backup.
type CreateBackupRequest struct {
	Type      string `json:"type"`       // "world" (default) or "full"
	WorldName string `json:"world_name"` // optional, defaults to active world
}

// EnsureBackupDir creates the backups directory if it doesn't already exist.
func EnsureBackupDir(workDir string) (string, error) {
	backupDir := filepath.Join(workDir, "backups")
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create backups directory: %w", err)
	}
	return backupDir, nil
}

// ValidateBackupFilename ensures a filename matches safe backup naming patterns.
func ValidateBackupFilename(filename string) error {
	cleaned := filepath.Base(filename)
	if cleaned != filename || !validBackupFilenameRegex.MatchString(filename) {
		return errors.New("invalid backup filename: must match backup_<name>.zip without path traversal")
	}
	return nil
}

// CalculateFileSHA256 computes the hexadecimal SHA-256 checksum of a file.
func CalculateFileSHA256(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

// ListBackups scans the backups directory and returns sorted backup metadata (newest first).
func ListBackups(workDir string) ([]BackupInfo, error) {
	backupDir := filepath.Join(workDir, "backups")
	if _, err := os.Stat(backupDir); os.IsNotExist(err) {
		return []BackupInfo{}, nil
	}

	entries, err := os.ReadDir(backupDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read backups directory: %w", err)
	}

	var backups []BackupInfo
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".zip") {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		name := entry.Name()
		bType := "unknown"
		worldName := ""

		if strings.HasPrefix(name, "backup_world_") {
			bType = "world"
			// Format: backup_world_<worldname>_<timestamp>.zip
			base := strings.TrimSuffix(strings.TrimPrefix(name, "backup_world_"), ".zip")
			lastIdx := strings.LastIndex(base, "_")
			if lastIdx != -1 {
				worldName = base[:lastIdx]
			} else {
				worldName = base
			}
		} else if strings.HasPrefix(name, "backup_full_") {
			bType = "full"
		}

		backups = append(backups, BackupInfo{
			Filename:      name,
			SizeBytes:     info.Size(),
			FormattedSize: minecraft.FormatBytes(info.Size()),
			CreatedAt:     info.ModTime(),
			BackupType:    bType,
			WorldName:     worldName,
		})
	}

	// Sort newest first
	sort.Slice(backups, func(i, j int) bool {
		return backups[i].CreatedAt.After(backups[j].CreatedAt)
	})

	return backups, nil
}

// GetBackupPath returns the validated absolute path to a backup archive.
func GetBackupPath(workDir, filename string) (string, error) {
	if err := ValidateBackupFilename(filename); err != nil {
		return "", err
	}

	backupDir := filepath.Join(workDir, "backups")
	targetPath := filepath.Join(backupDir, filename)

	if _, err := os.Stat(targetPath); os.IsNotExist(err) {
		return "", fmt.Errorf("backup archive '%s' not found", filename)
	}

	return targetPath, nil
}

// DeleteBackup removes a backup archive from disk.
func DeleteBackup(workDir, filename string) error {
	targetPath, err := GetBackupPath(workDir, filename)
	if err != nil {
		return err
	}

	return os.Remove(targetPath)
}

// CreateBackup performs coordinated snapshotting and compresses the target data into a zip file.
func CreateBackup(workDir string, req CreateBackupRequest, sc ServerController) (*BackupInfo, error) {
	backupDir, err := EnsureBackupDir(workDir)
	if err != nil {
		return nil, err
	}

	bType := strings.ToLower(strings.TrimSpace(req.Type))
	if bType == "" {
		bType = "world"
	}
	if bType != "world" && bType != "full" {
		return nil, errors.New("invalid backup type: must be 'world' or 'full'")
	}

	worldName := strings.TrimSpace(req.WorldName)
	if bType == "world" && worldName == "" {
		worldName = "world"
	}

	// 1. Snapshot coordination if server is running
	isRunning := sc != nil && sc.GetStatus() == minecraft.StatusRunning
	if isRunning {
		sc.Broadcast("[System] Initiating backup snapshot: freezing world saves...")
		_ = sc.SendCommand("save-off")
		_ = sc.SendCommand("save-all flush")
		// Always re-enable saves even if archiving encounters an error
		defer func() {
			_ = sc.SendCommand("save-on")
			sc.Broadcast("[System] World saves resumed.")
		}()
		time.Sleep(1 * time.Second)
	}

	timestamp := time.Now().Format("20060102-150405")
	var zipFilename string
	if bType == "world" {
		zipFilename = fmt.Sprintf("backup_world_%s_%s.zip", worldName, timestamp)
	} else {
		zipFilename = fmt.Sprintf("backup_full_%s.zip", timestamp)
	}

	finalZipPath := filepath.Join(backupDir, zipFilename)
	tempZipPath := finalZipPath + ".tmp"

	// Create temporary zip file
	zipFile, err := os.Create(tempZipPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create backup file: %w", err)
	}

	hasher := sha256.New()
	mw := io.MultiWriter(zipFile, hasher)
	zw := zip.NewWriter(mw)

	var zipErr error
	if bType == "world" {
		zipErr = archiveWorld(zw, workDir, worldName)
	} else {
		zipErr = archiveFull(zw, workDir)
	}

	if closeErr := zw.Close(); zipErr == nil {
		zipErr = closeErr
	}
	_ = zipFile.Close()

	if zipErr != nil {
		_ = os.Remove(tempZipPath)
		return nil, fmt.Errorf("failed to write backup archive: %w", zipErr)
	}

	// Rename .tmp to final .zip
	if err := os.Rename(tempZipPath, finalZipPath); err != nil {
		_ = os.Remove(tempZipPath)
		return nil, fmt.Errorf("failed to finalize backup file: %w", err)
	}

	fileInfo, err := os.Stat(finalZipPath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat completed backup: %w", err)
	}

	checksum := hex.EncodeToString(hasher.Sum(nil))

	if sc != nil {
		sc.Broadcast(fmt.Sprintf("[System] Backup created: %s (%s)", zipFilename, minecraft.FormatBytes(fileInfo.Size())))
	}

	return &BackupInfo{
		Filename:       zipFilename,
		SizeBytes:      fileInfo.Size(),
		FormattedSize:  minecraft.FormatBytes(fileInfo.Size()),
		CreatedAt:      fileInfo.ModTime(),
		BackupType:     bType,
		WorldName:      worldName,
		ChecksumSHA256: checksum,
	}, nil
}

// archiveWorld packs the world folder and any legacy dimension folders into the zip writer.
func archiveWorld(zw *zip.Writer, workDir, worldName string) error {
	worldDir := filepath.Join(workDir, worldName)
	if _, err := os.Stat(worldDir); os.IsNotExist(err) {
		return fmt.Errorf("target world directory '%s' does not exist", worldName)
	}

	// Primary world
	if err := addDirectoryToZip(zw, worldDir, worldName); err != nil {
		return err
	}

	// Legacy dimensions if present
	netherDir := filepath.Join(workDir, worldName+"_nether")
	if _, err := os.Stat(netherDir); err == nil {
		if err := addDirectoryToZip(zw, netherDir, worldName+"_nether"); err != nil {
			return err
		}
	}

	endDir := filepath.Join(workDir, worldName+"_the_end")
	if _, err := os.Stat(endDir); err == nil {
		if err := addDirectoryToZip(zw, endDir, worldName+"_the_end"); err != nil {
			return err
		}
	}

	return nil
}

// archiveFull packs the server working directory, excluding backups and caches.
func archiveFull(zw *zip.Writer, workDir string) error {
	backupDirName := "backups"

	return filepath.WalkDir(workDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(workDir, path)
		if err != nil {
			return err
		}

		if rel == "." {
			return nil
		}

		// Exclude backups directory itself to prevent recursive backup accumulation
		if rel == backupDirName || strings.HasPrefix(rel, backupDirName+string(os.PathSeparator)) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Exclude caches and git folders
		if rel == ".cache" || strings.HasPrefix(rel, ".cache"+string(os.PathSeparator)) ||
			rel == ".git" || strings.HasPrefix(rel, ".git"+string(os.PathSeparator)) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		return addEntryToZip(zw, path, rel, d)
	})
}

// addDirectoryToZip walks a directory and adds all files under the given zip root prefix.
func addDirectoryToZip(zw *zip.Writer, srcDir, zipPrefix string) error {
	return filepath.WalkDir(srcDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}

		var zipEntryPath string
		if rel == "." {
			zipEntryPath = zipPrefix
		} else {
			zipEntryPath = filepath.Join(zipPrefix, rel)
		}

		return addEntryToZip(zw, path, zipEntryPath, d)
	})
}

// addEntryToZip writes a single file or directory header to the zip writer.
func addEntryToZip(zw *zip.Writer, diskPath, zipEntryPath string, d fs.DirEntry) error {
	info, err := d.Info()
	if err != nil {
		return err
	}

	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return err
	}

	// Normalize to forward slashes for zip standards
	header.Name = filepath.ToSlash(zipEntryPath)
	if d.IsDir() {
		header.Name += "/"
	} else {
		header.Method = zip.Deflate
	}

	writer, err := zw.CreateHeader(header)
	if err != nil {
		return err
	}

	if d.IsDir() {
		return nil
	}

	file, err := os.Open(diskPath)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = io.Copy(writer, file)
	return err
}

// SafeExtractZip extracts all files from a zip archive into destDir, guarding against ZipSlip vulnerabilities.
func SafeExtractZip(zipPath, destDir string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("failed to open zip archive: %w", err)
	}
	defer r.Close()

	cleanDest := filepath.Clean(destDir)

	for _, file := range r.File {
		// Guard against ZipSlip path traversal
		filePath := filepath.Join(cleanDest, file.Name)
		rel, err := filepath.Rel(cleanDest, filePath)
		if err != nil || strings.HasPrefix(rel, "..") || strings.HasPrefix(filePath, "..") {
			return fmt.Errorf("illegal file path in zip archive: %s", file.Name)
		}

		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(filePath, 0755); err != nil {
				return fmt.Errorf("failed to create directory %s: %w", filePath, err)
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
			return fmt.Errorf("failed to create parent directory for %s: %w", filePath, err)
		}

		outFile, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.Mode())
		if err != nil {
			return fmt.Errorf("failed to open destination file %s: %w", filePath, err)
		}

		rc, err := file.Open()
		if err != nil {
			_ = outFile.Close()
			return fmt.Errorf("failed to read zip entry %s: %w", file.Name, err)
		}

		_, err = io.Copy(outFile, rc)
		_ = rc.Close()
		_ = outFile.Close()

		if err != nil {
			return fmt.Errorf("failed to extract file %s: %w", file.Name, err)
		}
	}

	return nil
}

// RestoreBackup safely restores a backup archive.
// If the server is running, it stops the server, extracts the archive, and restarts it.
func RestoreBackup(workDir, filename string, sc ServerController) error {
	backupPath, err := GetBackupPath(workDir, filename)
	if err != nil {
		return err
	}

	wasRunning := false
	if sc != nil && (sc.GetStatus() == minecraft.StatusRunning || sc.GetStatus() == minecraft.StatusStarting) {
		wasRunning = true
		sc.Broadcast("[System] Stopping server for backup restoration...")
		if err := sc.Stop(); err != nil {
			return fmt.Errorf("failed to stop server: %w", err)
		}

		// Await shutdown up to 30s
		deadline := time.Now().Add(30 * time.Second)
		for {
			if sc.GetStatus() == minecraft.StatusStopped {
				break
			}
			if time.Now().After(deadline) {
				return errors.New("timed out waiting for server to stop prior to restore")
			}
			time.Sleep(200 * time.Millisecond)
		}
	}

	// Extract backup into workDir
	if err := SafeExtractZip(backupPath, workDir); err != nil {
		return fmt.Errorf("failed to extract backup archive: %w", err)
	}

	if wasRunning && sc != nil {
		sc.Broadcast("[System] Backup restored. Restarting server...")
		if err := sc.Start(); err != nil {
			return fmt.Errorf("backup restored successfully, but failed to restart server: %w", err)
		}
	}

	return nil
}
