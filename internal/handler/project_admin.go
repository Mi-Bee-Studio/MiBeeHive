package handler

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/Mi-Bee-Studio/mibeehive/internal/config"
	"github.com/Mi-Bee-Studio/mibeehive/internal/crawler"
	"github.com/Mi-Bee-Studio/mibeehive/internal/middleware"
	"github.com/Mi-Bee-Studio/mibeehive/internal/model"

	dbrepo "github.com/Mi-Bee-Studio/mibeehive/internal/db"
)

// ProjectAdminHandler handles admin project CRUD endpoints.
type ProjectAdminHandler struct {
	projectRepo  *dbrepo.ProjectRepo
	fileRepo     *dbrepo.FileRepo
	crawlManager *crawler.CrawlManager
	config       *config.Config
}

func NewProjectAdminHandler(projectRepo *dbrepo.ProjectRepo, fileRepo *dbrepo.FileRepo, crawlManager *crawler.CrawlManager, cfg *config.Config) *ProjectAdminHandler {
	return &ProjectAdminHandler{
		projectRepo:  projectRepo,
		fileRepo:     fileRepo,
		crawlManager: crawlManager,
		config:       cfg,
	}
}

func (h *ProjectAdminHandler) ListProjects(w http.ResponseWriter, r *http.Request) {
	projects, err := h.projectRepo.ListAll(r.Context())
	if err != nil {
		middleware.WriteError(w, http.StatusInternalServerError, model.ERR_INTERNAL, "操作失败，请稍后重试", err)
		return
	}

	counts, _ := h.fileRepo.CountByProjects(r.Context())

	var result []model.AdminProjectResponse
	for _, p := range projects {
		resp := model.AdminProjectResponse{
			ID:            p.ID,
			Name:          p.Name,
			DisplayName:   p.DisplayName,
			SourceType:    p.SourceType,
			SourceURL:     p.SourceURL,
			Enabled:       p.Enabled,
			LatestVersion: p.LatestVersion,
			CreatedAt:     p.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
			Config:        json.RawMessage(p.Config),
			FileCount:     counts[p.ID],
			VersionPattern: crawler.SourceTypeVersionPattern(p.SourceType),
		}
		if p.LastCrawledAt != nil {
			s := p.LastCrawledAt.Format("2006-01-02T15:04:05Z07:00")
			resp.LastCrawledAt = &s
		}
		result = append(result, resp)
	}
	if result == nil {
		result = []model.AdminProjectResponse{}
	}

	writeJSON(w, http.StatusOK, model.ApiResponse[[]model.AdminProjectResponse]{
		Success: true,
		Data:    result,
	})
}

// GetProject handles GET /api/v1/admin/projects/{id}.
func (h *ProjectAdminHandler) GetProject(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, model.ApiResponse[any]{
			Success: false,
			Message: "invalid project id",
		})
		return
	}

	project, err := h.projectRepo.GetByID(r.Context(), id)
	if err != nil {
		middleware.WriteError(w, http.StatusInternalServerError, model.ERR_INTERNAL, "操作失败，请稍后重试", err)
		return
	}
	if project == nil {
		writeJSON(w, http.StatusNotFound, model.ApiResponse[any]{
			Success: false,
			Message: "project not found",
		})
		return
	}
	count, _ := h.fileRepo.CountByProject(r.Context(), id)

	resp := model.AdminProjectResponse{
		ID:            project.ID,
		Name:          project.Name,
		DisplayName:   project.DisplayName,
		SourceType:    project.SourceType,
		SourceURL:     project.SourceURL,
		Enabled:       project.Enabled,
		LatestVersion: project.LatestVersion,
		CreatedAt:     project.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		Config:        json.RawMessage(project.Config),
		FileCount:     count,
		VersionPattern: crawler.SourceTypeVersionPattern(project.SourceType),
	}
	if project.LastCrawledAt != nil {
		s := project.LastCrawledAt.Format("2006-01-02T15:04:05Z07:00")
		resp.LastCrawledAt = &s
	}

	writeJSON(w, http.StatusOK, model.ApiResponse[model.AdminProjectResponse]{
		Success: true,
		Data:    resp,
	})
}

