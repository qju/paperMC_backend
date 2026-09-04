package api

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"paperMC_backend/internal/minecraft"
	"paperMC_backend/internal/plugins"
)

func createTestJarFile(t *testing.T, targetPath, pluginName, version string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
		t.Fatalf("Failed to create dir: %v", err)
	}

	buf := new(bytes.Buffer)
	zw := zip.NewWriter(buf)
	w, err := zw.Create("plugin.yml")
	if err != nil {
		t.Fatalf("Failed to create zip entry: %v", err)
	}
	content := fmt.Sprintf("name: %s\nversion: %s\nmain: org.example.Plugin\n", pluginName, version)
	_, _ = w.Write([]byte(content))
	_ = zw.Close()

	if err := os.WriteFile(targetPath, buf.Bytes(), 0644); err != nil {
		t.Fatalf("Failed to write test jar: %v", err)
	}
}

func setupTestPluginEnvironment(t *testing.T) (*Handler, string, func()) {
	t.Helper()
	tempDir := t.TempDir()
	pluginsDir := filepath.Join(tempDir, "plugins")
	_ = os.MkdirAll(pluginsDir, 0755)

	mcServer := minecraft.NewServer(tempDir, "server.jar", "1G", nil)
	handler := NewServerHandler(mcServer, nil)

	cleanup := func() {
		handler.StopScheduler()
	}

	return handler, pluginsDir, cleanup
}

func TestPluginsAPI_GetToggleDelete(t *testing.T) {
	handler, pluginsDir, cleanup := setupTestPluginEnvironment(t)
	defer cleanup()

	// 1. Initial list empty
	req := httptest.NewRequest(http.MethodGet, "/api/plugins", nil)
	w := httptest.NewRecorder()
	handler.HandleGetPlugins(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", w.Code)
	}
	var listResp struct {
		Plugins []plugins.PluginInfo `json:"plugins"`
		Count   int                  `json:"count"`
	}
	_ = json.NewDecoder(w.Body).Decode(&listResp)
	if listResp.Count != 0 {
		t.Errorf("Expected 0 plugins, got %d", listResp.Count)
	}

	// 2. Create test plugin
	createTestJarFile(t, filepath.Join(pluginsDir, "LuckPerms.jar"), "LuckPerms", "5.4.102")

	// Get list after creation
	req = httptest.NewRequest(http.MethodGet, "/api/plugins", nil)
	w = httptest.NewRecorder()
	handler.HandleGetPlugins(w, req)
	_ = json.NewDecoder(w.Body).Decode(&listResp)
	if listResp.Count != 1 || listResp.Plugins[0].Name != "LuckPerms" {
		t.Fatalf("Expected LuckPerms in list, got %+v", listResp)
	}

	// 3. Toggle without filename
	req = httptest.NewRequest(http.MethodPost, "/api/plugins/toggle", nil)
	w = httptest.NewRecorder()
	handler.HandleTogglePlugin(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 for toggle without filename, got %d", w.Code)
	}

	// Toggle non-existent plugin
	req = httptest.NewRequest(http.MethodPost, "/api/plugins/toggle?filename=NonExistent.jar", nil)
	w = httptest.NewRecorder()
	handler.HandleTogglePlugin(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 for non-existent plugin, got %d", w.Code)
	}

	// Toggle valid plugin via JSON body
	reqBody, _ := json.Marshal(PluginActionRequest{Filename: "LuckPerms.jar"})
	req = httptest.NewRequest(http.MethodPost, "/api/plugins/toggle", bytes.NewBuffer(reqBody))
	w = httptest.NewRecorder()
	handler.HandleTogglePlugin(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200 on toggle, got %d: %s", w.Code, w.Body.String())
	}
	var toggleResp map[string]string
	_ = json.NewDecoder(w.Body).Decode(&toggleResp)
	if toggleResp["new_filename"] != "LuckPerms.jar.disabled" {
		t.Errorf("Expected LuckPerms.jar.disabled, got %s", toggleResp["new_filename"])
	}

	// 4. Delete without filename
	req = httptest.NewRequest(http.MethodDelete, "/api/plugins", nil)
	w = httptest.NewRecorder()
	handler.HandleDeletePlugin(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 for delete without filename, got %d", w.Code)
	}

	// Delete valid plugin
	req = httptest.NewRequest(http.MethodDelete, "/api/plugins?filename=LuckPerms.jar.disabled", nil)
	w = httptest.NewRecorder()
	handler.HandleDeletePlugin(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200 on delete, got %d: %s", w.Code, w.Body.String())
	}

	// Verify list is empty again
	req = httptest.NewRequest(http.MethodGet, "/api/plugins", nil)
	w = httptest.NewRecorder()
	handler.HandleGetPlugins(w, req)
	_ = json.NewDecoder(w.Body).Decode(&listResp)
	if listResp.Count != 0 {
		t.Errorf("Expected 0 plugins after deletion, got %d", listResp.Count)
	}
}

