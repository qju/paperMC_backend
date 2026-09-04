package plugins

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// ScanPlugins scans the given directory for all .jar and .jar.disabled plugin files.
func ScanPlugins(pluginsDir string) ([]PluginInfo, error) {
	if _, err := os.Stat(pluginsDir); os.IsNotExist(err) {
		return []PluginInfo{}, nil
	}

	entries, err := os.ReadDir(pluginsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read plugins directory: %w", err)
	}

	var list []PluginInfo
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if !isPluginFile(name) {
			continue
		}

		filePath := filepath.Join(pluginsDir, name)
		info, err := InspectPluginFile(filePath)
		if err != nil {
			// Fallback placeholder if unreadable
			fileInfo, _ := entry.Info()
			size := int64(0)
			if fileInfo != nil {
				size = fileInfo.Size()
			}
			fallback := PluginInfo{
				Filename:      name,
				Name:          cleanPluginName(name),
				Version:       "unknown",
				Description:   "Unreadable or corrupted plugin archive",
				IsEnabled:     !strings.HasSuffix(name, ".disabled"),
				SizeBytes:     size,
				FormattedSize: formatBytes(size),
			}
			list = append(list, fallback)
			continue
		}

		list = append(list, *info)
	}

	sort.Slice(list, func(i, j int) bool {
		return strings.ToLower(list[i].Name) < strings.ToLower(list[j].Name)
	})

	return list, nil
}

// InspectPluginFile inspects an individual jar file and extracts its plugin metadata.
func InspectPluginFile(filePath string) (*PluginInfo, error) {
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return nil, err
	}

	filename := filepath.Base(filePath)
	isEnabled := !strings.HasSuffix(filename, ".disabled")

	r, err := zip.OpenReader(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open jar archive: %w", err)
	}
	defer r.Close()

	// Prioritize paper-plugin.yml over plugin.yml
	var manifestFile *zip.File
	for _, f := range r.File {
		if f.Name == "paper-plugin.yml" {
			manifestFile = f
			break
		} else if f.Name == "plugin.yml" && manifestFile == nil {
			manifestFile = f
		}
	}

	var raw RawPluginYml
	if manifestFile != nil {
		rc, err := manifestFile.Open()
		if err == nil {
			data, readErr := io.ReadAll(rc)
			_ = rc.Close()
			if readErr == nil {
				_ = yaml.Unmarshal(data, &raw)
			}
		}
	}

	pluginName := strings.TrimSpace(raw.Name)
	if pluginName == "" {
		pluginName = cleanPluginName(filename)
	}

	versionStr := ""
	if raw.Version != nil {
		versionStr = fmt.Sprintf("%v", raw.Version)
	}
	if versionStr == "" {
		versionStr = "1.0.0"
	}

	authors := parseAuthors(raw.Author, raw.Authors)
	dependencies := parseStringSlice(raw.Depend)
	softDependencies := parseStringSlice(raw.SoftDepend)

	return &PluginInfo{
		Filename:         filename,
		Name:             pluginName,
		Version:          versionStr,
		Main:             raw.Main,
		Description:      raw.Description,
		Authors:          authors,
		Website:          raw.Website,
		APIVersion:       raw.APIVersion,
		Dependencies:     dependencies,
		SoftDependencies: softDependencies,
		IsEnabled:        isEnabled,
		SizeBytes:        fileInfo.Size(),
		FormattedSize:    formatBytes(fileInfo.Size()),
		ModTime:          fileInfo.ModTime(),
	}, nil
}

func isPluginFile(filename string) bool {
	lower := strings.ToLower(filename)
	return strings.HasSuffix(lower, ".jar") || strings.HasSuffix(lower, ".jar.disabled")
}

func cleanPluginName(filename string) string {
	base := strings.TrimSuffix(filename, ".disabled")
	base = strings.TrimSuffix(base, ".jar")
	return base
}

func parseAuthors(author string, authorsRaw interface{}) []string {
	var result []string
	if author != "" {
		result = append(result, author)
	}

	switch v := authorsRaw.(type) {
	case []interface{}:
		for _, item := range v {
			s := fmt.Sprintf("%v", item)
			if s != "" && !contains(result, s) {
				result = append(result, s)
			}
		}
	case []string:
		for _, s := range v {
			if s != "" && !contains(result, s) {
				result = append(result, s)
			}
		}
	case string:
		if v != "" && !contains(result, v) {
			result = append(result, v)
		}
	}

	return result
}

func parseStringSlice(raw interface{}) []string {
	var result []string
	switch v := raw.(type) {
	case []interface{}:
		for _, item := range v {
			s := fmt.Sprintf("%v", item)
			if s != "" {
				result = append(result, s)
			}
		}
	case []string:
		return v
	case string:
		if v != "" {
			result = append(result, v)
		}
	}
	return result
}

func contains(slice []string, val string) bool {
	for _, item := range slice {
		if strings.EqualFold(item, val) {
			return true
		}
	}
	return false
}

func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}
