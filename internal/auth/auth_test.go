package auth

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestHashPasswordAndCheck(t *testing.T) {
	password := "SecurePassword123!"

	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("Expected no error hashing password, got: %v", err)
	}

	if hash == password {
		t.Fatalf("Hashed password must not match plaintext password")
	}

	if !CheckPasswordHash(password, hash) {
		t.Fatalf("Expected password check to succeed for correct password")
	}

	if CheckPasswordHash("WrongPassword", hash) {
		t.Fatalf("Expected password check to fail for incorrect password")
	}
}

func TestGenerateAndValidateToken(t *testing.T) {
	username := "admin_tester"
	role := "admin"

	tokenStr, err := GenerateToken(username, role)
	if err != nil {
		t.Fatalf("Expected token generation to succeed, got: %v", err)
	}

	if tokenStr == "" {
		t.Fatalf("Generated token string must not be empty")
	}

	claims, err := ValidateToken(tokenStr)
	if err != nil {
		t.Fatalf("Expected token validation to succeed, got: %v", err)
	}

	if claims.Username != username {
		t.Errorf("Expected username '%s', got '%s'", username, claims.Username)
	}

	if claims.Role != role {
		t.Errorf("Expected role '%s', got '%s'", role, claims.Role)
	}
}

func TestValidateToken_Expired(t *testing.T) {
	// Generate an intentionally expired token
	claims := &Claims{
		Username: "expired_user",
		Role:     "operator",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-1 * time.Hour)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, err := token.SignedString(jwtSecret)
	if err != nil {
		t.Fatalf("Failed to sign test token: %v", err)
	}

	_, err = ValidateToken(tokenStr)
	if err == nil {
		t.Fatalf("Expected validation error for expired token, but got nil")
	}
}

func TestValidateToken_InvalidSignature(t *testing.T) {
	// Sign with a different key
	wrongSecret := []byte("different-secret-key-123456789")
	claims := &Claims{
		Username: "spoofed_user",
		Role:     "admin",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(1 * time.Hour)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, err := token.SignedString(wrongSecret)
	if err != nil {
		t.Fatalf("Failed to sign test token: %v", err)
	}

	_, err = ValidateToken(tokenStr)
	if err == nil {
		t.Fatalf("Expected validation error for token with invalid signature, but got nil")
	}
}

func TestValidateToken_InvalidSigningMethod(t *testing.T) {
	// Token with None algorithm (unsecured)
	claims := &Claims{
		Username: "none_algo_user",
		Role:     "admin",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(1 * time.Hour)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodNone, claims)
	tokenStr, err := token.SignedString(jwt.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Fatalf("Failed to create none token: %v", err)
	}

	_, err = ValidateToken(tokenStr)
	if err == nil {
		t.Fatalf("Expected validation to fail for SigningMethodNone, but got nil")
	}
}

func TestValidateToken_Malformed(t *testing.T) {
	_, err := ValidateToken("invalid.jwt.token.format")
	if err == nil {
		t.Fatalf("Expected error for malformed token, got nil")
	}
}

func TestGetSecret_CustomEnv(t *testing.T) {
	customKey := "my-custom-prod-secret-999"
	_ = os.Setenv("JWT_SECRET", customKey)
	defer os.Unsetenv("JWT_SECRET")

	secret := getSecret()
	if string(secret) != customKey {
		t.Fatalf("Expected getSecret() to return '%s', got '%s'", customKey, string(secret))
	}
}

func TestAuthMiddleware(t *testing.T) {
	tokenStr, err := GenerateToken("auth_user", "admin")
	if err != nil {
		t.Fatalf("Failed to generate test token: %v", err)
	}

	var extractedClaims *Claims
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		val := r.Context().Value(UserKey)
		if claims, ok := val.(*Claims); ok {
			extractedClaims = claims
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	protected := AuthMiddleware(testHandler)

	// 1. Valid Authorization Header
	t.Run("Valid Header", func(t *testing.T) {
		extractedClaims = nil
		req := httptest.NewRequest("GET", "/protected", nil)
		req.Header.Set("Authorization", "Bearer "+tokenStr)
		rec := httptest.NewRecorder()

		protected.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("Expected status 200, got %d", rec.Code)
		}
		if extractedClaims == nil || extractedClaims.Username != "auth_user" {
			t.Fatalf("Expected extracted claims for 'auth_user', got: %+v", extractedClaims)
		}
	})

	// 2. Valid URL Query Parameter fallback
	t.Run("Valid Query Param Fallback", func(t *testing.T) {
		extractedClaims = nil
		req := httptest.NewRequest("GET", "/protected?token="+tokenStr, nil)
		rec := httptest.NewRecorder()

		protected.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("Expected status 200, got %d", rec.Code)
		}
		if extractedClaims == nil || extractedClaims.Username != "auth_user" {
			t.Fatalf("Expected extracted claims for 'auth_user', got: %+v", extractedClaims)
		}
	})

	// 3. Missing Token
	t.Run("Missing Token", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/protected", nil)
		rec := httptest.NewRecorder()

		protected.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("Expected status 401 Unauthorized, got %d", rec.Code)
		}
	})

	// 4. Invalid Token
	t.Run("Invalid Token", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/protected", nil)
		req.Header.Set("Authorization", "Bearer invalid-garbage-token")
		rec := httptest.NewRecorder()

		protected.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("Expected status 401 Unauthorized, got %d", rec.Code)
		}
	})
}
