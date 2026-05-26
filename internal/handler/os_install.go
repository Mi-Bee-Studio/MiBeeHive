package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	dbrepo "github.com/Mi-Bee-Studio/mibeehive/internal/db"
	"github.com/Mi-Bee-Studio/mibeehive/internal/model"
	"github.com/Mi-Bee-Studio/mibeehive/internal/service"
)

// OSInstallHandler handles OS installation config endpoints.
type OSInstallHandler struct {
	repo            *dbrepo.OsInstallConfigRepo
	templateService *service.OsTemplateService
	storagePath     string
}

// NewOSInstallHandler creates a new OSInstallHandler with injected dependencies.
func NewOSInstallHandler(repo *dbrepo.OsInstallConfigRepo, templateService *service.OsTemplateService, storagePath string) *OSInstallHandler {
	return &OSInstallHandler{
		repo:            repo,
		templateService: templateService,
		storagePath:     storagePath,
	}
}

// CreateConfig handles POST /api/v1/admin/os-install/configs.
func (h *OSInstallHandler) CreateConfig(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name       string                `json:"name"`
		ConfigName string                `json:"config_name"`
		OsType     string                `json:"os_type"`
		Params     model.OsInstallParams `json:"params"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, model.ApiResponse[any]{
			Success: false,
			Message: "invalid request body",
		})
		return
	}

	if req.Name == "" || req.OsType == "" {
		writeJSON(w, http.StatusBadRequest, model.ApiResponse[any]{
			Success: false,
			Message: "name and os_type are required",
		})
		return
	}

	configName := req.ConfigName
	if configName == "" {
		configName = req.Name
	}

	paramsJSON, err := json.Marshal(req.Params)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ApiResponse[any]{
			Success: false,
			Message: fmt.Sprintf("failed to marshal params: %v", err),
		})
		return
	}

	cfg, err := h.repo.Create(r.Context(), req.Name, configName, req.OsType, string(paramsJSON))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ApiResponse[any]{
			Success: false,
			Message: fmt.Sprintf("failed to create config: %v", err),
		})
		return
	}

	// Generate and save config file to disk.
	if err := h.saveConfigFile(cfg.ID, cfg.OsType, req.Params); err != nil {
		slog.Error("failed to save generated config file", "id", cfg.ID, "error", err)
	}

	writeJSON(w, http.StatusCreated, model.ApiResponse[model.OsInstallConfig]{
		Success: true,
		Data:    *cfg,
	})
}

// ListConfigs handles GET /api/v1/admin/os-install/configs.
func (h *OSInstallHandler) ListConfigs(w http.ResponseWriter, r *http.Request) {
	configs, err := h.repo.List(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ApiResponse[any]{
			Success: false,
			Message: fmt.Sprintf("failed to list configs: %v", err),
		})
		return
	}

	if configs == nil {
		configs = []*model.OsInstallConfig{}
	}

	writeJSON(w, http.StatusOK, model.ApiResponse[[]*model.OsInstallConfig]{
		Success: true,
		Data:    configs,
	})
}

// GetConfig handles GET /api/v1/admin/os-install/configs/{id}.
func (h *OSInstallHandler) GetConfig(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, model.ApiResponse[any]{
			Success: false,
			Message: "invalid config id",
		})
		return
	}

	cfg, err := h.repo.GetByID(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ApiResponse[any]{
			Success: false,
			Message: fmt.Sprintf("failed to get config: %v", err),
		})
		return
	}

	if cfg == nil {
		writeJSON(w, http.StatusNotFound, model.ApiResponse[any]{
			Success: false,
			Message: "config not found",
		})
		return
	}

	writeJSON(w, http.StatusOK, model.ApiResponse[model.OsInstallConfig]{
		Success: true,
		Data:    *cfg,
	})
}

// UpdateConfig handles PUT /api/v1/admin/os-install/configs/{id}.
func (h *OSInstallHandler) UpdateConfig(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, model.ApiResponse[any]{
			Success: false,
			Message: "invalid config id",
		})
		return
	}

	var req struct {
		Name       string                `json:"name"`
		ConfigName string                `json:"config_name"`
		OsType     string                `json:"os_type"`
		Params     model.OsInstallParams `json:"params"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, model.ApiResponse[any]{
			Success: false,
			Message: "invalid request body",
		})
		return
	}

	configName := req.ConfigName
	if configName == "" {
		configName = req.Name
	}

	paramsJSON, err := json.Marshal(req.Params)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ApiResponse[any]{
			Success: false,
			Message: fmt.Sprintf("failed to marshal params: %v", err),
		})
		return
	}

	if err := h.repo.Update(r.Context(), id, req.Name, configName, req.OsType, string(paramsJSON)); err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ApiResponse[any]{
			Success: false,
			Message: fmt.Sprintf("failed to update config: %v", err),
		})
		return
	}

	// Regenerate config file on disk.
	if err := h.saveConfigFile(id, req.OsType, req.Params); err != nil {
		slog.Error("failed to save regenerated config file", "id", id, "error", err)
	}

	writeJSON(w, http.StatusOK, model.ApiResponse[any]{
		Success: true,
		Message: "config updated",
	})
}

