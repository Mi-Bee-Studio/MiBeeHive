package handler

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"path/filepath"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/Mi-Bee-Studio/mibeehive/internal/config"
	"github.com/Mi-Bee-Studio/mibeehive/internal/model"
	"github.com/Mi-Bee-Studio/mibeehive/internal/service"
	"gopkg.in/yaml.v3"
)

// MigrationService defines the interface for storage migration operations.
// The concrete service.MigrationService (T5) satisfies this interface.
type MigrationService interface {
	Enqueue(module, oldPath, newPath string) (int64, error)
	List() ([]service.MigrationTaskInfo, error)
	Get(id int64) (*service.MigrationTaskInfo, error)
	Cancel(id int64) error
}

// StorageConfigHandler handles storage path configuration and migration endpoints.
type StorageConfigHandler struct {
	config     *config.Config
	configPath string
	resolver   *service.StorageResolver
	migration  MigrationService
	logger     *slog.Logger
	mu         sync.Mutex
}

// NewStorageConfigHandler creates a new StorageConfigHandler.
func NewStorageConfigHandler(
	cfg *config.Config,
	configPath string,
	resolver *service.StorageResolver,
	migration MigrationService,
	logger *slog.Logger,
) *StorageConfigHandler {
	return &StorageConfigHandler{
		config:     cfg,
		configPath: configPath,
		resolver:   resolver,
		migration:  migration,
		logger:     logger,
	}
}

// GetStorageConfig handles GET /api/v1/admin/config/storage.
func (h *StorageConfigHandler) GetStorageConfig(w http.ResponseWriter, r *http.Request) {
	h.mu.Lock()
	cfg := h.config
	h.mu.Unlock()

	resp := model.StorageConfigResponse{
		OSS:               cfg.Storage.Modules.OSS,
		OSInstall:         cfg.Storage.Modules.OSInstall,
		ISO:               cfg.Storage.Modules.ISO,
		OSSFallback:       cfg.Storage.BasePath,
		OSInstallFallback: filepath.Join(cfg.Storage.BasePath, "os-install"),
		ISOFallback:       filepath.Join(cfg.Storage.BasePath, "os-install"),
	}

	writeJSON(w, http.StatusOK, model.ApiResponse[model.StorageConfigResponse]{
		Success: true,
		Data:    resp,
	})
}

// UpdateStorageConfig handles PUT /api/v1/admin/config/storage.
func (h *StorageConfigHandler) UpdateStorageConfig(w http.ResponseWriter, r *http.Request) {
	var req model.StorageConfigUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, model.ApiResponse[any]{
			Success: false,
			Message: "invalid request body",
		})
		return
	}

	// Validate: at least one field must be provided.
	if req.OSS == nil && req.OSInstall == nil && req.ISO == nil {
		writeJSON(w, http.StatusBadRequest, model.ApiResponse[any]{
			Success: false,
			Message: "at least one storage path must be provided",
		})
		return
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	// Validate all provided paths.
	type pathChange struct {
		module  string
		oldPath string
		newPath string
	}
	var changes []pathChange

	if req.OSS != nil {
		if err := validateStoragePath(*req.OSS, "oss"); err != nil {
			writeJSON(w, http.StatusBadRequest, model.ApiResponse[any]{Success: false, Message: err.Error()})
			return
		}
		oldPath := h.resolver.ResolveOSS()
		changes = append(changes, pathChange{module: "oss", oldPath: oldPath, newPath: *req.OSS})
	}
	if req.OSInstall != nil {
		if err := validateStoragePath(*req.OSInstall, "os_install"); err != nil {
			writeJSON(w, http.StatusBadRequest, model.ApiResponse[any]{Success: false, Message: err.Error()})
			return
		}
		oldPath := h.resolver.ResolveOSInstall()
		changes = append(changes, pathChange{module: "os_install", oldPath: oldPath, newPath: *req.OSInstall})
	}
	if req.ISO != nil {
		if err := validateStoragePath(*req.ISO, "iso"); err != nil {
			writeJSON(w, http.StatusBadRequest, model.ApiResponse[any]{Success: false, Message: err.Error()})
			return
		}
		oldPath := h.resolver.ResolveISO()
		changes = append(changes, pathChange{module: "iso", oldPath: oldPath, newPath: *req.ISO})
	}

	// Save original values for rollback.
	origOSS := h.config.Storage.Modules.OSS
	origOSInstall := h.config.Storage.Modules.OSInstall
	origISO := h.config.Storage.Modules.ISO

	// Enqueue migrations for changed paths.
	var migrationIDs []int64
	for _, ch := range changes {
		if ch.oldPath == ch.newPath {
			continue // No change needed.
		}
		if h.migration != nil {
			id, err := h.migration.Enqueue(ch.module, ch.oldPath, ch.newPath)
			if err != nil {
				h.logger.Error("failed to enqueue migration", "module", ch.module, "error", err)
				writeJSON(w, http.StatusInternalServerError, model.ApiResponse[any]{
					Success: false,
					Message: fmt.Sprintf("failed to enqueue migration for %s: %s", ch.module, err.Error()),
				})
				return
			}
			migrationIDs = append(migrationIDs, id)
		}
	}

	// Update config in memory.
	if req.OSS != nil {
		h.config.Storage.Modules.OSS = *req.OSS
	}
	if req.OSInstall != nil {
		h.config.Storage.Modules.OSInstall = *req.OSInstall
	}
	if req.ISO != nil {
		h.config.Storage.Modules.ISO = *req.ISO
	}

	// Hot-reload resolver.
	h.resolver.UpdateConfig(h.config)

	// Save config to disk.
	if h.configPath != "" {
		if err := h.saveConfig(); err != nil {
			// Rollback in-memory config.
			h.config.Storage.Modules.OSS = origOSS
			h.config.Storage.Modules.OSInstall = origOSInstall
			h.config.Storage.Modules.ISO = origISO
			h.resolver.UpdateConfig(h.config)

			writeJSON(w, http.StatusInternalServerError, model.ApiResponse[any]{
				Success: false,
				Message: "failed to save config: " + err.Error(),
			})
			return
		}
	}

	h.logger.Info("storage config updated", "changes", len(changes), "migrations", len(migrationIDs))

	writeJSON(w, http.StatusOK, model.ApiResponse[model.StorageConfigUpdateResponse]{
		Success: true,
		Message: "storage config updated",
		Data: model.StorageConfigUpdateResponse{
			MigrationIDs: migrationIDs,
		},
	})
}

