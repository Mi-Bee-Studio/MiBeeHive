package service

import (
	"path/filepath"
	"sync"
	"testing"

	"github.com/Mi-Bee-Studio/mibeehive/internal/config"
)

func TestStorageResolver_FallbackPaths(t *testing.T) {
	cfg := &config.Config{
		Storage: config.StorageConfig{
			BasePath: "/data/storage",
			Modules:  config.ModulePaths{}, // empty overrides
		},
	}
	r := NewStorageResolver(cfg)

	oss := r.ResolveOSS()
	if oss != "/data/storage" {
		t.Errorf("ResolveOSS fallback: expected /data/storage, got %q", oss)
	}

	wantOSInstall := filepath.Join("/data/storage", "os-install")
	gotOSInstall := r.ResolveOSInstall()
	if gotOSInstall != wantOSInstall {
		t.Errorf("ResolveOSInstall fallback: expected %q, got %q", wantOSInstall, gotOSInstall)
	}

	wantISO := filepath.Join("/data/storage", "os-install")
	gotISO := r.ResolveISO()
	if gotISO != wantISO {
		t.Errorf("ResolveISO fallback: expected %q, got %q", wantISO, gotISO)
	}
}

func TestStorageResolver_ModulePathsOverride(t *testing.T) {
	cfg := &config.Config{
		Storage: config.StorageConfig{
			BasePath: "/data/storage",
			Modules: config.ModulePaths{
				OSS:       "/mnt/oss",
				OSInstall: "/mnt/os-install",
				ISO:       "/mnt/iso",
			},
		},
	}
	r := NewStorageResolver(cfg)

	if got := r.ResolveOSS(); got != "/mnt/oss" {
		t.Errorf("ResolveOSS: expected /mnt/oss, got %q", got)
	}
	if got := r.ResolveOSInstall(); got != "/mnt/os-install" {
		t.Errorf("ResolveOSInstall: expected /mnt/os-install, got %q", got)
	}
	if got := r.ResolveISO(); got != "/mnt/iso" {
		t.Errorf("ResolveISO: expected /mnt/iso, got %q", got)
	}
}

func TestStorageResolver_UpdateConfig(t *testing.T) {
	initial := &config.Config{
		Storage: config.StorageConfig{
			BasePath: "/data/storage",
			Modules:  config.ModulePaths{},
		},
	}
	r := NewStorageResolver(initial)

	// Verify initial fallback.
	if got := r.ResolveOSS(); got != "/data/storage" {
		t.Fatalf("initial ResolveOSS: expected /data/storage, got %q", got)
	}

	// Hot-reload with new config.
	updated := &config.Config{
		Storage: config.StorageConfig{
			BasePath: "/new/storage",
			Modules: config.ModulePaths{
				OSS: "/mnt/oss",
			},
		},
	}
	r.UpdateConfig(updated)

	if got := r.ResolveOSS(); got != "/mnt/oss" {
		t.Errorf("after UpdateConfig ResolveOSS: expected /mnt/oss, got %q", got)
	}
	if got := r.ResolveOSInstall(); got != filepath.Join("/new/storage", "os-install") {
		t.Errorf("after UpdateConfig ResolveOSInstall: expected fallback, got %q", got)
	}
}

func TestStorageResolver_ConcurrentAccess(t *testing.T) {
	cfg := &config.Config{
		Storage: config.StorageConfig{
			BasePath: "/data/storage",
			Modules: config.ModulePaths{
				OSS:       "/mnt/oss",
				OSInstall: "/mnt/os-install",
				ISO:       "/mnt/iso",
			},
		},
	}
	r := NewStorageResolver(cfg)

	var wg sync.WaitGroup
	wg.Add(20)

	// 10 concurrent readers.
	for i := 0; i < 10; i++ {
		go func() {
			defer wg.Done()
			_ = r.ResolveOSS()
			_ = r.ResolveOSInstall()
			_ = r.ResolveISO()
		}()
	}

	// 10 concurrent writers (UpdateConfig).
	for i := 0; i < 10; i++ {
		go func() {
			defer wg.Done()
			alt := &config.Config{
				Storage: config.StorageConfig{
					BasePath: "/alt/storage",
					Modules: config.ModulePaths{
						OSS: "/alt/oss",
					},
				},
			}
			r.UpdateConfig(alt)
		}()
	}

	wg.Wait()

	// Verify final state is not empty after concurrent access.
	oss := r.ResolveOSS()
	if oss == "" {
		t.Error("ResolveOSS returned empty string after concurrent access")
	}
}
