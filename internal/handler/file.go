package handler

import (
	"context"
	"database/sql"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/golang-jwt/jwt/v5"

	dbrepo "github.com/Mi-Bee-Studio/mibeehive/internal/db"
	"github.com/Mi-Bee-Studio/mibeehive/internal/middleware"
	"github.com/Mi-Bee-Studio/mibeehive/internal/model"
	"github.com/Mi-Bee-Studio/mibeehive/internal/service"
)

// FileHandler handles file-related API endpoints.
type FileHandler struct {
	fileRepo    *dbrepo.FileRepo
	fileService *service.FileService
	jwtSecret   string
}

// NewFileHandler creates a new FileHandler.
func NewFileHandler(db *sql.DB, fileService *service.FileService, jwtSecret string) *FileHandler {
	return &FileHandler{
		fileRepo:    dbrepo.NewFileRepo(db),
		fileService: fileService,
		jwtSecret:   jwtSecret,
	}
}

// ListByProject handles GET /api/v1/projects/{id}/files.
func (h *FileHandler) ListByProject(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		middleware.WriteError(w, http.StatusBadRequest, model.ERR_VALIDATION, "invalid project id", nil)
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

	files, total, err := h.fileRepo.ListByProjectPaginated(r.Context(), id, limit, offset)
	if err != nil {
		middleware.WriteError(w, http.StatusInternalServerError, model.ERR_INTERNAL, "操作失败，请稍后重试", err)
		return
	}

	// Apply optional filters via query params.
	q := r.URL.Query()
	filterVersion := q.Get("version")
	filterOS := q.Get("os")
	filterArch := q.Get("arch")

	filtered := make([]*dbrepo.File, 0, len(files))
	for _, f := range files {
		if filterVersion != "" && f.Version != filterVersion {
			continue
		}
		if filterOS != "" && f.OS != filterOS {
			continue
		}
		if filterArch != "" && f.Arch != filterArch {
			continue
		}
		filtered = append(filtered, f)
	}

	resp := make([]model.FileResponse, 0, len(filtered))
	for _, f := range filtered {
		resp = append(resp, toFileResponse(f))
	}

	w.Header().Set("X-Total-Count", strconv.Itoa(total))
	writeJSON(w, http.StatusOK, model.ApiResponse[[]model.FileResponse]{
		Success: true,
		Data:    resp,
	})
}

// Search handles GET /api/v1/files/search?q=xxx.
func (h *FileHandler) Search(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	if q == "" {
		middleware.WriteError(w, http.StatusBadRequest, model.ERR_VALIDATION, "query parameter 'q' is required", nil)
		return
	}

	files, err := h.fileRepo.SearchByFilename(r.Context(), "%"+q+"%")
	if err != nil {
		middleware.WriteError(w, http.StatusInternalServerError, model.ERR_INTERNAL, "操作失败，请稍后重试", err)
		return
	}

	resp := make([]model.FileResponse, 0, len(files))
	for _, f := range files {
		resp = append(resp, toFileResponse(f))
	}

	writeJSON(w, http.StatusOK, model.ApiResponse[[]model.FileResponse]{
		Success: true,
		Data:    resp,
	})
}

// Download handles GET /api/v1/files/{id}/download.
func (h *FileHandler) Download(w http.ResponseWriter, r *http.Request) {
	// Auth: check Authorization header first, then ?token= query param
	tokenString := ""
	if authHeader := r.Header.Get("Authorization"); strings.HasPrefix(authHeader, "Bearer ") {
		tokenString = strings.TrimPrefix(authHeader, "Bearer ")
	} else if tok := r.URL.Query().Get("token"); tok != "" {
		tokenString = tok
	} else {
		middleware.WriteError(w, http.StatusUnauthorized, model.ERR_UNAUTHORIZED, "unauthorized", nil)
		return
	}
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return []byte(h.jwtSecret), nil
	})
	if err != nil || !token.Valid {
		middleware.WriteError(w, http.StatusUnauthorized, model.ERR_UNAUTHORIZED, "invalid token", nil)
		return
	}
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		middleware.WriteError(w, http.StatusBadRequest, model.ERR_VALIDATION, "invalid file id", nil)
		return
	}

	file, err := h.fileRepo.GetByID(r.Context(), id)
	if err != nil {
		middleware.WriteError(w, http.StatusInternalServerError, model.ERR_INTERNAL, "操作失败，请稍后重试", err)
		return
	}
	if file == nil {
		middleware.WriteError(w, http.StatusNotFound, model.ERR_NOT_FOUND, "file not found", nil)
		return
	}

	mFile := toModelFile(file)
	if err := h.fileService.StreamFile(w, mFile); err != nil {
		if !strings.Contains(err.Error(), "broken pipe") {
			middleware.WriteError(w, http.StatusInternalServerError, model.ERR_INTERNAL, "操作失败，请稍后重试", err)
		}
		return
	}
}

