package handler

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Mi-Bee-Studio/mibeehive/internal/cache"
	"github.com/Mi-Bee-Studio/mibeehive/internal/middleware"
	"github.com/Mi-Bee-Studio/mibeehive/internal/model"
	"golang.org/x/sync/semaphore"
)

const (
	// DownloadSemaphoreWeight is the weight of each download request (1 for simplicity)
	DownloadSemaphoreWeight int64 = 1
	// MaxConcurrentDownloads limits simultaneous download requests
	MaxConcurrentDownloads int64 = 64
)

// DownloadHandler handles public file download by public_token.
type DownloadHandler struct {
	db          *sql.DB
	readDB      *sql.DB
	storagePath string
	tokenCache  *cache.Cache[string, int64]
	sem         *semaphore.Weighted
}

// NewDownloadHandler creates a new DownloadHandler.
func NewDownloadHandler(db *sql.DB, storagePath string) *DownloadHandler {
	return &DownloadHandler{
		db:          db,
		readDB:      db, // Use the same connection for now (could be readDB in production)
		storagePath: storagePath,
		tokenCache:  cache.TokenCache,
		sem:         semaphore.NewWeighted(MaxConcurrentDownloads),
	}
}

// ServeDownload handles GET /api/v1/files/{public_token}/download.
// It resolves the public_token to a file and streams it using http.ServeContent
// for zero-copy sendfile support and Range request handling.
func (h *DownloadHandler) ServeDownload(w http.ResponseWriter, r *http.Request) {
	// Extract public_token from path
	// Path format: /api/v1/files/{public_token}/download
	path := r.URL.Path
	tokenPrefix := "/api/v1/files/"
	if !strings.HasPrefix(path, tokenPrefix) {
		middleware.WriteError(w, http.StatusNotFound, model.ERR_NOT_FOUND, "invalid path", nil)
		return
	}

	pathSuffix := strings.TrimPrefix(path, tokenPrefix)
	// Remove "/download" suffix to get the token
	token := strings.TrimSuffix(pathSuffix, "/download")
	if token == "" {
		middleware.WriteError(w, http.StatusNotFound, model.ERR_NOT_FOUND, "token not found", nil)
		return
	}

	// Acquire semaphore to limit concurrent downloads
	if err := h.sem.Acquire(r.Context(), DownloadSemaphoreWeight); err != nil {
		slog.Warn("failed to acquire download semaphore", "token", token, "error", err)
		middleware.WriteError(w, http.StatusServiceUnavailable, model.ERR_INTERNAL, "download queue full", nil)
		return
	}
	defer h.sem.Release(DownloadSemaphoreWeight)

	// Lookup file ID via token cache or DB
	fileID, ok := h.tokenCache.Get(token)
	if !ok {
		// Cache miss - query DB with timeout
		ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
		defer cancel()

		var id int64
		var localPath, filename, status string
		err := h.readDB.QueryRowContext(ctx,
			`SELECT id, local_path, filename, status FROM files WHERE public_token = ?`,
			token).Scan(&id, &localPath, &filename, &status)
		if err != nil {
			if err == sql.ErrNoRows {
				middleware.WriteError(w, http.StatusNotFound, model.ERR_NOT_FOUND, "file not found", nil)
			} else {
				slog.Error("failed to query file by token", "token", token, "error", err)
				middleware.WriteError(w, http.StatusInternalServerError, model.ERR_INTERNAL, "database error", err)
			}
			return
		}

		// Skip deleted files
		if status == "deleted" {
			middleware.WriteError(w, http.StatusNotFound, model.ERR_NOT_FOUND, "file not found", nil)
			return
		}

		fileID = id
		// Cache the token->fileID mapping
		h.tokenCache.Put(token, fileID)
	}

	// Get file info from DB (we have the fileID, but need filename and local_path)
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	var localPath, filename string
	err := h.readDB.QueryRowContext(ctx,
		`SELECT local_path, filename FROM files WHERE id = ?`,
		fileID).Scan(&localPath, &filename)
	if err != nil {
		slog.Error("failed to query file by ID", "file_id", fileID, "error", err)
		middleware.WriteError(w, http.StatusInternalServerError, model.ERR_INTERNAL, "database error", err)
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

	// Get file info for modTime and size
	info, err := file.Stat()
	if err != nil {
		slog.Error("failed to stat file", "path", fullPath, "error", err)
		middleware.WriteError(w, http.StatusInternalServerError, model.ERR_INTERNAL, "file stat error", err)
		return
	}

	// Set Content-Disposition header for download
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))

	// Use ServeContent for efficient streaming with Range support and sendfile zero-copy
	// This handles Range requests automatically and uses sendfile when available
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
		// Transfer timed out - ServeContent writes to w, so we can't cancel it cleanly
		// The client will likely get a partial response or connection close
		slog.Warn("download transfer timed out", "token", token, "file_id", fileID)
	}
}