package api

import (
	"encoding/json"
	"net/http"
	"paperMC_backend/internal/minecraft"
)

type Handler struct {
	mc *minecraft.Server
}

func NewServerHandler(mcServer *minecraft.Server) *Handler {
	return &Handler{
		mc: mcServer,
	}
}

type StatusResponse struct {
	Status string `json:"status"`
}

type CommandRequest struct {
	Command string `json:"command"`
}

func (h *Handler) GetStatus(w http.ResponseWriter, r *http.Request) {
	response := StatusResponse{Status: h.mc.GetStatus().String()}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (h *Handler) SendCommand(w http.ResponseWriter, r *http.Request) {
	var req = CommandRequest{}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if req.Command == "" {
		http.Error(w, "Command cannot be empty", http.StatusBadRequest)
		return
	}

	if err := h.mc.SendCommand(req.Command); err != nil {
		http.Error(w, "Error sending Command", http.StatusBadRequest)
		return
	}
	response := StatusResponse{Status: "200 OK JSON"}
	w.Header().Set("Content-type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (h *Handler) Start(w http.ResponseWriter, r *http.Request) {

	if err := h.mc.Start(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
	}
	go h.mc.StreamLogs()
	response := StatusResponse{Status: "200 Server started"}
	w.Header().Set("Content-type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (h *Handler) Stop(w http.ResponseWriter, r *http.Request) {

	if err := h.mc.Stop(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
	}
	go h.mc.StreamLogs()
	response := StatusResponse{Status: "200 Server started"}
	w.Header().Set("Content-type", "application/json")
	json.NewEncoder(w).Encode(response)
}
