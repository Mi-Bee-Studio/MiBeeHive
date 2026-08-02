package handler

import (
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/Mi-Bee-Studio/mibeehive/internal/middleware"
	"github.com/Mi-Bee-Studio/mibeehive/internal/model"
)

// AdminInternalHandler exposes internal file details (including the physical
// local_path) to authenticated admins. This is the ONLY endpoint allowed to
// reveal the physical path of a stored file.
type AdminInternalHandler struct {
	readDB *sql.DB
}

// NewAdminInternalHandler creates a new AdminInternalHandler backed by the
// read database pool.
func NewAdminInternalHandler(readDB *sql.DB) *AdminInternalHandler {
	return &AdminInternalHandler{readDB: readDB}
}

// FileInternalResponse is the JSON payload returned by GetFileInternal. It
// deliberately includes local_path — this endpoint is the sole sanctioned
// place where the physical path is exposed.
type FileInternalResponse struct {
	ID            int64  `json:"id"`
	Filename      string `json:"filename"`
	LocalPath     string `json:"local_path"`
	StorageSubdir string `json:"storage_subdir"`
	PublicToken   string `json:"public_token"`
	SourceType    string `json:"source_type"`
	SizeBytes     int64  `json:"size_bytes"`
}

// GetFileInternal handles GET /api/v1/admin/files/{id}/internal.
// It returns the file record including its physical local_path, marked with
// the X-Internal header so clients can distinguish it from public responses.
func (h *AdminInternalHandler) GetFileInternal(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		middleware.WriteError(w, http.StatusBadRequest, model.ERR_VALIDATION, "invalid file id", nil)
		return
	}

	var resp FileInternalResponse
	err = h.readDB.QueryRowContext(r.Context(),
		`SELECT id, filename, local_path, COALESCE(storage_subdir,''), COALESCE(public_token,''), COALESCE(source_type,''), size_bytes
		 FROM files WHERE id = ?`, id).
		Scan(&resp.ID, &resp.Filename, &resp.LocalPath, &resp.StorageSubdir, &resp.PublicToken, &resp.SourceType, &resp.SizeBytes)
	if err != nil {
		if err == sql.ErrNoRows {
			middleware.WriteError(w, http.StatusNotFound, model.ERR_NOT_FOUND, "file not found", nil)
			return
		}
		slog.Error("failed to query file internal details", "file_id", id, "error", err)
		middleware.WriteError(w, http.StatusInternalServerError, model.ERR_INTERNAL, "database error", err)
		return
	}

	// Mark this response as internal so clients can distinguish it.
	w.Header().Set("X-Internal", "true")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(model.ApiResponse[FileInternalResponse]{
		Success: true,
		Data:    resp,
	})
}