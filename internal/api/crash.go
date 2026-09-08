package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"paperMC_backend/internal/crash"
	"paperMC_backend/internal/database"
)

type AnalyzeRequest struct {
	Log  string `json:"log"`
	Save bool   `json:"save"`
}

type AIExplainRequest struct {
	ReportID     int    `json:"report_id"`
	CustomPrompt string `json:"custom_prompt"`
	RawLog       string `json:"raw_log"`
}

type AISettingsResponse struct {
	Provider   string    `json:"provider"`
	APIKey     string    `json:"api_key"`
	HasAPIKey  bool      `json:"has_api_key"`
	Model      string    `json:"model"`
	BaseURL    string    `json:"base_url"`
	IsEnabled  bool      `json:"is_enabled"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// maskAPIKey produces a masked string like "****" or "sk-...xxxx"
func maskAPIKey(key string) (string, bool) {
	clean := strings.TrimSpace(key)
	if clean == "" {
		return "", false
	}
	if len(clean) <= 6 {
		return "****", true
	}
	prefix := clean[:3]
	suffix := clean[len(clean)-3:]
	return fmt.Sprintf("%s****%s", prefix, suffix), true
}

// HandleListCrashReports lists recorded crash reports with pagination.
// Also discovers unrecorded crash dumps from disk to keep DB in sync.
func (h *Handler) HandleListCrashReports(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		respondWithError(w, http.StatusInternalServerError, "database store is not available")
		return
	}

	// Auto-ingest any unrecorded crash-reports/*.txt files from disk
	if h.mc != nil && h.mc.WorkDir != "" {
		if discovered, err := crash.ScanCrashReports(h.mc.WorkDir); err == nil && len(discovered) > 0 {
			existing, _, _ := h.store.ListCrashReports(100, 0)
			knownLogs := make(map[string]bool)
			for _, ex := range existing {
				knownLogs[ex.RawLog] = true
			}

			for _, disc := range discovered {
				if !knownLogs[disc.Content] {
					report := &database.CrashReport{
						Source:         "crash_file",
						Category:       disc.Analysis.Category,
						Title:          disc.Analysis.Title,
						Culprit:        disc.Analysis.Culprit,
						Summary:        disc.Analysis.Summary,
						Recommendation: disc.Analysis.Recommendation,
						RawLog:         disc.Content,
						CreatedAt:      disc.ModTime,
					}
					_ = h.store.RecordCrashReport(report)
					knownLogs[disc.Content] = true
				}
			}
		}
	}

	limit := 50
	offset := 0
	if l := r.URL.Query().Get("limit"); l != "" {
		if val, err := strconv.Atoi(l); err == nil && val > 0 {
			limit = val
		}
	}
	if o := r.URL.Query().Get("offset"); o != "" {
		if val, err := strconv.Atoi(o); err == nil && val >= 0 {
			offset = val
		}
	}

	reports, total, err := h.store.ListCrashReports(limit, offset)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "failed to query crash reports: "+err.Error())
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]interface{}{
		"reports": reports,
		"total":   total,
		"limit":   limit,
		"offset":  offset,
	})
}

// HandleGetCrashReport fetches a single crash report by ID.
func (h *Handler) HandleGetCrashReport(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		respondWithError(w, http.StatusInternalServerError, "database store is not available")
		return
	}

	idStr := r.PathValue("id")
	if idStr == "" {
		idStr = r.URL.Query().Get("id")
	}

	id, err := strconv.Atoi(idStr)
	if err != nil || id <= 0 {
		respondWithError(w, http.StatusBadRequest, "invalid crash report id")
		return
	}

	report, err := h.store.GetCrashReport(id)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "failed to fetch crash report: "+err.Error())
		return
	}
	if report == nil {
		respondWithError(w, http.StatusNotFound, "crash report not found")
		return
	}

	respondWithJSON(w, http.StatusOK, report)
}

// HandleAnalyzeCrash runs heuristic diagnostics on provided or recent server logs.
func (h *Handler) HandleAnalyzeCrash(w http.ResponseWriter, r *http.Request) {
	var req AnalyzeRequest
	_ = json.NewDecoder(r.Body).Decode(&req)

	rawLog := strings.TrimSpace(req.Log)
	if rawLog == "" && h.mc != nil {
		rawLog = strings.Join(h.mc.GetHistory(), "\n")
	}

	if rawLog == "" {
		respondWithError(w, http.StatusBadRequest, "no log content provided and server history is empty")
		return
	}

	result := crash.Analyze(rawLog)

	report := &database.CrashReport{
		Source:         "manual",
		Category:       result.Category,
		Title:          result.Title,
		Culprit:        result.Culprit,
		Summary:        result.Summary,
		Recommendation: result.Recommendation,
		RawLog:         rawLog,
		CreatedAt:      time.Now().UTC(),
	}

	if req.Save && h.store != nil {
		_ = h.store.RecordCrashReport(report)
	}

	respondWithJSON(w, http.StatusOK, map[string]interface{}{
		"analysis": result,
		"report":   report,
	})
}

// HandleDeleteCrashReport deletes a specific crash report or clears all crash reports.
func (h *Handler) HandleDeleteCrashReport(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		respondWithError(w, http.StatusInternalServerError, "database store is not available")
		return
	}

	idStr := r.PathValue("id")
	if idStr == "" {
		idStr = r.URL.Query().Get("id")
	}

	if idStr == "" || idStr == "all" {
		if err := h.store.ClearCrashReports(); err != nil {
			respondWithError(w, http.StatusInternalServerError, "failed to clear crash reports: "+err.Error())
			return
		}
		h.recordAudit(r, "crash.clear", http.StatusOK, "Cleared all crash reports")
		respondWithJSON(w, http.StatusOK, map[string]string{"status": "cleared"})
		return
	}

	id, err := strconv.Atoi(idStr)
	if err != nil || id <= 0 {
		respondWithError(w, http.StatusBadRequest, "invalid crash report id")
		return
	}

	if err := h.store.DeleteCrashReport(id); err != nil {
		respondWithError(w, http.StatusInternalServerError, "failed to delete crash report: "+err.Error())
		return
	}

	h.recordAudit(r, "crash.delete", http.StatusOK, fmt.Sprintf("Deleted crash report #%d", id))
	respondWithJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// HandleGetAISettings returns the current AI diagnostics settings with the API key masked.
func (h *Handler) HandleGetAISettings(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		respondWithError(w, http.StatusInternalServerError, "database store is not available")
		return
	}

	settings, err := h.store.GetAISettings()
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "failed to get AI settings: "+err.Error())
		return
	}

	maskedKey, hasKey := maskAPIKey(settings.APIKey)
	respondWithJSON(w, http.StatusOK, AISettingsResponse{
		Provider:   settings.Provider,
		APIKey:     maskedKey,
		HasAPIKey:  hasKey,
		Model:      settings.Model,
		BaseURL:    settings.BaseURL,
		IsEnabled:  settings.IsEnabled,
		UpdatedAt:  settings.UpdatedAt,
	})
}

// HandleSaveAISettings updates AI configuration, preserving existing API key if masked.
func (h *Handler) HandleSaveAISettings(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		respondWithError(w, http.StatusInternalServerError, "database store is not available")
		return
	}

	var req database.AISettings
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	existing, err := h.store.GetAISettings()
	if err == nil && existing != nil {
		// If api_key in req is empty or masked, keep the existing one
		cleanKey := strings.TrimSpace(req.APIKey)
		if cleanKey == "" || strings.Contains(cleanKey, "****") {
			req.APIKey = existing.APIKey
		}
	}

	if req.Provider == "" {
		req.Provider = "openai"
	}
	if req.Model == "" {
		if req.Provider == "gemini" {
			req.Model = "gemini-1.5-flash"
		} else {
			req.Model = "gpt-4o-mini"
		}
	}

	if err := h.store.SaveAISettings(&req); err != nil {
		respondWithError(w, http.StatusInternalServerError, "failed to save AI settings: "+err.Error())
		return
	}

	h.recordAudit(r, "crash.ai_config_update", http.StatusOK, fmt.Sprintf("Updated AI diagnostics config: provider=%s, model=%s, enabled=%t", req.Provider, req.Model, req.IsEnabled))

	maskedKey, hasKey := maskAPIKey(req.APIKey)
	respondWithJSON(w, http.StatusOK, AISettingsResponse{
		Provider:   req.Provider,
		APIKey:     maskedKey,
		HasAPIKey:  hasKey,
		Model:      req.Model,
		BaseURL:    req.BaseURL,
		IsEnabled:  req.IsEnabled,
		UpdatedAt:  time.Now().UTC(),
	})
}

// HandleAIExplainCrash requests an external LLM explanation for a crash report.
func (h *Handler) HandleAIExplainCrash(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		respondWithError(w, http.StatusInternalServerError, "database store is not available")
		return
	}
	if h.aiClient == nil {
		respondWithError(w, http.StatusInternalServerError, "AI client is not initialized")
		return
	}

	var req AIExplainRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	settings, err := h.store.GetAISettings()
	if err != nil || settings == nil || !settings.IsEnabled {
		respondWithError(w, http.StatusBadRequest, "AI diagnostics is currently disabled. Please enable it and configure an API key in AI Settings.")
		return
	}

	var targetReport *database.CrashReport
	if req.ReportID > 0 {
		targetReport, err = h.store.GetCrashReport(req.ReportID)
		if err != nil || targetReport == nil {
			respondWithError(w, http.StatusNotFound, "referenced crash report not found")
			return
		}
	} else if strings.TrimSpace(req.RawLog) != "" {
		analysis := crash.Analyze(req.RawLog)
		targetReport = &database.CrashReport{
			Source:         "manual",
			Category:       analysis.Category,
			Title:          analysis.Title,
			Culprit:        analysis.Culprit,
			Summary:        analysis.Summary,
			Recommendation: analysis.Recommendation,
			RawLog:         req.RawLog,
			CreatedAt:      time.Now().UTC(),
		}
	} else if h.mc != nil {
		rawLog := strings.Join(h.mc.GetHistory(), "\n")
		analysis := crash.Analyze(rawLog)
		targetReport = &database.CrashReport{
			Source:         "runtime",
			Category:       analysis.Category,
			Title:          analysis.Title,
			Culprit:        analysis.Culprit,
			Summary:        analysis.Summary,
			Recommendation: analysis.Recommendation,
			RawLog:         rawLog,
			CreatedAt:      time.Now().UTC(),
		}
	} else {
		respondWithError(w, http.StatusBadRequest, "no report_id or raw_log provided for AI explanation")
		return
	}

	explanation, err := h.aiClient.ExplainCrash(r.Context(), settings, targetReport, req.CustomPrompt)
	if err != nil {
		respondWithError(w, http.StatusBadGateway, "AI diagnostic query failed: "+err.Error())
		return
	}

	h.recordAudit(r, "crash.ai_explain", http.StatusOK, fmt.Sprintf("Generated AI explanation for %s (%s)", targetReport.Category, settings.Provider))

	respondWithJSON(w, http.StatusOK, map[string]interface{}{
		"explanation": explanation,
		"provider":    settings.Provider,
		"model":       settings.Model,
	})
}
