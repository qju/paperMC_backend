package auth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestGenerateMFASecret(t *testing.T) {
	secret, err := GenerateMFASecret()
	if err != nil {
		t.Fatalf("GenerateMFASecret failed: %v", err)
	}
	if len(secret) == 0 {
		t.Errorf("Expected non-empty secret")
	}

	// Secret must be decodable
	bytes, err := decodeBase32Secret(secret)
	if err != nil {
		t.Fatalf("decodeBase32Secret failed on generated secret: %v", err)
	}
	if len(bytes) != 20 {
		t.Errorf("Expected 20 bytes, got %d", len(bytes))
	}
}

func TestTOTPGenerationAndValidation(t *testing.T) {
	secret, err := GenerateMFASecret()
	if err != nil {
		t.Fatalf("GenerateMFASecret failed: %v", err)
	}

	// Test generation
	now := time.Now()
	code, err := GenerateTOTP(secret, now)
	if err != nil {
		t.Fatalf("GenerateTOTP failed: %v", err)
	}
	if len(code) != 6 {
		t.Errorf("Expected 6-digit code, got %s", code)
	}

	// Validate TOTP with current code
	if !ValidateTOTP(secret, code) {
		t.Errorf("Expected current TOTP code to validate successfully")
	}

	// Validate TOTP with code from 25 seconds ago (within current or -30s window)
	codePast, _ := GenerateTOTP(secret, now.Add(-25*time.Second))
	if !ValidateTOTP(secret, codePast) {
		t.Errorf("Expected code from 25s ago to validate within drift window")
	}

	// Validate TOTP with invalid codes
	if ValidateTOTP(secret, "000000") && code != "000000" && codePast != "000000" {
		t.Errorf("Expected bogus code to fail")
	}
	if ValidateTOTP(secret, "short") {
		t.Errorf("Expected short code to fail")
	}
	if ValidateTOTP("INVALIDBASE32!!!", code) {
		t.Errorf("Expected invalid secret to fail")
	}
}

func TestGetTOTPAuthURL(t *testing.T) {
	url := GetTOTPAuthURL("Lodestone Server", "admin", "JBSWY3DPEHPK3PXP")
	if !strings.HasPrefix(url, "otpauth://totp/") {
		t.Errorf("Expected otpauth scheme, got %s", url)
	}
	if !strings.Contains(url, "secret=JBSWY3DPEHPK3PXP") {
		t.Errorf("Expected secret query parameter in %s", url)
	}
	if !strings.Contains(url, "issuer=Lodestone+Server") {
		t.Errorf("Expected issuer query parameter in %s", url)
	}

	// Fallback to default issuer
	emptyIssuer := GetTOTPAuthURL("", "player1", "JBSWY3DPEHPK3PXP")
	if !strings.Contains(emptyIssuer, "Lodestone") {
		t.Errorf("Expected default Lodestone issuer in %s", emptyIssuer)
	}
}

func TestBackupCodesLifecycle(t *testing.T) {
	plainCodes, hashesJSON, err := GenerateBackupCodes(8)
	if err != nil {
		t.Fatalf("GenerateBackupCodes failed: %v", err)
	}
	if len(plainCodes) != 8 {
		t.Fatalf("Expected 8 plain codes, got %d", len(plainCodes))
	}

	var hashes []string
	if err := json.Unmarshal([]byte(hashesJSON), &hashes); err != nil {
		t.Fatalf("Failed to parse hashes JSON: %v", err)
	}
	if len(hashes) != 8 {
		t.Fatalf("Expected 8 hashes, got %d", len(hashes))
	}

	// Validate and consume the first code
	firstCode := plainCodes[0]
	valid, updatedJSON, err := ValidateAndConsumeBackupCode(firstCode, hashesJSON)
	if err != nil || !valid {
		t.Fatalf("Failed to validate first backup code: valid=%v, err=%v", valid, err)
	}

	var updatedHashes []string
	_ = json.Unmarshal([]byte(updatedJSON), &updatedHashes)
	if len(updatedHashes) != 7 {
		t.Errorf("Expected 7 remaining backup codes, got %d", len(updatedHashes))
	}

	// Attempt to reuse the consumed code
	reuseValid, _, err := ValidateAndConsumeBackupCode(firstCode, updatedJSON)
	if err != nil {
		t.Fatalf("Unexpected error on reuse check: %v", err)
	}
	if reuseValid {
		t.Errorf("Consumed backup code was accepted again!")
	}

	// Test invalid code
	badValid, _, err := ValidateAndConsumeBackupCode("WRONG-CODE", updatedJSON)
	if err != nil || badValid {
		t.Errorf("Expected invalid code to fail validation")
	}

	// Test empty or corrupt JSON
	_, _, err = ValidateAndConsumeBackupCode("1234", "{corrupt")
	if err == nil {
		t.Errorf("Expected unmarshal error on corrupted JSON")
	}

	emptyValid, _, _ := ValidateAndConsumeBackupCode("", updatedJSON)
	if emptyValid {
		t.Errorf("Expected empty code to fail")
	}
}

func TestRefreshTokenLifecycle(t *testing.T) {
	raw, hash, err := GenerateRefreshToken()
	if err != nil {
		t.Fatalf("GenerateRefreshToken failed: %v", err)
	}
	if len(raw) != 64 { // 32 bytes hex = 64 characters
		t.Errorf("Expected 64 char hex string, got %d", len(raw))
	}
	if HashRefreshToken(raw) != hash {
		t.Errorf("HashRefreshToken did not match generated hash")
	}
}

func TestMFAPendingTokenAndMiddlewareRejection(t *testing.T) {
	// Generate challenge token
	token, err := GenerateMFAPendingToken(42, "admin_pending")
	if err != nil {
		t.Fatalf("GenerateMFAPendingToken failed: %v", err)
	}

	// ValidateMFAPendingToken succeeds
	claims, err := ValidateMFAPendingToken(token)
	if err != nil {
		t.Fatalf("ValidateMFAPendingToken failed: %v", err)
	}
	if claims.UserID != 42 || claims.Username != "admin_pending" || !claims.MFAPending {
		t.Errorf("Unexpected claims: %+v", claims)
	}

	// Normal access token should fail ValidateMFAPendingToken
	normToken, _ := GenerateToken("admin_norm", "admin")
	_, err = ValidateMFAPendingToken(normToken)
	if err == nil {
		t.Errorf("Expected normal token to be rejected by ValidateMFAPendingToken")
	}

	// AuthMiddleware must reject MFAPending token with 401
	handlerCalled := false
	dummyHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	AuthMiddleware(dummyHandler).ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401 on MFAPending token, got %d", w.Code)
	}
	if handlerCalled {
		t.Errorf("Downstream handler should not have been called with pending MFA token")
	}
}
