package api

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"paperMC_backend/internal/config"
	"paperMC_backend/internal/minecraft"
)

type GetWorldsResponse struct {
	ActiveWorld string                `json:"active_world"`
	Worlds      []minecraft.WorldInfo `json:"worlds"`
}

type SetActiveWorldRequest struct {
	WorldName string  `json:"world_name"`
	Seed      *string `json:"seed,omitempty"`
}

type DuplicateWorldRequest struct {
	SourceWorld string `json:"source_world"`
	TargetWorld string `json:"target_world"`
}

type DeleteWorldRequest struct {
	WorldName string `json:"world_name"`
}

func (h *Handler) invalidateActiveWorldCache() {
	h.activeWorldMu.Lock()
	h.activeWorldCache = ""
	h.activeWorldModTime = time.Time{}
	h.activeWorldMu.Unlock()
}

func (h *Handler) getActiveWorld() string {
	propsPath := filepath.Join(h.mc.WorkDir, "server.properties")
	info, err := os.Stat(propsPath)
	if err != nil {
		h.invalidateActiveWorldCache()
		return "world"
	}

	h.activeWorldMu.Lock()
	defer h.activeWorldMu.Unlock()

	if h.activeWorldCache != "" && info.ModTime().Equal(h.activeWorldModTime) {
		return h.activeWorldCache
	}

	active := "world"
	if props, err := config.LoadProperties(h.mc.WorkDir); err == nil && props != nil {
		if lvl := strings.TrimSpace(props["level-name"]); lvl != "" {
			active = lvl
		}
	}

	h.activeWorldCache = active
	h.activeWorldModTime = info.ModTime()
	return active
}

func (h *Handler) HandleGetWorlds(w http.ResponseWriter, r *http.Request) {
	activeWorld := h.getActiveWorld()
	worlds, err := minecraft.ListWorlds(h.mc.WorkDir, activeWorld)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to scan worlds: "+err.Error())
		return
	}

	respondWithJSON(w, http.StatusOK, GetWorldsResponse{
		ActiveWorld: activeWorld,
		Worlds:      worlds,
	})
}

func (h *Handler) HandleSetActiveWorld(w http.ResponseWriter, r *http.Request) {
	var req SetActiveWorldRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid JSON body")
		return
	}
	h.applySetActiveWorld(w, r, req)
}

func (h *Handler) applySetActiveWorld(w http.ResponseWriter, r *http.Request, req SetActiveWorldRequest) {
	newWorld := strings.TrimSpace(req.WorldName)
	if !minecraft.IsValidWorldName(newWorld) {
		respondWithError(w, http.StatusBadRequest, "Invalid world name: only alphanumeric characters, underscores, and hyphens are allowed")
		return
	}

	// Update server.properties
	changes := map[string]string{
		"level-name": newWorld,
	}
	if req.Seed != nil {
		changes["level-seed"] = *req.Seed
	}

	if err := config.SaveProperties(h.mc.WorkDir, changes); err != nil {
		action := "world.switch"
		if strings.Contains(r.URL.Path, "create") {
			action = "world.create"
		}
		h.recordAudit(r, action, http.StatusInternalServerError, "Failed to update configuration: "+err.Error())
		respondWithError(w, http.StatusInternalServerError, "Failed to update configuration: "+err.Error())
		return
	}
	h.invalidateActiveWorldCache()

	action := "world.switch"
	if strings.Contains(r.URL.Path, "create") {
		action = "world.create"
	}
	h.recordAudit(r, action, http.StatusOK, "Active world set to "+newWorld)

	// Synchronized restart if server is running
	status := h.mc.GetStatus()
	if status == minecraft.StatusRunning || status == minecraft.StatusStarting {
		h.mc.Broadcast("[System] Flushing chunks and switching active world to '" + newWorld + "'...")
		_ = h.mc.SendCommand("save-all flush")
		time.Sleep(1 * time.Second)

		if err := h.mc.Stop(); err != nil {
			respondWithError(w, http.StatusInternalServerError, "Failed to stop server: "+err.Error())
			return
		}

		// Await server shutdown with a 30s timeout guard before restart
		go func() {
			if !h.mc.WaitForStop(30 * time.Second) {
				h.mc.Broadcast("[System] Timed out waiting for server to stop during world switch.")
				return
			}
			select {
			case <-h.stopCh:
				return
			default:
			}
			h.mc.Broadcast("[System] Restarting server under world '" + newWorld + "'...")
			_ = h.mc.Start()
		}()
	}

	respondWithJSON(w, http.StatusOK, map[string]interface{}{
		"status":       "Active world updated",
		"active_world": newWorld,
	})
}

