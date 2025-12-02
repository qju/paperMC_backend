package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"paperMC_backend/internal/minecraft"
)

// ServerController defines the interface for interacting with the Minecraft server.
type ServerController interface {
	GetStatus() minecraft.Status
	WhiteListUser(username string) error
	SendCommand(cmd string) error
	Start() error
	Stop() error
	StreamLogs()
	// Access to the LogChan
	GetLogChan() <-chan string
}

// ServerWrapper adapts *minecraft.Server to ServerController
type ServerWrapper struct {
	*minecraft.Server
}

func (s *ServerWrapper) GetLogChan() <-chan string {
	return s.LogChan
}

type Handler struct {
	mc ServerController
}

// BasicAuth middleware enforces basic authentication on the wrapped handler.
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

// NewServerHandler creates a new Handler with the given ServerController.
func NewServerHandler(mcServer ServerController) *Handler {
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

// GetStatus handles GET /status requests.
func (h *Handler) GetStatus(w http.ResponseWriter, r *http.Request) {
	response := StatusResponse{Status: h.mc.GetStatus().String()}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// Whitelisting handles POST /whitelist_add requests.
func (h *Handler) Whitelisting(w http.ResponseWriter, r *http.Request) {
	var req = CommandRequest{}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	if req.Command == "" {
		http.Error(w, "User name cannot be empty", http.StatusBadRequest)
		return
	}

	if err := h.mc.WhiteListUser(req.Command); err != nil {
		http.Error(w, "Error sending Command", http.StatusBadRequest)
		return
	}
	response := StatusResponse{Status: "200 OK JSON"}
	w.Header().Set("Content-type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// SendCommand handles POST /command requests.
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

// Start handles POST /start requests.
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

// Stop handles POST /stop requests.
func (h *Handler) Stop(w http.ResponseWriter, r *http.Request) {

	if err := h.mc.Stop(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	response := StatusResponse{Status: "200 Server stopped"}
	w.Header().Set("Content-type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// HandleLogs handles GET /logs requests (SSE).
func (h *Handler) HandleLogs(w http.ResponseWriter, r *http.Request) {

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	logChan := h.mc.GetLogChan()

	for {
		select {
		case response := <-logChan:
			fmt.Fprintf(w, "data: %s\n\n", response)
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
}
