package updater

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// Endpoint: https://api.papermc.io/v2/projects/paper/versions/<version>/builds
//
// -- Ver3 Endpoint ---
// https://fill.papermc.io/v3/projects/${PROJECT}/versions/${VERSION}/builds
//
// BuildsResponse represents the top-level JSON object

// -- New Endpoint API schema V3 ---
type BuildV3 struct {
	ID        int                     `json:"id"`
	Time      time.Time               `json:"time"`
	Channel   string                  `json:"channel"`
	Commits   []Commit                `json:"commits"`
	Downloads map[string]DownloadItem `json:"downloads"`
}

type Commit struct {
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

// Build represents a single entry in the "builds" list

func GetLatestBuild(version string) (int, string, string, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	//url := fmt.Sprintf("https://fill.papermc.io/v3/projects/paper/versions/%s/builds/latest", version)
	url := fmt.Sprintf("https://fill.papermc.io/v3/projects/paper/versions/%s/builds/latest", version)

	resp, err := client.Get(url)
	if err != nil {
		return 0, "", "", fmt.Errorf("failed to fetch build: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, "", "", fmt.Errorf("api error: %d", resp.StatusCode)
	}

	var result BuildV3
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, "", "", fmt.Errorf("invalid JSON: %w", err)
	}

	// The API returns build sorted by time, so the last one is the latest
	return result.ID,
		result.Downloads["server:default"].URL,
		result.Downloads["server:default"].Name,
		nil
}

func DownloadJar(version string, build int, fileName string, targetPath string) error {
	client := &http.Client{}
	url := fmt.Sprintf(
		"https://api.papermc.io/v2/projects/paper/versions/%s/builds/%d/downloads/%s",
		version, build, fileName,
	)

	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("failed to download jar: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("error when downloading jar: %d", resp.StatusCode)
	}

	fullPath := filepath.Join(targetPath, fileName)
	file, err := os.Create(fullPath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	_, err = io.Copy(file, resp.Body)
	if err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}
