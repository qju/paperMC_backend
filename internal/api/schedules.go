package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"paperMC_backend/internal/database"
	"paperMC_backend/internal/scheduler"
)

type CreateScheduleRequest struct {
	Name       string `json:"name"`
	CronExpr   string `json:"cron_expr"`
	ActionType string `json:"action_type"`
	Payload    string `json:"payload"`
	IsEnabled  *bool  `json:"is_enabled,omitempty"`
}

type UpdateScheduleRequest struct {
	ID         int    `json:"id"`
	Name       string `json:"name"`
	CronExpr   string `json:"cron_expr"`
	ActionType string `json:"action_type"`
	Payload    string `json:"payload"`
	IsEnabled  bool   `json:"is_enabled"`
}

type ScheduleActionRequest struct {
	ID int `json:"id"`
}

func isValidActionType(a string) bool {
	switch a {
	case "backup", "restart", "command", "broadcast", "start", "stop":
		return true
	default:
		return false
	}
}

func (h *Handler) HandleListSchedules(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		respondWithError(w, http.StatusInternalServerError, "Database store unavailable")
		return
	}

	schedules, err := h.store.ListSchedules()
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to query schedules: "+err.Error())
		return
	}

	// Populate next run times from active scheduler
	if h.scheduler != nil {
		for i := range schedules {
			if schedules[i].IsEnabled {
				schedules[i].NextRunAt = h.scheduler.GetNextRun(schedules[i].ID)
			}
		}
	}

	respondWithJSON(w, http.StatusOK, map[string]interface{}{
		"schedules": schedules,
	})
}

func (h *Handler) HandleCreateSchedule(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		respondWithError(w, http.StatusInternalServerError, "Database store unavailable")
		return
	}

	var req CreateScheduleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid JSON payload: "+err.Error())
		return
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		respondWithError(w, http.StatusBadRequest, "Schedule name cannot be empty")
		return
	}

	cronExpr := strings.TrimSpace(req.CronExpr)
	if err := scheduler.ValidateCronExpression(cronExpr); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid cron expression: "+err.Error())
		return
	}

	actionType := strings.ToLower(strings.TrimSpace(req.ActionType))
	if !isValidActionType(actionType) {
		respondWithError(w, http.StatusBadRequest, "Invalid action_type: must be 'backup', 'restart', 'command', 'broadcast', 'start', or 'stop'")
		return
	}

	isEnabled := true
	if req.IsEnabled != nil {
		isEnabled = *req.IsEnabled
	}

	sched := database.Schedule{
		Name:       name,
		CronExpr:   cronExpr,
		ActionType: actionType,
		Payload:    strings.TrimSpace(req.Payload),
		IsEnabled:  isEnabled,
	}

	if err := h.store.CreateSchedule(&sched); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to create schedule: "+err.Error())
		return
	}

	if h.scheduler != nil && sched.IsEnabled {
		_ = h.scheduler.RegisterSchedule(sched)
		sched.NextRunAt = h.scheduler.GetNextRun(sched.ID)
	}

	respondWithJSON(w, http.StatusCreated, sched)
}

func (h *Handler) HandleUpdateSchedule(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		respondWithError(w, http.StatusInternalServerError, "Database store unavailable")
		return
	}

	var req UpdateScheduleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid JSON payload: "+err.Error())
		return
	}

	if req.ID <= 0 {
		respondWithError(w, http.StatusBadRequest, "Valid schedule ID is required")
		return
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		respondWithError(w, http.StatusBadRequest, "Schedule name cannot be empty")
		return
	}

	cronExpr := strings.TrimSpace(req.CronExpr)
	if err := scheduler.ValidateCronExpression(cronExpr); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid cron expression: "+err.Error())
		return
	}

	actionType := strings.ToLower(strings.TrimSpace(req.ActionType))
	if !isValidActionType(actionType) {
		respondWithError(w, http.StatusBadRequest, "Invalid action_type")
		return
	}

	sched := database.Schedule{
		ID:         req.ID,
		Name:       name,
		CronExpr:   cronExpr,
		ActionType: actionType,
		Payload:    strings.TrimSpace(req.Payload),
		IsEnabled:  req.IsEnabled,
	}

	if err := h.store.UpdateSchedule(&sched); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to update schedule: "+err.Error())
		return
	}

	if h.scheduler != nil {
		if sched.IsEnabled {
			_ = h.scheduler.RegisterSchedule(sched)
			sched.NextRunAt = h.scheduler.GetNextRun(sched.ID)
		} else {
			h.scheduler.UnregisterSchedule(sched.ID)
		}
	}

	respondWithJSON(w, http.StatusOK, sched)
}

