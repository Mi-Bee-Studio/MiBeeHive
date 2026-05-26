package handler

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/Mi-Bee-Studio/mibeehive/internal/model"
	"github.com/Mi-Bee-Studio/mibeehive/internal/service"
)

// LogHandler handles log-related API endpoints.
type LogHandler struct {
	logService *service.LogService
}

// NewLogHandler creates a new LogHandler.
func NewLogHandler(svc *service.LogService) *LogHandler {
	return &LogHandler{logService: svc}
}

// HandleLogList handles GET /api/v1/admin/logs?type={crawl|app|download}&limit=50&offset=0.
func (h *LogHandler) HandleLogList(w http.ResponseWriter, r *http.Request) {
	logType := r.URL.Query().Get("type")
	if logType == "" {
		logType = "crawl"
	}

	// Validate log type early.
	if logType != "crawl" && logType != "download" && logType != "app" {
		writeJSON(w, http.StatusBadRequest, model.ApiResponse[any]{
			Success: false,
			Message: fmt.Sprintf("invalid log type %q: must be crawl, download, or app", logType),
		})
		return
	}

	limit := 50
	if v := r.URL.Query().Get("limit"); v != "" {
		if l, err := strconv.Atoi(v); err == nil && l > 0 {
			limit = l
		}
	}
	if limit > 200 {
		limit = 200
	}

	offset := 0
	if v := r.URL.Query().Get("offset"); v != "" {
		if o, err := strconv.Atoi(v); err == nil && o >= 0 {
			offset = o
		}
	}

	entries, total, err := h.logService.GetRecentLogsPaginated(r.Context(), logType, limit, offset)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ApiResponse[any]{
			Success: false,
			Message: fmt.Sprintf("failed to get logs: %v", err),
		})
		return
	}

	w.Header().Set("X-Total-Count", strconv.Itoa(total))
	writeJSON(w, http.StatusOK, model.ApiResponse[[]model.LogEntry]{
		Success: true,
		Data:    entries,
	})
}
