package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"paperMC_backend/internal/auth"
	"paperMC_backend/internal/database"
)

func setupTestStoreAndUser(t *testing.T, username, password string) (*Handler, database.Store, *database.User) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test_api_mfa.db")
	store, err := database.NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to init SQLite store: %v", err)
	}

	hashedPassword, _ := auth.HashPassword(password)
	user := &database.User{
		Username: username,
		Password: hashedPassword,
		Role:     "admin",
	}
	if err := store.CreateUser(user); err != nil {
		t.Fatalf("Failed to seed user: %v", err)
	}

	fetchedUser, err := store.GetUser(username)
	if err != nil {
		t.Fatalf("Failed to get user: %v", err)
	}

	handler := NewServerHandler(nil, store)
	return handler, store, fetchedUser
}

func Test2FASetupAndStatus(t *testing.T) {
	handler, store, user := setupTestStoreAndUser(t, "mfa_user", "ValidPassword123!")
	defer store.Close()

	// 1. Initial status should be disabled
	reqStatus := httptest.NewRequest(http.MethodGet, "/api/auth/2fa/status", nil)
	ctx := context.WithValue(reqStatus.Context(), auth.UserKey, &auth.Claims{Username: user.Username, UserID: user.ID})
	wStatus := httptest.NewRecorder()
	handler.Handle2FAStatus(wStatus, reqStatus.WithContext(ctx))
	if wStatus.Code != http.StatusOK {
		t.Fatalf("Expected 200 for status, got %d", wStatus.Code)
	}
	var statusResp map[string]bool
	_ = json.NewDecoder(wStatus.Body).Decode(&statusResp)
	if statusResp["enabled"] {
		t.Errorf("Expected enabled=false initially")
	}

	// 2. Setup 2FA
	reqSetup := httptest.NewRequest(http.MethodPost, "/api/auth/2fa/setup", nil)
	wSetup := httptest.NewRecorder()
	handler.Handle2FASetup(wSetup, reqSetup.WithContext(ctx))
	if wSetup.Code != http.StatusOK {
		t.Fatalf("Expected 200 for setup, got %d: %s", wSetup.Code, wSetup.Body.String())
	}

	var setupResp TwoFASetupResponse
	if err := json.NewDecoder(wSetup.Body).Decode(&setupResp); err != nil {
		t.Fatalf("Failed to decode setup response: %v", err)
	}
	if setupResp.Secret == "" || len(setupResp.BackupCodes) != 8 || setupResp.OTPAuthURL == "" {
		t.Errorf("Incomplete setup response: %+v", setupResp)
	}

	// MFA record in DB should still be enabled=false
	dbMFA, err := store.GetUserMFA(user.ID)
	if err != nil || dbMFA == nil || dbMFA.Enabled {
		t.Errorf("Expected MFA in DB to exist but enabled=false, got %+v", dbMFA)
	}

	// Setup without auth
	wNoAuth := httptest.NewRecorder()
	handler.Handle2FASetup(wNoAuth, httptest.NewRequest(http.MethodPost, "/api/auth/2fa/setup", nil))
	if wNoAuth.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401 without user context, got %d", wNoAuth.Code)
	}
}

