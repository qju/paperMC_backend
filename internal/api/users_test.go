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