// DeleteConfig handles DELETE /api/v1/admin/os-install/configs/{id}.
func (h *OSInstallHandler) DeleteConfig(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, model.ApiResponse[any]{
			Success: false,
			Message: "invalid config id",
		})
		return
	}

	if err := h.repo.Delete(r.Context(), id); err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ApiResponse[any]{
			Success: false,
			Message: fmt.Sprintf("failed to delete config: %v", err),
		})
		return
	}

	writeJSON(w, http.StatusOK, model.ApiResponse[any]{
		Success: true,
		Message: "config deleted",
	})
}

// ServePXEConfig handles GET /pxe/{format}/{name} — PUBLIC endpoint, no auth required.
func (h *OSInstallHandler) ServePXEConfig(w http.ResponseWriter, r *http.Request) {
	format := r.PathValue("format")
	name := r.PathValue("name")

	if format == "" || name == "" {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	cfg, err := h.repo.GetByName(r.Context(), name)
	if err != nil {
		slog.Error("failed to lookup PXE config", "name", name, "error", err)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	if cfg == nil || !cfg.Enabled {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	var params model.OsInstallParams
	if err := json.Unmarshal([]byte(cfg.Config), &params); err != nil {
		slog.Error("failed to unmarshal PXE config params", "name", name, "error", err)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	var content string
	switch strings.ToLower(format) {
	case "preseed":
		content, err = service.GeneratePreseed(params)
	case "kickstart":
		osType := cfg.OsType
		// Normalize osType aliases for template conditional matching.
		switch osType {
		case "rockylinux":
			osType = "rocky"
		case "almalinux":
			osType = "alma"
		}
		content, err = service.GenerateKickstart(params, osType)
	case "autoinstall":
		content, err = service.GenerateAutoinstall(params)
	default:
		w.WriteHeader(http.StatusNotFound)
		return
	}

	if err != nil {
		slog.Error("failed to generate PXE config", "name", name, "format", format, "error", err)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(content))
}

// PreviewConfig handles POST /api/v1/admin/os-install/configs/preview.
// It accepts os_type and params, generates config content without saving.
func (h *OSInstallHandler) PreviewConfig(w http.ResponseWriter, r *http.Request) {
	var req struct {
		OsType string                `json:"os_type"`
		Params model.OsInstallParams `json:"params"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, model.ApiResponse[any]{
			Success: false,
			Message: "invalid request body",
		})
		return
	}

	if req.OsType == "" {
		writeJSON(w, http.StatusBadRequest, model.ApiResponse[any]{
			Success: false,
			Message: "os_type is required",
		})
		return
	}

	content, err := h.templateService.Generate(req.OsType, req.Params)
	if err != nil {
		var validationErr *service.ValidationError
		if errors.As(err, &validationErr) {
			slog.Warn("config preview validation failed", "os_type", req.OsType, "error", err)
			writeJSON(w, http.StatusBadRequest, model.ApiResponse[any]{
				Success: false,
				Message: "Invalid configuration parameters",
			})
			return
		}
		slog.Error("config preview generation failed", "os_type", req.OsType, "error", err)
		writeJSON(w, http.StatusInternalServerError, model.ApiResponse[any]{
			Success: false,
			Message: "Failed to generate configuration",
		})
		return
	}

	writeJSON(w, http.StatusOK, model.ApiResponse[any]{
		Success: true,
		Data:    content,
	})
}

// configFilename returns the filename for a generated config file based on osType.
func configFilename(osType string) string {
	switch strings.ToLower(osType) {
	case "debian":
		return "preseed.cfg"
	case "ubuntu":
		return "autoinstall.yaml"
	case "centos", "rhel":
		return "kickstart.ks"
	default:
		return "config"
	}
}

// saveConfigFile generates a config file and saves it to disk at {storagePath}/os-install/configs/{id}/.
func (h *OSInstallHandler) saveConfigFile(id int64, osType string, params model.OsInstallParams) error {
	content, err := h.templateService.Generate(osType, params)
	if err != nil {
		return fmt.Errorf("generating config: %w", err)
	}

	dir := filepath.Join(h.storagePath, "os-install", "configs", strconv.FormatInt(id, 10))
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}

	filePath := filepath.Join(dir, configFilename(osType))
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		return fmt.Errorf("writing config file: %w", err)
	}

	slog.Info("saved generated config file", "id", id, "os_type", osType, "path", filePath)
	return nil
}