func TestPluginsAPI_UploadPlugin(t *testing.T) {
	handler, _, cleanup := setupTestPluginEnvironment(t)
	defer cleanup()

	// 1. Non-multipart upload
	req := httptest.NewRequest(http.MethodPost, "/api/plugins/upload", bytes.NewBufferString("not-multipart"))
	w := httptest.NewRecorder()
	handler.HandleUploadPlugin(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 on invalid multipart, got %d", w.Code)
	}

	// 2. Missing "file" part
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	_ = writer.Close()
	req = httptest.NewRequest(http.MethodPost, "/api/plugins/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w = httptest.NewRecorder()
	handler.HandleUploadPlugin(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 on missing file part, got %d", w.Code)
	}

	// 3. Valid plugin upload
	bodyValid := &bytes.Buffer{}
	writerValid := multipart.NewWriter(bodyValid)
	part, _ := writerValid.CreateFormFile("file", "WorldEdit.jar")

	// Create test jar in memory
	zw := zip.NewWriter(part)
	wZip, _ := zw.Create("plugin.yml")
	_, _ = wZip.Write([]byte("name: WorldEdit\nversion: 7.3.0\nmain: com.sk89q.worldedit.WorldEdit\n"))
	_ = zw.Close()
	_ = writerValid.Close()

	req = httptest.NewRequest(http.MethodPost, "/api/plugins/upload", bodyValid)
	req.Header.Set("Content-Type", writerValid.FormDataContentType())
	w = httptest.NewRecorder()
	handler.HandleUploadPlugin(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("Expected 201 on valid upload, got %d: %s", w.Code, w.Body.String())
	}

	var created plugins.PluginInfo
	_ = json.NewDecoder(w.Body).Decode(&created)
	if created.Name != "WorldEdit" || created.Version != "7.3.0" {
		t.Errorf("Unexpected created plugin info: %+v", created)
	}
}

func TestPluginsAPI_GeyserWorkflow(t *testing.T) {
	handler, _, cleanup := setupTestPluginEnvironment(t)
	defer cleanup()

	// Fake Geyser Jar
	fakeGeyserJar := createFakeJar(t, "Geyser-Spigot", "2.11.2-b1234")
	geyserSha := sha256.Sum256(fakeGeyserJar)
	geyserShaHex := hex.EncodeToString(geyserSha[:])

	// Mock upstream Geyser server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		if path == "/projects/geyser" {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"project_id": "geyser",
				"versions":   []string{"2.11.2"},
			})
			return
		}

		if path == "/projects/geyser/versions/2.11.2/builds" {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"project_id": "geyser",
				"version":    "2.11.2",
				"builds": []map[string]interface{}{
					{
						"build": 1234,
						"time":  time.Now(),
						"changes": []map[string]string{
							{"commit": "123", "summary": "Support Bedrock v26.45"},
						},
						"downloads": map[string]interface{}{
							"spigot": map[string]string{
								"name":   "Geyser-Spigot.jar",
								"sha256": geyserShaHex,
							},
						},
					},
				},
			})
			return
		}

		if path == "/projects/geyser/versions/2.11.2/builds/1234/downloads/spigot" {
			w.Header().Set("Content-Type", "application/java-archive")
			_, _ = w.Write(fakeGeyserJar)
			return
		}

		if path == "/projects/floodgate" {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"project_id": "floodgate",
				"versions":   []string{"2.2.5"},
			})
			return
		}

		if path == "/projects/floodgate/versions/2.2.5/builds" {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"project_id": "floodgate",
				"version":    "2.2.5",
				"builds": []map[string]interface{}{
					{
						"build": 140,
						"time":  time.Now(),
						"changes": []map[string]string{
							{"commit": "456", "summary": "Fix login"},
						},
						"downloads": map[string]interface{}{
							"spigot": map[string]string{
								"name":   "floodgate-spigot.jar",
								"sha256": geyserShaHex,
							},
						},
					},
				},
			})
			return
		}

		if path == "/projects/floodgate/versions/2.2.5/builds/140/downloads/spigot" {
			w.Header().Set("Content-Type", "application/java-archive")
			_, _ = w.Write(fakeGeyserJar)
			return
		}

		if path == "/GameProtocol.java" {
			_, _ = w.Write([]byte(`register(Bedrock_v924.CODEC, "26.0", "26.45");`))
			return
		}

		http.NotFound(w, r)
	}))
	defer server.Close()

	geyserClient := &plugins.GeyserClient{
		APIBaseURL:      server.URL,
		GameProtocolURL: server.URL + "/GameProtocol.java",
		HTTPClient:      server.Client(),
	}
	handler.SetGeyserClient(geyserClient)

	// 1. GET /api/plugins/geyser/status
	req := httptest.NewRequest(http.MethodGet, "/api/plugins/geyser/status", nil)
	w := httptest.NewRecorder()
	handler.HandleGetGeyserStatus(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200 on geyser status, got %d: %s", w.Code, w.Body.String())
	}
	var status plugins.BedrockBridgeStatus
	_ = json.NewDecoder(w.Body).Decode(&status)
	if status.Geyser.LatestBuild != 1234 || status.OverallStatus != "missing" {
		t.Errorf("Unexpected bedrock bridge status: %+v", status)
	}

	// 2. POST /api/plugins/geyser/update (Update Geyser)
	reqBody, _ := json.Marshal(GeyserUpdateRequest{Target: "geyser"})
	req = httptest.NewRequest(http.MethodPost, "/api/plugins/geyser/update", bytes.NewBuffer(reqBody))
	w = httptest.NewRecorder()
	handler.HandleUpdateGeyser(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200 on geyser update, got %d: %s", w.Code, w.Body.String())
	}

	var updateResp struct {
		Status  string               `json:"status"`
		Updated []plugins.PluginInfo `json:"updated"`
	}
	_ = json.NewDecoder(w.Body).Decode(&updateResp)
	if len(updateResp.Updated) != 1 || updateResp.Updated[0].Name != "Geyser-Spigot" {
		t.Errorf("Unexpected updated response: %+v", updateResp)
	}
}

