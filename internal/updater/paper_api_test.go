package updater

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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


