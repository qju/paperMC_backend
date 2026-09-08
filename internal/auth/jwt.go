package auth

import (
	"crypto/rand"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var (
	jwtSecretMu sync.RWMutex
	jwtSecret   = initSecret()
)

func initSecret() []byte {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		randomKey := make([]byte, 32)
		if _, err := rand.Read(randomKey); err != nil {
			panic("failed to generate secure ephemeral JWT secret: " + err.Error())
		}
		fmt.Println("[WARN] JWT_SECRET not configured. Generated ephemeral cryptographically secure key for this session.")
		return randomKey
	}
	if len(secret) < 32 {
		fmt.Println("[WARN] JWT_SECRET is shorter than 32 characters. Recommended 256-bit secret for production.")
	}
	return []byte(secret)
}

// SetJWTSecret allows explicit runtime or test configuration of the JWT signing key.
func SetJWTSecret(secret []byte) {
	jwtSecretMu.Lock()
	defer jwtSecretMu.Unlock()
	jwtSecret = secret
}

// GetJWTSecret retrieves a copy of the current JWT secret bytes.
func GetJWTSecret() []byte {
	jwtSecretMu.RLock()
	defer jwtSecretMu.RUnlock()
	cp := make([]byte, len(jwtSecret))
	copy(cp, jwtSecret)
	return cp
}

type Claims struct {
	Username   string `json:"username"`
	Role       string `json:"role"`
	UserID     int    `json:"user_id,omitempty"`
	SessionID  string `json:"session_id,omitempty"`
	MFAPending bool   `json:"mfa_pending,omitempty"`
	jwt.RegisteredClaims
}

func GenerateToken(username, role string) (string, error) {
	return GenerateSessionToken(0, username, role, "", 24*time.Hour)
}

func GenerateSessionToken(userID int, username, role, sessionID string, duration time.Duration) (string, error) {
	if duration <= 0 {
		duration = 24 * time.Hour
	}
	expiration := time.Now().Add(duration)
	claims := &Claims{
		Username:   username,
		Role:       role,
		UserID:     userID,
		SessionID:  sessionID,
		MFAPending: false,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiration),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(GetJWTSecret())
}