func toFileResponse(f *dbrepo.File) model.FileResponse {
	return model.FileResponse{
		ID:           f.ID,
		ProjectID:    int(f.ProjectID),
		Version:      f.Version,
		Filename:     f.Filename,
		OS:           f.OS,
		Arch:         f.Arch,
		Ext:          f.Ext,
		SizeBytes:    f.SizeBytes,
		DownloadURL:  f.DownloadURL,
		LocalPath:    f.LocalPath,
		Checksum:     f.Checksum,
		Status:       model.FileStatus(f.Status),
		ErrorMessage: f.ErrorMessage,
		CreatedAt:    f.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}

func toModelFile(f *dbrepo.File) *model.File {
	return &model.File{
		ID:           f.ID,
		ProjectID:    int(f.ProjectID),
		Filename:     f.Filename,
		DownloadURL:  f.DownloadURL,
		LocalPath:    f.LocalPath,
		SizeBytes:    f.SizeBytes,
		Checksum:     f.Checksum,
		Status:       model.FileStatus(f.Status),
		ErrorMessage: f.ErrorMessage,
	}
}

// ListQueue handles GET /api/v1/files/queue.
func (h *FileHandler) ListQueue(w http.ResponseWriter, r *http.Request) {
	files, err := h.fileRepo.ListQueue(r.Context())
	if err != nil {
		middleware.WriteError(w, http.StatusInternalServerError, model.ERR_INTERNAL, "操作失败，请稍后重试", err)
		return
	}

	resp := make([]model.FileResponse, 0, len(files))
	for _, f := range files {
		resp = append(resp, toFileResponse(f))
	}

	writeJSON(w, http.StatusOK, model.ApiResponse[[]model.FileResponse]{
		Success: true,
		Data:    resp,
	})
}

// QueueStats handles GET /api/v1/files/queue/stats.
func (h *FileHandler) QueueStats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.fileRepo.GetQueueStats(r.Context())
	if err != nil {
		middleware.WriteError(w, http.StatusInternalServerError, model.ERR_INTERNAL, "操作失败，请稍后重试", err)
		return
	}
	writeJSON(w, http.StatusOK, model.ApiResponse[*model.QueueStatsResponse]{
		Success: true,
		Data: &model.QueueStatsResponse{
			Pending:         stats.Pending,
			Downloading:     stats.Downloading,
			Complete:        stats.Complete,
			Error:           stats.Error,
			FailedPermanent: stats.FailedPermanent,
		},
	})
}

// QueueProgress handles GET /api/v1/files/queue/progress.
// Returns real-time download progress for all actively downloading files.
func (h *FileHandler) QueueProgress(w http.ResponseWriter, r *http.Request) {
	progress := h.fileService.GetActiveProgress()
	resp := make(map[int64]*model.DownloadProgressResponse)
	for id, p := range progress {
		pct := 0
		if p.Total > 0 {
			pct = int(p.BytesRead * 100 / p.Total)
		}
		resp[id] = &model.DownloadProgressResponse{
			BytesRead: p.BytesRead,
			Total:     p.Total,
			Percent:   pct,
			Speed:     p.Speed,
			ETA:       p.ETA,
		}
	}
	writeJSON(w, http.StatusOK, model.ApiResponse[map[int64]*model.DownloadProgressResponse]{
		Success: true,
		Data:    resp,
	})
}

// Retry handles POST /api/v1/admin/files/{id}/retry.
// Resets a failed or permanently failed file to pending and triggers a new download.
func (h *FileHandler) Retry(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		middleware.WriteError(w, http.StatusBadRequest, model.ERR_VALIDATION, "invalid file id", nil)
		return
	}

	file, err := h.fileRepo.GetByID(r.Context(), id)
	if err != nil {
		middleware.WriteError(w, http.StatusInternalServerError, model.ERR_INTERNAL, "操作失败，请稍后重试", err)
		return
	}
	if file == nil {
		middleware.WriteError(w, http.StatusNotFound, model.ERR_NOT_FOUND, "file not found", nil)
		return
	}

	if file.Status != string(model.FileStatusError) && file.Status != string(model.FileStatusFailedPermanent) {
		middleware.WriteError(w, http.StatusBadRequest, model.ERR_VALIDATION, "file status must be 'error' or 'failed_permanent'", nil)
		return
	}

	// Reset status to pending and retry count to 0.
	if err := h.fileRepo.ResetRetry(r.Context(), id); err != nil {
		middleware.WriteError(w, http.StatusInternalServerError, model.ERR_INTERNAL, "操作失败，请稍后重试", err)
		return
	}

	// Trigger download in background.
	mFile := toModelFile(file)
	mFile.Status = model.FileStatusPending
	go func() {
		if err := h.fileService.DownloadFile(context.Background(), mFile); err != nil {
			slog.Debug("manual retry download failed", "file_id", id, "filename", file.Filename, "error", err)
		} else {
			slog.Info("manual retry download succeeded", "file_id", id, "filename", file.Filename)
		}
	}()

	writeJSON(w, http.StatusOK, model.ApiResponse[string]{
		Success: true,
		Data:    "file queued for retry",
	})
}
