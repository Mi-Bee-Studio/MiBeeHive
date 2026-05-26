package service

import (
	"path/filepath"
	"sync"

	"github.com/Mi-Bee-Studio/mibeehive/internal/config"
)

// StorageResolver provides thread-safe path resolution for all storage modules.
// It supports hot-reload via UpdateConfig() without server restart.
type StorageResolver struct {
	cfg *config.Config
	mu  sync.RWMutex
}

// NewStorageResolver creates a new StorageResolver with the given config.
func NewStorageResolver(cfg *config.Config) *StorageResolver {
	return &StorageResolver{cfg: cfg}
}

// ResolveOSS returns the storage path for OSS (Foraging) module.
// Fallback: base_path (no subdirectory — projects use project-name directly under base_path)
func (r *StorageResolver) ResolveOSS() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.cfg.Storage.Modules.OSS != "" {
		return r.cfg.Storage.Modules.OSS
	}
	return r.cfg.Storage.BasePath
}

// ResolveOSInstall returns the storage path for OS Install configs.
// Fallback: base_path/os-install
func (r *StorageResolver) ResolveOSInstall() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.cfg.Storage.Modules.OSInstall != "" {
		return r.cfg.Storage.Modules.OSInstall
	}
	return filepath.Join(r.cfg.Storage.BasePath, "os-install")
}

// ResolveISO returns the storage path for ISO images.
// Fallback: base_path/os-install
func (r *StorageResolver) ResolveISO() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.cfg.Storage.Modules.ISO != "" {
		return r.cfg.Storage.Modules.ISO
	}
	return filepath.Join(r.cfg.Storage.BasePath, "os-install")
}

// ResolveWebDAV returns the storage path for WebDAV shared files.
// Fallback: base_path/webdav
func (r *StorageResolver) ResolveWebDAV() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return filepath.Join(r.cfg.Storage.BasePath, "webdav")
}

// UpdateConfig atomically swaps the config for hot-reload.
func (r *StorageResolver) UpdateConfig(newCfg *config.Config) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.cfg = newCfg
}
