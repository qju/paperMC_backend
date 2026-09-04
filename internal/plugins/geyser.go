package plugins

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	DefaultGeyserAPIBase      = "https://download.geysermc.org/v2"
	DefaultGameProtocolRawURL = "https://raw.githubusercontent.com/GeyserMC/Geyser/master/core/src/main/java/org/geysermc/geyser/network/GameProtocol.java"
)

// GeyserClient interacts with the GeyserMC API and GitHub protocol definitions.
type GeyserClient struct {
	APIBaseURL      string
	GameProtocolURL string
	HTTPClient      *http.Client
}

// NewGeyserClient creates a default GeyserClient.
func NewGeyserClient() *GeyserClient {
	return &GeyserClient{
		APIBaseURL:      DefaultGeyserAPIBase,
		GameProtocolURL: DefaultGameProtocolRawURL,
		HTTPClient:      &http.Client{Timeout: 15 * time.Second},
	}
}

type geyserProjectResp struct {
	ProjectID   string   `json:"project_id"`
	ProjectName string   `json:"project_name"`
	Versions    []string `json:"versions"`
}

type geyserBuildsResp struct {
	ProjectID string        `json:"project_id"`
	Version   string        `json:"version"`
	Builds    []geyserBuild `json:"builds"`
}

type geyserBuild struct {
	Build     int                       `json:"build"`
	Time      time.Time                 `json:"time"`
	Changes   []geyserChange            `json:"changes"`
	Downloads map[string]geyserDownload `json:"downloads"`
}

type geyserChange struct {
	Commit  string `json:"commit"`
	Summary string `json:"summary"`
}

type geyserDownload struct {
	Name   string `json:"name"`
	Sha256 string `json:"sha256"`
}

// GetBedrockBridgeStatus checks local plugins and upstream GeyserMC APIs to report Bedrock status.
func (c *GeyserClient) GetBedrockBridgeStatus(pluginsDir string) (*BedrockBridgeStatus, error) {
	plugins, _ := ScanPlugins(pluginsDir)

	var geyserLocal *PluginInfo
	var floodgateLocal *PluginInfo

	for i := range plugins {
		p := &plugins[i]
		lowerName := strings.ToLower(p.Name)
		lowerFile := strings.ToLower(p.Filename)

		if strings.Contains(lowerName, "geyser") || strings.Contains(lowerFile, "geyser") {
			geyserLocal = p
		}
		if strings.Contains(lowerName, "floodgate") || strings.Contains(lowerFile, "floodgate") {
			floodgateLocal = p
		}
	}

	geyserStatus := GeyserStatus{
		Installed: geyserLocal != nil,
	}
	if geyserLocal != nil {
		geyserStatus.InstalledFile = geyserLocal.Filename
		geyserStatus.InstalledVersion = geyserLocal.Version
		geyserStatus.IsEnabled = geyserLocal.IsEnabled
		geyserStatus.InstalledBuild = extractBuildNumber(geyserLocal.Version, geyserLocal.Filename)
		hash, _ := computeFileSha256(filepath.Join(pluginsDir, geyserLocal.Filename))
		geyserStatus.InstalledSha256 = hash
	}

	floodgateStatus := FloodgateStatus{
		Installed: floodgateLocal != nil,
	}
	if floodgateLocal != nil {
		floodgateStatus.InstalledFile = floodgateLocal.Filename
		floodgateStatus.InstalledVersion = floodgateLocal.Version
		floodgateStatus.IsEnabled = floodgateLocal.IsEnabled
		floodgateStatus.InstalledBuild = extractBuildNumber(floodgateLocal.Version, floodgateLocal.Filename)
		hash, _ := computeFileSha256(filepath.Join(pluginsDir, floodgateLocal.Filename))
		floodgateStatus.InstalledSha256 = hash
	}

	// Query upstream Geyser
	latestGeyserVer, latestGeyserBuild, err := c.fetchLatestProjectBuild("geyser", "spigot")
	if err == nil && latestGeyserBuild != nil {
		geyserStatus.LatestVersion = latestGeyserVer
		geyserStatus.LatestBuild = latestGeyserBuild.Build
		geyserStatus.LatestSha256 = latestGeyserBuild.Downloads["spigot"].Sha256
		geyserStatus.ReleaseDate = latestGeyserBuild.Time

		for _, ch := range latestGeyserBuild.Changes {
			geyserStatus.RecentChanges = append(geyserStatus.RecentChanges, ch.Summary)
		}

		if geyserStatus.Installed {
			// Check if update is available via hash or build number
			if geyserStatus.InstalledSha256 != "" && geyserStatus.LatestSha256 != "" {
				geyserStatus.UpdateAvailable = !strings.EqualFold(geyserStatus.InstalledSha256, geyserStatus.LatestSha256)
			} else if geyserStatus.InstalledBuild > 0 {
				geyserStatus.UpdateAvailable = geyserStatus.InstalledBuild < geyserStatus.LatestBuild
			}
		}
	}

	// Query upstream Floodgate
	latestFloodgateVer, latestFloodgateBuild, err := c.fetchLatestProjectBuild("floodgate", "spigot")
	if err == nil && latestFloodgateBuild != nil {
		floodgateStatus.LatestVersion = latestFloodgateVer
		floodgateStatus.LatestBuild = latestFloodgateBuild.Build
		floodgateStatus.LatestSha256 = latestFloodgateBuild.Downloads["spigot"].Sha256
		floodgateStatus.ReleaseDate = latestFloodgateBuild.Time

		for _, ch := range latestFloodgateBuild.Changes {
			floodgateStatus.RecentChanges = append(floodgateStatus.RecentChanges, ch.Summary)
		}

		if floodgateStatus.Installed {
			if floodgateStatus.InstalledSha256 != "" && floodgateStatus.LatestSha256 != "" {
				floodgateStatus.UpdateAvailable = !strings.EqualFold(floodgateStatus.InstalledSha256, floodgateStatus.LatestSha256)
			} else if floodgateStatus.InstalledBuild > 0 {
				floodgateStatus.UpdateAvailable = floodgateStatus.InstalledBuild < floodgateStatus.LatestBuild
			}
		}
	}

	// Determine Bedrock compatibility info
	bedrockSupport, latestBedrockVer := c.resolveBedrockCompatibility(geyserStatus.RecentChanges)
	geyserStatus.SupportedBedrock = bedrockSupport
	geyserStatus.LatestBedrockVer = latestBedrockVer

	// Overall Status
	overall := "missing"
	if geyserStatus.Installed && floodgateStatus.Installed {
		if geyserStatus.UpdateAvailable || floodgateStatus.UpdateAvailable {
			overall = "update_available"
		} else {
			overall = "ready"
		}
	} else if geyserStatus.Installed || floodgateStatus.Installed {
		overall = "incomplete"
	}

	return &BedrockBridgeStatus{
		Geyser:             geyserStatus,
		Floodgate:          floodgateStatus,
		OverallStatus:      overall,
		BedrockSupportInfo: fmt.Sprintf("Supported Bedrock Versions: %s (Latest: %s)", bedrockSupport, latestBedrockVer),
	}, nil
}

