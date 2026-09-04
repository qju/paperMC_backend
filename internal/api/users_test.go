package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"paperMC_backend/internal/database"
	"paperMC_backend/internal/minecraft"
)

func TestUserManagementAPI(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")
	store, err := database.NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to init store: %v", err)
	}
	defer store.Close()

	mcServer := &minecraft.Server{
		WorkDir: tempDir,
		JarFile: "server.jar",
		RAM:     "2G",
	}
	handler := NewServerHandler(mcServer, store)

	// 1. Test POST /api/users (Create admin)
	user1 := CreateUserRequest{
		Username: "admin_user",
		Password: "password123",
		Role:     "admin",
	}
	body1, _ := json.Marshal(user1)
	req1 := httptest.NewRequest(http.MethodPost, "/api/users", bytes.NewReader(body1))
	w1 := httptest.NewRecorder()
	handler.HandleCreateUser(w1, req1)

	if w1.Code != http.StatusCreated {
		t.Fatalf("Expected 201 Created, got %d: %s", w1.Code, w1.Body.String())
	}

	// 2. Test POST /api/users (Create second operator)
	user2 := CreateUserRequest{
		Username: "operator_user",
		Password: "operatorpass",
		Role:     "operator",
	}
	body2, _ := json.Marshal(user2)
	req2 := httptest.NewRequest(http.MethodPost, "/api/users", bytes.NewReader(body2))
	w2 := httptest.NewRecorder()
	handler.HandleCreateUser(w2, req2)

	if w2.Code != http.StatusCreated {
		t.Fatalf("Expected 201 Created, got %d", w2.Code)
	}

	// 3. Test GET /api/users (List users)
	reqList := httptest.NewRequest(http.MethodGet, "/api/users", nil)
	wList := httptest.NewRecorder()
	handler.HandleListUsers(wList, reqList)

	if wList.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK, got %d", wList.Code)
	}
	var users []database.User
	if err := json.NewDecoder(wList.Body).Decode(&users); err != nil {
		t.Fatalf("Failed to decode user list: %v", err)
	}
	if len(users) != 2 {
		t.Fatalf("Expected 2 users, got %d", len(users))
	}

	// 4. Test PUT /api/users/password (Reset password)
	updateReq := UpdatePasswordRequest{
		Username: "operator_user",
		Password: "newoperatorpassword",
	}
	bodyUp, _ := json.Marshal(updateReq)
	reqUp := httptest.NewRequest(http.MethodPut, "/api/users/password", bytes.NewReader(bodyUp))
	wUp := httptest.NewRecorder()
	handler.HandleUpdatePassword(wUp, reqUp)

	if wUp.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK, got %d: %s", wUp.Code, wUp.Body.String())
	}

	// 5. Test DELETE /api/users (Delete operator)
	reqDel := httptest.NewRequest(http.MethodDelete, "/api/users?username=operator_user", nil)
	wDel := httptest.NewRecorder()
	handler.HandleDeleteUser(wDel, reqDel)

	if wDel.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK, got %d", wDel.Code)
	}

	// 6. Test DELETE /api/users (Cannot delete only remaining user)
	reqDelLast := httptest.NewRequest(http.MethodDelete, "/api/users?username=admin_user", nil)
	wDelLast := httptest.NewRecorder()
	handler.HandleDeleteUser(wDelLast, reqDelLast)

	if wDelLast.Code != http.StatusBadRequest {
		t.Fatalf("Expected 400 Bad Request when deleting last user, got %d", wDelLast.Code)
	}
}

