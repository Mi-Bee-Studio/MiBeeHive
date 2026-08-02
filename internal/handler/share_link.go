package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/Mi-Bee-Studio/mibeehive/internal/cache"
	"github.com/Mi-Bee-Studio/mibeehive/internal/middleware"
	"github.com/Mi-Bee-Studio/mibeehive/internal/model"
	"github.com/Mi-Bee-Studio/mibeehive/internal/service"
	"golang.org/x/sync/semaphore"
)

// ShareLinkHandler handles share link admin operations.
type ShareLinkHandler struct {
	db           *sql.DB
	readDB       *sql.DB
	storagePath  string
	shareSvc     *service.ShareLinkService
	tokenCache   *cache.Cache[string, int64]
	downloadSem  *semaphore.Weighted
}

// NewShareLinkHandler creates a new ShareLinkHandler.
func NewShareLinkHandler(db, readDB *sql.DB, storagePath string) *ShareLinkHandler {
	shareSvc := service.NewShareLinkService(db, readDB)
	return &ShareLinkHandler{
		db:          db,
		readDB:      readDB,
		storagePath: storagePath,
		shareSvc:    shareSvc,
		tokenCache:  cache.ShareTokenCache,
		downloadSem: semaphore.NewWeighted(64),
	}
}

// CreateShareLinkRequest is the request body for creating a share link.
type CreateShareLinkRequest struct {
	FileID       int64  `json:"file_id"`
	ExpiresAt    *int64 `json:"expires_at"`    // Unix timestamp, null = no expiry
	MaxDownloads int    `json:"max_downloads"` // 0 = unlimited
	Note         string `json:"note"`
}

// ShareLinkResponse is the response for a share link.
type ShareLinkResponse struct {
	Token         string `json:"token"`
	FileID        int64  `json:"file_id"`
	ExpiresAt     *int64 `json:"expires_at"`
	MaxDownloads  int    `json:"max_downloads"`
	DownloadCount int    `json:"download_count"`
	Note          string `json:"note"`
	CreatedAt     int64  `json:"created_at"`
}

