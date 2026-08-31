package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"paperMC_backend/internal/auth"
	"paperMC_backend/internal/database"
)

type CreateUserRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Role     string `json:"role"`
}

type UpdatePasswordRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (h *Handler) HandleListUsers(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		respondWithError(w, http.StatusInternalServerError, "Database store not configured")
		return
	}

	users, err := h.store.ListUsers()
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to list users: "+err.Error())
		return
	}

	respondWithJSON(w, http.StatusOK, users)
}

func (h *Handler) HandleCreateUser(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		respondWithError(w, http.StatusInternalServerError, "Database store not configured")
		return
	}

	var req CreateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		if err == io.EOF {
			respondWithError(w, http.StatusBadRequest, "Empty request body")
			return
		}
		respondWithError(w, http.StatusBadRequest, "Invalid JSON body")
		return
	}

	username := strings.TrimSpace(req.Username)
	password := strings.TrimSpace(req.Password)
	role := strings.TrimSpace(req.Role)

	if username == "" || password == "" {
		respondWithError(w, http.StatusBadRequest, "Username and password cannot be empty")
		return
	}
	if len(password) < 4 {
		respondWithError(w, http.StatusBadRequest, "Password must be at least 4 characters")
		return
	}
	if role == "" {
		role = "admin"
	}

	// Check if user already exists
	_, err := h.store.GetUser(username)
	if err == nil {
		respondWithError(w, http.StatusConflict, "User '"+username+"' already exists")
		return
	} else if !errors.Is(err, sql.ErrNoRows) {
		respondWithError(w, http.StatusInternalServerError, "Database error: "+err.Error())
		return
	}

	hashedPassword, err := auth.HashPassword(password)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to hash password: "+err.Error())
		return
	}

	newUser := &database.User{
		Username: username,
		Password: hashedPassword,
		Role:     role,
	}

	if err := h.store.CreateUser(newUser); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to create user: "+err.Error())
		return
	}

	respondWithJSON(w, http.StatusCreated, map[string]interface{}{
		"status":   "User created successfully",
		"username": username,
		"role":     role,
	})
}

func (h *Handler) HandleUpdatePassword(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		respondWithError(w, http.StatusInternalServerError, "Database store not configured")
		return
	}

	var req UpdatePasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid JSON body")
		return
	}

	username := strings.TrimSpace(req.Username)
	password := strings.TrimSpace(req.Password)

	if username == "" || password == "" {
		respondWithError(w, http.StatusBadRequest, "Username and new password cannot be empty")
		return
	}
	if len(password) < 4 {
		respondWithError(w, http.StatusBadRequest, "Password must be at least 4 characters")
		return
	}

	_, err := h.store.GetUser(username)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			respondWithError(w, http.StatusNotFound, "User not found")
			return
		}
		respondWithError(w, http.StatusInternalServerError, "Database error: "+err.Error())
		return
	}

	hashedPassword, err := auth.HashPassword(password)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to hash password: "+err.Error())
		return
	}

	if err := h.store.UpdateUserPassword(username, hashedPassword); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to update password: "+err.Error())
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]interface{}{
		"status":   "Password updated successfully",
		"username": username,
	})
}

func (h *Handler) HandleDeleteUser(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		respondWithError(w, http.StatusInternalServerError, "Database store not configured")
		return
	}

	username := strings.TrimSpace(r.URL.Query().Get("username"))
	if username == "" {
		respondWithError(w, http.StatusBadRequest, "Missing 'username' query parameter")
		return
	}

	users, err := h.store.ListUsers()
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Database error: "+err.Error())
		return
	}

	if len(users) <= 1 {
		respondWithError(w, http.StatusBadRequest, "Cannot delete the only remaining user account")
		return
	}

	if err := h.store.DeleteUser(username); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to delete user: "+err.Error())
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]interface{}{
		"status":   "User deleted successfully",
		"username": username,
	})
}