// ListMigrations handles GET /api/v1/admin/storage/migrations.
func (h *StorageConfigHandler) ListMigrations(w http.ResponseWriter, r *http.Request) {
	if h.migration == nil {
		writeJSON(w, http.StatusOK, model.ApiResponse[[]model.MigrationTaskResponse]{
			Success: true,
			Data:    []model.MigrationTaskResponse{},
		})
		return
	}

	tasks, err := h.migration.List()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ApiResponse[any]{
			Success: false,
			Message: "failed to list migrations",
		})
		return
	}

	resp := make([]model.MigrationTaskResponse, 0, len(tasks))
	for _, t := range tasks {
		resp = append(resp, migrationTaskToResponse(t))
	}

	writeJSON(w, http.StatusOK, model.ApiResponse[[]model.MigrationTaskResponse]{
		Success: true,
		Data:    resp,
	})
}

// GetMigration handles GET /api/v1/admin/storage/migrations/{id}.
func (h *StorageConfigHandler) GetMigration(w http.ResponseWriter, r *http.Request) {
	if h.migration == nil {
		WriteError(w, http.StatusNotFound, "migration service not available")
		return
	}

	id, err := parseMigrationID(r)
	if err != nil {
		WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	task, err := h.migration.Get(id)
	if err != nil {
		WriteError(w, http.StatusNotFound, "migration not found")
		return
	}

	writeJSON(w, http.StatusOK, model.ApiResponse[model.MigrationTaskResponse]{
		Success: true,
		Data:    migrationTaskToResponse(*task),
	})
}

// CancelMigration handles POST /api/v1/admin/storage/migrations/{id}/cancel.
func (h *StorageConfigHandler) CancelMigration(w http.ResponseWriter, r *http.Request) {
	if h.migration == nil {
		WriteError(w, http.StatusNotFound, "migration service not available")
		return
	}

	id, err := parseMigrationID(r)
	if err != nil {
		WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.migration.Cancel(id); err != nil {
		WriteError(w, http.StatusNotFound, "migration not found")
		return
	}

	writeJSON(w, http.StatusOK, model.ApiResponse[any]{
		Success: true,
		Message: "migration cancelled",
	})
}

func (h *StorageConfigHandler) saveConfig() error {
	data, err := yaml.Marshal(h.config)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}
	if err := writeFile(h.configPath, data, 0644); err != nil {
		return fmt.Errorf("writing config file: %w", err)
	}
	return nil
}

// parseMigrationID extracts the migration ID from the URL path.
// URL format: /api/v1/admin/storage/migrations/{id} or /api/v1/admin/storage/migrations/{id}/cancel
func parseMigrationID(r *http.Request) (int64, error) {
	// Try Go 1.22+ path value first.
	if id := r.PathValue("id"); id != "" {
		return strconv.ParseInt(id, 10, 64)
	}
	// Fallback: parse from URL path.
	path := r.URL.Path
	prefix := "/api/v1/admin/storage/migrations/"
	if !strings.HasPrefix(path, prefix) {
		return 0, fmt.Errorf("invalid migration URL path")
	}
	remainder := strings.TrimPrefix(path, prefix)
	// Extract ID (before any trailing segment like /cancel).
	parts := strings.SplitN(remainder, "/", 2)
	if len(parts) == 0 || parts[0] == "" {
		return 0, fmt.Errorf("missing migration ID")
	}
	return strconv.ParseInt(parts[0], 10, 64)
}

// validateStoragePath validates a storage module path.
func validateStoragePath(path, module string) error {
	if path == "" {
		return fmt.Errorf("%s path cannot be empty", module)
	}
	if path[0] != '/' {
		return fmt.Errorf("%s path %q is not absolute (must start with /)", module, path)
	}
	return nil
}

// migrationTaskToResponse converts a service-level task to API response.
func migrationTaskToResponse(t service.MigrationTaskInfo) model.MigrationTaskResponse {
	resp := model.MigrationTaskResponse{
		ID:            t.ID,
		Module:        t.Module,
		OldPath:       t.OldPath,
		NewPath:       t.NewPath,
		Status:        t.Status,
		Progress:      t.Progress,
		TotalFiles:    t.TotalFiles,
		MigratedFiles: t.MigratedFiles,
		TotalBytes:    t.TotalBytes,
		MigratedBytes: t.MigratedBytes,
		ErrorMessage:  t.ErrorMessage,
		CreatedAt:     t.CreatedAt,
	}
	if t.StartedAt != nil {
		resp.StartedAt = *t.StartedAt
	}
	if t.CompletedAt != nil {
		resp.CompletedAt = *t.CompletedAt
	}
	return resp
}

var writeFile = os.WriteFile
