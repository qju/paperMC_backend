package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

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

	// 3. Unknown User (Anti-enumeration returns 401 with identical timing)
	t.Run("Unknown User", func(t *testing.T) {
		loginBody, _ := json.Marshal(LoginRequest{
			Username: "ghost_user",
			Password: "Password",
		})
		req := httptest.NewRequest(http.MethodPost, "/login", bytes.NewReader(loginBody))
		w := httptest.NewRecorder()

		handler.Login(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Fatalf("Expected 401 StatusUnauthorized, got %d", w.Code)
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

	// 5. Missing fields
	t.Run("Missing Fields", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/login", bytes.NewReader([]byte(`{"username":""}`)))
		w := httptest.NewRecorder()
		handler.Login(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("Expected 400 on empty fields, got %d", w.Code)
		}
	})

	// 6. Nil Store
	t.Run("Nil Store", func(t *testing.T) {
		nilHandler := &Handler{}
		loginBody, _ := json.Marshal(LoginRequest{Username: "u", Password: "p"})
		req := httptest.NewRequest(http.MethodPost, "/login", bytes.NewReader(loginBody))
		w := httptest.NewRecorder()
		nilHandler.Login(w, req)
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("Expected 500 on nil store, got %d", w.Code)
		}
	})

	// 7. Rate Limiter Lockout
	t.Run("Rate Limiter Lockout", func(t *testing.T) {
		// Re-initialize rate limiter with 3 max attempts for fast testing
		handler.loginLimiter = NewLoginRateLimiter(3, 1*time.Minute, 5*time.Minute)
		clientIP := "192.168.1.99:12345"

		badBody, _ := json.Marshal(LoginRequest{Username: "admin_user", Password: "wrong"})

		for i := 0; i < 3; i++ {
			req := httptest.NewRequest(http.MethodPost, "/login", bytes.NewReader(badBody))
			req.RemoteAddr = clientIP
			w := httptest.NewRecorder()
			handler.Login(w, req)
			if w.Code != http.StatusUnauthorized {
				t.Fatalf("Attempt %d: expected 401, got %d", i+1, w.Code)
			}
		}

		// 4th attempt should trigger 429 Too Many Requests
		reqBlocked := httptest.NewRequest(http.MethodPost, "/login", bytes.NewReader(badBody))
		reqBlocked.RemoteAddr = clientIP
		wBlocked := httptest.NewRecorder()
		handler.Login(wBlocked, reqBlocked)
		if wBlocked.Code != http.StatusTooManyRequests {
			t.Fatalf("Expected 429 TooManyRequests on lockout, got %d", wBlocked.Code)
		}
		if wBlocked.Header().Get("Retry-After") == "" {
			t.Errorf("Expected Retry-After header to be set on 429 response")
		}
	})
}
