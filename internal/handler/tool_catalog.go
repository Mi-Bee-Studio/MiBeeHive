package handler

import (
	"errors"
	"net/http"

	"github.com/Mi-Bee-Studio/mibeehive/internal/db"
	"github.com/Mi-Bee-Studio/mibeehive/internal/middleware"
	"github.com/Mi-Bee-Studio/mibeehive/internal/model"
	"github.com/Mi-Bee-Studio/mibeehive/internal/service"
)

// ToolCatalogHandler serves the built-in tool catalog and the one-click
// enable flow that materializes a catalog entry as a crawl project.
type ToolCatalogHandler struct {
	svc  *service.ToolCatalogService
	repo *db.ProjectRepo
}

// NewToolCatalogHandler creates a ToolCatalogHandler.
func NewToolCatalogHandler(svc *service.ToolCatalogService, repo *db.ProjectRepo) *ToolCatalogHandler {
	return &ToolCatalogHandler{svc: svc, repo: repo}
}

// ListCatalog handles GET /api/v1/admin/tool-catalog.
func (h *ToolCatalogHandler) ListCatalog(w http.ResponseWriter, r *http.Request) {
	catalog := h.svc.ListCatalog()
	if catalog == nil {
		catalog = []service.ToolCatalogEntry{}
	}
	writeJSON(w, http.StatusOK, model.ApiResponse[[]service.ToolCatalogEntry]{
		Success: true,
		Data:    catalog,
	})
}

// EnableTool handles POST /api/v1/admin/tool-catalog/{slug}/enable.
func (h *ToolCatalogHandler) EnableTool(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	if slug == "" {
		writeJSON(w, http.StatusBadRequest, model.ApiResponse[any]{
			Success: false,
			Message: "slug is required",
		})
		return
	}

	project, err := h.svc.EnableTool(r.Context(), h.repo, slug)
	if err != nil {
		if errors.Is(err, service.ErrToolNotFound) {
			writeJSON(w, http.StatusNotFound, model.ApiResponse[any]{
				Success: false,
				Message: "tool not found in catalog",
			})
			return
		}
		middleware.WriteError(w, http.StatusInternalServerError, model.ERR_INTERNAL, "操作失败，请稍后重试", err)
		return
	}

	writeJSON(w, http.StatusOK, model.ApiResponse[model.AdminProjectResponse]{
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
		},
	})
}