func Test2FAEnableAndDisable(t *testing.T) {
	handler, store, user := setupTestStoreAndUser(t, "enable_user", "P@ssword123")
	defer store.Close()

	ctx := context.WithValue(context.Background(), auth.UserKey, &auth.Claims{Username: user.Username, UserID: user.ID})

	// Setup first
	reqSetup := httptest.NewRequest(http.MethodPost, "/api/auth/2fa/setup", nil).WithContext(ctx)
	wSetup := httptest.NewRecorder()
	handler.Handle2FASetup(wSetup, reqSetup)

	var setupResp TwoFASetupResponse
	_ = json.NewDecoder(wSetup.Body).Decode(&setupResp)

	// 1. Try to enable with invalid code
	badBody, _ := json.Marshal(TwoFAVerifyRequest{Code: "000000"})
	reqBad := httptest.NewRequest(http.MethodPost, "/api/auth/2fa/enable", bytes.NewReader(badBody)).WithContext(ctx)
	wBad := httptest.NewRecorder()
	handler.Handle2FAEnable(wBad, reqBad)
	if wBad.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 for bad code, got %d", wBad.Code)
	}

	// 2. Enable with valid code
	validCode, _ := auth.GenerateTOTP(setupResp.Secret, time.Now())
	goodBody, _ := json.Marshal(TwoFAVerifyRequest{Code: validCode})
	reqGood := httptest.NewRequest(http.MethodPost, "/api/auth/2fa/enable", bytes.NewReader(goodBody)).WithContext(ctx)
	wGood := httptest.NewRecorder()
	handler.Handle2FAEnable(wGood, reqGood)
	if wGood.Code != http.StatusOK {
		t.Fatalf("Expected 200 on valid code, got %d: %s", wGood.Code, wGood.Body.String())
	}

	// Verify status now true
	reqStatus := httptest.NewRequest(http.MethodGet, "/api/auth/2fa/status", nil).WithContext(ctx)
	wStatus := httptest.NewRecorder()
	handler.Handle2FAStatus(wStatus, reqStatus)
	var statusResp map[string]bool
	_ = json.NewDecoder(wStatus.Body).Decode(&statusResp)
	if !statusResp["enabled"] {
		t.Errorf("Expected 2FA to be enabled")
	}

	// 3. Disable with password
	disBody, _ := json.Marshal(TwoFAVerifyRequest{Password: "P@ssword123"})
	reqDis := httptest.NewRequest(http.MethodPost, "/api/auth/2fa/disable", bytes.NewReader(disBody)).WithContext(ctx)
	wDis := httptest.NewRecorder()
	handler.Handle2FADisable(wDis, reqDis)
	if wDis.Code != http.StatusOK {
		t.Fatalf("Expected 200 disabling 2FA, got %d: %s", wDis.Code, wDis.Body.String())
	}

	// Status should be disabled
	wStatusAfter := httptest.NewRecorder()
	handler.Handle2FAStatus(wStatusAfter, reqStatus)
	var statusAfter map[string]bool
	_ = json.NewDecoder(wStatusAfter.Body).Decode(&statusAfter)
	if statusAfter["enabled"] {
		t.Errorf("Expected 2FA to be disabled after deletion")
	}
}

