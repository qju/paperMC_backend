package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"paperMC_backend/internal/database"
	"paperMC_backend/internal/minecraft"
	"paperMC_backend/internal/profiler"
)

type ProfilerHealthRequest struct {
	Log  string `json:"log"`
	Save bool   `json:"save"`
}

type TriggerProfilerRequest struct {
	Action  string `json:"action"`  // "health", "sampler_start", "sampler_stop", "timings_paste", "timings_reset", "custom"
	Command string `json:"command"` // for "custom" action
}

// HandleListProfilerReports retrieves recorded spark and timings reports with pagination and type filtering.
func (h *Handler) HandleListProfilerReports(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		respondWithError(w, http.StatusInternalServerError, "database store is not available")
		return
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

	reportType := strings.TrimSpace(r.URL.Query().Get("type"))

	reports, total, err := h.store.ListProfilerReports(limit, offset, reportType)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "failed to query profiler reports: "+err.Error())
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]interface{}{
		"reports": reports,
		"total":   total,
		"limit":   limit,
		"offset":  offset,
	})
}

// HandleGetProfilerReport retrieves a single profiler report by ID.
func (h *Handler) HandleGetProfilerReport(w http.ResponseWriter, r *http.Request) {
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
		respondWithError(w, http.StatusBadRequest, "invalid profiler report id")
		return
	}

	report, err := h.store.GetProfilerReport(id)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "failed to fetch profiler report: "+err.Error())
		return
	}
	if report == nil {
		respondWithError(w, http.StatusNotFound, "profiler report not found")
		return
	}

	respondWithJSON(w, http.StatusOK, report)
}

// HandleDeleteProfilerReport deletes a specific profiler report or clears all reports.
func (h *Handler) HandleDeleteProfilerReport(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		respondWithError(w, http.StatusInternalServerError, "database store is not available")
		return
	}

	idStr := r.PathValue("id")
	if idStr == "" {
		idStr = r.URL.Query().Get("id")
	}

	if idStr == "" || idStr == "all" {
		if err := h.store.ClearProfilerReports(); err != nil {
			respondWithError(w, http.StatusInternalServerError, "failed to clear profiler reports: "+err.Error())
			return
		}
		h.recordAudit(r, "profiler.clear", http.StatusOK, "Cleared all profiler reports")
		respondWithJSON(w, http.StatusOK, map[string]string{"status": "cleared"})
		return
	}

	id, err := strconv.Atoi(idStr)
	if err != nil || id <= 0 {
		respondWithError(w, http.StatusBadRequest, "invalid profiler report id")
		return
	}

	if err := h.store.DeleteProfilerReport(id); err != nil {
		respondWithError(w, http.StatusInternalServerError, "failed to delete profiler report: "+err.Error())
		return
	}

	h.recordAudit(r, "profiler.delete", http.StatusOK, fmt.Sprintf("Deleted profiler report #%d", id))
	respondWithJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// HandleProfilerHealth generates or parses a performance health snapshot with actionable advice.