func TestUserManagementValidationAndErrorCases(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test_errors.db")
	store, err := database.NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to init store: %v", err)
	}
	defer store.Close()

	mcServer := &minecraft.Server{
		WorkDir: tempDir,
		JarFile: "server.jar",
		RAM:     "2G",
	}
	handler := NewServerHandler(mcServer, store)
	nilStoreHandler := NewServerHandler(mcServer, nil)

	// 1. Nil store error handling
	reqNilList := httptest.NewRequest(http.MethodGet, "/api/users", nil)
	wNilList := httptest.NewRecorder()
	nilStoreHandler.HandleListUsers(wNilList, reqNilList)
	if wNilList.Code != http.StatusInternalServerError {
		t.Errorf("Expected 500 when store is nil in ListUsers, got %d", wNilList.Code)
	}

	reqNilCreate := httptest.NewRequest(http.MethodPost, "/api/users", bytes.NewReader([]byte(`{"username":"u","password":"p"}`)))
	wNilCreate := httptest.NewRecorder()
	nilStoreHandler.HandleCreateUser(wNilCreate, reqNilCreate)
	if wNilCreate.Code != http.StatusInternalServerError {
		t.Errorf("Expected 500 when store is nil in CreateUser, got %d", wNilCreate.Code)
	}

	reqNilPass := httptest.NewRequest(http.MethodPut, "/api/users/password", bytes.NewReader([]byte(`{"username":"u","password":"p"}`)))
	wNilPass := httptest.NewRecorder()
	nilStoreHandler.HandleUpdatePassword(wNilPass, reqNilPass)
	if wNilPass.Code != http.StatusInternalServerError {
		t.Errorf("Expected 500 when store is nil in UpdatePassword, got %d", wNilPass.Code)
	}

	reqNilDel := httptest.NewRequest(http.MethodDelete, "/api/users?username=admin", nil)
	wNilDel := httptest.NewRecorder()
	nilStoreHandler.HandleDeleteUser(wNilDel, reqNilDel)
	if wNilDel.Code != http.StatusInternalServerError {
		t.Errorf("Expected 500 when store is nil in DeleteUser, got %d", wNilDel.Code)
	}

	// 2. Create user validation
	// Empty body
	reqEmpty := httptest.NewRequest(http.MethodPost, "/api/users", bytes.NewReader([]byte{}))
	wEmpty := httptest.NewRecorder()
	handler.HandleCreateUser(wEmpty, reqEmpty)
	if wEmpty.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 on empty body, got %d", wEmpty.Code)
	}

	// Invalid JSON
	reqBadJSON := httptest.NewRequest(http.MethodPost, "/api/users", bytes.NewReader([]byte(`invalid`)))
	wBadJSON := httptest.NewRecorder()
	handler.HandleCreateUser(wBadJSON, reqBadJSON)
	if wBadJSON.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 on bad JSON, got %d", wBadJSON.Code)
	}

	// Empty username or password
	reqBlank := httptest.NewRequest(http.MethodPost, "/api/users", bytes.NewReader([]byte(`{"username":"","password":""}`)))
	wBlank := httptest.NewRecorder()
	handler.HandleCreateUser(wBlank, reqBlank)
	if wBlank.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 on blank credentials, got %d", wBlank.Code)
	}

	// Short password
	reqShortPass := httptest.NewRequest(http.MethodPost, "/api/users", bytes.NewReader([]byte(`{"username":"validuser","password":"123"}`)))
	wShortPass := httptest.NewRecorder()
	handler.HandleCreateUser(wShortPass, reqShortPass)
	if wShortPass.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 on short password, got %d", wShortPass.Code)
	}

	// Successful create
	reqSuccess := httptest.NewRequest(http.MethodPost, "/api/users", bytes.NewReader([]byte(`{"username":"alice","password":"secretpassword"}`)))
	wSuccess := httptest.NewRecorder()
	handler.HandleCreateUser(wSuccess, reqSuccess)
	if wSuccess.Code != http.StatusCreated {
		t.Fatalf("Expected 201 on valid user create, got %d", wSuccess.Code)
	}

	// Duplicate user conflict
	wConflict := httptest.NewRecorder()
	reqConflict := httptest.NewRequest(http.MethodPost, "/api/users", bytes.NewReader([]byte(`{"username":"alice","password":"secretpassword"}`)))
	handler.HandleCreateUser(wConflict, reqConflict)
	if wConflict.Code != http.StatusConflict {
		t.Errorf("Expected 409 Conflict on duplicate user, got %d", wConflict.Code)
	}

	// 3. Update password validation
	// Invalid JSON
	reqBadPassJSON := httptest.NewRequest(http.MethodPut, "/api/users/password", bytes.NewReader([]byte(`bad-json`)))
	wBadPassJSON := httptest.NewRecorder()
	handler.HandleUpdatePassword(wBadPassJSON, reqBadPassJSON)
	if wBadPassJSON.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 on bad JSON, got %d", wBadPassJSON.Code)
	}

	// Empty username / password
	reqBlankUpdate := httptest.NewRequest(http.MethodPut, "/api/users/password", bytes.NewReader([]byte(`{"username":"","password":""}`)))
	wBlankUpdate := httptest.NewRecorder()
	handler.HandleUpdatePassword(wBlankUpdate, reqBlankUpdate)
	if wBlankUpdate.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 on blank update, got %d", wBlankUpdate.Code)
	}

	// Short password
	reqShortUpdate := httptest.NewRequest(http.MethodPut, "/api/users/password", bytes.NewReader([]byte(`{"username":"alice","password":"1"}`)))
	wShortUpdate := httptest.NewRecorder()
	handler.HandleUpdatePassword(wShortUpdate, reqShortUpdate)
	if wShortUpdate.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 on short password, got %d", wShortUpdate.Code)
	}

	// Non-existent user
	reqNonExistent := httptest.NewRequest(http.MethodPut, "/api/users/password", bytes.NewReader([]byte(`{"username":"ghost","password":"newpassword"}`)))
	wNonExistent := httptest.NewRecorder()
	handler.HandleUpdatePassword(wNonExistent, reqNonExistent)
	if wNonExistent.Code != http.StatusNotFound {
		t.Errorf("Expected 404 on non-existent user, got %d", wNonExistent.Code)
	}

	// 4. Delete user missing username query
	reqMissingUser := httptest.NewRequest(http.MethodDelete, "/api/users", nil)
	wMissingUser := httptest.NewRecorder()
	handler.HandleDeleteUser(wMissingUser, reqMissingUser)
	if wMissingUser.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 on missing username query param, got %d", wMissingUser.Code)
	}
}
