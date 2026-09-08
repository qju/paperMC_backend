package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base32"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// GenerateMFASecret creates a cryptographically secure 160-bit Base32 secret for TOTP.
func GenerateMFASecret() (string, error) {
	buf := make([]byte, 20)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("failed to generate random secret: %w", err)
	}
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(buf), nil
}

// decodeBase32Secret decodes a base32 string with or without padding, case-insensitively.
func decodeBase32Secret(secret string) ([]byte, error) {
	clean := strings.ToUpper(strings.ReplaceAll(secret, " ", ""))
	// Try without padding
	data, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(clean)
	if err == nil {
		return data, nil
	}
	// Try with standard padding
	return base32.StdEncoding.DecodeString(clean)
}

// GenerateTOTP generates a 6-digit TOTP code for the given secret and timestamp according to RFC 6238.
func GenerateTOTP(secret string, t time.Time) (string, error) {
	key, err := decodeBase32Secret(secret)
	if err != nil {
		return "", fmt.Errorf("invalid base32 secret: %w", err)
	}

	counter := uint64(t.Unix() / 30)
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, counter)

	mac := hmac.New(sha1.New, key)
	mac.Write(buf)
	h := mac.Sum(nil)

	offset := h[len(h)-1] & 0x0f
	binaryVal := (binary.BigEndian.Uint32(h[offset:offset+4]) & 0x7fffffff) % 1000000

	return fmt.Sprintf("%06d", binaryVal), nil
}

// ValidateTOTP verifies a 6-digit code allowing a +/- 1 step (30s) drift window.
func ValidateTOTP(secret, passcode string) bool {
	cleanCode := strings.TrimSpace(passcode)
	if len(cleanCode) != 6 {
		return false
	}

	now := time.Now()
	// Test current, previous (-30s), and next (+30s) windows to accommodate clock skew
	windows := []time.Time{
		now,
		now.Add(-30 * time.Second),
		now.Add(30 * time.Second),
	}

	for _, win := range windows {
		expected, err := GenerateTOTP(secret, win)
		if err == nil && hmac.Equal([]byte(expected), []byte(cleanCode)) {
			return true
		}
	}
	return false
}

// GetTOTPAuthURL constructs the otpauth:// URI for authenticator applications.
func GetTOTPAuthURL(issuer, accountName, secret string) string {
	cleanIssuer := strings.TrimSpace(issuer)
	if cleanIssuer == "" {
		cleanIssuer = "Lodestone"
	}
	label := fmt.Sprintf("%s:%s", cleanIssuer, strings.TrimSpace(accountName))
	return fmt.Sprintf("otpauth://totp/%s?secret=%s&issuer=%s&algorithm=SHA1&digits=6&period=30",
		url.PathEscape(label),
		url.QueryEscape(secret),
		url.QueryEscape(cleanIssuer),
	)
}

// GenerateBackupCodes produces count human-readable backup codes and their SHA-256 JSON representation.
func GenerateBackupCodes(count int) (plainCodes []string, hashedCodesJSON string, err error) {
	if count <= 0 {
		count = 8
	}

	plainCodes = make([]string, count)
	hashes := make([]string, count)

	for i := 0; i < count; i++ {
		b := make([]byte, 4)
		if _, err := rand.Read(b); err != nil {
			return nil, "", fmt.Errorf("failed to generate backup code: %w", err)
		}
		rawHex := strings.ToUpper(hex.EncodeToString(b)) // 8 characters
		formatted := fmt.Sprintf("%s-%s", rawHex[:4], rawHex[4:])
		plainCodes[i] = formatted

		// Hash normalized code without hyphens
		hash := HashBackupCode(rawHex)
		hashes[i] = hash
	}

	jsonBytes, err := json.Marshal(hashes)
	if err != nil {
		return nil, "", err
	}
	return plainCodes, string(jsonBytes), nil
}

// HashBackupCode normalizes a code and computes its SHA-256 hex digest.
func HashBackupCode(code string) string {
	normalized := strings.ToUpper(strings.ReplaceAll(strings.ReplaceAll(code, "-", ""), " ", ""))
	sum := sha256.Sum256([]byte(normalized))
	return hex.EncodeToString(sum[:])
}

// ValidateAndConsumeBackupCode verifies if rawCode matches any stored backup code hash.
// If valid, the code is removed from the JSON list and the new JSON is returned.
func ValidateAndConsumeBackupCode(rawCode string, backupCodesJSON string) (bool, string, error) {
	if strings.TrimSpace(rawCode) == "" || strings.TrimSpace(backupCodesJSON) == "" {
		return false, backupCodesJSON, nil
	}

	var hashes []string
	if err := json.Unmarshal([]byte(backupCodesJSON), &hashes); err != nil {
		return false, backupCodesJSON, fmt.Errorf("malformed backup codes store: %w", err)
	}

	targetHash := HashBackupCode(rawCode)
	for i, h := range hashes {
		if h == targetHash {
			// Remove consumed code
			updatedHashes := append(hashes[:i], hashes[i+1:]...)
			updatedJSON, err := json.Marshal(updatedHashes)
			if err != nil {
				return false, backupCodesJSON, err
			}
			return true, string(updatedJSON), nil
		}
	}

	return false, backupCodesJSON, nil
}

// GenerateRefreshToken creates a random 256-bit token and its SHA-256 digest for database lookup.
func GenerateRefreshToken() (rawToken string, tokenHash string, err error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", "", fmt.Errorf("failed to generate random refresh token: %w", err)
	}
	rawToken = hex.EncodeToString(b)
	tokenHash = HashRefreshToken(rawToken)
	return rawToken, tokenHash, nil
}

// HashRefreshToken calculates the SHA-256 hex string of a refresh token.
func HashRefreshToken(rawToken string) string {
	sum := sha256.Sum256([]byte(rawToken))
	return hex.EncodeToString(sum[:])
}

// GenerateMFAPendingToken generates a short-lived (5m) challenge token required to verify MFA.
func GenerateMFAPendingToken(userID int, username string) (string, error) {
	expiration := time.Now().Add(5 * time.Minute)
	claims := &Claims{
		Username:   username,
		UserID:     userID,
		MFAPending: true,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiration),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(GetJWTSecret())
}

// ValidateMFAPendingToken verifies that a token is a valid, unexpired MFA challenge token.
func ValidateMFAPendingToken(tokenString string) (*Claims, error) {
	claims, err := ValidateToken(tokenString)
	if err != nil {
		return nil, err
	}
	if !claims.MFAPending {
		return nil, errors.New("token is not a valid MFA challenge token")
	}
	return claims, nil
}
