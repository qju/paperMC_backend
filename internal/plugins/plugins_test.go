package plugins

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Helper to create a fake plugin jar with plugin.yml
func createTestJar(t *testing.T, targetPath, manifestName, yamlContent string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
		t.Fatalf("Failed to create parent dir: %v", err)
	}

	buf := new(bytes.Buffer)
	zw := zip.NewWriter(buf)

	w, err := zw.Create(manifestName)
	if err != nil {
		t.Fatalf("Failed to create zip entry: %v", err)
	}
	if _, err := w.Write([]byte(yamlContent)); err != nil {
		t.Fatalf("Failed to write yaml content: %v", err)
	}

	if err := zw.Close(); err != nil {
		t.Fatalf("Failed to close zip writer: %v", err)
	}

	if err := os.WriteFile(targetPath, buf.Bytes(), 0644); err != nil {
		t.Fatalf("Failed to write jar file: %v", err)
	}
}

func TestPlugins_ScanAndInspect(t *testing.T) {
	tempDir := t.TempDir()
	pluginsDir := filepath.Join(tempDir, "plugins")

	// 1. Scan non-existent directory
	list, err := ScanPlugins(pluginsDir)
	if err != nil {
		t.Fatalf("Expected nil err on non-existent dir, got: %v", err)
	}
	if len(list) != 0 {
		t.Errorf("Expected 0 plugins, got %d", len(list))
	}

	// 2. Create plugins
	createTestJar(t, filepath.Join(pluginsDir, "Vault.jar"), "plugin.yml", `
name: Vault
version: 1.7.3-b131
main: net.milkbowl.vault.Vault
author: Sleaker
authors: [carlgo11, morganm]
description: Vault is a Permissions, Chat, & Economy API
api-version: 1.13
depend: []
softdepend: [EssentialsX]
`)

	createTestJar(t, filepath.Join(pluginsDir, "Geyser-Spigot.jar.disabled"), "paper-plugin.yml", `
name: Geyser-Spigot
version: 2.4.2-SNAPSHOT-b1230
main: org.geysermc.geyser.platform.spigot.GeyserSpigotPlugin
description: Bedrock bridge for Minecraft Java servers
authors: [GeyserMC]
`)

	// Corrupted jar
	_ = os.WriteFile(filepath.Join(pluginsDir, "Corrupt.jar"), []byte("not-a-zip"), 0644)
	// Non-jar file
	_ = os.WriteFile(filepath.Join(pluginsDir, "readme.txt"), []byte("hello"), 0644)

	list, err = ScanPlugins(pluginsDir)
	if err != nil {
		t.Fatalf("ScanPlugins failed: %v", err)
	}

	if len(list) != 3 { // Vault, Geyser-Spigot (disabled), Corrupt
		t.Fatalf("Expected 3 plugins, got %d", len(list))
	}

	// Corrupt should be recognized with fallback metadata
	var corruptFound, vaultFound, geyserFound bool
	for _, p := range list {
		if p.Name == "Corrupt" {
			corruptFound = true
			if p.Version != "unknown" || !p.IsEnabled {
				t.Errorf("Unexpected corrupt metadata: %+v", p)
			}
		}
		if p.Name == "Vault" {
			vaultFound = true
			if !p.IsEnabled || p.Version != "1.7.3-b131" || len(p.Authors) != 3 {
				t.Errorf("Unexpected Vault metadata: %+v", p)
			}
		}
		if p.Name == "Geyser-Spigot" {
			geyserFound = true
			if p.IsEnabled {
				t.Errorf("Expected Geyser to be disabled (.jar.disabled)")
			}
		}
	}

	if !corruptFound || !vaultFound || !geyserFound {
		t.Errorf("Missing expected plugins: corrupt=%v, vault=%v, geyser=%v", corruptFound, vaultFound, geyserFound)
	}
}

