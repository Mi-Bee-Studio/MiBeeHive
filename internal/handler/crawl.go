package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/Mi-Bee-Studio/mibeehive/internal/crawler"
	dbrepo "github.com/Mi-Bee-Studio/mibeehive/internal/db"
	"github.com/Mi-Bee-Studio/mibeehive/internal/middleware"
	"github.com/Mi-Bee-Studio/mibeehive/internal/model"
)

// CrawlHandler handles crawl-related API endpoints.
type CrawlHandler struct {
	crawlManager *crawler.CrawlManager
	crawlLogRepo *dbrepo.CrawlLogRepo
	projectRepo  *dbrepo.ProjectRepo
}

// NewCrawlHandler creates a new CrawlHandler.
func NewCrawlHandler(cm *crawler.CrawlManager, clr *dbrepo.CrawlLogRepo, pr *dbrepo.ProjectRepo) *CrawlHandler {
	return &CrawlHandler{crawlManager: cm, crawlLogRepo: clr, projectRepo: pr}
}

// Status handles GET /api/v1/crawl/status.
func (h *CrawlHandler) Status(w http.ResponseWriter, r *http.Request) {
	statuses := h.crawlManager.GetCrawlStatus()
	writeJSON(w, http.StatusOK, model.ApiResponse[map[string]crawler.CrawlStatusInfo]{
		Success: true,
		Data:    statuses,
	})
}

// Trigger handles POST /api/v1/crawl/trigger.
func (h *CrawlHandler) Trigger(w http.ResponseWriter, r *http.Request) {
	var req model.CrawlTriggerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		// Empty body is OK — triggers all projects.
		req = model.CrawlTriggerRequest{}
	}

	if req.ProjectName != "" {
		result, err := h.crawlManager.TriggerCrawl(r.Context(), req.ProjectName)
		if err != nil {
			if result != nil && result.Status == model.CrawlStatusRateLimited {
				middleware.WriteError(w, http.StatusTooManyRequests, "RATE_LIMITED", "Rate limited by upstream API, please try again later", err)
				return
			}
			msg := err.Error()
			if result != nil && result.Status == model.CrawlStatusError {
				msg = result.Error.Error()
			}
			middleware.WriteError(w, http.StatusInternalServerError, model.ERR_INTERNAL, msg, err)
			return
		}
		writeJSON(w, http.StatusOK, model.ApiResponse[model.CrawlResult]{
			Success: true,
			Data:    *result,
		})
		return
	}

	// Trigger all crawls.
	results := h.crawlManager.TriggerAllCrawls(r.Context())
	writeJSON(w, http.StatusOK, model.ApiResponse[[]model.CrawlResult]{
		Success: true,
		Data:    results,
	})
}

// ListLogs handles GET /api/v1/crawl/logs.
func (h *CrawlHandler) ListLogs(w http.ResponseWriter, r *http.Request) {
	limitStr := r.URL.Query().Get("limit")
	limit := 50
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil {
			limit = l
		}
	}
	logs, err := h.crawlLogRepo.ListRecent(r.Context(), limit)
	if err != nil {
		middleware.WriteError(w, http.StatusInternalServerError, model.ERR_INTERNAL, "Failed to list crawl logs", err)
		return
	}
	projects, err := h.projectRepo.List(r.Context())
	if err != nil {
		slog.Error("failed to list projects for crawl logs", "error", err)
	}
	projectMap := make(map[int64]string)
	for _, p := range projects {
		projectMap[p.ID] = p.Name
	}
	var result []model.CrawlLogResponse
	for _, l := range logs {
		var finishedAt *string
		if l.FinishedAt != nil {
			s := l.FinishedAt.Format("2006-01-02T15:04:05Z07:00")
			finishedAt = &s
		}
		result = append(result, model.CrawlLogResponse{
			ID:              l.ID,
			ProjectID:       l.ProjectID,
			ProjectName:     projectMap[l.ProjectID],
			StartedAt:       l.StartedAt.Format("2006-01-02T15:04:05Z07:00"),
			FinishedAt:      finishedAt,
			Status:          l.Status,
			VersionsFound:   l.VersionsFound,
			FilesDownloaded: l.FilesDownloaded,
		})
	}
	if result == nil {
		result = []model.CrawlLogResponse{}
	}
	writeJSON(w, http.StatusOK, model.ApiResponse[[]model.CrawlLogResponse]{Success: true, Data: result})
}