func (h *Handler) HandleDuplicateWorld(w http.ResponseWriter, r *http.Request) {
	var req DuplicateWorldRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid JSON body")
		return
	}

	source := strings.TrimSpace(req.SourceWorld)
	target := strings.TrimSpace(req.TargetWorld)
	if source == "" || target == "" {
		respondWithError(w, http.StatusBadRequest, "Both 'source_world' and 'target_world' are required")
		return
	}

	activeWorld := h.getActiveWorld()
	if source == activeWorld && h.mc.GetStatus() == minecraft.StatusRunning {
		h.mc.Broadcast("[System] Flushing chunks to disk for safe world duplication...")
		_ = h.mc.SendCommand("save-all flush")
		time.Sleep(1 * time.Second)
	}

	if err := minecraft.DuplicateWorld(h.mc.WorkDir, source, target); err != nil {
		h.recordAudit(r, "world.duplicate", http.StatusBadRequest, "Failed cloning "+source+" to "+target+": "+err.Error())
		respondWithError(w, http.StatusBadRequest, err.Error())
		return
	}

	h.recordAudit(r, "world.duplicate", http.StatusOK, "Cloned world "+source+" to "+target)
	newWorldInfo, err := minecraft.InspectWorld(h.mc.WorkDir, target, false)
	if err != nil {
		respondWithJSON(w, http.StatusOK, map[string]string{"status": "World cloned successfully", "target_world": target})
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]interface{}{
		"status": "World duplicated successfully",
		"world":  newWorldInfo,
	})
}

func (h *Handler) HandleDeleteWorld(w http.ResponseWriter, r *http.Request) {
	worldName := r.URL.Query().Get("name")
	if worldName == "" {
		var req DeleteWorldRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err == nil {
			worldName = req.WorldName
		}
	}
	worldName = strings.TrimSpace(worldName)
	if worldName == "" {
		respondWithError(w, http.StatusBadRequest, "World name parameter is missing")
		return
	}

	activeWorld := h.getActiveWorld()
	if err := minecraft.DeleteWorld(h.mc.WorkDir, worldName, activeWorld); err != nil {
		h.recordAudit(r, "world.delete", http.StatusBadRequest, "Failed to delete "+worldName+": "+err.Error())
		respondWithError(w, http.StatusBadRequest, err.Error())
		return
	}

	h.recordAudit(r, "world.delete", http.StatusOK, "Deleted world "+worldName)
	respondWithJSON(w, http.StatusOK, map[string]interface{}{
		"status":     "World deleted",
		"world_name": worldName,
	})
}

func (h *Handler) HandleCreateWorld(w http.ResponseWriter, r *http.Request) {
	var req SetActiveWorldRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		if err == io.EOF {
			respondWithError(w, http.StatusBadRequest, "Empty request body")
			return
		}
		respondWithError(w, http.StatusBadRequest, "Invalid JSON body")
		return
	}

	worldName := strings.TrimSpace(req.WorldName)
	if worldName == "" {
		respondWithError(w, http.StatusBadRequest, "World name cannot be empty")
		return
	}

	// World creation in Paper is achieved by setting level-name and booting
	h.applySetActiveWorld(w, r, req)
}

