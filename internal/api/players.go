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
		h.recordAudit(r, "player.whitelist_add", http.StatusInternalServerError, "Failed to whitelist "+req.Username+": "+err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// If successful, also remove from rejected list (cleanup)
	_ = h.store.DeleteRejectedPlayer(req.Username)

	h.recordAudit(r, "player.whitelist_add", http.StatusOK, "Whitelisted player "+req.Username)
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
		h.recordAudit(r, "player.whitelist_remove", http.StatusInternalServerError, "Failed to remove "+username+": "+err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.recordAudit(r, "player.whitelist_remove", http.StatusOK, "Removed "+username+" from whitelist")
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
		h.recordAudit(r, "player.ban", http.StatusInternalServerError, "Failed to ban "+req.Username+": "+err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	details := "Banned player " + req.Username
	if req.Reason != "" {
		details += " (Reason: " + req.Reason + ")"
	}
	h.recordAudit(r, "player.ban", http.StatusOK, details)
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
		h.recordAudit(r, "player.unban", http.StatusInternalServerError, "Failed to unban "+username+": "+err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.recordAudit(r, "player.unban", http.StatusOK, "Unbanned player "+username)
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
	auditAction := "player.op"
	if action == "remove" {
		auditAction = "player.deop"
		err = h.mc.DeopUser(req.Username)
	} else {
		err = h.mc.OpUser(req.Username)
	}

	if err != nil {
		h.recordAudit(r, auditAction, http.StatusInternalServerError, "Failed for "+req.Username+": "+err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.recordAudit(r, auditAction, http.StatusOK, "Updated operator status for "+req.Username)
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
		h.recordAudit(r, "player.delete_rejected", http.StatusInternalServerError, "Failed to delete "+username+": "+err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.recordAudit(r, "player.delete_rejected", http.StatusOK, "Deleted rejected record for "+username)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(StatusResponse{Status: "Rejected entry deleted"})
}
