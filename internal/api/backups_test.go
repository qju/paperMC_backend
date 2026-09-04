package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"paperMC_backend/internal/backup"
	"paperMC_backend/internal/minecraft"
)

func setupTestWorldForAPI(t *testing.T, workDir, worldName string) {
	worldDir := filepath.Join(workDir, worldName)
	if err := os.MkdirAll(worldDir, 0755); err != nil {
		t.Fatalf("Failed to create world dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(worldDir, "level.dat"), []byte("api-level-data"), 0644); err != nil {
		t.Fatalf("Failed to write level.dat: %v", err)
	}
	// Also create server.properties setting level-name
	props := "level-name=" + worldName + "\n"
	_ = os.WriteFile(filepath.Join(workDir, "server.properties"), []byte(props), 0644)
}

func TestBackupsAPIEndpoints(t *testing.T) {
	tempDir := t.TempDir()
	setupTestWorldForAPI(t, tempDir, "world")

	mcServer := minecraft.NewServer(tempDir, "server.jar", "2G", nil)
	handler := NewServerHandler(mcServer, nil)

	// 1. GET /api/backups initially empty
	reqGet := httptest.NewRequest(http.MethodGet, "/api/backups", nil)
	wGet := httptest.NewRecorder()
	handler.HandleGetBackups(wGet, reqGet)

	if wGet.Code != http.StatusOK {
		t.Fatalf("Expected 200 on initial GET /api/backups, got %d: %s", wGet.Code, wGet.Body.String())
	}

	var initialListResp map[string]interface{}
	if err := json.NewDecoder(wGet.Body).Decode(&initialListResp); err != nil {
		t.Fatalf("Failed to parse GET /api/backups response: %v", err)
	}
	backupsSlice := initialListResp["backups"].([]interface{})
	if len(backupsSlice) != 0 {
		t.Errorf("Expected 0 backups initially, got %d", len(backupsSlice))
	}

	// 2. POST /api/backups/create with invalid JSON
	reqBadJSON := httptest.NewRequest(http.MethodPost, "/api/backups/create", bytes.NewBufferString("invalid-json"))
	wBadJSON := httptest.NewRecorder()
	handler.HandleCreateBackup(wBadJSON, reqBadJSON)
	if wBadJSON.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 on bad JSON, got %d", wBadJSON.Code)
	}

	// 3. POST /api/backups/create invalid type
	badTypePayload := `{"type":"unknown"}`
	reqBadType := httptest.NewRequest(http.MethodPost, "/api/backups/create", bytes.NewBufferString(badTypePayload))
	wBadType := httptest.NewRecorder()
	handler.HandleCreateBackup(wBadType, reqBadType)
	if wBadType.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 on bad backup type, got %d", wBadType.Code)
	}

	// 4. POST /api/backups/create default world snapshot
	createPayload := `{"type":"world","world_name":"world"}`
	reqCreate := httptest.NewRequest(http.MethodPost, "/api/backups/create", bytes.NewBufferString(createPayload))
	wCreate := httptest.NewRecorder()
	handler.HandleCreateBackup(wCreate, reqCreate)

	if wCreate.Code != http.StatusCreated {
		t.Fatalf("Expected 201 Created on valid backup creation, got %d: %s", wCreate.Code, wCreate.Body.String())
	}

	var createdInfo backup.BackupInfo
	if err := json.NewDecoder(wCreate.Body).Decode(&createdInfo); err != nil {
		t.Fatalf("Failed to decode created backup info: %v", err)
	}
	if createdInfo.BackupType != "world" || createdInfo.WorldName != "world" {
		t.Errorf("Unexpected created backup metadata: %+v", createdInfo)
	}

	// 5. GET /api/backups with created backup
	reqGetAfter := httptest.NewRequest(http.MethodGet, "/api/backups", nil)
	wGetAfter := httptest.NewRecorder()
	handler.HandleGetBackups(wGetAfter, reqGetAfter)

	if wGetAfter.Code != http.StatusOK {
		t.Fatalf("Expected 200 on GET /api/backups, got %d", wGetAfter.Code)
	}
	var afterResp map[string]interface{}
	_ = json.NewDecoder(wGetAfter.Body).Decode(&afterResp)
	afterBackups := afterResp["backups"].([]interface{})
	if len(afterBackups) != 1 {
		t.Fatalf("Expected 1 backup in list, got %d", len(afterBackups))
	}

	// 6. GET /api/backups/download missing file param
	reqDlMissing := httptest.NewRequest(http.MethodGet, "/api/backups/download", nil)
	wDlMissing := httptest.NewRecorder()
	handler.HandleDownloadBackup(wDlMissing, reqDlMissing)
	if wDlMissing.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 on missing file param, got %d", wDlMissing.Code)
	}

	// 7. GET /api/backups/download path traversal
	reqDlTraversal := httptest.NewRequest(http.MethodGet, "/api/backups/download?file=../../etc/shadow", nil)
	wDlTraversal := httptest.NewRecorder()
	handler.HandleDownloadBackup(wDlTraversal, reqDlTraversal)
	if wDlTraversal.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 on path traversal download, got %d", wDlTraversal.Code)
	}

	// 8. GET /api/backups/download success
	reqDl := httptest.NewRequest(http.MethodGet, "/api/backups/download?file="+createdInfo.Filename, nil)
	wDl := httptest.NewRecorder()
	handler.HandleDownloadBackup(wDl, reqDl)

	if wDl.Code != http.StatusOK {
		t.Fatalf("Expected 200 on valid download, got %d", wDl.Code)
	}
	if !strings.Contains(wDl.Header().Get("Content-Disposition"), createdInfo.Filename) {
		t.Errorf("Expected Content-Disposition header with filename, got %s", wDl.Header().Get("Content-Disposition"))
	}
	if wDl.Body.Len() == 0 {
		t.Error("Expected non-empty download body")
	}

	// 9. POST /api/backups/restore invalid JSON & empty file
	reqRestoreBad := httptest.NewRequest(http.MethodPost, "/api/backups/restore", bytes.NewBufferString(`{"file":""}`))
	wRestoreBad := httptest.NewRecorder()
	handler.HandleRestoreBackup(wRestoreBad, reqRestoreBad)
	if wRestoreBad.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 on empty restore filename, got %d", wRestoreBad.Code)
	}

	// 10. POST /api/backups/restore valid
	restorePayload := `{"file":"` + createdInfo.Filename + `"}`
	reqRestore := httptest.NewRequest(http.MethodPost, "/api/backups/restore", bytes.NewBufferString(restorePayload))
	wRestore := httptest.NewRecorder()
	handler.HandleRestoreBackup(wRestore, reqRestore)
	if wRestore.Code != http.StatusOK {
		t.Fatalf("Expected 200 on valid restore, got %d: %s", wRestore.Code, wRestore.Body.String())
	}

	// 11. DELETE /api/backups empty file
	reqDelEmpty := httptest.NewRequest(http.MethodDelete, "/api/backups", nil)
	wDelEmpty := httptest.NewRecorder()
	handler.HandleDeleteBackup(wDelEmpty, reqDelEmpty)
	if wDelEmpty.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 on empty delete filename, got %d", wDelEmpty.Code)
	}

	// 12. DELETE /api/backups success via query parameter
	reqDel := httptest.NewRequest(http.MethodDelete, "/api/backups?file="+createdInfo.Filename, nil)
	wDel := httptest.NewRecorder()
	handler.HandleDeleteBackup(wDel, reqDel)
	if wDel.Code != http.StatusOK {
		t.Fatalf("Expected 200 on valid delete, got %d: %s", wDel.Code, wDel.Body.String())
	}

	// 13. DELETE /api/backups already deleted file
	reqDelAgain := httptest.NewRequest(http.MethodDelete, "/api/backups?file="+createdInfo.Filename, nil)
	wDelAgain := httptest.NewRecorder()
	handler.HandleDeleteBackup(wDelAgain, reqDelAgain)
	if wDelAgain.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 on deleting non-existent backup, got %d", wDelAgain.Code)
	}
}
