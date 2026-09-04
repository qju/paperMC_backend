package plugins

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// TogglePlugin toggles a plugin between enabled (.jar) and disabled (.jar.disabled).
func TogglePlugin(pluginsDir, filename string) (string, error) {
	cleanName := filepath.Base(strings.TrimSpace(filename))
	if cleanName == "" || cleanName == "." || cleanName == "/" || strings.Contains(filename, "..") {
		return "", fmt.Errorf("invalid filename")
	}

	var newFilename string
	if strings.HasSuffix(cleanName, ".disabled") {
		newFilename = strings.TrimSuffix(cleanName, ".disabled")
	} else if strings.HasSuffix(strings.ToLower(cleanName), ".jar") {
		newFilename = cleanName + ".disabled"
	} else {
		return "", fmt.Errorf("file is not a valid .jar or .jar.disabled plugin")
	}

	oldPath := filepath.Join(pluginsDir, cleanName)
	newPath := filepath.Join(pluginsDir, newFilename)

	if _, err := os.Stat(oldPath); err != nil {
		return "", fmt.Errorf("plugin file not found: %w", err)
	}

	if err := os.Rename(oldPath, newPath); err != nil {
		return "", fmt.Errorf("failed to toggle plugin: %w", err)
	}

	return newFilename, nil
}

// DeletePlugin deletes a plugin jar file safely from the plugins directory.
func DeletePlugin(pluginsDir, filename string) error {
	cleanName := filepath.Base(strings.TrimSpace(filename))
	if cleanName == "" || cleanName == "." || cleanName == "/" || strings.Contains(filename, "..") {
		return fmt.Errorf("invalid filename")
	}

	targetPath := filepath.Join(pluginsDir, cleanName)
	if _, err := os.Stat(targetPath); err != nil {
		return fmt.Errorf("plugin file not found: %w", err)
	}

	if err := os.Remove(targetPath); err != nil {
		return fmt.Errorf("failed to delete plugin file: %w", err)
	}

	return nil
}

// SaveUploadedPlugin saves an uploaded reader into pluginsDir and validates it as a Minecraft plugin jar.
func SaveUploadedPlugin(pluginsDir, originalFilename string, r io.Reader) (*PluginInfo, error) {
	cleanName := filepath.Base(strings.TrimSpace(originalFilename))
	if cleanName == "" || cleanName == "." || strings.Contains(originalFilename, "..") {
		return nil, fmt.Errorf("invalid filename")
	}

	if !strings.HasSuffix(strings.ToLower(cleanName), ".jar") {
		return nil, fmt.Errorf("uploaded file must be a .jar archive")
	}

	if err := os.MkdirAll(pluginsDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create plugins directory: %w", err)
	}

	// Write to temporary file first for verification
	tmpFile, err := os.CreateTemp(pluginsDir, "upload-*.tmp")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer func() {
		_ = tmpFile.Close()
		_ = os.Remove(tmpPath) // safe cleanup if rename didn't happen
	}()

	if _, err := io.Copy(tmpFile, r); err != nil {
		return nil, fmt.Errorf("failed to save uploaded file: %w", err)
	}
	_ = tmpFile.Close()

	// Verify valid zip archive
	zr, err := zip.OpenReader(tmpPath)
	if err != nil {
		return nil, fmt.Errorf("invalid jar archive: %w", err)
	}

	hasManifest := false
	for _, f := range zr.File {
		if f.Name == "plugin.yml" || f.Name == "paper-plugin.yml" {
			hasManifest = true
			break
		}
	}
	_ = zr.Close()

	if !hasManifest {
		return nil, fmt.Errorf("archive does not contain a valid plugin.yml or paper-plugin.yml manifest")
	}

	destPath := filepath.Join(pluginsDir, cleanName)
	if err := os.Rename(tmpPath, destPath); err != nil {
		return nil, fmt.Errorf("failed to install plugin: %w", err)
	}

	return InspectPluginFile(destPath)
}
