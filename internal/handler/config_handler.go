package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/Mi-Bee-Studio/mibeehive/internal/config"
	"github.com/Mi-Bee-Studio/mibeehive/internal/middleware"
	"github.com/Mi-Bee-Studio/mibeehive/internal/model"

	"golang.org/x/crypto/bcrypt"
	"gopkg.in/yaml.v3"
)

// ConfigHandler handles admin configuration endpoints (password change, monitor config).
type ConfigHandler struct {
	config     *config.Config
	configPath string
	configMu   sync.Mutex
}

// NewConfigHandler creates a new ConfigHandler.
func NewConfigHandler(cfg *config.Config, configPath string) *ConfigHandler {
	return &ConfigHandler{
		config:     cfg,
		configPath: configPath,
	}
}

// ChangePassword handles POST /api/v1/admin/password.
func (h *ConfigHandler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	var req model.ChangePasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, model.ApiResponse[any]{
			Success: false,
			Message: "invalid request body",
		})
		return
	}

	if req.OldPassword == "" || req.NewPassword == "" {
		writeJSON(w, http.StatusBadRequest, model.ApiResponse[any]{
			Success: false,
			Message: "old_password and new_password are required",
		})
		return
	}

	// Verify old password.
	if err := bcrypt.CompareHashAndPassword([]byte(h.config.Auth.PasswordHash), []byte(req.OldPassword)); err != nil {
		middleware.WriteError(w, http.StatusUnauthorized, model.ERR_UNAUTHORIZED, "current password is incorrect", nil)
		return
	}

	// Hash new password.
	newHash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		middleware.WriteError(w, http.StatusInternalServerError, model.ERR_INTERNAL, "操作失败，请稍后重试", err)
		return
	}

	// Save to file FIRST, then update in-memory config for atomicity.
	h.configMu.Lock()
	configPath := h.configPath
	h.configMu.Unlock()

	if configPath != "" {
		// Build a copy with the new values for file save.
		h.configMu.Lock()
		oldHash := h.config.Auth.PasswordHash
		oldChangedAt := h.config.Auth.PasswordChangedAt
		h.config.Auth.PasswordHash = string(newHash)
		h.config.Auth.PasswordChangedAt = time.Now().UTC().Format(time.RFC3339)
		h.configMu.Unlock()

		if err := h.saveConfig(configPath); err != nil {
			// Rollback in-memory config on save failure.
			h.configMu.Lock()
			h.config.Auth.PasswordHash = oldHash
			h.config.Auth.PasswordChangedAt = oldChangedAt
			h.configMu.Unlock()
			middleware.WriteError(w, http.StatusInternalServerError, model.ERR_INTERNAL, "配置保存失败", err)
			return
		}
	} else {
		// No config path — update in-memory only.
		h.configMu.Lock()
		h.config.Auth.PasswordHash = string(newHash)
		h.config.Auth.PasswordChangedAt = time.Now().UTC().Format(time.RFC3339)
		h.configMu.Unlock()
	}

	writeJSON(w, http.StatusOK, model.ApiResponse[any]{
		Success: true,
		Message: "password changed",
	})
}

// GetMonitorConfig handles GET /api/v1/admin/config/monitor.
func (h *ConfigHandler) GetMonitorConfig(w http.ResponseWriter, r *http.Request) {
	resp := model.MonitorConfigResponse{
		DiskWarningPercent:  h.config.Monitor.DiskWarningPercent,
		DiskCriticalPercent: h.config.Monitor.DiskCriticalPercent,
		DiskCheckEnabled:    h.config.Monitor.DiskCheckEnabled,
	}
	writeJSON(w, http.StatusOK, model.ApiResponse[model.MonitorConfigResponse]{
		Success: true,
		Data:    resp,
	})
}

// UpdateMonitorConfig handles PUT /api/v1/admin/config/monitor.
func (h *ConfigHandler) UpdateMonitorConfig(w http.ResponseWriter, r *http.Request) {
	var req model.MonitorConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, model.ApiResponse[any]{
			Success: false,
			Message: "invalid request body",
		})
		return
	}

	// Validate disk monitor thresholds.
	if req.DiskWarningPercent < 1 || req.DiskWarningPercent > 100 {
		writeJSON(w, http.StatusBadRequest, model.ApiResponse[any]{
			Success: false,
			Message: "disk_warning_percent must be between 1 and 100",
		})
		return
	}
	if req.DiskCriticalPercent < 1 || req.DiskCriticalPercent > 100 {
		writeJSON(w, http.StatusBadRequest, model.ApiResponse[any]{
			Success: false,
			Message: "disk_critical_percent must be between 1 and 100",
		})
		return
	}
	if req.DiskCriticalPercent <= req.DiskWarningPercent {
		writeJSON(w, http.StatusBadRequest, model.ApiResponse[any]{
			Success: false,
			Message: "disk_critical_percent must be greater than disk_warning_percent",
		})
		return
	}

	h.configMu.Lock()
	h.config.Monitor.DiskWarningPercent = req.DiskWarningPercent
	h.config.Monitor.DiskCriticalPercent = req.DiskCriticalPercent
	h.config.Monitor.DiskCheckEnabled = req.DiskCheckEnabled
	configPath := h.configPath
	h.configMu.Unlock()

	if configPath != "" {
		if err := h.saveConfig(configPath); err != nil {
			middleware.WriteError(w, http.StatusInternalServerError, model.ERR_INTERNAL, "配置保存失败", err)
			return
		}
	}

	writeJSON(w, http.StatusOK, model.ApiResponse[any]{
		Success: true,
		Message: "monitor config updated",
	})
}

// saveConfig writes the current config to disk as YAML.
func (h *ConfigHandler) saveConfig(path string) error {
	data, err := yaml.Marshal(h.config)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("writing config file: %w", err)
	}
	return nil
}
