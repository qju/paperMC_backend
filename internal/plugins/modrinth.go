package plugins

import (
	"crypto/sha1"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const DefaultModrinthBaseURL = "https://api.modrinth.com/v2"

// ModrinthClient communicates with the Modrinth v2 API.
type ModrinthClient struct {
	BaseURL    string
	HTTPClient *http.Client
}

// NewModrinthClient initializes a default Modrinth client.
func NewModrinthClient() *ModrinthClient {
	return &ModrinthClient{
		BaseURL:    DefaultModrinthBaseURL,
		HTTPClient: &http.Client{Timeout: 20 * time.Second},
	}
}

type modrinthRawSearch struct {
	Hits      []modrinthRawHit `json:"hits"`
	TotalHits int              `json:"total_hits"`
	Limit     int              `json:"limit"`
	Offset    int              `json:"offset"`
}

type modrinthRawHit struct {
	ProjectID    string    `json:"project_id"`
	Slug         string    `json:"slug"`
	Title        string    `json:"title"`
	Description  string    `json:"description"`
	Categories   []string  `json:"categories"`
	IconURL      string    `json:"icon_url"`
	Author       string    `json:"author"`
	Downloads    int       `json:"downloads"`
	Followers    int       `json:"follows"`
	DateModified time.Time `json:"date_modified"`
	Versions     []string  `json:"versions"`
}

// Search searches for Paper/Spigot plugins on Modrinth.
func (m *ModrinthClient) Search(query string, limit, offset int) (*ModrinthSearchResult, error) {
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	facets := `[["project_type:plugin"],["categories:paper","categories:spigot","categories:purpur"]]`
	params := url.Values{}
	params.Set("query", query)
	params.Set("limit", fmt.Sprintf("%d", limit))
	params.Set("offset", fmt.Sprintf("%d", offset))
	params.Set("facets", facets)

	reqURL := fmt.Sprintf("%s/search?%s", m.BaseURL, params.Encode())
	req, err := http.NewRequest(http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Lodestone-Minecraft-Manager/2.0")

	resp, err := m.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("modrinth search request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("modrinth search returned HTTP %d", resp.StatusCode)
	}

	var raw modrinthRawSearch
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("failed to decode search response: %w", err)
	}

	result := &ModrinthSearchResult{
		Hits:      make([]ModrinthHit, 0, len(raw.Hits)),
		TotalHits: raw.TotalHits,
		Limit:     raw.Limit,
		Offset:    raw.Offset,
	}

	for _, h := range raw.Hits {
		latestVer := ""
		if len(h.Versions) > 0 {
			latestVer = h.Versions[len(h.Versions)-1]
		}
		result.Hits = append(result.Hits, ModrinthHit{
			ProjectID:    h.ProjectID,
			Slug:         h.Slug,
			Title:        h.Title,
			Description:  h.Description,
			Categories:   h.Categories,
			IconURL:      h.IconURL,
			Author:       h.Author,
			Downloads:    h.Downloads,
			Followers:    h.Followers,
			DateModified: h.DateModified,
			LatestVer:    latestVer,
		})
	}

	return result, nil
}

// InstallPlugin downloads and installs a plugin directly from Modrinth into pluginsDir.
func (m *ModrinthClient) InstallPlugin(pluginsDir, projectID, versionID string) (*PluginInfo, error) {
	if err := os.MkdirAll(pluginsDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to ensure plugins directory exists: %w", err)
	}

	var targetVer ModrinthVersion
	if strings.TrimSpace(versionID) == "" {
		// Fetch versions for project
		reqURL := fmt.Sprintf("%s/project/%s/version?loaders=[\"paper\",\"spigot\",\"purpur\"]", m.BaseURL, projectID)
		req, err := http.NewRequest(http.MethodGet, reqURL, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", "Lodestone-Minecraft-Manager/2.0")

		resp, err := m.HTTPClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch project versions: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("versions API returned HTTP %d", resp.StatusCode)
		}

		var versions []ModrinthVersion
		if err := json.NewDecoder(resp.Body).Decode(&versions); err != nil {
			return nil, fmt.Errorf("failed to decode versions list: %w", err)
		}

		if len(versions) == 0 {
			return nil, fmt.Errorf("no compatible versions found for project %s", projectID)
		}

		targetVer = versions[0]
	} else {
		reqURL := fmt.Sprintf("%s/version/%s", m.BaseURL, versionID)
		req, err := http.NewRequest(http.MethodGet, reqURL, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", "Lodestone-Minecraft-Manager/2.0")

		resp, err := m.HTTPClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch version %s: %w", versionID, err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("version details returned HTTP %d", resp.StatusCode)
		}

		if err := json.NewDecoder(resp.Body).Decode(&targetVer); err != nil {
			return nil, fmt.Errorf("failed to decode version info: %w", err)
		}
	}

	var targetFile *ModrinthFile
	for i := range targetVer.Files {
		f := &targetVer.Files[i]
		if strings.HasSuffix(strings.ToLower(f.Filename), ".jar") {
			if f.Primary || targetFile == nil {
				targetFile = f
			}
		}
	}

	if targetFile == nil {
		return nil, fmt.Errorf("no downloadable jar file found in release %s", targetVer.Name)
	}

	// Download file
	tmpFile, err := os.CreateTemp(pluginsDir, "modrinth-*.tmp")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer func() {
		_ = tmpFile.Close()
		_ = os.Remove(tmpPath)
	}()

	dlReq, err := http.NewRequest(http.MethodGet, targetFile.URL, nil)
	if err != nil {
		return nil, err
	}
	dlReq.Header.Set("User-Agent", "Lodestone-Minecraft-Manager/2.0")

	dlResp, err := m.HTTPClient.Do(dlReq)
	if err != nil {
		return nil, fmt.Errorf("file download failed: %w", err)
	}
	defer dlResp.Body.Close()

	if dlResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("file server returned HTTP %d", dlResp.StatusCode)
	}

	sha512Hasher := sha512.New()
	sha1Hasher := sha1.New()
	writer := io.MultiWriter(tmpFile, sha512Hasher, sha1Hasher)

	if _, err := io.Copy(writer, dlResp.Body); err != nil {
		return nil, fmt.Errorf("failed to write downloaded plugin: %w", err)
	}
	_ = tmpFile.Close()

	// Verify hash if available
	if expected512, ok := targetFile.Hashes["sha512"]; ok && expected512 != "" {
		calc512 := hex.EncodeToString(sha512Hasher.Sum(nil))
		if !strings.EqualFold(calc512, expected512) {
			return nil, fmt.Errorf("sha512 hash mismatch: expected %s, got %s", expected512, calc512)
		}
	} else if expected1, ok := targetFile.Hashes["sha1"]; ok && expected1 != "" {
		calc1 := hex.EncodeToString(sha1Hasher.Sum(nil))
		if !strings.EqualFold(calc1, expected1) {
			return nil, fmt.Errorf("sha1 hash mismatch: expected %s, got %s", expected1, calc1)
		}
	}

	destPath := filepath.Join(pluginsDir, targetFile.Filename)
	if err := os.Rename(tmpPath, destPath); err != nil {
		return nil, fmt.Errorf("failed to install downloaded jar: %w", err)
	}

	return InspectPluginFile(destPath)
}
