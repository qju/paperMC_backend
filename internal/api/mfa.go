package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"paperMC_backend/internal/auth"
	"paperMC_backend/internal/database"
)

const RefreshCookieName = "lodestone_refresh"

// Set refresh token in HttpOnly cookie
func setRefreshTokenCookie(w http.ResponseWriter, r *http.Request, token string, expiresAt time.Time) {
	isSecure := r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https"
	http.SetCookie(w, &http.Cookie{
		Name:     RefreshCookieName,
		Value:    token,
		Path:     "/api/auth",
		Expires:  expiresAt,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		Secure:   isSecure,
	})
}

// Clear refresh token cookie
func clearRefreshTokenCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     RefreshCookieName,
		Value:    "",
		Path:     "/api/auth",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	})
}

// Extract refresh token from cookie or request body
func getRefreshTokenFromRequest(r *http.Request) string {
	if cookie, err := r.Cookie(RefreshCookieName); err == nil && strings.TrimSpace(cookie.Value) != "" {
		return strings.TrimSpace(cookie.Value)
	}

	authHeader := r.Header.Get("X-Refresh-Token")
	if strings.TrimSpace(authHeader) != "" {
		return strings.TrimSpace(authHeader)
	}

	return ""
}

func generateSessionID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return fmt.Sprintf("sess-%s", hex.EncodeToString(b))
}

type TwoFASetupResponse struct {
	Secret      string   `json:"secret"`
	OTPAuthURL  string   `json:"otpauth_url"`
	BackupCodes []string `json:"backup_codes"`
}

type TwoFAVerifyRequest struct {
	Code     string `json:"code"`
	Password string `json:"password,omitempty"`
}

type TwoFALoginRequest struct {
	MFAToken string `json:"mfa_token"`
	Code     string `json:"code"`
}

type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token,omitempty"`
}

// Handle2FASetup generates a new TOTP secret and 8 backup recovery codes for the authenticated user.
func (h *Handler) Handle2FASetup(w http.ResponseWriter, r *http.Request) {
	username := auth.GetUsername(r)
	if username == "" {
		respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	user, err := h.store.GetUser(username)
	if err != nil || user == nil {
		respondWithError(w, http.StatusNotFound, "User not found")
		return
	}

	secret, err := auth.GenerateMFASecret()
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to generate MFA secret")
		return
	}

	plainCodes, hashesJSON, err := auth.GenerateBackupCodes(8)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to generate recovery backup codes")
		return
	}

	mfa := &database.UserMFA{
		UserID:      user.ID,
		Secret:      secret,
		BackupCodes: hashesJSON,
		Enabled:     false,
	}

	if err := h.store.UpsertUserMFA(mfa); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to save MFA configuration")
		return
	}

	authURL := auth.GetTOTPAuthURL("Lodestone", user.Username, secret)
	h.recordAuditWithUser(username, r, "auth.2fa_setup_init", http.StatusOK, "Initiated 2FA configuration")

	respondWithJSON(w, http.StatusOK, TwoFASetupResponse{
		Secret:      secret,
		OTPAuthURL:  authURL,
		BackupCodes: plainCodes,
	})
}

// Handle2FAEnable verifies the first TOTP code and marks 2FA as active.
func (h *Handler) Handle2FAEnable(w http.ResponseWriter, r *http.Request) {
	username := auth.GetUsername(r)
	if username == "" {
		respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	user, err := h.store.GetUser(username)
	if err != nil || user == nil {
		respondWithError(w, http.StatusNotFound, "User not found")
		return
	}

	var req TwoFAVerifyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Code) == "" {
		respondWithError(w, http.StatusBadRequest, "Verification code is required")
		return
	}

	mfa, err := h.store.GetUserMFA(user.ID)
	if err != nil || mfa == nil {
		respondWithError(w, http.StatusBadRequest, "Two-factor authentication has not been initiated. Please run setup first.")
		return
	}

	if !auth.ValidateTOTP(mfa.Secret, req.Code) {
		h.recordAuditWithUser(username, r, "auth.2fa_enable_failed", http.StatusBadRequest, "Invalid confirmation passcode")
		respondWithError(w, http.StatusBadRequest, "Invalid verification code")
		return
	}

	mfa.Enabled = true
	if err := h.store.UpsertUserMFA(mfa); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to activate 2FA")
		return
	}

	h.recordAuditWithUser(username, r, "auth.2fa_enabled", http.StatusOK, "Two-factor authentication activated")
	respondWithJSON(w, http.StatusOK, map[string]string{"message": "Two-factor authentication successfully enabled"})
}