func (h *Handler) HandleToggleSchedule(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		respondWithError(w, http.StatusInternalServerError, "Database store unavailable")
		return
	}

	id := parseScheduleID(r)
	if id <= 0 {
		respondWithError(w, http.StatusBadRequest, "Valid schedule ID is required")
		return
	}

	sched, err := h.store.GetSchedule(id)
	if err != nil {
		respondWithError(w, http.StatusNotFound, "Schedule not found")
		return
	}

	newStatus := !sched.IsEnabled
	if err := h.store.ToggleSchedule(id, newStatus); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to toggle schedule: "+err.Error())
		return
	}

	sched.IsEnabled = newStatus
	if h.scheduler != nil {
		if newStatus {
			_ = h.scheduler.RegisterSchedule(*sched)
			sched.NextRunAt = h.scheduler.GetNextRun(id)
		} else {
			h.scheduler.UnregisterSchedule(id)
			sched.NextRunAt = nil
		}
	}

	respondWithJSON(w, http.StatusOK, sched)
}

func (h *Handler) HandleRunSchedule(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		respondWithError(w, http.StatusInternalServerError, "Database store unavailable")
		return
	}

	id := parseScheduleID(r)
	if id <= 0 {
		respondWithError(w, http.StatusBadRequest, "Valid schedule ID is required")
		return
	}

	if h.scheduler == nil {
		respondWithError(w, http.StatusInternalServerError, "Scheduler service not initialized")
		return
	}

	err := h.scheduler.ExecuteJob(id)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Task execution failed: "+err.Error())
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]interface{}{
		"status":      "Task executed successfully",
		"schedule_id": id,
	})
}

func (h *Handler) HandleDeleteSchedule(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		respondWithError(w, http.StatusInternalServerError, "Database store unavailable")
		return
	}

	id := parseScheduleID(r)
	if id <= 0 {
		respondWithError(w, http.StatusBadRequest, "Valid schedule ID is required")
		return
	}

	if h.scheduler != nil {
		h.scheduler.UnregisterSchedule(id)
	}

	if err := h.store.DeleteSchedule(id); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to delete schedule: "+err.Error())
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]interface{}{
		"status":      "Schedule deleted",
		"schedule_id": id,
	})
}

func (h *Handler) HandleGetScheduleLogs(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		respondWithError(w, http.StatusInternalServerError, "Database store unavailable")
		return
	}

	schedIDStr := r.URL.Query().Get("schedule_id")
	schedID, _ := strconv.Atoi(schedIDStr)

	limitStr := r.URL.Query().Get("limit")
	limit, _ := strconv.Atoi(limitStr)
	if limit <= 0 || limit > 500 {
		limit = 100
	}

	logs, err := h.store.ListScheduleLogs(schedID, limit)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to query schedule logs: "+err.Error())
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]interface{}{
		"logs": logs,
	})
}

func (h *Handler) HandleClearScheduleLogs(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		respondWithError(w, http.StatusInternalServerError, "Database store unavailable")
		return
	}

	schedIDStr := r.URL.Query().Get("schedule_id")
	schedID, _ := strconv.Atoi(schedIDStr)

	if err := h.store.ClearScheduleLogs(schedID); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to clear schedule logs: "+err.Error())
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]string{
		"status": "Schedule logs cleared successfully",
	})
}

func parseScheduleID(r *http.Request) int {
	idStr := r.URL.Query().Get("id")
	if idStr != "" {
		if id, err := strconv.Atoi(idStr); err == nil {
			return id
		}
	}

	if r.Body != nil && r.ContentLength > 0 {
		var req ScheduleActionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err == nil && req.ID > 0 {
			return req.ID
		}
	}

	return 0
}
