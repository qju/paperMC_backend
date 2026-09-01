package updater

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	DefaultProject  = "paper"
	UserAgentHeader = "Lodestone-Manager/2.0 (+https://github.com/qju/paperMC_backend)"
)

var FillAPIBaseURL = "https://fill.papermc.io/v3"


// Raw API response from /v3/projects/{project}
type projectResponseV3 struct {
	Project struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"project"`
	Versions map[string][]string `json:"versions"`
}

// VersionGroup represents a major version group and its specific releases.
type VersionGroup struct {
	Group    string   `json:"group"`
	Versions []string `json:"versions"`
}

// ProjectVersionsResponse provides structured version groups for the frontend.
type ProjectVersionsResponse struct {
	Project       string         `json:"project"`
	VersionGroups []VersionGroup `json:"version_groups"`
}

// BuildResponseV3 represents the schema returned by /v3/projects/{project}/versions/{version}/builds/latest
type BuildResponseV3 struct {
	ID        int                     `json:"id"`
	Time      time.Time               `json:"time"`
	Channel   string                  `json:"channel"`
	Commits   []CommitV3              `json:"commits"`
	Downloads map[string]DownloadItem `json:"downloads"`
}

type CommitV3 struct {
	SHA     string    `json:"sha"`
	Time    time.Time `json:"time"`
	Message string    `json:"message"`
}

type DownloadItem struct {
	Name      string    `json:"name"`
	Checksums Checksums `json:"checksums"`
	Size      int64     `json:"size"`
	URL       string    `json:"url"`
}

type Checksums struct {
	SHA256 string `json:"sha256"`
}

// BuildInfo is the clean, decoupled struct passed to API handlers.
type BuildInfo struct {
	Project     string    `json:"project"`
	Version     string    `json:"version"`
	BuildID     int       `json:"build_id"`
	Time        time.Time `json:"time"`
	Channel     string    `json:"channel"`
	FileName    string    `json:"file_name"`
	SHA256      string    `json:"sha256"`
	DownloadURL string    `json:"download_url"`
	Size        int64     `json:"size"`
}

var httpClient = &http.Client{
	Timeout: 15 * time.Second,
}

// GetProjectVersions fetches all supported version families and releases.
func GetProjectVersions(project string) (*ProjectVersionsResponse, error) {
	if project == "" {
		project = DefaultProject
	}

	url := fmt.Sprintf("%s/projects/%s", FillAPIBaseURL, project)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to construct request: %w", err)
	}
	req.Header.Set("User-Agent", UserAgentHeader)

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("network request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("papermc api error (status %d)", resp.StatusCode)
	}

	var raw projectResponseV3
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("failed to parse json response: %w", err)
	}

	groups := make([]VersionGroup, 0, len(raw.Versions))
	for groupName, versionList := range raw.Versions {
		groups = append(groups, VersionGroup{
			Group:    groupName,
			Versions: versionList,
		})
	}

	// Sort groups in descending semantic order (e.g. 26.2, 26.1, 1.21, 1.20)
	sort.Slice(groups, func(i, j int) bool {
		return compareVersions(groups[i].Group, groups[j].Group) > 0
	})

	return &ProjectVersionsResponse{
		Project:       project,
		VersionGroups: groups,
	}, nil
}

// GetLatestBuild retrieves metadata for the newest build of a project version.
func GetLatestBuild(project, version string) (*BuildInfo, error) {
	if project == "" {
		project = DefaultProject
	}
	if strings.TrimSpace(version) == "" {
		return nil, errors.New("version cannot be empty")
	}

	url := fmt.Sprintf("%s/projects/%s/versions/%s/builds/latest", FillAPIBaseURL, project, version)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to construct request: %w", err)
	}
	req.Header.Set("User-Agent", UserAgentHeader)

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("network request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("version '%s' not found", version)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("papermc api error (status %d)", resp.StatusCode)
	}

	var buildData BuildResponseV3
	if err := json.NewDecoder(resp.Body).Decode(&buildData); err != nil {
		return nil, fmt.Errorf("failed to parse build response: %w", err)
	}

	download, ok := buildData.Downloads["server:default"]
	if !ok || download.URL == "" || download.Checksums.SHA256 == "" {
		return nil, errors.New("no server download artifact found in build response")
	}

	return &BuildInfo{
		Project:     project,
		Version:     version,
		BuildID:     buildData.ID,
		Time:        buildData.Time,
		Channel:     buildData.Channel,
		FileName:    download.Name,
		SHA256:      download.Checksums.SHA256,
		DownloadURL: download.URL,
		Size:        download.Size,
	}, nil
}

// DownloadJar streams the JAR from the direct CDN URL into a temporary file,
// verifying the SHA-256 digest on the fly.
func DownloadJar(downloadURL, targetDir, tempFileName, expectedSHA256 string) error {
	req, err := http.NewRequest(http.MethodGet, downloadURL, nil)
	if err != nil {
		return fmt.Errorf("failed to build download request: %w", err)
	}
	req.Header.Set("User-Agent", UserAgentHeader)

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("download request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed with HTTP status %d", resp.StatusCode)
	}

	tempPath := filepath.Join(targetDir, tempFileName)
	tempFile, err := os.Create(tempPath)
	if err != nil {
		return fmt.Errorf("failed to create staging file: %w", err)
	}
	defer tempFile.Close()

	hasher := sha256.New()
	writer := io.MultiWriter(tempFile, hasher)

	if _, err := io.Copy(writer, resp.Body); err != nil {
		_ = os.Remove(tempPath)
		return fmt.Errorf("failed while streaming download: %w", err)
	}

	actualSHA256 := hex.EncodeToString(hasher.Sum(nil))
	if expectedSHA256 != "" && !strings.EqualFold(actualSHA256, expectedSHA256) {
		_ = os.Remove(tempPath)
		return fmt.Errorf("checksum mismatch: expected %s, got %s", expectedSHA256, actualSHA256)
	}

	return nil
}

// compareVersions compares version strings like "26.2" vs "1.21.1".
func compareVersions(v1, v2 string) int {
	parts1 := strings.Split(strings.TrimPrefix(v1, "v"), ".")
	parts2 := strings.Split(strings.TrimPrefix(v2, "v"), ".")

	maxLen := len(parts1)
	if len(parts2) > maxLen {
		maxLen = len(parts2)
	}

	for i := 0; i < maxLen; i++ {
		num1 := 0
		if i < len(parts1) {
			num1, _ = strconv.Atoi(parts1[i])
		}
		num2 := 0
		if i < len(parts2) {
			num2, _ = strconv.Atoi(parts2[i])
		}

		if num1 > num2 {
			return 1
		}
		if num1 < num2 {
			return -1
		}
	}
	return strings.Compare(v1, v2)
}
