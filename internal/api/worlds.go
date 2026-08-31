package api

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
	//"regexp"

	"paperMC_backend/internal/config"
	"paperMC_backend/internal/minecraft"
)

type WorldResponse struct {
	ActiveWorld    string   `json:"active_world"`
	InactiveWorlds []string `json:"inactive_worlds"`
}

type SetActiveWorldRequest struct {
	WorldName string  `json:"world_name"`
	Seed      *string `json:"seed,omitempty"`
}

func (h *Handler) HandleGetWorlds(w http.ResponseWriter, r *http.Request) {
	// Read current server.properties to get active world
	props, err := config.LoadProperties(h.mc.WorkDir)
	if err != nil {
		// If file doesn't exist yet, we might have no active world configured
		props = make(map[string]string)
	}

	activeWorld := props["level-name"]
	if activeWorld == "" {
		activeWorld = "world" // Default minecraft world name
	}

	var inactiveWorlds []string

	// Scan the workdir for directories containing level.dat
	entries, err := os.ReadDir(h.mc.WorkDir)
	if err == nil {
		for _, entry := range entries {
			if entry.IsDir() {
				name := entry.Name()
				// Skip _nether and _the_end directories as they are part of the main world
				if strings.HasSuffix(name, "_nether") || strings.HasSuffix(name, "_the_end") {
					continue
				}

				// Check if level.dat exists in this directory
				levelDatPath := filepath.Join(h.mc.WorkDir, name, "level.dat")
				if _, err := os.Stat(levelDatPath); err == nil {
					// It's a world directory
					if name != activeWorld {
						inactiveWorlds = append(inactiveWorlds, name)
					}
				}
			}
		}
	}

	response := WorldResponse{
		ActiveWorld:    activeWorld,
		InactiveWorlds: inactiveWorlds,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (h *Handler) HandleSetActiveWorld(w http.ResponseWriter, r *http.Request) {
	var req SetActiveWorldRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	newWorld := strings.TrimSpace(req.WorldName)
	if newWorld == "" {
		http.Error(w, "World name cannot be empty", http.StatusBadRequest)
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
		http.Error(w, "Failed to update config: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// If server is running, stop and start it to apply the new world
	status := h.mc.GetStatus()
	if status == minecraft.StatusRunning || status == minecraft.StatusStarting {
		h.mc.Broadcast("[System] Changing active world to " + newWorld + ". Restarting server...")
		if err := h.mc.Stop(); err != nil {
			http.Error(w, "Failed to stop server: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// Run a goroutine to wait for the server to stop and then start it
		go func() {
			for {
				if h.mc.GetStatus() == minecraft.StatusStopped {
					break
				}
				time.Sleep(500 * time.Millisecond)
			}
			h.mc.Start()
		}()
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(StatusResponse{Status: "Active world updated"})
}
