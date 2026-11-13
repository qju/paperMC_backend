package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"path/filepath"
	"testing"
	"testing/fstest"
	"time"

	"github.com/stretchr/testify/assert"
	"paperMC_backend/internal/api"
	"paperMC_backend/internal/auth"
	"paperMC_backend/internal/database"
	"paperMC_backend/internal/minecraft"
)

func TestSPAHandler(t *testing.T) {
	mockFS := fstest.MapFS{
		"index.html": &fstest.MapFile{
			Data: []byte("<!doctype html><html><body>SPA Root</body></html>"),
		},
		"assets/app.js": &fstest.MapFile{
			Data: []byte("console.log('loaded');"),
		},
	}

	fileserver := http.FileServer(http.FS(mockFS))
	handler := spaHandler(mockFS, fileserver)

	t.Run("API route returns 404 JSON not index.html", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/flags", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusNotFound, rec.Code)
		assert.Contains(t, rec.Header().Get("Content-Type"), "application/json")
		assert.JSONEq(t, `{"error":"API route not found"}`, rec.Body.String())
	})

	t.Run("Root /api route returns 404 JSON", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusNotFound, rec.Code)
		assert.Contains(t, rec.Header().Get("Content-Type"), "application/json")
		assert.JSONEq(t, `{"error":"API route not found"}`, rec.Body.String())
	})

	t.Run("SPA route falls back to index.html with 200 OK", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/config", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Contains(t, rec.Body.String(), "SPA Root")
	})

	t.Run("Static asset serves directly", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/assets/app.js", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Contains(t, rec.Body.String(), "console.log('loaded');")
	})
}

func TestRegisterProtectedRoutesRBAC(t *testing.T) {
	origExec := minecraft.ExecCommandContext
	defer func() { minecraft.ExecCommandContext = origExec }()
	minecraft.ExecCommandContext = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "cat")
	}

	tempDir := t.TempDir()
	store, err := database.NewSQLiteStore(filepath.Join(tempDir, "rbac_test.db"))
	assert.NoError(t, err)
	defer store.Close()

	mcServer := minecraft.NewServer(tempDir, "server.jar", "2G", store)
	defer func() {
		_ = mcServer.Kill()
		mcServer.WaitForStop(2 * time.Second)
	}()
	mcHandler := api.NewServerHandler(mcServer, store)

	mux := http.NewServeMux()
	registerProtectedRoutes(mux, mcHandler)

	viewerToken, err := auth.GenerateToken("viewer_u", auth.RoleViewer)
	assert.NoError(t, err)
	operatorToken, err := auth.GenerateToken("operator_u", auth.RoleOperator)
	assert.NoError(t, err)
	adminToken, err := auth.GenerateToken("admin_u", auth.RoleAdmin)
	assert.NoError(t, err)

	doReq := func(method, path, token string) int {
		req := httptest.NewRequest(method, path, nil)
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		return rec.Code
	}

	// Unauthenticated request is rejected with 401
	assert.Equal(t, http.StatusUnauthorized, doReq(http.MethodGet, "/status", ""))

	// Viewer: read-only + 2FA self-service allowed; mutations/admin forbidden
	for _, tc := range []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/status"},
		{http.MethodGet, "/api/players"},
		{http.MethodGet, "/api/worlds"},
		{http.MethodGet, "/api/auth/2fa/status"},
		{http.MethodGet, "/api/crash/ai-config"},
	} {
		code := doReq(tc.method, tc.path, viewerToken)
		assert.NotEqual(t, http.StatusUnauthorized, code, "%s %s should be authorized for viewer", tc.method, tc.path)
		assert.NotEqual(t, http.StatusForbidden, code, "%s %s should be permitted for viewer", tc.method, tc.path)
	}

	for _, tc := range []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/start"},
		{http.MethodPost, "/api/players"},
		{http.MethodPost, "/api/worlds/active"},
		{http.MethodPost, "/api/backups/create"},
		{http.MethodPost, "/command"},
		{http.MethodPost, "/config"},
		{http.MethodGet, "/api/users"},
		{http.MethodPost, "/api/users"},
		{http.MethodPost, "/api/plugins/toggle"},
		{http.MethodDelete, "/api/audit"},
	} {
		assert.Equal(t, http.StatusForbidden, doReq(tc.method, tc.path, viewerToken), "viewer must get 403 for %s %s", tc.method, tc.path)
	}

	// Operator: lifecycle/players/worlds/backups/crash/profiler permitted; admin-only endpoints forbidden
	for _, tc := range []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/start"},
		{http.MethodPost, "/stop"},
		{http.MethodPost, "/api/worlds/active"},
		{http.MethodPost, "/api/crash/analyze"},
		{http.MethodPost, "/api/profiler/health"},
	} {
		code := doReq(tc.method, tc.path, operatorToken)
		assert.NotEqual(t, http.StatusUnauthorized, code, "%s %s should be authorized for operator", tc.method, tc.path)
		assert.NotEqual(t, http.StatusForbidden, code, "%s %s should be permitted for operator", tc.method, tc.path)
	}

	for _, tc := range []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/command"},
		{http.MethodPost, "/kill"},
		{http.MethodPost, "/config"},
		{http.MethodPost, "/update"},
		{http.MethodPost, "/api/updater/apply"},
		{http.MethodGet, "/api/users"},
		{http.MethodPost, "/api/users"},
		{http.MethodPut, "/api/users/password"},
		{http.MethodDelete, "/api/users"},
		{http.MethodPost, "/api/users/reset-2fa"},
		{http.MethodPost, "/api/plugins/toggle"},
		{http.MethodDelete, "/api/plugins"},
		{http.MethodPost, "/api/plugins/upload"},
		{http.MethodPost, "/api/plugins/geyser/update"},
		{http.MethodPost, "/api/plugins/market/install"},
		{http.MethodPost, "/api/flags"},
		{http.MethodDelete, "/api/audit"},
		{http.MethodPost, "/api/crash/ai-config"},
	} {
		assert.Equal(t, http.StatusForbidden, doReq(tc.method, tc.path, operatorToken), "operator must get 403 for %s %s", tc.method, tc.path)
	}

	// Admin: privileged endpoints are never blocked by RBAC
	for _, tc := range []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/users"},
		{http.MethodPost, "/command"},
		{http.MethodPost, "/config"},
		{http.MethodPost, "/api/flags"},
		{http.MethodDelete, "/api/audit"},
		{http.MethodPost, "/api/crash/ai-config"},
	} {
		code := doReq(tc.method, tc.path, adminToken)
		assert.NotEqual(t, http.StatusUnauthorized, code, "%s %s should be authorized for admin", tc.method, tc.path)
		assert.NotEqual(t, http.StatusForbidden, code, "%s %s should be permitted for admin", tc.method, tc.path)
	}
}
