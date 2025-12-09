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

// BuildsResponse represents the top-level JSON object
type BuildsResponse struct {
	ProjectId   string  `json:"project_id"`
	ProjectName string  `json:"project_name"`
	Version     string  `json:"version"`
	Builds      []Build `json:"builds"`
}

// Build represents a single entry in the "builds" list
type Build struct {
	Build     int       `json:"build"`
	Time      string    `json:"time"`
	Channel   string    `json:"channel"`
	Promoted  bool      `json:"promoted"`
	Changes   []Change  `json:"changes"`
	Downloads Downloads `json:"downloads"`
}

type Change struct {
	Commit  string `json:"commit"`
	Summary string `json:"summary"`
	Message string `json:"message"`
}

type Downloads struct {
	Application Application `json:"application"`
}

type Application struct {
	Name   string `json:"name"`
	Sha256 string `json:"sha256"`
}

func GetLatestBuild(version string) (int, string, string, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	url := fmt.Sprintf("https://api.papermc.io/v2/projects/paper/versions/%s/builds", version)

	resp, err := client.Get(url)
	if err != nil {
		return 0, "", "", fmt.Errorf("failed to fetch builds: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, "", "", fmt.Errorf("api error: %d", resp.StatusCode)
	}

	var result BuildsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, "", "", fmt.Errorf("invalid JSON: %w", err)
	}

	if len(result.Builds) == 0 {
		return 0, "", "", fmt.Errorf("no builds found for version: %s", version)
	}

	// The API returns build sorted by time, so the last one is the latest
	latest := result.Builds[len(result.Builds)-1]
	return latest.Build,
		latest.Downloads.Application.Name,
		latest.Downloads.Application.Sha256,
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
