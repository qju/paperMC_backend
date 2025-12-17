package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"paperMC_backend/internal/config"
	"paperMC_backend/internal/database"
	"paperMC_backend/internal/minecraft"
	"paperMC_backend/internal/updater"
	"path/filepath"
	"sync"
)

type Handler struct {
	mc       *minecraft.Server
	updateMu sync.Mutex
	store    database.Store
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

func NewServerHandler(mcServer *minecraft.Server, store database.Store) *Handler {
	return &Handler{
		mc:       mcServer,
		updateMu: sync.Mutex{},
		store:    store,
	}
}

type StatusResponse struct {
	Status string `json:"status"`
}

type CommandRequest struct {
	Command string `json:"command"`
}

type UpdateRequest struct {
	Version string `json:"version"`
}

func (h *Handler) HandleStatus(w http.ResponseWriter, r *http.Request) {
	vitals := h.mc.GetVitals()

	// 2. Send as JSON
	w.Header().Set("Content-type", "application/json")
	if err := json.NewEncoder(w).Encode(vitals); err != nil {
		http.Error(w, "Failed to encode vitals", http.StatusInternalServerError)
	}
}

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

func (h *Handler) GetConfig(w http.ResponseWriter, r *http.Request) {
	config, err := config.LoadProperties(h.mc.WorkDir)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(config)
}

func (h *Handler) PostConfig(w http.ResponseWriter, r *http.Request) {
	var data map[string]string
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if err := config.SaveProperties(h.mc.WorkDir, data); err != nil {
		http.Error(w, "Failed to save config"+err.Error(), http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(StatusResponse{Status: "Config Saved"})
}

func (h *Handler) HandleUpdate(w http.ResponseWriter, r *http.Request) {
	// 0. Try Lock, only one Update at a time, otherwise return 409
	if !h.updateMu.TryLock() {
		http.Error(w, "Update already in progress", http.StatusConflict)
		return
	}
	defer h.updateMu.Unlock()

	// 1. decode the request to get the version
	var version = UpdateRequest{}
	if err := json.NewDecoder(r.Body).Decode(&version); err != nil {
		if err == io.EOF {
			http.Error(w, "Empty request body", http.StatusBadRequest)
			return
		}
		msg := `Invalid JSON. Expected format: {"version": "1.21.10"}`
		http.Error(w, msg, http.StatusBadRequest)
		return
	}
	// 2. Get the latest build info
	latestBuild, latestFileName, latestHash, err := updater.GetLatestBuild(version.Version)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.mc.Broadcast(fmt.Sprintf("[System] Found Build %d. Hash: %s", latestBuild, latestHash))
	// 3. Get old server.jar sha256 hash
	fullPath := filepath.Join(h.mc.WorkDir, h.mc.JarFile)
	hash, err := updater.GetFileHash(fullPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// 4. Compare
	if latestHash == hash {
		h.mc.Broadcast("Latest build already in use")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(StatusResponse{Status: ""})
		return
	}
	// 5. Update

	// a. Download to server.jar.tmp
	h.mc.Broadcast("[System] Downloading update...")
	err = updater.DownloadJar(
		version.Version, latestBuild, latestFileName, h.mc.WorkDir)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// b. Stop server
	h.mc.Broadcast("[System] Download complete. Stopping server...")
	h.mc.SendCommand("msg @a Closing Server")
	if err := h.mc.Stop(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// c. Move old server.jar file in case failure later
	oldPath := filepath.Join(h.mc.WorkDir, h.mc.JarFile)
	oldPathTemp := oldPath + ".tmp"
	if err := os.Rename(oldPath, oldPathTemp); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// d. Rename downloaded file
	// 	if OK move forward
	// 	if Not OK start server
	newPath := filepath.Join(h.mc.WorkDir, latestFileName)
	if err := os.Rename(newPath, oldPath); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		if err := os.Rename(oldPathTemp, oldPath); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	// e. Start server
	h.mc.Broadcast("[System] Files swapped. Restarting server...")
	if err := h.mc.Start(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// f. Return 200
	w.WriteHeader(http.StatusOK)
}