func Test2FALoginFlowAndBackupCodes(t *testing.T) {
	handler, store, user := setupTestStoreAndUser(t, "mfa_login_user", "SecurePass123!")
	defer store.Close()

	ctx := context.WithValue(context.Background(), auth.UserKey, &auth.Claims{Username: user.Username, UserID: user.ID})

	// Setup and enable 2FA
	wSetup := httptest.NewRecorder()
	handler.Handle2FASetup(wSetup, httptest.NewRequest(http.MethodPost, "/api/auth/2fa/setup", nil).WithContext(ctx))
	var setupResp TwoFASetupResponse
	_ = json.NewDecoder(wSetup.Body).Decode(&setupResp)

	code, _ := auth.GenerateTOTP(setupResp.Secret, time.Now())
	enableBody, _ := json.Marshal(TwoFAVerifyRequest{Code: code})
	wEnable := httptest.NewRecorder()
	handler.Handle2FAEnable(wEnable, httptest.NewRequest(http.MethodPost, "/api/auth/2fa/enable", bytes.NewReader(enableBody)).WithContext(ctx))
	if wEnable.Code != http.StatusOK {
		t.Fatalf("Failed to enable 2FA: %s", wEnable.Body.String())
	}

	// 1. Initial Login should return mfa_required: true and an mfa_token
	loginBody, _ := json.Marshal(LoginRequest{Username: "mfa_login_user", Password: "SecurePass123!"})
	reqLogin := httptest.NewRequest(http.MethodPost, "/login", bytes.NewReader(loginBody))
	wLogin := httptest.NewRecorder()
	handler.Login(wLogin, reqLogin)
	if wLogin.Code != http.StatusOK {
		t.Fatalf("Expected 200 on login, got %d", wLogin.Code)
	}

	var loginResp LoginResponse
	_ = json.NewDecoder(wLogin.Body).Decode(&loginResp)
	if !loginResp.MFARequired || loginResp.MFAToken == "" || loginResp.Token != "" {
		t.Fatalf("Expected MFA prompt response, got %+v", loginResp)
	}

	// 2. Verify login with bad code
	verifyBadBody, _ := json.Marshal(TwoFALoginRequest{MFAToken: loginResp.MFAToken, Code: "999999"})
	wVerifyBad := httptest.NewRecorder()
	handler.Handle2FAVerifyLogin(wVerifyBad, httptest.NewRequest(http.MethodPost, "/api/auth/2fa/verify-login", bytes.NewReader(verifyBadBody)))
	if wVerifyBad.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401 for bad MFA code, got %d", wVerifyBad.Code)
	}

	// 3. Verify login with valid TOTP code
	totpCode, _ := auth.GenerateTOTP(setupResp.Secret, time.Now())
	verifyGoodBody, _ := json.Marshal(TwoFALoginRequest{MFAToken: loginResp.MFAToken, Code: totpCode})
	wVerifyGood := httptest.NewRecorder()
	handler.Handle2FAVerifyLogin(wVerifyGood, httptest.NewRequest(http.MethodPost, "/api/auth/2fa/verify-login", bytes.NewReader(verifyGoodBody)))
	if wVerifyGood.Code != http.StatusOK {
		t.Fatalf("Expected 200 on successful 2FA verify-login, got %d: %s", wVerifyGood.Code, wVerifyGood.Body.String())
	}

	var verifyResp LoginResponse
	_ = json.NewDecoder(wVerifyGood.Body).Decode(&verifyResp)
	if verifyResp.Token == "" || verifyResp.RefreshToken == "" || verifyResp.User == nil {
		t.Errorf("Expected valid tokens and user in response, got %+v", verifyResp)
	}

	// Refresh token cookie must be present
	cookies := wVerifyGood.Result().Cookies()
	var refreshCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == RefreshCookieName {
			refreshCookie = c
			break
		}
	}
	if refreshCookie == nil || !refreshCookie.HttpOnly {
		t.Errorf("Expected HttpOnly refresh token cookie, got %+v", refreshCookie)
	}

	// 4. Verify login using a Backup Code
	firstBackupCode := setupResp.BackupCodes[0]
	// Re-prompt login to get fresh mfa_token
	wLogin2 := httptest.NewRecorder()
	handler.Login(wLogin2, httptest.NewRequest(http.MethodPost, "/login", bytes.NewReader(loginBody)))
	var loginResp2 LoginResponse
	_ = json.NewDecoder(wLogin2.Body).Decode(&loginResp2)

	verifyBackupBody, _ := json.Marshal(TwoFALoginRequest{MFAToken: loginResp2.MFAToken, Code: firstBackupCode})
	wVerifyBackup := httptest.NewRecorder()
	handler.Handle2FAVerifyLogin(wVerifyBackup, httptest.NewRequest(http.MethodPost, "/api/auth/2fa/verify-login", bytes.NewReader(verifyBackupBody)))
	if wVerifyBackup.Code != http.StatusOK {
		t.Fatalf("Expected 200 with backup code, got %d: %s", wVerifyBackup.Code, wVerifyBackup.Body.String())
	}

	// Reusing the same backup code must fail
	wLogin3 := httptest.NewRecorder()
	handler.Login(wLogin3, httptest.NewRequest(http.MethodPost, "/login", bytes.NewReader(loginBody)))
	var loginResp3 LoginResponse
	_ = json.NewDecoder(wLogin3.Body).Decode(&loginResp3)

	wReuse := httptest.NewRecorder()
	handler.Handle2FAVerifyLogin(wReuse, httptest.NewRequest(http.MethodPost, "/api/auth/2fa/verify-login", bytes.NewReader(verifyBackupBody)))
	if wReuse.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401 when reusing consumed backup code, got %d", wReuse.Code)
	}
}

