package handler

import (
	"net/http"

	"github.com/Mi-Bee-Studio/mibeehive/internal/model"
	"github.com/Mi-Bee-Studio/mibeehive/internal/service"
)

// DashboardHandler handles the aggregated dashboard summary endpoint.
type DashboardHandler struct {
	service *service.DashboardService
}

// NewDashboardHandler creates a new DashboardHandler.
func NewDashboardHandler(svc *service.DashboardService) *DashboardHandler {
	return &DashboardHandler{
		service: svc,
	}
}

// Summary handles GET /api/v1/admin/dashboard/summary.
func (h *DashboardHandler) Summary(w http.ResponseWriter, r *http.Request) {
	summary := h.service.GetSummary(r.Context())
	writeJSON(w, http.StatusOK, model.ApiResponse[model.DashboardSummaryResponse]{
		Success: true,
		Data:    summary,
	})
}