// Handle2FADisable allows disabling 2FA using a valid code or current account password.
func (h *Handler) Handle2FADisable(w http.ResponseWriter, r *http.Request) {
	username := auth.GetUsername(r)
	if username == "" {
		respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	user, err := h.store.GetUser(username)
	if err != nil || user == nil {
		respondWithError(w, http.StatusNotFound, "User not found")
		return
	}

	mfa, err := h.store.GetUserMFA(user.ID)
	if err != nil || mfa == nil || !mfa.Enabled {
		respondWithError(w, http.StatusBadRequest, "Two-factor authentication is not active")
		return
	}

	var req TwoFAVerifyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	authorized := false
	if req.Password != "" && auth.CheckPasswordHash(req.Password, user.Password) {
		authorized = true
	} else if req.Code != "" {
		if auth.ValidateTOTP(mfa.Secret, req.Code) {
			authorized = true
		} else {
			validBackup, _, _ := auth.ValidateAndConsumeBackupCode(req.Code, mfa.BackupCodes)
			if validBackup {
				authorized = true
			}
		}
	}

	if !authorized {
		h.recordAuditWithUser(username, r, "auth.2fa_disable_failed", http.StatusUnauthorized, "Invalid credentials to disable 2FA")
		respondWithError(w, http.StatusUnauthorized, "Invalid password or verification code")
		return
	}

	if err := h.store.DeleteUserMFA(user.ID); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to disable 2FA")
		return
	}

	h.recordAuditWithUser(username, r, "auth.2fa_disabled", http.StatusOK, "Two-factor authentication disabled")
	respondWithJSON(w, http.StatusOK, map[string]string{"message": "Two-factor authentication disabled"})
}

// Handle2FAStatus returns whether 2FA is currently active for the authenticated user.
func (h *Handler) Handle2FAStatus(w http.ResponseWriter, r *http.Request) {
	username := auth.GetUsername(r)
	if username == "" {
		respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	user, err := h.store.GetUser(username)
	if err != nil || user == nil {
		respondWithError(w, http.StatusNotFound, "User not found")
		return
	}

	mfa, err := h.store.GetUserMFA(user.ID)
	enabled := mfa != nil && mfa.Enabled

	respondWithJSON(w, http.StatusOK, map[string]bool{"enabled": enabled})
}

// Handle2FAVerifyLogin finalizes authentication by validating TOTP or recovery backup code.
func (h *Handler) Handle2FAVerifyLogin(w http.ResponseWriter, r *http.Request) {
	ip := getClientIP(r)

	var req TwoFALoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	if strings.TrimSpace(req.MFAToken) == "" || strings.TrimSpace(req.Code) == "" {
		respondWithError(w, http.StatusBadRequest, "MFA token and verification code are required")
		return
	}

	claims, err := auth.ValidateMFAPendingToken(req.MFAToken)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "MFA session expired or invalid")
		return
	}

	user, err := h.store.GetUserByID(claims.UserID)
	if err != nil || user == nil {
		respondWithError(w, http.StatusUnauthorized, "User not found")
		return
	}

	mfa, err := h.store.GetUserMFA(user.ID)
	if err != nil || mfa == nil || !mfa.Enabled {
		respondWithError(w, http.StatusBadRequest, "MFA is not enabled for this user")
		return
	}

	// Validate TOTP or backup code
	valid := auth.ValidateTOTP(mfa.Secret, req.Code)
	if !valid {
		var consumeErr error
		var updatedCodes string
		valid, updatedCodes, consumeErr = auth.ValidateAndConsumeBackupCode(req.Code, mfa.BackupCodes)
		if valid && consumeErr == nil {
			mfa.BackupCodes = updatedCodes
			_ = h.store.UpsertUserMFA(mfa)
		}
	}

	if !valid {
		h.recordAuditWithUser(user.Username, r, "auth.login_2fa_failed", http.StatusUnauthorized, "Invalid two-factor code")
		respondWithError(w, http.StatusUnauthorized, "Invalid two-factor authentication code")
		return
	}

	// Create user session and issue tokens
	rawRefresh, tokenHash, err := auth.GenerateRefreshToken()
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to issue session")
		return
	}

	sessionID := generateSessionID()
	expiresAt := time.Now().Add(7 * 24 * time.Hour)
	sess := &database.UserSession{
		ID:               sessionID,
		UserID:           user.ID,
		RefreshTokenHash: tokenHash,
		UserAgent:        r.UserAgent(),
		IPAddress:        ip,
		ExpiresAt:        expiresAt,
		Revoked:          false,
	}

	if err := h.store.CreateSession(sess); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to record session")
		return
	}

	accessToken, err := auth.GenerateSessionToken(user.ID, user.Username, user.Role, sessionID, 15*time.Minute)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to generate access token")
		return
	}

	setRefreshTokenCookie(w, r, rawRefresh, expiresAt)
	h.recordAuditWithUser(user.Username, r, "auth.login_2fa_success", http.StatusOK, "Successful 2FA authentication")

	respondWithJSON(w, http.StatusOK, LoginResponse{
		Token:        accessToken,
		RefreshToken: rawRefresh,
		User:         user,
	})
}