func TestSessionRefreshAndLogout(t *testing.T) {
	handler, store, _ := setupTestStoreAndUser(t, "sess_user", "Password987#")
	defer store.Close()

	// 1. Standard login (without MFA)
	loginBody, _ := json.Marshal(LoginRequest{Username: "sess_user", Password: "Password987#"})
	wLogin := httptest.NewRecorder()
	handler.Login(wLogin, httptest.NewRequest(http.MethodPost, "/login", bytes.NewReader(loginBody)))
	if wLogin.Code != http.StatusOK {
		t.Fatalf("Login failed: %d", wLogin.Code)
	}

	var loginResp LoginResponse
	_ = json.NewDecoder(wLogin.Body).Decode(&loginResp)
	if loginResp.Token == "" || loginResp.RefreshToken == "" {
		t.Fatalf("Expected access and refresh tokens")
	}

	// 2. Token refresh via JSON body
	refBody, _ := json.Marshal(RefreshTokenRequest{RefreshToken: loginResp.RefreshToken})
	wRef := httptest.NewRecorder()
	handler.HandleRefreshToken(wRef, httptest.NewRequest(http.MethodPost, "/api/auth/refresh", bytes.NewReader(refBody)))
	if wRef.Code != http.StatusOK {
		t.Fatalf("Expected 200 refreshing token, got %d: %s", wRef.Code, wRef.Body.String())
	}

	var refResp map[string]string
	_ = json.NewDecoder(wRef.Body).Decode(&refResp)
	newAccess := refResp["token"]
	newRefresh := refResp["refresh_token"]
	if newAccess == "" || newRefresh == "" || newRefresh == loginResp.RefreshToken {
		t.Errorf("Expected rotated refresh token, got %s vs %s", newRefresh, loginResp.RefreshToken)
	}

	// 3. Old refresh token must now be invalid
	wRefOld := httptest.NewRecorder()
	handler.HandleRefreshToken(wRefOld, httptest.NewRequest(http.MethodPost, "/api/auth/refresh", bytes.NewReader(refBody)))
	if wRefOld.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401 using rotated-out refresh token, got %d", wRefOld.Code)
	}

	// 4. Token refresh via Cookie
	reqCookie := httptest.NewRequest(http.MethodPost, "/api/auth/refresh", nil)
	reqCookie.AddCookie(&http.Cookie{Name: RefreshCookieName, Value: newRefresh})
	wCookie := httptest.NewRecorder()
	handler.HandleRefreshToken(wCookie, reqCookie)
	if wCookie.Code != http.StatusOK {
		t.Fatalf("Expected 200 via cookie refresh, got %d", wCookie.Code)
	}

	// 5. Logout
	claims, _ := auth.ValidateToken(newAccess)
	reqLogout := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	ctx := context.WithValue(reqLogout.Context(), auth.UserKey, claims)
	reqLogout.AddCookie(&http.Cookie{Name: RefreshCookieName, Value: newRefresh})
	wLogout := httptest.NewRecorder()
	handler.HandleLogout(wLogout, reqLogout.WithContext(ctx))
	if wLogout.Code != http.StatusOK {
		t.Fatalf("Logout failed: %d", wLogout.Code)
	}

	// Ensure session is revoked in DB
	sess, err := store.GetSession(claims.SessionID)
	if err != nil || sess == nil || !sess.Revoked {
		t.Errorf("Expected session %s to be marked revoked, got %+v", claims.SessionID, sess)
	}

	// Refresh after logout must fail
	wRefAfterLogout := httptest.NewRecorder()
	handler.HandleRefreshToken(wRefAfterLogout, reqCookie)
	if wRefAfterLogout.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401 after logout, got %d", wRefAfterLogout.Code)
	}
}
