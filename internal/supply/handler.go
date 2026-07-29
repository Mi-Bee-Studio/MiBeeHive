// Package supply implements the ops-tool "supply layer": public endpoints
// that serve artifacts collected by Foraging to external servers.
//
// This is the minimal batch-1 endpoint from Issue #1's validation scope: a
// generic file-repository index + per-file download. It is intentionally NOT a
// real protocol (Go proxy / PyPI / APT) yet — those come later. It reuses the
// existing FileService.StreamFile so the download path is unchanged.
package supply

import (
	"encoding/json"
	"net/http"
	"strconv"

	db "github.com/Mi-Bee-Studio/mibeehive/internal/db"
	"github.com/Mi-Bee-Studio/mibeehive/internal/middleware"
	"github.com/Mi-Bee-Studio/mibeehive/internal/model"
	"github.com/Mi-Bee-Studio/mibeehive/internal/service"
)

// ListedFile is one entry in the repository index. It carries only public
// metadata, never local_path or error fields.
type ListedFile struct {
	ID          int64  `json:"id"`
	ProjectID   int64  `json:"project_id"`
	Version     string `json:"version"`
	Filename    string `json:"filename"`
	OS          string `json:"os,omitempty"`
	Arch        string `json:"arch,omitempty"`
	Ext         string `json:"ext,omitempty"`
	SizeBytes   int64  `json:"size_bytes"`
	Checksum    string `json:"checksum,omitempty"`
	DownloadURL string `json:"download_url"` // relative: /repo/files/{id}
}

// Handler serves the supply-layer repository index and downloads.
type Handler struct {
	fileRepo *db.FileRepo
	svc      *service.FileService
}

// NewHandler builds a supply Handler backed by the existing FileRepo and
// FileService (the same ones the admin layer uses).
func NewHandler(fileRepo *db.FileRepo, svc *service.FileService) *Handler {
	return &Handler{fileRepo: fileRepo, svc: svc}
}

// indexResponse is the JSON shape of GET /repo/index.
type indexResponse struct {
	Count int          `json:"count"`
	Items []*ListedFile `json:"items"`
}

// ServeIndex handles GET /repo/index — a JSON manifest of servable artifacts.
// Public (no auth): external servers must be able to discover tools.
func (h *Handler) ServeIndex(w http.ResponseWriter, r *http.Request) {
	files, err := h.fileRepo.ListComplete(r.Context(), 0)
	if err != nil {
		middleware.WriteError(w, http.StatusInternalServerError, model.ERR_INTERNAL, "list servable files", err)
		return
	}
	items := make([]*ListedFile, 0, len(files))
	for _, f := range files {
		items = append(items, &ListedFile{
			ID:          f.ID,
			ProjectID:   f.ProjectID,
			Version:     f.Version,
			Filename:    f.Filename,
			OS:          f.OS,
			Arch:        f.Arch,
			Ext:         f.Ext,
			SizeBytes:   f.SizeBytes,
			Checksum:    f.Checksum,
			DownloadURL: "/repo/files/" + strconv.FormatInt(f.ID, 10),
		})
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(indexResponse{Count: len(items), Items: items})
}

// ServeFile handles GET /repo/files/{id} — streams an artifact, reusing the
// existing FileService.StreamFile so the download path is byte-for-byte the
// same as the admin endpoint.
func (h *Handler) ServeFile(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		middleware.WriteError(w, http.StatusBadRequest, model.ERR_VALIDATION, "invalid file id", err)
		return
	}
	f, err := h.fileRepo.GetByID(r.Context(), id)
	if err != nil || f == nil {
		middleware.WriteError(w, http.StatusNotFound, model.ERR_NOT_FOUND, "file not found", err)
		return
	}
	mFile := &model.File{
		ID:          f.ID,
		ProjectID:   int(f.ProjectID),
		Version:     f.Version,
		Filename:    f.Filename,
		OS:          f.OS,
		Arch:        f.Arch,
		Ext:         f.Ext,
		SizeBytes:   f.SizeBytes,
		LocalPath:   f.LocalPath,
		Checksum:    f.Checksum,
		Status:      model.FileStatus(f.Status),
	}
	if err := h.svc.StreamFile(w, mFile); err != nil {
		// Client may have disconnected; mirror the file handler's silent handling.
		_ = err
	}
}
