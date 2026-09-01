package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"paperMC_backend/internal/auth"
	"paperMC_backend/internal/database"
	"paperMC_backend/internal/minecraft"
)

func TestAuthLoginAPI(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "auth_test.db")
	store, err := database.NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to init store: %v", err)
	}
	defer store.Close()

	// Seed user
	hashedPass, _ := auth.HashPassword("CorrectPassword123")
	_ = store.CreateUser(&database.User{
		Username: "login_test_user",
		Password: hashedPass,
		Role:     "admin",
	})

	mcServer := minecraft.NewServer(tempDir, "server.jar", "2G", store)
	handler := NewServerHandler(mcServer, store)

	// 1. Success Login
	t.Run("Success", func(t *testing.T) {
		loginBody, _ := json.Marshal(LoginRequest{
			Username: "login_test_user",
			Password: "CorrectPassword123",
		})
		req := httptest.NewRequest(http.MethodPost, "/login", bytes.NewReader(loginBody))
		w := httptest.NewRecorder()

		handler.Login(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("Expected 200 OK, got %d: %s", w.Code, w.Body.String())
		}

		var resp LoginResponse
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("Failed to decode login response: %v", err)
		}
		if resp.Token == "" {
			t.Fatalf("Expected non-empty JWT token")
		}

		// Validate returned token
		claims, err := auth.ValidateToken(resp.Token)
		if err != nil || claims.Username != "login_test_user" {
			t.Errorf("Invalid token claims: %v, %+v", err, claims)
		}
	})

	// 2. Wrong Password
	t.Run("Wrong Password", func(t *testing.T) {
		loginBody, _ := json.Marshal(LoginRequest{
			Username: "login_test_user",
			Password: "WrongPassword",
		})
		req := httptest.NewRequest(http.MethodPost, "/login", bytes.NewReader(loginBody))
		w := httptest.NewRecorder()

		handler.Login(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Fatalf("Expected 401 Unauthorized, got %d", w.Code)
		}
	})

	// 3. Unknown User
	t.Run("Unknown User", func(t *testing.T) {
		loginBody, _ := json.Marshal(LoginRequest{
			Username: "ghost_user",
			Password: "Password",
		})
		req := httptest.NewRequest(http.MethodPost, "/login", bytes.NewReader(loginBody))
		w := httptest.NewRecorder()

		handler.Login(w, req)

		if w.Code != http.StatusNotFound {
			t.Fatalf("Expected 404 StatusNotFound, got %d", w.Code)
		}
	})

	// 4. Invalid JSON
	t.Run("Invalid JSON", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/login", bytes.NewReader([]byte("not-json")))
		w := httptest.NewRecorder()

		handler.Login(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("Expected 400 BadRequest, got %d", w.Code)
		}
	})
}