func TestPlugins_ToggleAndDelete(t *testing.T) {
	tempDir := t.TempDir()
	pluginsDir := filepath.Join(tempDir, "plugins")

	createTestJar(t, filepath.Join(pluginsDir, "EssentialsX.jar"), "plugin.yml", "name: EssentialsX\nversion: 2.20.1\n")

	// Invalid filenames
	if _, err := TogglePlugin(pluginsDir, "../evil.jar"); err == nil {
		t.Errorf("Expected error for path traversal in TogglePlugin")
	}
	if _, err := TogglePlugin(pluginsDir, "nonexistent.jar"); err == nil {
		t.Errorf("Expected error for non-existent file in TogglePlugin")
	}
	if _, err := TogglePlugin(pluginsDir, "test.txt"); err == nil {
		t.Errorf("Expected error for non-jar in TogglePlugin")
	}

	// Toggle EssentialsX.jar -> EssentialsX.jar.disabled
	newFile, err := TogglePlugin(pluginsDir, "EssentialsX.jar")
	if err != nil {
		t.Fatalf("TogglePlugin failed: %v", err)
	}
	if newFile != "EssentialsX.jar.disabled" {
		t.Errorf("Expected EssentialsX.jar.disabled, got %s", newFile)
	}

	// Toggle back: EssentialsX.jar.disabled -> EssentialsX.jar
	newFile2, err := TogglePlugin(pluginsDir, "EssentialsX.jar.disabled")
	if err != nil {
		t.Fatalf("Toggle back failed: %v", err)
	}
	if newFile2 != "EssentialsX.jar" {
		t.Errorf("Expected EssentialsX.jar, got %s", newFile2)
	}

	// Delete invalid
	if err := DeletePlugin(pluginsDir, "../evil.jar"); err == nil {
		t.Errorf("Expected error on path traversal in DeletePlugin")
	}
	if err := DeletePlugin(pluginsDir, "nonexistent.jar"); err == nil {
		t.Errorf("Expected error on non-existent file in DeletePlugin")
	}

	// Delete valid
	if err := DeletePlugin(pluginsDir, "EssentialsX.jar"); err != nil {
		t.Fatalf("DeletePlugin failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(pluginsDir, "EssentialsX.jar")); !os.IsNotExist(err) {
		t.Errorf("File should not exist after deletion")
	}
}

func TestPlugins_SaveUploadedPlugin(t *testing.T) {
	tempDir := t.TempDir()
	pluginsDir := filepath.Join(tempDir, "plugins")

	// 1. Invalid filename
	if _, err := SaveUploadedPlugin(pluginsDir, "bad.txt", bytes.NewReader([]byte("123"))); err == nil {
		t.Errorf("Expected error for non-jar upload")
	}
	if _, err := SaveUploadedPlugin(pluginsDir, "../evil.jar", bytes.NewReader([]byte("123"))); err == nil {
		t.Errorf("Expected error for path traversal upload")
	}

	// 2. Corrupt jar
	if _, err := SaveUploadedPlugin(pluginsDir, "test.jar", bytes.NewReader([]byte("bad-zip"))); err == nil {
		t.Errorf("Expected error for corrupt zip upload")
	}

	// 3. Zip missing plugin.yml
	bufNoManifest := new(bytes.Buffer)
	zw := zip.NewWriter(bufNoManifest)
	w, _ := zw.Create("hello.txt")
	_, _ = w.Write([]byte("hello"))
	_ = zw.Close()

	if _, err := SaveUploadedPlugin(pluginsDir, "test.jar", bufNoManifest); err == nil {
		t.Errorf("Expected error for zip without plugin manifest")
	}

	// 4. Valid plugin upload
	bufValid := new(bytes.Buffer)
	zw2 := zip.NewWriter(bufValid)
	w2, _ := zw2.Create("plugin.yml")
	_, _ = w2.Write([]byte("name: LuckPerms\nversion: 5.4.102\nauthor: Luck\n"))
	_ = zw2.Close()

	info, err := SaveUploadedPlugin(pluginsDir, "LuckPerms.jar", bufValid)
	if err != nil {
		t.Fatalf("Failed to save valid plugin: %v", err)
	}
	if info.Name != "LuckPerms" || info.Version != "5.4.102" || !info.IsEnabled {
		t.Errorf("Unexpected uploaded plugin info: %+v", info)
	}
}

func TestGeyserClient_MockWorkflow(t *testing.T) {
	tempDir := t.TempDir()
	pluginsDir := filepath.Join(tempDir, "plugins")

	// Setup fake jar for download
	fakeGeyserJarBytes := createFakeJarBytes(t, "Geyser-Spigot", "2.11.2-b1234")
	geyserSha := sha256.Sum256(fakeGeyserJarBytes)
	geyserShaHex := hex.EncodeToString(geyserSha[:])

	fakeFloodgateJarBytes := createFakeJarBytes(t, "floodgate", "2.2.5-b140")
	floodgateSha := sha256.Sum256(fakeFloodgateJarBytes)
	floodgateShaHex := hex.EncodeToString(floodgateSha[:])

	// Mock server for Geyser API
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		if path == "/projects/geyser" {
			_ = json.NewEncoder(w).Encode(geyserProjectResp{
				ProjectID:   "geyser",
				ProjectName: "Geyser",
				Versions:    []string{"2.11.1", "2.11.2"},
			})
			return
		}

		if path == "/projects/geyser/versions/2.11.2/builds" {
			_ = json.NewEncoder(w).Encode(geyserBuildsResp{
				ProjectID: "geyser",
				Version:   "2.11.2",
				Builds: []geyserBuild{
					{
						Build: 1234,
						Time:  time.Now(),
						Changes: []geyserChange{
							{Commit: "abc1234", Summary: "Support Bedrock v26.45 protocol"},
						},
						Downloads: map[string]geyserDownload{
							"spigot": {
								Name:   "Geyser-Spigot.jar",
								Sha256: geyserShaHex,
							},
						},
					},
				},
			})
			return
		}

		if path == "/projects/geyser/versions/2.11.2/builds/1234/downloads/spigot" {
			w.Header().Set("Content-Type", "application/java-archive")
			_, _ = w.Write(fakeGeyserJarBytes)
			return
		}

		if path == "/projects/floodgate" {
			_ = json.NewEncoder(w).Encode(geyserProjectResp{
				ProjectID:   "floodgate",
				ProjectName: "Floodgate",
				Versions:    []string{"2.2.4", "2.2.5"},
			})
			return
		}

		if path == "/projects/floodgate/versions/2.2.5/builds" {
			_ = json.NewEncoder(w).Encode(geyserBuildsResp{
				ProjectID: "floodgate",
				Version:   "2.2.5",
				Builds: []geyserBuild{
					{
						Build: 140,
						Time:  time.Now(),
						Changes: []geyserChange{
							{Commit: "def5678", Summary: "Fix skin loading"},
						},
						Downloads: map[string]geyserDownload{
							"spigot": {
								Name:   "floodgate-spigot.jar",
								Sha256: floodgateShaHex,
							},
						},
					},
				},
			})
			return
		}

		if path == "/projects/floodgate/versions/2.2.5/builds/140/downloads/spigot" {
			w.Header().Set("Content-Type", "application/java-archive")
			_, _ = w.Write(fakeFloodgateJarBytes)
			return
		}

		if path == "/GameProtocol.java" {
			_, _ = w.Write([]byte(`
register(Bedrock_v924.CODEC, "26.0", "26.1");
register(Bedrock_v2169.CODEC, "26.45");
`))
			return
		}

		http.NotFound(w, r)
	}))
	defer server.Close()

	client := &GeyserClient{
		APIBaseURL:      server.URL,
		GameProtocolURL: server.URL + "/GameProtocol.java",
		HTTPClient:      server.Client(),
	}

	// 1. Initial status: nothing installed
	status, err := client.GetBedrockBridgeStatus(pluginsDir)
	if err != nil {
		t.Fatalf("GetBedrockBridgeStatus failed: %v", err)
	}
	if status.OverallStatus != "missing" {
		t.Errorf("Expected 'missing' status, got %s", status.OverallStatus)
	}
	if status.Geyser.LatestBuild != 1234 || status.Floodgate.LatestBuild != 140 {
		t.Errorf("Unexpected latest build numbers: geyser=%d, floodgate=%d", status.Geyser.LatestBuild, status.Floodgate.LatestBuild)
	}
	if !strings.Contains(status.Geyser.SupportedBedrock, "26.0") || !strings.Contains(status.Geyser.SupportedBedrock, "26.45") {
		t.Errorf("Expected Bedrock v26.0 - v26.45, got %s", status.Geyser.SupportedBedrock)
	}

	// 2. Update both Geyser and Floodgate
	updated, err := client.UpdateBedrockBridge(pluginsDir, "both")
	if err != nil {
		t.Fatalf("UpdateBedrockBridge failed: %v", err)
	}
	if len(updated) != 2 {
		t.Fatalf("Expected 2 updated plugins, got %d", len(updated))
	}

	// 3. Status check after installation: should be 'ready' and up to date
	statusAfter, err := client.GetBedrockBridgeStatus(pluginsDir)
	if err != nil {
		t.Fatalf("GetBedrockBridgeStatus failed after install: %v", err)
	}
	if statusAfter.OverallStatus != "ready" {
		t.Errorf("Expected 'ready', got %s", statusAfter.OverallStatus)
	}
	if statusAfter.Geyser.UpdateAvailable || statusAfter.Floodgate.UpdateAvailable {
		t.Errorf("Expected both to be up to date")
	}

	// 4. Test invalid update target
	if _, err := client.UpdateBedrockBridge(pluginsDir, "invalid_target"); err == nil {
		t.Errorf("Expected error on invalid update target")
	}
}

