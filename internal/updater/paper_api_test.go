package updater

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		v1       string
		v2       string
		expected int
	}{
		{"26.2", "26.1", 1},
		{"26.1", "26.2", -1},
		{"26.2", "1.21", 1},
		{"1.21", "1.20", 1},
		{"1.21.1", "1.21.1", 0},
		{"1.21.2", "1.21.1", 1},
	}

	for _, tt := range tests {
		got := compareVersions(tt.v1, tt.v2)
		if (tt.expected > 0 && got <= 0) || (tt.expected < 0 && got >= 0) || (tt.expected == 0 && got != 0) {
			t.Errorf("compareVersions(%q, %q) = %d; want sign %d", tt.v1, tt.v2, got, tt.expected)
		}
	}
}

func TestDownloadJarChecksumValidation(t *testing.T) {
	content := []byte("fake-server-jar-content-12345")
	hasher := sha256.New()
	hasher.Write(content)
	validSHA256 := hex.EncodeToString(hasher.Sum(nil))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(content)
	}))
	defer server.Close()

	tempDir := t.TempDir()

	// 1. Success case
	err := DownloadJar(server.URL, tempDir, "server.jar.download", validSHA256)
	if err != nil {
		t.Fatalf("DownloadJar failed on valid checksum: %v", err)
	}

	downloadedBytes, err := os.ReadFile(filepath.Join(tempDir, "server.jar.download"))
	if err != nil {
		t.Fatalf("Failed to read downloaded file: %v", err)
	}
	if string(downloadedBytes) != string(content) {
		t.Errorf("Downloaded content mismatch")
	}

	// 2. Failure on checksum mismatch
	invalidSHA256 := "0000000000000000000000000000000000000000000000000000000000000000"
	err = DownloadJar(server.URL, tempDir, "bad.jar.download", invalidSHA256)
	if err == nil {
		t.Errorf("Expected checksum mismatch error, got nil")
	}

	// Verify temp file was cleaned up on mismatch
	if _, err := os.Stat(filepath.Join(tempDir, "bad.jar.download")); !os.IsNotExist(err) {
		t.Errorf("Expected corrupted temp file to be deleted")
	}
}

func TestGetFileHash(t *testing.T) {
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.jar")
	content := []byte("hash-testing-content")

	_ = os.WriteFile(testFile, content, 0644)

	hasher := sha256.New()
	hasher.Write(content)
	expectedHash := hex.EncodeToString(hasher.Sum(nil))

	gotHash, err := GetFileHash(testFile)
	if err != nil {
		t.Fatalf("GetFileHash failed: %v", err)
	}
	if gotHash != expectedHash {
		t.Errorf("GetFileHash = %s, want %s", gotHash, expectedHash)
	}

	// Non-existent file returns empty string, nil error
	emptyHash, err := GetFileHash(filepath.Join(tempDir, "non_existent.jar"))
	if err != nil || emptyHash != "" {
		t.Errorf("Expected empty hash and nil error for missing file, got '%s', %v", emptyHash, err)
	}
}

func TestGetLatestBuild_EmptyVersion(t *testing.T) {
	_, err := GetLatestBuild("paper", "")
	if err == nil {
		t.Errorf("Expected error when version is empty, got nil")
	}
}

func TestGetProjectVersions_MockServer(t *testing.T) {
	mockJSON := `{
		"project": {"id": "paper", "name": "Paper"},
		"versions": {
			"26.2": ["26.2.0", "26.2.1"],
			"1.21": ["1.21.0", "1.21.1"]
		}
	}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(mockJSON))
	}))
	defer server.Close()

	origURL := FillAPIBaseURL
	FillAPIBaseURL = server.URL
	defer func() { FillAPIBaseURL = origURL }()

	resp, err := GetProjectVersions("paper")
	if err != nil {
		t.Fatalf("GetProjectVersions failed: %v", err)
	}

	if resp.Project != "paper" || len(resp.VersionGroups) != 2 {
		t.Errorf("Unexpected project versions response: %+v", resp)
	}
	if resp.VersionGroups[0].Group != "26.2" {
		t.Errorf("Expected first group '26.2' sorted descending, got '%s'", resp.VersionGroups[0].Group)
	}
}

func TestGetLatestBuild_MockServer(t *testing.T) {
	mockBuildJSON := `{
		"id": 45,
		"time": "2026-08-30T10:00:00Z",
		"channel": "default",
		"downloads": {
			"server:default": {
				"name": "paper-26.2-45.jar",
				"checksums": {"sha256": "abcdef1234567890"},
				"size": 52428800,
				"url": "https://download.papermc.io/v3/paper-26.2-45.jar"
			}
		}
	}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(mockBuildJSON))
	}))
	defer server.Close()

	origURL := FillAPIBaseURL
	FillAPIBaseURL = server.URL
	defer func() { FillAPIBaseURL = origURL }()

	info, err := GetLatestBuild("paper", "26.2")
	if err != nil {
		t.Fatalf("GetLatestBuild failed: %v", err)
	}

	if info.BuildID != 45 || info.FileName != "paper-26.2-45.jar" || info.SHA256 != "abcdef1234567890" {
		t.Errorf("Unexpected build info: %+v", info)
	}
}

