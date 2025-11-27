package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"paperMC_backend/internal/minecraft"
)

type Handler struct {
	mc *minecraft.Server
}

func (h *Handler) BasicAuth(next http.Handler, user, pass string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, p, ok := r.BasicAuth()

		if !ok || u != user || p != pass {
			w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})

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
		return
	}
	go h.mc.StreamLogs()
	response := StatusResponse{Status: "200 Server started"}
	w.Header().Set("Content-type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (h *Handler) Stop(w http.ResponseWriter, r *http.Request) {

	if err := h.mc.Stop(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	response := StatusResponse{Status: "200 Server stopped"}
	w.Header().Set("Content-type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (h *Handler) HandleLogs(w http.ResponseWriter, r *http.Request) {

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	for {
		if !ok {
			return
		}
		select {
		case response := <-h.mc.LogChan:
			fmt.Fprintf(w, "data: %s\n\n", response)
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
}
