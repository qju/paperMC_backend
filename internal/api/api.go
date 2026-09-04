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
	"paperMC_backend/internal/plugins"
	"paperMC_backend/internal/scheduler"
	"paperMC_backend/internal/updater"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type Handler struct {
	mc             *minecraft.Server
	updateMu       sync.Mutex
	store          database.Store
	hub            *Hub
	scheduler      *scheduler.Service
	geyserClient   *plugins.GeyserClient
	modrinthClient *plugins.ModrinthClient
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

// HELPER Functions
func respondWithError(w http.ResponseWriter, code int, message string) {
	respondWithJSON(w, code, map[string]string{"error": message})
}

func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(payload)
}

func NewServerHandler(mcServer *minecraft.Server, store database.Store) *Handler {
	hub := NewHub()
	go hub.Run()

	var sched *scheduler.Service
	if store != nil && mcServer != nil {
		sched = scheduler.NewService(store, mcServer, mcServer.WorkDir)
	}

	h := &Handler{
		mc:             mcServer,
		updateMu:       sync.Mutex{},
		store:          store,
		hub:            hub,
		scheduler:      sched,
		geyserClient:   plugins.NewGeyserClient(),
		modrinthClient: plugins.NewModrinthClient(),
	}

	if mcServer != nil {
		mcServer.AddListener(func(msg string) {
			h.hub.Broadcast(WSMessage{Type: "log", Data: msg})
		})

		// Broadcast live vitals over WebSockets every 1.5 seconds
		go func() {
			ticker := time.NewTicker(1500 * time.Millisecond)
			defer ticker.Stop()
			for range ticker.C {
				vitals := mcServer.GetVitals()
				props, err := config.LoadProperties(mcServer.WorkDir)
				if err == nil && props["level-name"] != "" {
					vitals.ActiveWorld = props["level-name"]
				} else {
					vitals.ActiveWorld = "world"
				}
				h.hub.Broadcast(WSMessage{
					Type: "vitals",
					Data: vitals,
				})
			}
		}()
	}

	return h
}

func (h *Handler) StartScheduler() error {
	if h.scheduler != nil {
		return h.scheduler.Start()
	}
	return nil
}

func (h *Handler) StopScheduler() {
	if h.scheduler != nil {
		h.scheduler.Stop()
	}
}

func (h *Handler) SetScheduler(s *scheduler.Service) {
	h.scheduler = s
}

func (h *Handler) GetScheduler() *scheduler.Service {
	return h.scheduler
}

func (h *Handler) SetGeyserClient(c *plugins.GeyserClient) {
	h.geyserClient = c
}