func TestModrinthClient_MockWorkflow(t *testing.T) {
	tempDir := t.TempDir()
	pluginsDir := filepath.Join(tempDir, "plugins")

	fakePluginJar := createFakeJarBytes(t, "Spark", "1.10.53")
	sha512Hash := sha512.Sum512(fakePluginJar)
	sha512Hex := hex.EncodeToString(sha512Hash[:])

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		if path == "/search" {
			_ = json.NewEncoder(w).Encode(modrinthRawSearch{
				TotalHits: 1,
				Limit:     20,
				Offset:    0,
				Hits: []modrinthRawHit{
					{
						ProjectID:   "l6YH9Als",
						Slug:        "spark",
						Title:       "spark",
						Description: "A performance profiler for Minecraft clients and servers",
						Author:      "lucko",
						Downloads:   5000000,
						Versions:    []string{"1.10.53"},
					},
				},
			})
			return
		}

		if path == "/project/spark/version" || path == "/project/l6YH9Als/version" {
			_ = json.NewEncoder(w).Encode([]ModrinthVersion{
				{
					ID:         "ver123",
					ProjectID:  "l6YH9Als",
					Name:       "spark 1.10.53",
					VersionNum: "1.10.53",
					Files: []ModrinthFile{
						{
							Filename: "spark-1.10.53-bukkit.jar",
							Primary:  true,
							URL:      "http://" + r.Host + "/download/spark.jar",
							Hashes: map[string]string{
								"sha512": sha512Hex,
							},
						},
					},
				},
			})
			return
		}

		if path == "/download/spark.jar" {
			w.Header().Set("Content-Type", "application/java-archive")
			_, _ = w.Write(fakePluginJar)
			return
		}

		http.NotFound(w, r)
	}))
	defer server.Close()

	client := &ModrinthClient{
		BaseURL:    server.URL,
		HTTPClient: server.Client(),
	}

	// 1. Search
	res, err := client.Search("spark", 10, 0)
	if err != nil {
		t.Fatalf("Modrinth search failed: %v", err)
	}
	if len(res.Hits) != 1 || res.Hits[0].Slug != "spark" {
		t.Fatalf("Unexpected search hits: %+v", res.Hits)
	}

	// 2. Install
	installed, err := client.InstallPlugin(pluginsDir, "spark", "")
	if err != nil {
		t.Fatalf("InstallPlugin failed: %v", err)
	}
	if installed.Name != "Spark" || installed.Version != "1.10.53" {
		t.Errorf("Unexpected installed plugin info: %+v", installed)
	}
	if _, err := os.Stat(filepath.Join(pluginsDir, "spark-1.10.53-bukkit.jar")); err != nil {
		t.Errorf("Downloaded jar file does not exist: %v", err)
	}
}