// CreateProject handles POST /api/v1/admin/projects.
func (h *ProjectAdminHandler) CreateProject(w http.ResponseWriter, r *http.Request) {
	var req model.CreateProjectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, model.ApiResponse[any]{
			Success: false,
			Message: "invalid request body",
		})
		return
	}

	if req.Name == "" || req.SourceType == "" {
		writeJSON(w, http.StatusBadRequest, model.ApiResponse[any]{
			Success: false,
			Message: "name and source_type are required",
		})
		return
	}

	if len(req.Name) > 255 {
		writeJSON(w, http.StatusBadRequest, model.ApiResponse[any]{
			Success: false,
			Message: "project name must not exceed 255 characters",
		})
		return
	}

	project, err := h.projectRepo.CreateWithSettings(r.Context(), req.Name, req.DisplayName, string(req.SourceType), req.SourceURL, req.Settings)
	if err != nil {
		middleware.WriteError(w, http.StatusInternalServerError, model.ERR_INTERNAL, "操作失败，请稍后重试", err)
		return
	}

	writeJSON(w, http.StatusCreated, model.ApiResponse[model.AdminProjectResponse]{
		Success: true,
		Data: model.AdminProjectResponse{
			ID:            project.ID,
			Name:          project.Name,
			DisplayName:   project.DisplayName,
			SourceType:    project.SourceType,
			SourceURL:     project.SourceURL,
			Enabled:       project.Enabled,
			LatestVersion: project.LatestVersion,
			CreatedAt:     project.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
			Config:        json.RawMessage(project.Config),
			VersionPattern: crawler.SourceTypeVersionPattern(project.SourceType),
		},
	})
}

// UpdateProject handles PUT /api/v1/admin/projects/{id}.
func (h *ProjectAdminHandler) UpdateProject(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, model.ApiResponse[any]{
			Success: false,
			Message: "invalid project id",
		})
		return
	}

	var req model.UpdateProjectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, model.ApiResponse[any]{
			Success: false,
			Message: "invalid request body",
		})
		return
	}

	if err := h.projectRepo.UpdateProject(r.Context(), id, req.Name, req.DisplayName, string(req.SourceType), req.SourceURL, req.Settings); err != nil {
		middleware.WriteError(w, http.StatusInternalServerError, model.ERR_INTERNAL, "操作失败，请稍后重试", err)
		return
	}

	writeJSON(w, http.StatusOK, model.ApiResponse[any]{
		Success: true,
		Message: "project updated",
	})
}

// DeleteProject handles DELETE /api/v1/admin/projects/{id}.
func (h *ProjectAdminHandler) DeleteProject(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, model.ApiResponse[any]{
			Success: false,
			Message: "invalid project id",
		})
		return
	}

	// Fetch project before deleting, so we can stop scheduler and clean disk.
	project, _ := h.projectRepo.GetByID(r.Context(), id)

	if err := h.projectRepo.Delete(r.Context(), id); err != nil {
		middleware.WriteError(w, http.StatusInternalServerError, model.ERR_INTERNAL, "操作失败，请稍后重试", err)
		return
	}

	if project != nil {
		h.crawlManager.Scheduler().StopProject(project.Name)

		// Remove the project directory from disk.
		projectDir := filepath.Join(h.config.Storage.BasePath, project.Name)
		if err := os.RemoveAll(projectDir); err != nil && !os.IsNotExist(err) {
			slog.Warn("failed to remove project dir", "path", projectDir, "error", err)
		}
	}

	writeJSON(w, http.StatusOK, model.ApiResponse[any]{
		Success: true,
		Message: "project deleted",
	})
}

// ToggleProject handles PATCH /api/v1/admin/projects/{id}/toggle.
func (h *ProjectAdminHandler) ToggleProject(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, model.ApiResponse[any]{
			Success: false,
			Message: "invalid project id",
		})
		return
	}

	project, err := h.projectRepo.GetByID(r.Context(), id)
	if err != nil || project == nil {
		writeJSON(w, http.StatusNotFound, model.ApiResponse[any]{
			Success: false,
			Message: "project not found",
		})
		return
	}

	newEnabled := !project.Enabled
	if err := h.projectRepo.SetEnabled(r.Context(), id, newEnabled); err != nil {
		middleware.WriteError(w, http.StatusInternalServerError, model.ERR_INTERNAL, "操作失败，请稍后重试", err)
		return
	}

	if newEnabled {
		// Resume scheduling.
		h.crawlManager.Scheduler().StartProject(project.Name, 0, func(ctx context.Context) error {
			_, err := h.crawlManager.TriggerCrawl(ctx, project.Name)
			return err
		})
	} else {
		h.crawlManager.Scheduler().StopProject(project.Name)
	}

	writeJSON(w, http.StatusOK, model.ApiResponse[map[string]bool]{
		Success: true,
		Data:    map[string]bool{"enabled": newEnabled},
	})
}