// Create handles POST /api/v1/admin/share-links
func (h *ShareLinkHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req CreateShareLinkRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		middleware.WriteError(w, http.StatusBadRequest, model.ERR_VALIDATION, "invalid request body", err)
		return
	}

	// Verify file exists
	var exists bool
	err := h.readDB.QueryRowContext(r.Context(),
		"SELECT 1 FROM files WHERE id = ? AND status != 'deleted'", req.FileID).Scan(&exists)
	if err != nil {
		if err == sql.ErrNoRows {
			middleware.WriteError(w, http.StatusNotFound, model.ERR_NOT_FOUND, "file not found", nil)
			return
		}
		slog.Error("failed to query file", "file_id", req.FileID, "error", err)
		middleware.WriteError(w, http.StatusInternalServerError, model.ERR_INTERNAL, "database error", err)
		return
	}

	var expiresAt *time.Time
	if req.ExpiresAt != nil {
		t := time.Unix(*req.ExpiresAt, 0)
		expiresAt = &t
	}

	link, err := h.shareSvc.Create(r.Context(), req.FileID, expiresAt, req.MaxDownloads, req.Note)
	if err != nil {
		slog.Error("failed to create share link", "file_id", req.FileID, "error", err)
		middleware.WriteError(w, http.StatusInternalServerError, model.ERR_INTERNAL, "failed to create share link", err)
		return
	}

	// Convert time.Time to *int64 for JSON
	var expiresAtInt *int64
	if link.ExpiresAt != nil {
		ts := link.ExpiresAt.Unix()
		expiresAtInt = &ts
	}

	resp := ShareLinkResponse{
		Token:         link.Token,
		FileID:        link.FileID,
		ExpiresAt:     expiresAtInt,
		MaxDownloads:  link.MaxDownloads,
		DownloadCount: link.DownloadCount,
		Note:          link.Note,
		CreatedAt:     link.CreatedAt.Unix(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(model.ApiResponse[ShareLinkResponse]{
		Success: true,
		Data:    resp,
	})

	slog.Info("share link created", "token", link.Token, "file_id", link.FileID)
}

// List handles GET /api/v1/admin/share-links
func (h *ShareLinkHandler) List(w http.ResponseWriter, r *http.Request) {
	links, err := h.shareSvc.List(r.Context())
	if err != nil {
		slog.Error("failed to list share links", "error", err)
		middleware.WriteError(w, http.StatusInternalServerError, model.ERR_INTERNAL, "failed to list share links", err)
		return
	}

	// Convert to response format
	resp := make([]ShareLinkResponse, 0, len(links))
	for _, link := range links {
		var expiresAtInt *int64
		if link.ExpiresAt != nil {
			ts := link.ExpiresAt.Unix()
			expiresAtInt = &ts
		}
		resp = append(resp, ShareLinkResponse{
			Token:         link.Token,
			FileID:        link.FileID,
			ExpiresAt:     expiresAtInt,
			MaxDownloads:  link.MaxDownloads,
			DownloadCount: link.DownloadCount,
			Note:          link.Note,
			CreatedAt:     link.CreatedAt.Unix(),
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(model.ApiResponse[[]ShareLinkResponse]{
		Success: true,
		Data:    resp,
	})
}

// Revoke handles DELETE /api/v1/admin/share-links/{token}
func (h *ShareLinkHandler) Revoke(w http.ResponseWriter, r *http.Request) {
	// Extract token from path
	// Path format: /api/v1/admin/share-links/{token}
	token := r.PathValue("token")
	if token == "" {
		middleware.WriteError(w, http.StatusBadRequest, model.ERR_VALIDATION, "token required", nil)
		return
	}

	err := h.shareSvc.Revoke(r.Context(), token)
	if err != nil {
		if err.Error() == "share link not found" {
			middleware.WriteError(w, http.StatusNotFound, model.ERR_NOT_FOUND, "share link not found", nil)
			return
		}
		slog.Error("failed to revoke share link", "token", token, "error", err)
		middleware.WriteError(w, http.StatusInternalServerError, model.ERR_INTERNAL, "failed to revoke share link", err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(model.ApiResponse[any]{
		Success: true,
		Message: "Share link revoked",
	})
}

// ShareDownload handles GET /s/{token} (public endpoint, no JWT)
// It validates the token and streams the file using ServeContent.
func (h *ShareLinkHandler) ShareDownload(w http.ResponseWriter, r *http.Request) {
	// Extract token from path
	// Path format: /s/{token}
	token := r.PathValue("token")
	if token == "" {
		middleware.WriteError(w, http.StatusNotFound, model.ERR_NOT_FOUND, "invalid token", nil)
		return
	}

	// Acquire semaphore to limit concurrent downloads
	if err := h.downloadSem.Acquire(r.Context(), 1); err != nil {
		slog.Warn("failed to acquire download semaphore", "token", token, "error", err)
		middleware.WriteError(w, http.StatusServiceUnavailable, model.ERR_INTERNAL, "download queue full", nil)
		return
	}
	defer h.downloadSem.Release(1)

	// Check cache first
	fileID, ok := h.tokenCache.Get(token)
	if !ok {
		// Validate token and increment download count
		var err error
		fileID, err = h.shareSvc.Validate(r.Context(), token)
		if err != nil {
			if err.Error() == "share token not found" {
				middleware.WriteError(w, http.StatusNotFound, model.ERR_NOT_FOUND, "share link not found", nil)
			} else if err.Error() == "share token expired" || err.Error() == "max downloads exceeded" {
				middleware.WriteError(w, http.StatusGone, model.ERR_TOKEN_EXPIRED, "share link expired or max downloads exceeded", nil)
			} else {
				slog.Error("failed to validate share token", "token", token, "error", err)
				middleware.WriteError(w, http.StatusInternalServerError, model.ERR_INTERNAL, "validation error", err)
			}
			return
		}
		// Cache the token->fileID mapping
		h.tokenCache.Put(token, fileID)
	}

	// Get file info from DB
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	var localPath, filename string
	err := h.readDB.QueryRowContext(ctx,
		`SELECT local_path, filename FROM files WHERE id = ? AND status != 'deleted'`,
		fileID).Scan(&localPath, &filename)
	if err != nil {
		if err == sql.ErrNoRows {
			middleware.WriteError(w, http.StatusNotFound, model.ERR_NOT_FOUND, "file not found", nil)
		} else {
			slog.Error("failed to query file by ID", "file_id", fileID, "error", err)
			middleware.WriteError(w, http.StatusInternalServerError, model.ERR_INTERNAL, "database error", err)
		}
		return
	}

	// Open file on disk
	fullPath := filepath.Join(h.storagePath, localPath)
	file, err := os.Open(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			middleware.WriteError(w, http.StatusNotFound, model.ERR_NOT_FOUND, "file not found", nil)
		} else {
			slog.Error("failed to open file", "path", fullPath, "error", err)
			middleware.WriteError(w, http.StatusInternalServerError, model.ERR_INTERNAL, "file open error", err)
		}
		return
	}
	defer file.Close()

	// Get file info for modTime
	info, err := file.Stat()
	if err != nil {
		slog.Error("failed to stat file", "path", fullPath, "error", err)
		middleware.WriteError(w, http.StatusInternalServerError, model.ERR_INTERNAL, "file stat error", err)
		return
	}

	// Set Content-Disposition header for download
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))

	// Use ServeContent for efficient streaming with Range support
	// Wrap with a 5-minute timeout for the transfer phase
	transferCtx, transferCancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer transferCancel()

	// Create a channel to capture ServeContent completion
	done := make(chan struct{})
	go func() {
		defer close(done)
		http.ServeContent(w, r, filename, info.ModTime(), file)
	}()

	select {
	case <-done:
		// Transfer completed
	case <-transferCtx.Done():
		// Transfer timed out
		slog.Warn("share download transfer timed out", "token", token, "file_id", fileID)
	}
}