// UpdateBedrockBridge downloads and installs the latest Geyser and/or Floodgate jars.
func (c *GeyserClient) UpdateBedrockBridge(pluginsDir, target string) ([]PluginInfo, error) {
	if err := os.MkdirAll(pluginsDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create plugins directory: %w", err)
	}

	var updated []PluginInfo
	targets := []string{}
	target = strings.ToLower(strings.TrimSpace(target))

	if target == "both" || target == "" {
		targets = []string{"geyser", "floodgate"}
	} else if target == "geyser" || target == "floodgate" {
		targets = []string{target}
	} else {
		return nil, fmt.Errorf("invalid update target: must be 'geyser', 'floodgate', or 'both'")
	}

	for _, t := range targets {
		version, build, err := c.fetchLatestProjectBuild(t, "spigot")
		if err != nil {
			return nil, fmt.Errorf("failed to discover latest %s build: %w", t, err)
		}

		downloadInfo, ok := build.Downloads["spigot"]
		if !ok {
			return nil, fmt.Errorf("spigot platform download not available for %s", t)
		}

		downloadURL := fmt.Sprintf("%s/projects/%s/versions/%s/builds/%d/downloads/spigot",
			c.APIBaseURL, t, version, build.Build)

		destFilename := downloadInfo.Name
		if destFilename == "" {
			if t == "geyser" {
				destFilename = "Geyser-Spigot.jar"
			} else {
				destFilename = "floodgate-spigot.jar"
			}
		}

		info, err := c.downloadAndInstallJar(pluginsDir, downloadURL, destFilename, downloadInfo.Sha256)
		if err != nil {
			return nil, fmt.Errorf("failed to download %s: %w", t, err)
		}

		updated = append(updated, *info)
	}

	return updated, nil
}