func TestGetProjectVersions_EscapingAndErrors(t *testing.T) {
	origURL := FillAPIBaseURL
	defer func() { FillAPIBaseURL = origURL }()

	t.Run("escapes project path and handles empty project default", func(t *testing.T) {
		var seenURIs []string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			seenURIs = append(seenURIs, r.RequestURI)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"versions":{"1.21":["1.21.1"]}}`))
		}))
		defer server.Close()
		FillAPIBaseURL = server.URL

		if _, err := GetProjectVersions("paper/..?"); err != nil {
			t.Fatalf("Expected escaped GetProjectVersions to succeed, got: %v", err)
		}
		if resp, err := GetProjectVersions(""); err != nil || resp.Project != DefaultProject {
			t.Fatalf("Expected default project %q, got resp=%+v err=%v", DefaultProject, resp, err)
		}
		if len(seenURIs) != 2 || seenURIs[0] != "/projects/paper%2F..%3F" || seenURIs[1] != "/projects/paper" {
			t.Errorf("Unexpected RequestURIs: %v", seenURIs)
		}
	})

	t.Run("non-200 status and malformed json", func(t *testing.T) {
		mode := "status"
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if mode == "status" {
				w.WriteHeader(http.StatusBadGateway)
				return
			}
			_, _ = w.Write([]byte(`{not-json`))
		}))
		defer server.Close()
		FillAPIBaseURL = server.URL

		if _, err := GetProjectVersions("paper"); err == nil || !strings.Contains(err.Error(), "status 502") {
			t.Errorf("Expected HTTP 502 error, got: %v", err)
		}

		mode = "json"
		if _, err := GetProjectVersions("paper"); err == nil || !strings.Contains(err.Error(), "parse json") {
			t.Errorf("Expected parse json error, got: %v", err)
		}
	})
}

func TestGetLatestBuild_EscapingAndErrors(t *testing.T) {
	origURL := FillAPIBaseURL
	defer func() { FillAPIBaseURL = origURL }()

	if _, err := GetLatestBuild("paper", "   "); err == nil || !strings.Contains(err.Error(), "cannot be empty") {
		t.Errorf("Expected empty version error, got: %v", err)
	}

	var seenURI string
	responseCode := http.StatusOK
	responseBody := `{"id":1,"channel":"STABLE","downloads":{}}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenURI = r.RequestURI
		w.WriteHeader(responseCode)
		_, _ = w.Write([]byte(responseBody))
	}))
	defer server.Close()
	FillAPIBaseURL = server.URL

	// Missing server:default artifact + empty project default + version path escaping
	if _, err := GetLatestBuild("", "1.21/../../bad"); err == nil || !strings.Contains(err.Error(), "no server download artifact") {
		t.Errorf("Expected missing download artifact error, got: %v", err)
	}
	if seenURI != "/projects/paper/versions/1.21%2F..%2F..%2Fbad/builds/latest" {
		t.Errorf("Expected escaped RequestURI, got %q", seenURI)
	}

	// 404 Not Found
	responseCode = http.StatusNotFound
	if _, err := GetLatestBuild("paper", "9.99"); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Errorf("Expected 404 not found error, got: %v", err)
	}

	// 500 Server Error
	responseCode = http.StatusInternalServerError
	if _, err := GetLatestBuild("paper", "1.21.4"); err == nil || !strings.Contains(err.Error(), "status 500") {
		t.Errorf("Expected 500 error, got: %v", err)
	}

	// Malformed JSON
	responseCode = http.StatusOK
	responseBody = `{bad`
	if _, err := GetLatestBuild("paper", "1.21.4"); err == nil || !strings.Contains(err.Error(), "parse build response") {
		t.Errorf("Expected parse build error, got: %v", err)
	}
}

func TestDownloadJar_ErrorCases(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/fail" {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte("jar-bytes"))
	}))
	defer server.Close()

	tmpDir := t.TempDir()

	if err := DownloadJar(server.URL+"/fail", tmpDir, "out.jar", ""); err == nil || !strings.Contains(err.Error(), "HTTP status 503") {
		t.Errorf("Expected HTTP 503 download error, got: %v", err)
	}

	if err := DownloadJar("://invalid-url", tmpDir, "out.jar", ""); err == nil || !strings.Contains(err.Error(), "build download request") {
		t.Errorf("Expected invalid URL request error, got: %v", err)
	}

	missingDir := filepath.Join(tmpDir, "does_not_exist")
	if err := DownloadJar(server.URL+"/ok", missingDir, "out.jar", ""); err == nil || !strings.Contains(err.Error(), "staging file") {
		t.Errorf("Expected staging file creation error, got: %v", err)
	}
}


