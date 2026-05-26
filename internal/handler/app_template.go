package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	dbrepo "github.com/Mi-Bee-Studio/mibeehive/internal/db"
	"github.com/Mi-Bee-Studio/mibeehive/internal/model"
)

// AppTemplateHandler handles admin API endpoints for application template management.
type AppTemplateHandler struct {
	repo *dbrepo.AppTemplateRepo
}

// NewAppTemplateHandler creates a new AppTemplateHandler.
func NewAppTemplateHandler(db *dbrepo.AppTemplateRepo) *AppTemplateHandler {
	return &AppTemplateHandler{repo: db}
}

// HandleTemplateList handles GET /api/v1/admin/templates.
func (h *AppTemplateHandler) HandleTemplateList(w http.ResponseWriter, r *http.Request) {
	templates, err := h.repo.ListAll(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ApiResponse[any]{
			Success: false,
			Message: fmt.Sprintf("failed to list templates: %v", err),
		})
		return
	}

	if templates == nil {
		templates = []model.AppTemplate{}
	}

	writeJSON(w, http.StatusOK, model.ApiResponse[[]model.AppTemplate]{
		Success: true,
		Data:    templates,
	})
}

// HandleTemplateGet handles GET /api/v1/admin/templates/{id}.
func (h *AppTemplateHandler) HandleTemplateGet(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, model.ApiResponse[any]{
			Success: false,
			Message: "invalid template id",
		})
		return
	}

	t, err := h.repo.GetByID(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ApiResponse[any]{
			Success: false,
			Message: fmt.Sprintf("failed to get template: %v", err),
		})
		return
	}
	if t == nil {
		writeJSON(w, http.StatusNotFound, model.ApiResponse[any]{
			Success: false,
			Message: "template not found",
		})
		return
	}

	writeJSON(w, http.StatusOK, model.ApiResponse[model.AppTemplate]{
		Success: true,
		Data:    *t,
	})
}

// HandleTemplateCreate handles POST /api/v1/admin/templates.
func (h *AppTemplateHandler) HandleTemplateCreate(w http.ResponseWriter, r *http.Request) {
	var t model.AppTemplate
	if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
		writeJSON(w, http.StatusBadRequest, model.ApiResponse[any]{
			Success: false,
			Message: "invalid request body",
		})
		return
	}

	if t.Name == "" || t.Image == "" {
		writeJSON(w, http.StatusBadRequest, model.ApiResponse[any]{
			Success: false,
			Message: "name and image are required",
		})
		return
	}

	if t.Enabled == false {
		t.Enabled = true
	}

	if err := h.repo.Create(r.Context(), &t); err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ApiResponse[any]{
			Success: false,
			Message: fmt.Sprintf("failed to create template: %v", err),
		})
		return
	}

	writeJSON(w, http.StatusCreated, model.ApiResponse[model.AppTemplate]{
		Success: true,
		Data:    t,
	})
}

// HandleTemplateDelete handles DELETE /api/v1/admin/templates/{id}.
func (h *AppTemplateHandler) HandleTemplateDelete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, model.ApiResponse[any]{
			Success: false,
			Message: "invalid template id",
		})
		return
	}

	// Check existence first for 404 semantics.
	t, err := h.repo.GetByID(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ApiResponse[any]{
			Success: false,
			Message: fmt.Sprintf("failed to get template: %v", err),
		})
		return
	}
	if t == nil {
		writeJSON(w, http.StatusNotFound, model.ApiResponse[any]{
			Success: false,
			Message: "template not found",
		})
		return
	}

	if err := h.repo.Delete(r.Context(), id); err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ApiResponse[any]{
			Success: false,
			Message: fmt.Sprintf("failed to delete template: %v", err),
		})
		return
	}

	writeJSON(w, http.StatusOK, model.ApiResponse[any]{
		Success: true,
		Message: "template deleted",
	})
}