// HandleRefreshToken rotates refresh tokens and issues a new access token.
func (h *Handler) HandleRefreshToken(w http.ResponseWriter, r *http.Request) {
	refreshToken := getRefreshTokenFromRequest(r)

	// Fallback to reading from JSON body
	if refreshToken == "" && r.Body != nil {
		var req RefreshTokenRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err == nil {
			refreshToken = strings.TrimSpace(req.RefreshToken)
		}
	}

	if refreshToken == "" {
		clearRefreshTokenCookie(w)
		respondWithError(w, http.StatusUnauthorized, "Missing refresh token")
		return
	}

	tokenHash := auth.HashRefreshToken(refreshToken)
	session, err := h.store.GetSessionByTokenHash(tokenHash)
	if err != nil || session == nil || session.Revoked || time.Now().After(session.ExpiresAt) {
		clearRefreshTokenCookie(w)
		respondWithError(w, http.StatusUnauthorized, "Session expired or revoked")
		return
	}

	user, err := h.store.GetUserByID(session.UserID)
	if err != nil || user == nil {
		clearRefreshTokenCookie(w)
		respondWithError(w, http.StatusUnauthorized, "User associated with session not found")
		return
	}

	// Rotate refresh token
	newRawRefresh, newTokenHash, err := auth.GenerateRefreshToken()
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to generate rotated token")
		return
	}

	newExpiry := time.Now().Add(7 * 24 * time.Hour)
	if err := h.store.UpdateSessionTokenHash(session.ID, newTokenHash, newExpiry); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to update session")
		return
	}

	newAccessToken, err := auth.GenerateSessionToken(user.ID, user.Username, user.Role, session.ID, 15*time.Minute)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to generate new access token")
		return
	}

	setRefreshTokenCookie(w, r, newRawRefresh, newExpiry)
	respondWithJSON(w, http.StatusOK, map[string]interface{}{
		"token":         newAccessToken,
		"refresh_token": newRawRefresh,
	})
}

// HandleLogout revokes the current session and clears refresh cookies.
func (h *Handler) HandleLogout(w http.ResponseWriter, r *http.Request) {
	// Revoke by claims session_id if authenticated
	if claims := auth.GetClaims(r); claims != nil && claims.SessionID != "" {
		_ = h.store.RevokeSession(claims.SessionID)
		h.recordAuditWithUser(claims.Username, r, "auth.logout", http.StatusOK, "User logged out")
	}

	// Revoke by refresh token cookie if present
	if cookie, err := r.Cookie(RefreshCookieName); err == nil && cookie.Value != "" {
		tokenHash := auth.HashRefreshToken(cookie.Value)
		if sess, _ := h.store.GetSessionByTokenHash(tokenHash); sess != nil {
			_ = h.store.RevokeSession(sess.ID)
		}
	}

	clearRefreshTokenCookie(w)
	respondWithJSON(w, http.StatusOK, map[string]string{"message": "Logged out successfully"})
}