func (h *Handler) HandleProfilerHealth(w http.ResponseWriter, r *http.Request) {
	var req ProfilerHealthRequest
	_ = json.NewDecoder(r.Body).Decode(&req)

	rawLog := strings.TrimSpace(req.Log)
	if rawLog == "" && h.mc != nil {
		history := h.mc.GetHistory()
		var sparkLines []string
		inSparkBlock := false
		for i := len(history) - 1; i >= 0; i-- {
			line := history[i]
			if strings.Contains(line, "TPS from last") || strings.Contains(line, "Tick durations") || strings.Contains(line, "CPU usage") {
				inSparkBlock = true
			}
			if inSparkBlock {
				sparkLines = append([]string{line}, sparkLines...)
				if len(sparkLines) >= 15 {
					break
				}
			}
		}
		if len(sparkLines) > 0 {
			rawLog = strings.Join(sparkLines, "\n")
		}
	}

	var snapshot *profiler.HealthSnapshot
	if rawLog != "" {
		snapshot = profiler.ParseHealth(rawLog)
	} else if h.mc != nil {
		vitals := h.mc.GetVitals()
		snapshot = &profiler.HealthSnapshot{
			TPS:           fmt.Sprintf("%.2f", vitals.TPS),
			MSPT:          fmt.Sprintf("%.2f ms", vitals.MSPT),
			CPUProcess:    fmt.Sprintf("%.1f%%", vitals.CPU),
			CPUSystem:     fmt.Sprintf("%.1f%%", vitals.SystemCPU),
			MemoryUsed:    fmt.Sprintf("%d MB", vitals.RAM/(1024*1024)),
			MemoryMax:     vitals.TotalMemory,
			DiskUsage:     fmt.Sprintf("%.1f%%", vitals.DiskUsedPct),
			Raw:           fmt.Sprintf("Live Server Vitals - TPS: %.2f, MSPT: %.2f ms, CPU: %.1f%%", vitals.TPS, vitals.MSPT, vitals.CPU),
		}
		snapshot.Advice = profiler.GenerateAdvice(snapshot)
	} else {
		snapshot = &profiler.HealthSnapshot{
			Advice: []string{"Server vitals are not available."},
		}
	}

	if req.Save && h.store != nil {
		summary := fmt.Sprintf("TPS: %s | MSPT: %s | CPU: %s", snapshot.TPS, snapshot.MSPT, snapshot.CPUProcess)
		report := &database.ProfilerReport{
			ReportType: "spark_health",
			Title:      "Spark Health Snapshot",
			Summary:    summary,
			RawOutput:  snapshot.Raw,
			CreatedAt:  time.Now().UTC(),
		}
		_ = h.store.RecordProfilerReport(report)
	}

	respondWithJSON(w, http.StatusOK, map[string]interface{}{
		"health": snapshot,
	})
}

// HandleTriggerProfiler dispatches spark or timings commands directly to the running server.
func (h *Handler) HandleTriggerProfiler(w http.ResponseWriter, r *http.Request) {
	if h.mc == nil {
		respondWithError(w, http.StatusInternalServerError, "server instance not available")
		return
	}

	if h.mc.GetStatus() != minecraft.StatusRunning {
		respondWithError(w, http.StatusBadRequest, "Minecraft server must be running to execute profiler commands")
		return
	}

	var req TriggerProfilerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	var resolvedCmd string
	switch req.Action {
	case "health":
		resolvedCmd = "spark health"
	case "sampler_start":
		resolvedCmd = "spark sampler start"
	case "sampler_stop":
		resolvedCmd = "spark sampler stop"
	case "timings_paste":
		resolvedCmd = "timings paste"
	case "timings_reset":
		resolvedCmd = "timings reset"
	case "custom":
		cleanCmd := strings.TrimSpace(req.Command)
		cleanCmd = strings.TrimPrefix(cleanCmd, "/")
		lower := strings.ToLower(cleanCmd)
		if !strings.HasPrefix(lower, "spark") && !strings.HasPrefix(lower, "timings") {
			respondWithError(w, http.StatusBadRequest, "custom command must start with 'spark' or 'timings'")
			return
		}
		resolvedCmd = cleanCmd
	default:
		respondWithError(w, http.StatusBadRequest, "invalid profiler action; must be health, sampler_start, sampler_stop, timings_paste, timings_reset, or custom")
		return
	}

	if err := h.mc.SendCommand(resolvedCmd); err != nil {
		respondWithError(w, http.StatusInternalServerError, "failed to send profiler command: "+err.Error())
		return
	}

	h.recordAudit(r, "profiler.trigger", http.StatusOK, "Triggered profiler command: "+resolvedCmd)
	respondWithJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "triggered",
		"action":  req.Action,
		"command": resolvedCmd,
	})
}
