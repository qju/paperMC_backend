package minecraft

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var validWorldNameRegex = regexp.MustCompile(`^[a-zA-Z0-9_\-]+$`)

// WorldInfo provides comprehensive diagnostics and metadata for a single world.
type WorldInfo struct {
	Name          string    `json:"name"`
	DiskPath      string    `json:"disk_path"`
	SizeBytes     int64     `json:"size_bytes"`
	FormattedSize string    `json:"formatted_size"`
	IsActive      bool      `json:"is_active"`
	Format        string    `json:"format"` // "Modern (26.1+)" or "Legacy"
	Dimensions    []string  `json:"dimensions"`
	LastPlayed    time.Time `json:"last_played"`
	MinecraftVer  string    `json:"minecraft_version,omitempty"`
	GameMode      string    `json:"game_mode,omitempty"`
	Difficulty    string    `json:"difficulty,omitempty"`
	Hardcore      bool      `json:"hardcore"`
}

// FormatBytes formats byte count into human-readable string.
func FormatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// CalculateDirSize recursively calculates the total size in bytes of a directory.
func CalculateDirSize(path string) int64 {
	var totalSize int64
	_ = filepath.WalkDir(path, func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			info, err := d.Info()
			if err == nil {
				totalSize += info.Size()
			}
		}
		return nil
	})
	return totalSize
}

// InspectWorld inspects a world directory, reads its level.dat and dimension folders, and calculates size.
func InspectWorld(workDir, worldName string, isActive bool) (*WorldInfo, error) {
	worldDir := filepath.Join(workDir, worldName)
	levelDatPath := filepath.Join(worldDir, "level.dat")

	stat, err := os.Stat(levelDatPath)
	if err != nil {
		return nil, fmt.Errorf("not a valid world (missing level.dat): %w", err)
	}

	info := &WorldInfo{
		Name:          worldName,
		DiskPath:      worldDir,
		IsActive:      isActive,
		Format:        "Legacy",
		Dimensions:    []string{"overworld"},
		LastPlayed:    stat.ModTime(),
		GameMode:      "Survival",
		Difficulty:    "Normal",
	}

	// 1. Parse level.dat if available
	if meta, err := ReadLevelDat(levelDatPath); err == nil {
		if meta.MinecraftVer != "" {
			info.MinecraftVer = meta.MinecraftVer
		}
		if meta.GameMode != "" {
			info.GameMode = meta.GameMode
		}
		if meta.Difficulty != "" {
			info.Difficulty = meta.Difficulty
		}
		info.Hardcore = meta.Hardcore
		if !meta.LastPlayed.IsZero() {
			info.LastPlayed = meta.LastPlayed
		}
	}

	// 2. Calculate size & detect dimensions
	totalBytes := CalculateDirSize(worldDir)

	// Check for Modern (26.1+) unified dimension storage: world/dimensions/
	modernDimPath := filepath.Join(worldDir, "dimensions")
	if dimEntries, err := os.ReadDir(modernDimPath); err == nil && len(dimEntries) > 0 {
		info.Format = "Modern (26.1+)"
		// Walk inside dimensions to list them
		var dims []string
		_ = filepath.WalkDir(modernDimPath, func(p string, d fs.DirEntry, err error) error {
			if err == nil && d.IsDir() {
				base := d.Name()
				if base == "overworld" || base == "the_nether" || base == "the_end" || base == "nether" || base == "end" {
					dims = append(dims, base)
				}
			}
			return nil
		})
		if len(dims) > 0 {
			info.Dimensions = dims
		}
	} else {
		// Check for Legacy sibling folders: {world}_nether and {world}_the_end
		netherDir := filepath.Join(workDir, worldName+"_nether")
		if _, err := os.Stat(netherDir); err == nil {
			totalBytes += CalculateDirSize(netherDir)
			info.Dimensions = append(info.Dimensions, "the_nether")
		}

		endDir := filepath.Join(workDir, worldName+"_the_end")
		if _, err := os.Stat(endDir); err == nil {
			totalBytes += CalculateDirSize(endDir)
			info.Dimensions = append(info.Dimensions, "the_end")
		}
	}

	info.SizeBytes = totalBytes
	info.FormattedSize = FormatBytes(totalBytes)

	return info, nil
}

