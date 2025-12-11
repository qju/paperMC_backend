package auth

import (
	"fmt"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// ToDo: add secret loaded from ENV
var jwtSecret = getSecret()

func getSecret() []byte {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		fmt.Println("WARNING: JWT_SECRET not set, using default for dev")
		return []byte("unsafe-dev-secret-do-not-use-in-prod")
	}
	return []byte(secret)
}

type Claims struct {
	Username string `json:"username"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

func GenerateToken(username, role string) (string, error) {
	expiration := time.Now().Add(24 * time.Hour)
	claims := &Claims{
		Username: username,
		Role:     role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiration),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtSecret)
}
