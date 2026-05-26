package handler

import (
	"database/sql"
	"net/http"
	"strconv"

	dbrepo "github.com/Mi-Bee-Studio/mibeehive/internal/db"
	"github.com/Mi-Bee-Studio/mibeehive/internal/middleware"
	"github.com/Mi-Bee-Studio/mibeehive/internal/model"
)

// ProjectHandler handles project-related API endpoints.
type ProjectHandler struct {
	projectRepo *dbrepo.ProjectRepo
	fileRepo    *dbrepo.FileRepo
}

// NewProjectHandler creates a new ProjectHandler.
func NewProjectHandler(db *sql.DB) *ProjectHandler {
	return &ProjectHandler{
		projectRepo: dbrepo.NewProjectRepo(db),
		fileRepo:    dbrepo.NewFileRepo(db),
	}
}

// List handles GET /api/v1/projects.
func (h *ProjectHandler) List(w http.ResponseWriter, r *http.Request) {
	projects, err := h.projectRepo.List(r.Context())
	if err != nil {
		middleware.WriteError(w, http.StatusInternalServerError, model.ERR_INTERNAL, "Failed to list projects", err)
		return
	}

	resp := make([]model.ProjectResponse, 0, len(projects))
	for _, p := range projects {
		count, err := h.fileRepo.CountByProject(r.Context(), p.ID)
		if err != nil {
			count = 0
		}
		resp = append(resp, toProjectResponse(p, count))
	}

	writeJSON(w, http.StatusOK, model.ApiResponse[[]model.ProjectResponse]{
		Success: true,
		Data:    resp,
	})
}

func (h *ProjectHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, model.ApiResponse[any]{
			Success: false,
			Message: "invalid project id",
		})
		return
	}

	proj, err := h.projectRepo.GetByID(r.Context(), id)
	if err != nil {
		middleware.WriteError(w, http.StatusInternalServerError, model.ERR_INTERNAL, "Failed to get project", err)
		return
	}
	if proj == nil {
		writeJSON(w, http.StatusNotFound, model.ApiResponse[any]{
			Success: false,
			Message: "project not found",
		})
		return
	}

	count, err := h.fileRepo.CountByProject(r.Context(), proj.ID)
	if err != nil {
		count = 0
	}

	writeJSON(w, http.StatusOK, model.ApiResponse[model.ProjectResponse]{
		Success: true,
		Data:    toProjectResponse(proj, count),
	})
}

func toProjectResponse(p *dbrepo.Project, fileCount int) model.ProjectResponse {
	var lastCrawled *string
	if p.LastCrawledAt != nil {
		s := p.LastCrawledAt.Format("2006-01-02T15:04:05Z07:00")
		lastCrawled = &s
	}
	return model.ProjectResponse{
		ID:            int(p.ID),
		Name:          p.Name,
		DisplayName:   p.DisplayName,
		SourceType:    model.SourceType(p.SourceType),
		SourceURL:     p.SourceURL,
		LatestVersion: p.LatestVersion,
		LastCrawledAt: lastCrawled,
		CreatedAt:     p.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		FileCount:     fileCount,
	}
}
