package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"paperMC_backend/internal/backup"
)

type RestoreBackupRequest struct {
	File string `json:"file"`
}

type DeleteBackupRequest struct {
	File string `json:"file"`
}

func (h *Handler) HandleGetBackups(w http.ResponseWriter, r *http.Request) {
	backups, err := backup.ListBackups(h.mc.WorkDir)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to list backups: "+err.Error())
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]interface{}{
		"active_world": h.getActiveWorld(),
		"backups":      backups,
	})
}

func (h *Handler) HandleCreateBackup(w http.ResponseWriter, r *http.Request) {
	var req backup.CreateBackupRequest
	if r.Body != nil && r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondWithError(w, http.StatusBadRequest, "Invalid JSON body: "+err.Error())
			return
		}
	}

	req.Type = strings.ToLower(strings.TrimSpace(req.Type))
	if req.Type == "" {
		req.Type = "world"
	}

	if req.Type == "world" && strings.TrimSpace(req.WorldName) == "" {
		req.WorldName = h.getActiveWorld()
	}

	info, err := backup.CreateBackup(h.mc.WorkDir, req, h.mc)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Failed to create backup: "+err.Error())
		return
	}

	respondWithJSON(w, http.StatusCreated, info)
}

func (h *Handler) HandleDownloadBackup(w http.ResponseWriter, r *http.Request) {
	filename := r.URL.Query().Get("file")
	if strings.TrimSpace(filename) == "" {
		respondWithError(w, http.StatusBadRequest, "Missing required query parameter 'file'")
		return
	}

	targetPath, err := backup.GetBackupPath(h.mc.WorkDir, filename)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, err.Error())
		return
	}

	w.Header().Set("Content-Disposition", "attachment; filename="+filename)
	w.Header().Set("Content-Type", "application/zip")
	http.ServeFile(w, r, targetPath)
}

func (h *Handler) HandleRestoreBackup(w http.ResponseWriter, r *http.Request) {
	var req RestoreBackupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid JSON body: "+err.Error())
		return
	}

	filename := strings.TrimSpace(req.File)
	if filename == "" {
		respondWithError(w, http.StatusBadRequest, "Filename cannot be empty")
		return
	}

	if err := backup.RestoreBackup(h.mc.WorkDir, filename, h.mc); err != nil {
		respondWithError(w, http.StatusBadRequest, "Failed to restore backup: "+err.Error())
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]string{
		"status":  "Backup restored successfully",
		"archive": filename,
	})
}

func (h *Handler) HandleDeleteBackup(w http.ResponseWriter, r *http.Request) {
	filename := strings.TrimSpace(r.URL.Query().Get("file"))
	if filename == "" {
		var req DeleteBackupRequest
		if r.Body != nil && r.ContentLength > 0 {
			_ = json.NewDecoder(r.Body).Decode(&req)
			filename = strings.TrimSpace(req.File)
		}
	}

	if filename == "" {
		respondWithError(w, http.StatusBadRequest, "Filename cannot be empty")
		return
	}

	if err := backup.DeleteBackup(h.mc.WorkDir, filename); err != nil {
		respondWithError(w, http.StatusBadRequest, "Failed to delete backup: "+err.Error())
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]string{
		"status":  "Backup deleted successfully",
		"archive": filename,
	})
}
