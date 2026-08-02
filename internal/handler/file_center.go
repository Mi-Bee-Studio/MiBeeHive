package handler

import (
	"database/sql"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"

	dbrepo "github.com/Mi-Bee-Studio/mibeehive/internal/db"
	"github.com/Mi-Bee-Studio/mibeehive/internal/middleware"
	"github.com/Mi-Bee-Studio/mibeehive/internal/model"
)

// FileCenterHandler handles cross-project file listing with server-side filtering.
type FileCenterHandler struct {
	readDB *sql.DB
}

// NewFileCenterHandler creates a new FileCenterHandler.
func NewFileCenterHandler(readDB *sql.DB) *FileCenterHandler {
	return &FileCenterHandler{readDB: readDB}
}

// ServeFileCenter handles GET /api/v1/admin/files.
func (h *FileCenterHandler) ServeFileCenter(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	fileRepo := dbrepo.NewFileRepo(h.readDB)

	// Parse and validate pagination parameters.
	limit, offset := parsePagination(r.URL.Query())
	if limit < 1 || limit > 200 {
		limit = 50 // default limit
	}
	if offset < 0 {
		offset = 0
	}

	// Parse filter parameters.
	q := r.URL.Query()
	filters := dbrepo.FileFilters{
		Version:    q.Get("version"),
		OS:         q.Get("os"),
		Arch:       q.Get("arch"),
		Category:   q.Get("category"),
		SourceType: q.Get("source_type"),
		Keyword:    q.Get("q"),
	}

	// Parse project filter (numeric ID or name).
	projectParam := q.Get("project")
	if projectParam != "" {
		// Try to parse as numeric project_id.
		if projectID, err := strconv.ParseInt(projectParam, 10, 64); err == nil && projectID > 0 {
			filters.ProjectID = &projectID
		} else {
			// Use as project name.
			filters.ProjectName = projectParam
		}
	}

	// Parse sorting parameters.
	sortField := q.Get("sort")
	sortOrder := q.Get("order")

	// Query files from repository.
	files, total, err := fileRepo.ListFilesCrossProject(ctx, filters, sortField, sortOrder, limit, offset)
	if err != nil {
		slog.Error("failed to list files", "error", err)
		middleware.WriteError(w, http.StatusInternalServerError, model.ERR_INTERNAL, "操作失败，请稍后重试", err)
		return
	}

	// Convert to response format (exclude local_path and storage_subdir).
	respData := make([]FileCenterFileResponse, 0, len(files))
	for _, f := range files {
		respData = append(respData, FileCenterFileResponse{
			ID:           f.ID,
			ProjectID:    int(f.ProjectID),
			Version:      f.Version,
			Filename:     f.Filename,
			OS:           f.OS,
			Arch:         f.Arch,
			SizeBytes:    f.SizeBytes,
			DownloadURL:  f.DownloadURL,
			Checksum:     f.Checksum,
			Status:       model.FileStatus(f.Status),
			CreatedAt:    f.CreatedAt,
			PublicToken:  f.PublicToken,
			SourceType:   f.SourceType,
			Category:     f.Category,
		})
	}

	// Return paginated response with metadata in headers.
	w.Header().Set("X-Total-Count", strconv.Itoa(total))
	w.Header().Set("X-Limit", strconv.Itoa(limit))
	w.Header().Set("X-Offset", strconv.Itoa(offset))
	writeJSON(w, http.StatusOK, model.ApiResponse[[]FileCenterFileResponse]{
		Success: true,
		Data:    respData,
	})
}

// parsePagination extracts and validates limit and offset from query params.
func parsePagination(q url.Values) (limit, offset int) {
	limit = 50
	offset = 0

	if v := q.Get("limit"); v != "" {
		if l, err := strconv.Atoi(v); err == nil && l > 0 {
			limit = l
		}
	}
	if limit > 200 {
		limit = 200
	}

	if v := q.Get("offset"); v != "" {
		if o, err := strconv.Atoi(v); err == nil && o >= 0 {
			offset = o
		}
	}

	return limit, offset
}

// FileCenterFileResponse is a file response without sensitive paths.
type FileCenterFileResponse struct {
	ID          int64           `json:"id"`
	ProjectID   int             `json:"project_id"`
	Version     string          `json:"version"`
	Filename    string          `json:"filename"`
	OS          string          `json:"os"`
	Arch        string          `json:"arch"`
	SizeBytes   int64           `json:"size_bytes"`
	DownloadURL string          `json:"download_url"`
	Checksum    string          `json:"checksum"`
	Status      model.FileStatus `json:"status"`
	CreatedAt   string          `json:"created_at"`
	PublicToken string          `json:"public_token"`
	SourceType  string          `json:"source_type"`
	Category    string          `json:"category"`
}
