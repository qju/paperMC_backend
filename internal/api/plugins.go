package api

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"paperMC_backend/internal/plugins"
)

type PluginActionRequest struct {
	Filename string `json:"filename"`
}

type GeyserUpdateRequest struct {
	Target string `json:"target"` // "geyser", "floodgate", or "both"
}

type MarketInstallRequest struct {
	ProjectID string `json:"project_id"`
	VersionID string `json:"version_id,omitempty"`
}

func (h *Handler) getPluginsDir() string {
	if h.mc != nil && h.mc.WorkDir != "" {
		return filepath.Join(h.mc.WorkDir, "plugins")
	}
	return "plugins"
}

// HandleGetPlugins lists all installed plugins in the plugins directory.
func (h *Handler) HandleGetPlugins(w http.ResponseWriter, r *http.Request) {
	pluginsDir := h.getPluginsDir()
	list, err := plugins.ScanPlugins(pluginsDir)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to scan plugins: "+err.Error())
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]interface{}{
		"plugins": list,
		"count":   len(list),
	})
}

// HandleTogglePlugin enables or disables a plugin by renaming between .jar and .jar.disabled.
func (h *Handler) HandleTogglePlugin(w http.ResponseWriter, r *http.Request) {
	filename := r.URL.Query().Get("filename")
	if filename == "" && r.Body != nil && r.ContentLength > 0 {
		var req PluginActionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err == nil {
			filename = req.Filename
		}
	}

	filename = strings.TrimSpace(filename)
	if filename == "" {
		respondWithError(w, http.StatusBadRequest, "Filename parameter is required")
		return
	}

	pluginsDir := h.getPluginsDir()
	newFilename, err := plugins.TogglePlugin(pluginsDir, filename)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Failed to toggle plugin: "+err.Error())
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]interface{}{
		"status":       "success",
		"old_filename": filename,
		"new_filename": newFilename,
	})
}

// HandleDeletePlugin removes a plugin file from the plugins directory.
func (h *Handler) HandleDeletePlugin(w http.ResponseWriter, r *http.Request) {
	filename := r.URL.Query().Get("filename")
	if filename == "" && r.Body != nil && r.ContentLength > 0 {
		var req PluginActionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err == nil {
			filename = req.Filename
		}
	}

	filename = strings.TrimSpace(filename)
	if filename == "" {
		respondWithError(w, http.StatusBadRequest, "Filename parameter is required")
		return
	}

	pluginsDir := h.getPluginsDir()
	if err := plugins.DeletePlugin(pluginsDir, filename); err != nil {
		respondWithError(w, http.StatusBadRequest, "Failed to delete plugin: "+err.Error())
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]interface{}{
		"status":   "Plugin deleted successfully",
		"filename": filename,
	})
}

// HandleUploadPlugin handles multipart file upload of .jar plugins.
func (h *Handler) HandleUploadPlugin(w http.ResponseWriter, r *http.Request) {
	// Limit max file size to 100MB
	if err := r.ParseMultipartForm(100 << 20); err != nil {
		respondWithError(w, http.StatusBadRequest, "Failed to parse upload form: "+err.Error())
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Missing 'file' in upload form: "+err.Error())
		return
	}
	defer file.Close()

	pluginsDir := h.getPluginsDir()
	info, err := plugins.SaveUploadedPlugin(pluginsDir, header.Filename, file)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Plugin validation or installation failed: "+err.Error())
		return
	}

	respondWithJSON(w, http.StatusCreated, info)
}

// HandleGetGeyserStatus returns the Bedrock bridge status including installed vs latest builds and Bedrock version compatibility.
func (h *Handler) HandleGetGeyserStatus(w http.ResponseWriter, r *http.Request) {
	pluginsDir := h.getPluginsDir()
	client := h.geyserClient
	if client == nil {
		client = plugins.NewGeyserClient()
	}

	status, err := client.GetBedrockBridgeStatus(pluginsDir)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to inspect Geyser status: "+err.Error())
		return
	}

	respondWithJSON(w, http.StatusOK, status)
}

// HandleUpdateGeyser updates Geyser and/or Floodgate plugins to their latest verified builds.
func (h *Handler) HandleUpdateGeyser(w http.ResponseWriter, r *http.Request) {
	h.updateMu.Lock()
	defer h.updateMu.Unlock()

	var req GeyserUpdateRequest
	if r.Body != nil && r.ContentLength > 0 {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}
	if req.Target == "" {
		req.Target = "both"
	}

	pluginsDir := h.getPluginsDir()
	client := h.geyserClient
	if client == nil {
		client = plugins.NewGeyserClient()
	}

	updated, err := client.UpdateBedrockBridge(pluginsDir, req.Target)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to update Bedrock Bridge: "+err.Error())
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "Bedrock bridge update completed successfully",
		"updated": updated,
	})
}

// HandleSearchMarketPlugins searches Modrinth for Paper/Spigot plugins.
func (h *Handler) HandleSearchMarketPlugins(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("query")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	client := h.modrinthClient
	if client == nil {
		client = plugins.NewModrinthClient()
	}

	result, err := client.Search(query, limit, offset)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Marketplace search failed: "+err.Error())
		return
	}

	respondWithJSON(w, http.StatusOK, result)
}

// HandleInstallMarketPlugin downloads and installs a plugin directly from Modrinth.
func (h *Handler) HandleInstallMarketPlugin(w http.ResponseWriter, r *http.Request) {
	h.updateMu.Lock()
	defer h.updateMu.Unlock()

	var req MarketInstallRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid JSON payload: "+err.Error())
		return
	}

	projectID := strings.TrimSpace(req.ProjectID)
	if projectID == "" {
		respondWithError(w, http.StatusBadRequest, "project_id is required")
		return
	}

	pluginsDir := h.getPluginsDir()
	client := h.modrinthClient
	if client == nil {
		client = plugins.NewModrinthClient()
	}

	info, err := client.InstallPlugin(pluginsDir, projectID, req.VersionID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to install plugin from marketplace: "+err.Error())
		return
	}

	respondWithJSON(w, http.StatusCreated, info)
}