// ListWorlds scans the server directory for all valid worlds.
func ListWorlds(workDir, activeWorld string) ([]WorldInfo, error) {
	entries, err := os.ReadDir(workDir)
	if err != nil {
		return nil, err
	}

	var worlds []WorldInfo
	var activeInfo *WorldInfo

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()

		// Skip legacy dimension directories from primary listing
		if strings.HasSuffix(name, "_nether") || strings.HasSuffix(name, "_the_end") {
			continue
		}

		levelDat := filepath.Join(workDir, name, "level.dat")
		if _, err := os.Stat(levelDat); err == nil {
			isActive := name == activeWorld
			wInfo, err := InspectWorld(workDir, name, isActive)
			if err == nil {
				if isActive {
					activeInfo = wInfo
				} else {
					worlds = append(worlds, *wInfo)
				}
			}
		}
	}

	// Place active world first if present
	if activeInfo != nil {
		worlds = append([]WorldInfo{*activeInfo}, worlds...)
	}

	return worlds, nil
}

// DuplicateWorld copies a world and any of its associated dimension directories.
func DuplicateWorld(workDir, sourceWorld, targetWorld string) error {
	sourceWorld = strings.TrimSpace(sourceWorld)
	targetWorld = strings.TrimSpace(targetWorld)

	if !validWorldNameRegex.MatchString(targetWorld) {
		return errors.New("invalid target world name: only alphanumeric characters, underscores, and hyphens are allowed")
	}

	sourcePath := filepath.Join(workDir, sourceWorld)
	targetPath := filepath.Join(workDir, targetWorld)

	if _, err := os.Stat(sourcePath); os.IsNotExist(err) {
		return fmt.Errorf("source world '%s' does not exist", sourceWorld)
	}

	if _, err := os.Stat(targetPath); err == nil {
		return fmt.Errorf("target world '%s' already exists", targetWorld)
	}

	// 1. Copy primary world folder
	if err := copyDirectory(sourcePath, targetPath); err != nil {
		_ = os.RemoveAll(targetPath)
		return fmt.Errorf("failed to copy primary world directory: %w", err)
	}

	// 2. Copy legacy nether if present
	sourceNether := filepath.Join(workDir, sourceWorld+"_nether")
	targetNether := filepath.Join(workDir, targetWorld+"_nether")
	if _, err := os.Stat(sourceNether); err == nil {
		_ = copyDirectory(sourceNether, targetNether)
	}

	// 3. Copy legacy end if present
	sourceEnd := filepath.Join(workDir, sourceWorld+"_the_end")
	targetEnd := filepath.Join(workDir, targetWorld+"_the_end")
	if _, err := os.Stat(sourceEnd); err == nil {
		_ = copyDirectory(sourceEnd, targetEnd)
	}

	return nil
}

// DeleteWorld deletes a world directory and any legacy dimension folders.
func DeleteWorld(workDir, worldName, activeWorld string) error {
	worldName = strings.TrimSpace(worldName)
	if worldName == "" {
		return errors.New("world name cannot be empty")
	}
	if !validWorldNameRegex.MatchString(worldName) {
		return errors.New("invalid world name")
	}
	if worldName == activeWorld {
		return errors.New("cannot delete the currently active world")
	}

	worldPath := filepath.Join(workDir, worldName)
	if err := os.RemoveAll(worldPath); err != nil {
		return fmt.Errorf("failed to delete world directory: %w", err)
	}

	// Clean legacy dimensions if present
	_ = os.RemoveAll(filepath.Join(workDir, worldName+"_nether"))
	_ = os.RemoveAll(filepath.Join(workDir, worldName+"_the_end"))

	return nil
}

func copyDirectory(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		targetPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			return os.MkdirAll(targetPath, info.Mode())
		}

		return copyFile(path, targetPath)
	})
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err = io.Copy(out, in); err != nil {
		return err
	}

	srcInfo, err := os.Stat(src)
	if err == nil {
		return os.Chmod(dst, srcInfo.Mode())
	}
	return nil
}
