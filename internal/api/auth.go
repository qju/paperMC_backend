package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"paperMC_backend/internal/auth"
)

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type LoginResponse struct {
	Token string `json:"token"`
}

// dummyBcryptHash is a valid bcrypt hash format used to equalize execution timing on failed lookups
const dummyBcryptHash = "$2a$10$abcdefghijklmnopqrstuuABCDEFGHIJKLMNOPQRSTUU1234567890"

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	ip := getClientIP(r)

	// 1. Enforce rate limiting
	if h.loginLimiter != nil {
		if allowed, retryAfter := h.loginLimiter.Allow(ip); !allowed {
			w.Header().Set("Retry-After", strconv.Itoa(int(retryAfter.Seconds())))
			h.recordAuditWithUser("anonymous", r, "auth.login_blocked", http.StatusTooManyRequests, "Rate limit exceeded")
			respondWithError(w, http.StatusTooManyRequests, "Too many failed login attempts. Please try again later.")
			return
		}
	}

	var req = LoginRequest{}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	username := strings.TrimSpace(req.Username)
	password := req.Password

	if username == "" || password == "" {
		respondWithError(w, http.StatusBadRequest, "Username and password are required")
		return
	}

	if h.store == nil {
		respondWithError(w, http.StatusInternalServerError, "database store is not available")
		return
	}

	user, err := h.store.GetUser(username)
	if err != nil || user == nil {
		// Prevent user enumeration and timing side-channel attacks
		_ = auth.CheckPasswordHash(password, dummyBcryptHash)
		if h.loginLimiter != nil {
			h.loginLimiter.RecordFailure(ip)
		}
		h.recordAuditWithUser(username, r, "auth.login_failed", http.StatusUnauthorized, "Invalid credentials")
		respondWithError(w, http.StatusUnauthorized, "Invalid username or password")
		return
	}

	if !auth.CheckPasswordHash(password, user.Password) {
		if h.loginLimiter != nil {
			h.loginLimiter.RecordFailure(ip)
		}
		h.recordAuditWithUser(username, r, "auth.login_failed", http.StatusUnauthorized, "Invalid credentials")
		respondWithError(w, http.StatusUnauthorized, "Invalid username or password")
		return
	}

	// Successful login: reset rate limit failure count
	if h.loginLimiter != nil {
		h.loginLimiter.Reset(ip)
	}

	token, err := auth.GenerateToken(user.Username, user.Role)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to generate authentication token")
		return
	}

	h.recordAuditWithUser(user.Username, r, "auth.login", http.StatusOK, "Successful authentication")
	respondWithJSON(w, http.StatusOK, LoginResponse{Token: token})
}