func TestModrinthClient_InstallPlugin_FallbackAndEncoding(t *testing.T) {
	pluginsDir := t.TempDir()
	fakeJar := createFakeJarBytes(t, "FallbackPlugin", "2.0.0")
	hasher := sha512.New()
	hasher.Write(fakeJar)
	sha512Hex := hex.EncodeToString(hasher.Sum(nil))

	loaderQueryChecked := false
	fallbackCalled := false

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		if path == "/project/testproj/version" {
			loadersParam := r.URL.Query().Get("loaders")
			if loadersParam != "" {
				loaderQueryChecked = true
				// Verify loaders is valid JSON array containing paper/spigot
				var loaders []string
				if err := json.Unmarshal([]byte(loadersParam), &loaders); err != nil {
					t.Errorf("Failed to unmarshal encoded loaders: %v", err)
				}
				// Return empty to test fallback
				_ = json.NewEncoder(w).Encode([]ModrinthVersion{})
				return
			}

			// Fallback call without loaders
			fallbackCalled = true
			_ = json.NewEncoder(w).Encode([]ModrinthVersion{
				{
					ID:         "ver999",
					ProjectID:  "testproj",
					Name:       "Release 2.0.0",
					VersionNum: "2.0.0",
					Files: []ModrinthFile{
						{
							Filename: "notes.txt",
							Primary:  false,
							URL:      "http://" + r.Host + "/download/notes.txt",
						},
						{
							Filename: "fallback-2.0.0.jar",
							Primary:  true,
							URL:      "http://" + r.Host + "/download/plugin.jar",
							Hashes: map[string]string{
								"sha512": sha512Hex,
							},
						},
					},
				},
			})
			return
		}

		if path == "/download/plugin.jar" {
			w.Header().Set("Content-Type", "application/java-archive")
			_, _ = w.Write(fakeJar)
			return
		}

		http.NotFound(w, r)
	}))
	defer server.Close()

	client := &ModrinthClient{
		BaseURL:    server.URL,
		HTTPClient: server.Client(),
	}

	info, err := client.InstallPlugin(pluginsDir, "testproj", "")
	if err != nil {
		t.Fatalf("InstallPlugin failed: %v", err)
	}

	if !loaderQueryChecked {
		t.Errorf("Expected initial request with encoded loaders parameter")
	}
	if !fallbackCalled {
		t.Errorf("Expected fallback request without loaders parameter")
	}
	if info.Name != "FallbackPlugin" || info.Version != "2.0.0" {
		t.Errorf("Unexpected installed info: %+v", info)
	}
	if _, err := os.Stat(filepath.Join(pluginsDir, "fallback-2.0.0.jar")); err != nil {
		t.Errorf("Expected fallback jar to be installed: %v", err)
	}
}

func createFakeJarBytes(t *testing.T, pluginName, version string) []byte {
	t.Helper()
	buf := new(bytes.Buffer)
	zw := zip.NewWriter(buf)

	w, err := zw.Create("plugin.yml")
	if err != nil {
		t.Fatalf("Failed to create zip file: %v", err)
	}

	content := fmt.Sprintf("name: %s\nversion: %s\nmain: org.example.Plugin\n", pluginName, version)
	_, _ = w.Write([]byte(content))
	_ = zw.Close()

	return buf.Bytes()
}