func TestPluginsAPI_MarketplaceWorkflow(t *testing.T) {
	handler, _, cleanup := setupTestPluginEnvironment(t)
	defer cleanup()

	fakeSparkJar := createFakeJar(t, "Spark", "1.10.53")
	sha512Hash := sha512.Sum512(fakeSparkJar)
	sha512Hex := hex.EncodeToString(sha512Hash[:])

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		if path == "/search" {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"total_hits": 1,
				"limit":      20,
				"offset":     0,
				"hits": []map[string]interface{}{
					{
						"project_id":  "spark_proj",
						"slug":        "spark",
						"title":       "spark",
						"description": "Profiler",
						"versions":    []string{"1.10.53"},
					},
				},
			})
			return
		}

		if path == "/project/spark_proj/version" {
			_ = json.NewEncoder(w).Encode([]map[string]interface{}{
				{
					"id":             "ver1",
					"project_id":     "spark_proj",
					"name":           "1.10.53",
					"version_number": "1.10.53",
					"files": []map[string]interface{}{
						{
							"filename": "spark-paper.jar",
							"primary":  true,
							"url":      "http://" + r.Host + "/download/spark.jar",
							"hashes": map[string]string{
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
			_, _ = w.Write(fakeSparkJar)
			return
		}

		http.NotFound(w, r)
	}))
	defer server.Close()

	modrinthClient := &plugins.ModrinthClient{
		BaseURL:    server.URL,
		HTTPClient: server.Client(),
	}
	handler.SetModrinthClient(modrinthClient)

	// 1. GET /api/plugins/market/search
	req := httptest.NewRequest(http.MethodGet, "/api/plugins/market/search?query=spark", nil)
	w := httptest.NewRecorder()
	handler.HandleSearchMarketPlugins(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200 on market search, got %d", w.Code)
	}
	var searchResp plugins.ModrinthSearchResult
	_ = json.NewDecoder(w.Body).Decode(&searchResp)
	if len(searchResp.Hits) != 1 || searchResp.Hits[0].Slug != "spark" {
		t.Errorf("Unexpected search hits: %+v", searchResp)
	}

	// 2. POST /api/plugins/market/install without project_id
	req = httptest.NewRequest(http.MethodPost, "/api/plugins/market/install", bytes.NewBufferString("{}"))
	w = httptest.NewRecorder()
	handler.HandleInstallMarketPlugin(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 without project_id, got %d", w.Code)
	}

	// 3. POST /api/plugins/market/install valid
	reqBody, _ := json.Marshal(MarketInstallRequest{ProjectID: "spark_proj"})
	req = httptest.NewRequest(http.MethodPost, "/api/plugins/market/install", bytes.NewBuffer(reqBody))
	w = httptest.NewRecorder()
	handler.HandleInstallMarketPlugin(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("Expected 201 on market install, got %d: %s", w.Code, w.Body.String())
	}
	var installed plugins.PluginInfo
	_ = json.NewDecoder(w.Body).Decode(&installed)
	if installed.Name != "Spark" || installed.Version != "1.10.53" {
		t.Errorf("Unexpected installed plugin: %+v", installed)
	}
}

func createFakeJar(t *testing.T, name, version string) []byte {
	t.Helper()
	buf := new(bytes.Buffer)
	zw := zip.NewWriter(buf)
	w, _ := zw.Create("plugin.yml")
	content := fmt.Sprintf("name: %s\nversion: %s\nmain: org.example.Plugin\n", name, version)
	_, _ = w.Write([]byte(content))
	_ = zw.Close()
	return buf.Bytes()
}