func (c *GeyserClient) fetchLatestProjectBuild(project, platform string) (string, *geyserBuild, error) {
	projURL := fmt.Sprintf("%s/projects/%s", c.APIBaseURL, project)
	resp, err := c.HTTPClient.Get(projURL)
	if err != nil {
		return "", nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	var projResp geyserProjectResp
	if err := json.NewDecoder(resp.Body).Decode(&projResp); err != nil {
		return "", nil, err
	}

	if len(projResp.Versions) == 0 {
		return "", nil, fmt.Errorf("no versions found for project %s", project)
	}

	latestVersion := projResp.Versions[len(projResp.Versions)-1]

	buildsURL := fmt.Sprintf("%s/projects/%s/versions/%s/builds", c.APIBaseURL, project, latestVersion)
	bResp, err := c.HTTPClient.Get(buildsURL)
	if err != nil {
		return "", nil, err
	}
	defer bResp.Body.Close()

	if bResp.StatusCode != http.StatusOK {
		return "", nil, fmt.Errorf("builds API returned status %d", bResp.StatusCode)
	}

	var buildsResp geyserBuildsResp
	if err := json.NewDecoder(bResp.Body).Decode(&buildsResp); err != nil {
		return "", nil, err
	}

	if len(buildsResp.Builds) == 0 {
		return "", nil, fmt.Errorf("no builds found for %s version %s", project, latestVersion)
	}

	// Filter and pick latest build with spigot download
	for i := len(buildsResp.Builds) - 1; i >= 0; i-- {
		b := &buildsResp.Builds[i]
		if _, ok := b.Downloads[platform]; ok {
			return latestVersion, b, nil
		}
	}

	return latestVersion, &buildsResp.Builds[len(buildsResp.Builds)-1], nil
}

func (c *GeyserClient) resolveBedrockCompatibility(recentChanges []string) (string, string) {
	// Try fetching GameProtocol.java from GitHub for ground truth
	if c.GameProtocolURL != "" {
		req, err := http.NewRequest(http.MethodGet, c.GameProtocolURL, nil)
		if err == nil {
			resp, err := c.HTTPClient.Do(req)
			if err == nil && resp.StatusCode == http.StatusOK {
				data, _ := io.ReadAll(resp.Body)
				_ = resp.Body.Close()
				codecsText := string(data)

				reRegister := regexp.MustCompile(`register\([^,]+,\s*([^)]+)\);`)
				matches := reRegister.FindAllStringSubmatch(codecsText, -1)
				var versions []string
				for _, m := range matches {
					if len(m) > 1 {
						parts := strings.Split(m[1], ",")
						for _, p := range parts {
							clean := strings.Trim(strings.TrimSpace(p), `"`)
							if clean != "" && !contains(versions, clean) {
								versions = append(versions, clean)
							}
						}
					}
				}

				if len(versions) > 0 {
					first := versions[0]
					last := versions[len(versions)-1]
					return fmt.Sprintf("Bedrock v%s - v%s", first, last), last
				}
			}
		}
	}

	// Fallback heuristic: check recent changes for "Bedrock" or "protocol"
	reBedrock := regexp.MustCompile(`(?i)Bedrock\s+v?([0-9]+\.[0-9]+(?:\.[0-9]+)?)`)
	for _, change := range recentChanges {
		m := reBedrock.FindStringSubmatch(change)
		if len(m) > 1 {
			return fmt.Sprintf("Bedrock v%s+", m[1]), m[1]
		}
	}

	return "Bedrock v1.21.x / v26.x (Latest)", "Latest"
}

func (c *GeyserClient) downloadAndInstallJar(pluginsDir, url, destFilename, expectedSha256 string) (*PluginInfo, error) {
	destPath := filepath.Join(pluginsDir, destFilename)
	tmpFile, err := os.CreateTemp(pluginsDir, "download-*.tmp")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp download file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer func() {
		_ = tmpFile.Close()
		_ = os.Remove(tmpPath)
	}()

	resp, err := c.HTTPClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("download request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download server returned HTTP %d", resp.StatusCode)
	}

	hasher := sha256.New()
	writer := io.MultiWriter(tmpFile, hasher)

	if _, err := io.Copy(writer, resp.Body); err != nil {
		return nil, fmt.Errorf("failed to write download stream: %w", err)
	}
	_ = tmpFile.Close()

	calculatedHash := hex.EncodeToString(hasher.Sum(nil))
	if expectedSha256 != "" && !strings.EqualFold(calculatedHash, expectedSha256) {
		return nil, fmt.Errorf("checksum mismatch: expected %s, got %s", expectedSha256, calculatedHash)
	}

	// If there are existing legacy or variant jars (e.g. Geyser-Spigot-old.jar.disabled), remove them
	removeConflictingJars(pluginsDir, destFilename)

	if err := os.Rename(tmpPath, destPath); err != nil {
		return nil, fmt.Errorf("failed to place downloaded plugin jar: %w", err)
	}

	return InspectPluginFile(destPath)
}

func removeConflictingJars(pluginsDir, newFilename string) {
	prefix := "geyser"
	if strings.Contains(strings.ToLower(newFilename), "floodgate") {
		prefix = "floodgate"
	}

	entries, _ := os.ReadDir(pluginsDir)
	for _, e := range entries {
		name := e.Name()
		lower := strings.ToLower(name)
		if strings.HasPrefix(lower, prefix) && (strings.HasSuffix(lower, ".jar") || strings.HasSuffix(lower, ".jar.disabled")) {
			if !strings.EqualFold(name, newFilename) {
				_ = os.Remove(filepath.Join(pluginsDir, name))
			}
		}
	}
}

func computeFileSha256(filePath string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func extractBuildNumber(version, filename string) int {
	re := regexp.MustCompile(`b([0-9]+)`)
	m := re.FindStringSubmatch(version)
	if len(m) > 1 {
		if b, err := strconv.Atoi(m[1]); err == nil {
			return b
		}
	}
	m = re.FindStringSubmatch(filename)
	if len(m) > 1 {
		if b, err := strconv.Atoi(m[1]); err == nil {
			return b
		}
	}
	return 0
}
