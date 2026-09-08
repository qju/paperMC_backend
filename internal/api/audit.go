package api

import (
	"math"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"paperMC_backend/internal/auth"
	"paperMC_backend/internal/database"
)

// AuditLogsResponse formats paginated audit log results.
type AuditLogsResponse struct {
	Logs       []database.AuditLog `json:"logs"`
	Total      int                 `json:"total"`
	Page       int                 `json:"page"`
	Limit      int                 `json:"limit"`
	TotalPages int                 `json:"total_pages"`
}

// getClientIP extracts the real client IP from standard proxy headers or remote address.
func getClientIP(r *http.Request) string {
	if r == nil {
		return "127.0.0.1"
	}

	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		first := strings.TrimSpace(parts[0])
		if first != "" {
			return first
		}
	}

	if xrip := strings.TrimSpace(r.Header.Get("X-Real-IP")); xrip != "" {
		return xrip
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil && host != "" {
		return host
	}

	if r.RemoteAddr != "" {
		return r.RemoteAddr
	}

	return "127.0.0.1"
}

// recordAuditWithUser records an administrative audit entry with an explicit username.
func (h *Handler) recordAuditWithUser(username string, r *http.Request, action string, statusCode int, details string) {
	if h == nil || h.store == nil {
		return
	}

	endpoint := ""
	method := "SYSTEM"
	ip := "127.0.0.1"

	if r != nil {
		endpoint = r.URL.Path
		method = r.Method
		ip = getClientIP(r)
	}

	cleanUser := strings.TrimSpace(username)
	if cleanUser == "" {
		cleanUser = "anonymous"
	}

	entry := &database.AuditLog{
		Username:   cleanUser,
		Action:     action,
		Endpoint:   endpoint,
		Method:     method,
		Details:    details,
		IPAddress:  ip,
		StatusCode: statusCode,
		CreatedAt:  time.Now(),
	}

	_ = h.store.RecordAuditLog(entry)
}

// recordAudit records an administrative audit entry extracting authenticated user context from request.
func (h *Handler) recordAudit(r *http.Request, action string, statusCode int, details string) {
	username := ""
	if r != nil {
		username = auth.GetUsername(r)
	}
	if username == "" {
		username = "system"
	}
	h.recordAuditWithUser(username, r, action, statusCode, details)
}

// HandleGetAuditLogs queries and paginates administrative audit records.
func (h *Handler) HandleGetAuditLogs(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		respondWithError(w, http.StatusInternalServerError, "Database store not configured")
		return
	}

	page := 1
	if pStr := r.URL.Query().Get("page"); pStr != "" {
		if p, err := strconv.Atoi(pStr); err == nil && p > 0 {
			page = p
		}
	}

	limit := 50
	if lStr := r.URL.Query().Get("limit"); lStr != "" {
		if l, err := strconv.Atoi(lStr); err == nil && l > 0 {
			limit = l
			if limit > 100 {
				limit = 100
			}
		}
	}

	actionFilter := strings.TrimSpace(r.URL.Query().Get("action"))
	userFilter := strings.TrimSpace(r.URL.Query().Get("username"))

	offset := (page - 1) * limit
	logs, total, err := h.store.ListAuditLogs(limit, offset, actionFilter, userFilter)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to retrieve audit logs: "+err.Error())
		return
	}

	totalPages := 0
	if total > 0 {
		totalPages = int(math.Ceil(float64(total) / float64(limit)))
	}

	respondWithJSON(w, http.StatusOK, AuditLogsResponse{
		Logs:       logs,
		Total:      total,
		Page:       page,
		Limit:      limit,
		TotalPages: totalPages,
	})
}

// HandleClearAuditLogs clears all stored audit logs.
func (h *Handler) HandleClearAuditLogs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		respondWithError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	if h.store == nil {
		respondWithError(w, http.StatusInternalServerError, "Database store not configured")
		return
	}

	if err := h.store.ClearAuditLogs(); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to clear audit logs: "+err.Error())
		return
	}

	// Record that logs were purged
	h.recordAudit(r, "audit.clear", http.StatusOK, "Purged administrative audit log history")

	respondWithJSON(w, http.StatusOK, map[string]string{
		"status": "Audit logs cleared successfully",
	})
}
