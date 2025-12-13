// Package auth implements JSON Web Token (JWT) authentication middleware and utilities.
//
// It provides functionality to:
//   - Validate JWT tokens signed with HMAC-SHA256.
//   - Extract user claims (Username, Role) from tokens.
//   - Protect HTTP handlers using AuthMiddleware, which enforces the presence of a valid "Authorization" header.
//   - Inject validated claims into the HTTP request context for use by downstream handlers.
package auth

import (
	"context"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

type contextKey string

const UserKey contextKey = "user"

func myKeyFunc(token *jwt.Token) (any, error) {
	if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
		return nil, jwt.ErrSignatureInvalid
	}
	return jwtSecret, nil
}

func ValidateToken(tokenString string) (*Claims, error) {
	// Parse token and Validate it - protects from algo vun.
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, myKeyFunc)
	if err != nil {
		return nil, err
	}

	// Check the claims for the expiration and validity
	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}

	return nil, jwt.ErrInvalidKey
}

func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokenString := ""

		// 1. Try extracting from Header
		authHeader := r.Header.Get("Authorization")
		if strings.HasPrefix(authHeader, "Bearer ") {
			tokenString = strings.TrimPrefix(tokenString, "Bearer ")
		}

		// 2. Fallback if header didn't provided token fallback to URL Query
		if tokenString == "" {
			tokenString = r.URL.Query().Get("token")
		}

		// 3. If BOTH are empty, then fail
		if tokenString == "" {
			http.Error(w, "Unauthorized: No token provided", http.StatusUnauthorized)
			return
		}

		// 4. Validate
		claims, err := ValidateToken(tokenString)
		if err != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), UserKey, claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
