package api

import (
	"encoding/json"
	"net/http"
	"paperMC_backend/internal/auth"
)

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type LoginResponse struct {
	Token string `json:"token"`
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req = LoginRequest{}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	User, err := h.store.GetUser(req.Username)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	if User == nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}
	if !auth.CheckPasswordHash(req.Password, User.Password) {
		http.Error(w, "Invalid user or password", http.StatusUnauthorized)
		return
	}
	token, err := auth.GenerateToken(User.Username, User.Role)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-type", "application/json")
	json.NewEncoder(w).Encode(LoginResponse{Token: token})
}
