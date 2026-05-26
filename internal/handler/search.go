package handler

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/Mi-Bee-Studio/mibeehive/internal/model"
	"github.com/Mi-Bee-Studio/mibeehive/internal/service"
)

// SearchHandler handles global search API endpoints.
type SearchHandler struct {
	searchService *service.SearchService
}

// NewSearchHandler creates a new SearchHandler.
func NewSearchHandler(svc *service.SearchService) *SearchHandler {
	return &SearchHandler{searchService: svc}
}

// HandleSearch handles GET /api/v1/admin/search.
func (h *SearchHandler) HandleSearch(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	if q == "" {
		writeJSON(w, http.StatusBadRequest, model.ApiResponse[any]{
			Success: false,
			Message: "query parameter 'q' is required",
		})
		return
	}

	searchType := r.URL.Query().Get("type")
	if searchType == "" {
		searchType = "all"
	}

	limit := 10
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

	resp, err := h.searchService.SearchPaginated(r.Context(), q, searchType, limit, offset)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ApiResponse[any]{
			Success: false,
			Message: fmt.Sprintf("search failed: %v", err),
		})
		return
	}

	w.Header().Set("X-Total-Count", strconv.Itoa(resp.Total))
	writeJSON(w, http.StatusOK, model.ApiResponse[*model.SearchResponse]{
		Success: true,
		Data:    resp,
	})
}
