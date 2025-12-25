package api

import (
	"encoding/json"
	"net/http"
)

type PlayerRequest struct {
	Username string `json:"username"`
	Reason   string `json:"reason,omitempty"`
}

// --- WHITELIST ---

func (h *Handler) HandleGetPlayers(w http.ResponseWriter, r *http.Request) {
	players, err := h.mc.GetWhiteList()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(players)
}

func (h *Handler) HandleAddPlayer(w http.ResponseWriter, r *http.Request) {
	var req PlayerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	if req.Username == "" {
		http.Error(w, "Username required", http.StatusBadRequest)
		return
	}
	if err := h.mc.WhiteListUser(req.Username); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// If successful, also remove from rejected list (cleanup)
	_ = h.store.DeleteRejectedPlayer(req.Username)

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(StatusResponse{Status: "Player whitelisted"})
}

func (h *Handler) HandleRemovePlayer(w http.ResponseWriter, r *http.Request) {
	username := r.URL.Query().Get("username")
	if username == "" {
		http.Error(w, "Username required", http.StatusBadRequest)
		return
	}
	if err := h.mc.RemoveWhitelist(username); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(StatusResponse{Status: "Player removed"})
}

// --- BANNED PLAYERS ---

func (h *Handler) HandleGetBanned(w http.ResponseWriter, r *http.Request) {
	players, err := h.mc.GetBanned()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(players)
}

func (h *Handler) HandleBanPlayer(w http.ResponseWriter, r *http.Request) {
	var req PlayerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	if req.Username == "" {
		http.Error(w, "Username required", http.StatusBadRequest)
		return
	}
	if err := h.mc.BanUser(req.Username, req.Reason); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(StatusResponse{Status: "Player banned"})
}

func (h *Handler) HandleUnbanPlayer(w http.ResponseWriter, r *http.Request) {
	username := r.URL.Query().Get("username")
	if username == "" {
		http.Error(w, "Username required", http.StatusBadRequest)
		return
	}
	if err := h.mc.UnbanUser(username); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(StatusResponse{Status: "Player unbanned"})
}

// --- OPS ---

func (h *Handler) HandleGetOps(w http.ResponseWriter, r *http.Request) {
	players, err := h.mc.GetOps()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(players)
}

func (h *Handler) HandleOpPlayer(w http.ResponseWriter, r *http.Request) {
	var req PlayerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	// Action depends on query param ?action=add|remove
	action := r.URL.Query().Get("action")

	var err error
	if action == "remove" {
		err = h.mc.DeopUser(req.Username)
	} else {
		err = h.mc.OpUser(req.Username)
	}

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(StatusResponse{Status: "Op status changed"})
}

// --- REJECTED PLAYERS (DB) ---

func (h *Handler) HandleGetRejected(w http.ResponseWriter, r *http.Request) {
	players, err := h.store.GetRejectedPlayers()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(players)
}

func (h *Handler) HandleDeleteRejected(w http.ResponseWriter, r *http.Request) {
	username := r.URL.Query().Get("username")
	if username == "" {
		http.Error(w, "Username required", http.StatusBadRequest)
		return
	}
	if err := h.store.DeleteRejectedPlayer(username); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(StatusResponse{Status: "Rejected entry deleted"})
}
