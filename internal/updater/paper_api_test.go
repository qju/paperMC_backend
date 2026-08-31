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