func (h *Handler) SetModrinthClient(m *plugins.ModrinthClient) {
	h.modrinthClient = m
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

func (h *Handler) HandleStatus(w http.ResponseWriter, r *http.Request) {
	vitals := h.mc.GetVitals()

	props, err := config.LoadProperties(h.mc.WorkDir)
	if err == nil {
		vitals.ActiveWorld = props["level-name"]
	}
	if vitals.ActiveWorld == "" {
		vitals.ActiveWorld = "world" // fallback
	}

	// 2. Send as JSON
	w.Header().Set("Content-type", "application/json")
	if err := json.NewEncoder(w).Encode(vitals); err != nil {
		http.Error(w, "Failed to encode vitals", http.StatusInternalServerError)
	}
}

func (h *Handler) WhiteListing(w http.ResponseWriter, r *http.Request) {
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
	err := h.mc.Start()
	if err != nil {
		// Differentiate between State conflicts (400) and OS errors (500)
		if strings.Contains(err.Error(), "Status is not") {
			respondWithError(w, http.StatusBadRequest, err.Error())
			return
		}
		respondWithError(w, http.StatusInternalServerError, "Failed to start server: "+err.Error())
		return
	}
	respondWithJSON(w, http.StatusOK, map[string]string{"message": "Server started"})
}

func (h *Handler) Stop(w http.ResponseWriter, r *http.Request) {
	err := h.mc.Stop()
	if err != nil {
		if strings.Contains(err.Error(), "not running") {
			respondWithError(w, http.StatusBadRequest, err.Error())
			return
		}
		respondWithError(w, http.StatusInternalServerError, "Failed to stop server: "+err.Error())
		return
	}
	respondWithJSON(w, http.StatusOK, map[string]string{"message": "Server stopped"})
}

func (h *Handler) Kill(w http.ResponseWriter, r *http.Request) {
	err := h.mc.Kill()
	if err != nil {
		if strings.Contains(err.Error(), "already stopped") {
			respondWithError(w, http.StatusBadRequest, err.Error())
			return
		}
		respondWithError(w, http.StatusInternalServerError, "Failed to Kill the server: "+err.Error())
		return
	}
	respondWithJSON(w, http.StatusOK, map[string]string{"message": "Server Killed"})
}

func (h *Handler) HandleLogs(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	ch := make(chan string, 100)
	unsubscribe := h.mc.AddListener(func(msg string) {
		select {
		case ch <- msg:
		default:
		}
	})
	defer unsubscribe()

	for {
		select {
		case response := <-ch:
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
		http.Error(w, "Failed to save config: "+err.Error(), http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(StatusResponse{Status: "Config Saved"})
}

func (h *Handler) HandleGetVersions(w http.ResponseWriter, r *http.Request) {
	project := r.URL.Query().Get("project")
	if project == "" {
		project = updater.DefaultProject
	}

	versions, err := updater.GetProjectVersions(project)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondWithJSON(w, http.StatusOK, versions)
}

type CheckUpdateResponse struct {
	Project         string `json:"project"`
	Version         string `json:"version"`
	LatestBuild     int    `json:"latest_build"`
	LatestHash      string `json:"latest_hash"`
	CurrentHash     string `json:"current_hash"`
	UpdateAvailable bool   `json:"update_available"`
	Channel         string `json:"channel"`
	FileName        string `json:"file_name"`
	Size            int64  `json:"size"`
}

func (h *Handler) HandleCheckUpdate(w http.ResponseWriter, r *http.Request) {
	version := r.URL.Query().Get("version")
	if strings.TrimSpace(version) == "" {
		respondWithError(w, http.StatusBadRequest, "Missing 'version' query parameter")
		return
	}
	project := r.URL.Query().Get("project")
	if project == "" {
		project = updater.DefaultProject
	}

	buildInfo, err := updater.GetLatestBuild(project, version)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	fullPath := filepath.Join(h.mc.WorkDir, h.mc.JarFile)
	currentHash, _ := updater.GetFileHash(fullPath)

	updateAvailable := !strings.EqualFold(buildInfo.SHA256, currentHash)

	respondWithJSON(w, http.StatusOK, CheckUpdateResponse{
		Project:         project,
		Version:         buildInfo.Version,
		LatestBuild:     buildInfo.BuildID,
		LatestHash:      buildInfo.SHA256,
		CurrentHash:     currentHash,
		UpdateAvailable: updateAvailable,
		Channel:         buildInfo.Channel,
		FileName:        buildInfo.FileName,
		Size:            buildInfo.Size,
	})
}

func (h *Handler) HandleUpdate(w http.ResponseWriter, r *http.Request) {
	// 0. Try Lock, only one Update at a time
	if !h.updateMu.TryLock() {
		respondWithError(w, http.StatusConflict, "Update already in progress")
		return
	}
	defer h.updateMu.Unlock()

	// 1. Decode the request
	var req UpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		if err == io.EOF {
			respondWithError(w, http.StatusBadRequest, "Empty request body")
			return
		}
		respondWithError(w, http.StatusBadRequest, `Invalid JSON. Expected format: {"version": "26.2"}`)
		return
	}

	targetVersion := strings.TrimSpace(req.Version)
	if targetVersion == "" {
		respondWithError(w, http.StatusBadRequest, "Target version cannot be empty")
		return
	}

	// 2. Fetch the latest build info via Fill API v3
	buildInfo, err := updater.GetLatestBuild(updater.DefaultProject, targetVersion)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to resolve build: "+err.Error())
		return
	}

	h.mc.Broadcast(fmt.Sprintf("[System] Found Build %d (%s) for %s. SHA256: %s", buildInfo.BuildID, buildInfo.Channel, buildInfo.Version, buildInfo.SHA256))

	// 3. Check existing JAR hash
	fullPath := filepath.Join(h.mc.WorkDir, h.mc.JarFile)
	currentHash, _ := updater.GetFileHash(fullPath)

	if currentHash != "" && strings.EqualFold(buildInfo.SHA256, currentHash) {
		h.mc.Broadcast("[System] Latest build is already installed")
		respondWithJSON(w, http.StatusOK, map[string]interface{}{
			"status":   "up_to_date",
			"version":  buildInfo.Version,
			"build_id": buildInfo.BuildID,
		})
		return
	}

	// 4. Download staging file with integrity check
	stagingFileName := h.mc.JarFile + ".download"
	h.mc.Broadcast(fmt.Sprintf("[System] Downloading %s (%.1f MB)...", buildInfo.FileName, float64(buildInfo.Size)/(1024*1024)))

	if err := updater.DownloadJar(buildInfo.DownloadURL, h.mc.WorkDir, stagingFileName, buildInfo.SHA256); err != nil {
		h.mc.Broadcast("[System] Download failed: " + err.Error())
		respondWithError(w, http.StatusInternalServerError, "Download failed: "+err.Error())
		return
	}

	// 5. Stop server if running
	wasRunning := h.mc.GetStatus() == minecraft.StatusRunning
	stagingPath := filepath.Join(h.mc.WorkDir, stagingFileName)
	backupPath := filepath.Join(h.mc.WorkDir, h.mc.JarFile+".bak")

	if wasRunning {
		h.mc.Broadcast("[System] Download verified. Stopping server for update...")
		_ = h.mc.SendCommand("msg @a Server is updating, restarting shortly...")
		if err := h.mc.Stop(); err != nil {
			_ = os.Remove(stagingPath)
			respondWithError(w, http.StatusInternalServerError, "Failed to stop server: "+err.Error())
			return
		}
	}

	// 6. Backup existing file and atomic rename
	if _, err := os.Stat(fullPath); err == nil {
		_ = os.Remove(backupPath) // remove previous backup if present
		if err := os.Rename(fullPath, backupPath); err != nil {
			_ = os.Remove(stagingPath)
			respondWithError(w, http.StatusInternalServerError, "Failed to backup existing server JAR: "+err.Error())
			return
		}
	}

	if err := os.Rename(stagingPath, fullPath); err != nil {
		_ = os.Rename(backupPath, fullPath) // Rollback
		respondWithError(w, http.StatusInternalServerError, "Failed to install new server JAR: "+err.Error())
		return
	}

	// 7. Restart server if it was active
	if wasRunning {
		h.mc.Broadcast("[System] Binary installed. Starting server...")
		if err := h.mc.Start(); err != nil {
			h.mc.Broadcast("[System] Failed to restart server: " + err.Error())
			respondWithError(w, http.StatusInternalServerError, "Update succeeded but server restart failed: "+err.Error())
			return
		}
	}

	h.mc.Broadcast(fmt.Sprintf("[System] PaperMC updated to %s (Build %d)", buildInfo.Version, buildInfo.BuildID))
	respondWithJSON(w, http.StatusOK, map[string]interface{}{
		"status":   "updated",
		"version":  buildInfo.Version,
		"build_id": buildInfo.BuildID,
		"channel":  buildInfo.Channel,
	})
}
