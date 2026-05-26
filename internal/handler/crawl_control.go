package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/Mi-Bee-Studio/mibeehive/internal/crawler"
	"github.com/Mi-Bee-Studio/mibeehive/internal/middleware"
	"github.com/Mi-Bee-Studio/mibeehive/internal/model"

	dbrepo "github.com/Mi-Bee-Studio/mibeehive/internal/db"
)

// CrawlControlHandler handles admin crawl control and credential endpoints.
type CrawlControlHandler struct {
	projectRepo  *dbrepo.ProjectRepo
	credRepo     *dbrepo.SourceCredentialRepo
	crawlManager *crawler.CrawlManager
}

// NewCrawlControlHandler creates a new CrawlControlHandler.
func NewCrawlControlHandler(projectRepo *dbrepo.ProjectRepo, credRepo *dbrepo.SourceCredentialRepo, crawlManager *crawler.CrawlManager) *CrawlControlHandler {
	return &CrawlControlHandler{
		projectRepo:  projectRepo,
		credRepo:     credRepo,
		crawlManager: crawlManager,
	}
}

// TriggerCrawl handles POST /api/v1/admin/crawl/trigger/{name}.
func (h *CrawlControlHandler) TriggerCrawl(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		writeJSON(w, http.StatusBadRequest, model.ApiResponse[any]{
			Success: false,
			Message: "project name is required",
		})
		return
	}

	result, err := h.crawlManager.TriggerCrawl(r.Context(), name)
	if err != nil {
		statusCode := http.StatusInternalServerError
		msg := err.Error()
		if result != nil && result.Status == model.CrawlStatusError {
			msg = result.Error.Error()
		}
		writeJSON(w, statusCode, model.ApiResponse[any]{
			Success: false,
			Message: msg,
		})
		return
	}

	writeJSON(w, http.StatusOK, model.ApiResponse[model.CrawlResult]{
		Success: true,
		Data:    *result,
	})
}

// TriggerAllCrawls handles POST /api/v1/admin/crawl/trigger-all.
func (h *CrawlControlHandler) TriggerAllCrawls(w http.ResponseWriter, r *http.Request) {
	results := h.crawlManager.TriggerAllCrawls(r.Context())
	writeJSON(w, http.StatusOK, model.ApiResponse[[]model.CrawlResult]{
		Success: true,
		Data:    results,
	})
}

// PauseProject handles POST /api/v1/admin/crawl/pause/{name}.
func (h *CrawlControlHandler) PauseProject(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		writeJSON(w, http.StatusBadRequest, model.ApiResponse[any]{
			Success: false,
			Message: "project name is required",
		})
		return
	}

	h.crawlManager.Scheduler().StopProject(name)
	slog.Info("project crawl paused via admin API", "project", name)

	writeJSON(w, http.StatusOK, model.ApiResponse[any]{
		Success: true,
		Message: fmt.Sprintf("project %q paused", name),
	})
}

// ResumeProject handles POST /api/v1/admin/crawl/resume/{name}.
func (h *CrawlControlHandler) ResumeProject(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		writeJSON(w, http.StatusBadRequest, model.ApiResponse[any]{
			Success: false,
			Message: "project name is required",
		})
		return
	}

	// Read interval from project settings.
	proj, err := h.projectRepo.GetByName(r.Context(), name)
	if err != nil || proj == nil {
		writeJSON(w, http.StatusNotFound, model.ApiResponse[any]{
			Success: false,
			Message: fmt.Sprintf("project %q not found", name),
		})
		return
	}

	h.crawlManager.Scheduler().StartProject(name, 0, func(ctx context.Context) error {
		_, err := h.crawlManager.TriggerCrawl(ctx, name)
		return err
	})
	slog.Info("project crawl resumed via admin API", "project", name)

	writeJSON(w, http.StatusOK, model.ApiResponse[any]{
		Success: true,
		Message: fmt.Sprintf("project %q resumed", name),
	})
}

// GetCrawlStatus handles GET /api/v1/admin/crawl/status.
func (h *CrawlControlHandler) GetCrawlStatus(w http.ResponseWriter, r *http.Request) {
	statuses := h.crawlManager.GetCrawlStatus()
	writeJSON(w, http.StatusOK, model.ApiResponse[map[string]crawler.CrawlStatusInfo]{
		Success: true,
		Data:    statuses,
	})
}

// === Credentials ===

// ListCredentials handles GET /api/v1/admin/credentials.
func (h *CrawlControlHandler) ListCredentials(w http.ResponseWriter, r *http.Request) {
	creds, err := h.credRepo.List(r.Context())
	if err != nil {
		middleware.WriteError(w, http.StatusInternalServerError, model.ERR_INTERNAL, "操作失败，请稍后重试", err)
		return
	}

	var result []model.CredentialResponse
	for _, c := range creds {
		token := maskToken(c.Token)
		result = append(result, model.CredentialResponse{
			ID:         c.ID,
			SourceType: c.SourceType,
			Token:      token,
			CreatedAt:  c.CreatedAt,
			UpdatedAt:  c.UpdatedAt,
		})
	}
	if result == nil {
		result = []model.CredentialResponse{}
	}

	writeJSON(w, http.StatusOK, model.ApiResponse[[]model.CredentialResponse]{
		Success: true,
		Data:    result,
	})
}

// UpsertCredential handles PUT /api/v1/admin/credentials.
func (h *CrawlControlHandler) UpsertCredential(w http.ResponseWriter, r *http.Request) {
	var req model.UpsertCredentialRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, model.ApiResponse[any]{
			Success: false,
			Message: "invalid request body",
		})
		return
	}

	if req.SourceType == "" || req.Token == "" {
		writeJSON(w, http.StatusBadRequest, model.ApiResponse[any]{
			Success: false,
			Message: "source_type and token are required",
		})
		return
	}

	// Validate source_type against known values.
	validSourceTypes := []string{
		string(model.SourceTypeGitHub),
		string(model.SourceTypeGo),
		string(model.SourceTypeHashiCorp),
		string(model.SourceTypeGrafana),
		string(model.SourceTypeNPM),
		string(model.SourceTypePyPI),
		string(model.SourceTypeCrates),
	}
	valid := false
	for _, st := range validSourceTypes {
		if req.SourceType == st {
			valid = true
			break
		}
	}
	if !valid {
		writeJSON(w, http.StatusBadRequest, model.ApiResponse[any]{
			Success: false,
			Message: "invalid source_type. Valid values: github, go, hashicorp, grafana, npm, pypi, crates",
		})
		return
	}

	if err := h.credRepo.Upsert(r.Context(), req.SourceType, req.Token); err != nil {
		middleware.WriteError(w, http.StatusInternalServerError, model.ERR_INTERNAL, "操作失败，请稍后重试", err)
		return
	}

	writeJSON(w, http.StatusOK, model.ApiResponse[any]{
		Success: true,
		Message: "credential saved",
	})
}
