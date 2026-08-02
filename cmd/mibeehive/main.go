package main

import (
	"context"
	"encoding/json"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"syscall"

	"github.com/Mi-Bee-Studio/mibeehive/internal/db"
	"github.com/Mi-Bee-Studio/mibeehive/internal/model"
	"github.com/Mi-Bee-Studio/mibeehive/internal/service"
)

// version is set at build time via -ldflags "-X main.version=..."
var version = "dev"

// retryFailedDownloads attempts to re-download failed files that haven't exceeded max retries.
func retryFailedDownloads(ctx context.Context, fileRepo *db.FileRepo, fileService *service.FileService, maxRetries int) {
	files, err := fileRepo.ListRetryable(ctx, maxRetries)
	if err != nil {
		slog.Debug("failed to list retryable files", "error", err)
		return
	}
	for _, f := range files {
		// Convert db.File to model.File for FileService
		mFile := &model.File{
			ID:          f.ID,
			ProjectID:   int(f.ProjectID),
			Filename:    f.Filename,
			DownloadURL: f.DownloadURL,
			LocalPath:   f.LocalPath,
			SizeBytes:   f.SizeBytes,
			Checksum:    f.Checksum,
		}
		if err := fileService.DownloadFile(ctx, mFile); err != nil {
			slog.Debug("retry download failed", "file_id", f.ID, "filename", f.Filename, "error", err)
		} else {
			slog.Info("retry download succeeded", "file_id", f.ID, "filename", f.Filename)
		}
	}
}

// apiNotFoundMiddleware wraps an http.Handler to return JSON 404/405 for /api/v1/ routes.
func apiNotFoundMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/api/v1/") {
			next.ServeHTTP(w, r)
			return
		}
		rec := &statusRecorder{ResponseWriter: w}
		next.ServeHTTP(rec, r)
	})
}

type statusRecorder struct {
	http.ResponseWriter
	statusCode int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.statusCode = code
	if code == http.StatusNotFound || code == http.StatusMethodNotAllowed {
		r.Header().Set("Content-Type", "application/json")
	}
	r.ResponseWriter.WriteHeader(code)
}

func (r *statusRecorder) Write(b []byte) (int, error) {
	if r.statusCode == http.StatusNotFound || r.statusCode == http.StatusMethodNotAllowed {
		msg := "Not found"
		if r.statusCode == http.StatusMethodNotAllowed {
			msg = "Method not allowed"
		}
		j, _ := json.Marshal(map[string]any{"success": false, "message": msg})
		return r.ResponseWriter.Write(j)
	}
	return r.ResponseWriter.Write(b)
}

func main() {
	configPath := flag.String("config", "./configs/config.yaml", "path to config file")
	flag.Parse()

	cfg := loadConfig(*configPath)
	database, readDB := initDatabase(cfg)
	defer db.Close(database)
	// Backfill empty versions from filenames (one-time migration, idempotent).
	if n, err := db.NewFileRepo(database).BackfillEmptyVersions(context.Background()); err != nil {
		slog.Warn("version backfill failed", "error", err)
	} else if n > 0 {
		slog.Info("version backfill completed", "updated", n)
	}

	// Backfill public_token for files that predate the column (idempotent).
	if err := service.BackfillPublicTokens(database); err != nil {
		slog.Warn("public_token backfill failed", "error", err)
	}


	quit := make(chan os.Signal, 1)
	requestShutdown := func() { quit <- syscall.SIGTERM }

	svcs := initServices(cfg, database, readDB)
	handlers := initHandlers(cfg, svcs, database, *configPath, requestShutdown)
	httpHandler, httpsHandler := buildRouter(cfg, handlers, svcs, database)

	runServers(cfg, httpHandler, httpsHandler, svcs, database, quit, *configPath)
}
