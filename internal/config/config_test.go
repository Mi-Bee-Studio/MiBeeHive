package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfigValidation(t *testing.T) {
	cfg := DefaultConfig()
	if err := cfg.Validate(); err != nil {
		t.Fatalf("default config should be valid: %v", err)
	}
}

func TestGenerateDefault(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "subdir", "config.yaml")

	if err := GenerateDefault(path); err != nil {
		t.Fatalf("GenerateDefault: %v", err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("config file not created: %v", err)
	}

	// Verify we can load it back
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("loading generated config: %v", err)
	}
	if cfg.Server.Port != 9090 {
		t.Errorf("expected port 9090, got %d", cfg.Server.Port)
	}
}

func TestLoadFromYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	yamlContent := `server:
  port: 8080
  bind_addr: "127.0.0.1"
database:
  path: "/tmp/test.db"
storage:
  base_path: "/tmp/storage"
auth:
  password_hash: "hashed123"
crawler:
  max_concurrent: 3
  default_interval: "12h"
projects:
  - name: "prometheus"
    display_name: "Prometheus"
    source_type: "github"
    source_url: "https://github.com/prometheus/prometheus"
    crawl_interval: "6h"
    github_owner: "prometheus"
    github_repo: "prometheus"
  - name: "golang"
    display_name: "Go"
    source_type: "go"
    source_url: "https://go.dev/dl/"
    crawl_interval: "24h"
`

	if err := os.WriteFile(path, []byte(yamlContent), 0o644); err != nil {
		t.Fatalf("writing test config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.Server.Port != 8080 {
		t.Errorf("expected port 8080, got %d", cfg.Server.Port)
	}
	if cfg.Server.BindAddr != "127.0.0.1" {
		t.Errorf("expected bind_addr 127.0.0.1, got %s", cfg.Server.BindAddr)
	}
	if cfg.Crawler.MaxConcurrent != 3 {
		t.Errorf("expected max_concurrent 3, got %d", cfg.Crawler.MaxConcurrent)
	}
	if len(cfg.Projects) != 2 {
		t.Errorf("expected 2 projects, got %d", len(cfg.Projects))
	}
	if cfg.Projects[0].Name != "prometheus" {
		t.Errorf("expected first project prometheus, got %s", cfg.Projects[0].Name)
	}
}

func TestValidationErrors(t *testing.T) {
	tests := []struct {
		name    string
		modify  func(*Config)
		wantErr string
	}{
		{
			name:    "invalid port",
			modify:  func(c *Config) { c.Server.Port = 0 },
			wantErr: "invalid server port",
		},
		{
			name:    "port too high",
			modify:  func(c *Config) { c.Server.Port = 70000 },
			wantErr: "invalid server port",
		},
		{
			name:    "empty bind_addr",
			modify:  func(c *Config) { c.Server.BindAddr = "" },
			wantErr: "bind_addr is required",
		},
		{
			name:    "empty database path",
			modify:  func(c *Config) { c.Database.Path = "" },
			wantErr: "database path is required",
		},
		{
			name:    "empty storage path",
			modify:  func(c *Config) { c.Storage.BasePath = "" },
			wantErr: "storage base_path is required",
		},
		{
			name:    "invalid crawl interval",
			modify:  func(c *Config) { c.Crawler.DefaultInterval = "not-a-duration" },
			wantErr: "invalid crawler default_interval",
		},
		{
			name:    "logging max_size zero",
			modify:  func(c *Config) { c.Logging.MaxSize = 0 },
			wantErr: "must be > 0",
		},
		{
			name:    "logging max_backups negative",
			modify:  func(c *Config) { c.Logging.MaxBackups = -1 },
			wantErr: "must be >= 0",
		},
		{
			name:    "backup enabled with zero retention",
			modify:  func(c *Config) { c.Backup.Enabled = true; c.Backup.Retention = 0 },
			wantErr: "must be > 0",
		},
		{
			name:    "backup enabled with invalid schedule",
			modify:  func(c *Config) { c.Backup.Enabled = true; c.Backup.Schedule = "24:00" },
			wantErr: "backup schedule",
		},
		{
			name:    "disk warning percent too high",
			modify:  func(c *Config) { c.Monitor.DiskWarningPercent = 101 },
			wantErr: "disk_warning_percent must be between",
		},
		{
			name:    "disk critical percent too low",
			modify:  func(c *Config) { c.Monitor.DiskCriticalPercent = 0 },
			wantErr: "disk_critical_percent must be between",
		},
		{
			name:    "disk critical not greater than warning",
			modify:  func(c *Config) { c.Monitor.DiskWarningPercent = 80; c.Monitor.DiskCriticalPercent = 80 },
			wantErr: "disk_critical_percent must be greater than",
		},
		{
			name:    "empty container docker_host",
			modify:  func(c *Config) { c.Container.Local.DockerHost = "" },
			wantErr: "docker_host is required",
		},
		{
			name:    "container sync_concurrency zero",
			modify:  func(c *Config) { c.Container.Remote.SyncConcurrency = 0 },
			wantErr: "sync_concurrency must be >=",
		},
		{
			name:    "container sync_concurrency negative",
			modify:  func(c *Config) { c.Container.Remote.SyncConcurrency = -1 },
			wantErr: "sync_concurrency must be >=",
		},
		{
			name:    "invalid container retention_check_interval",
			modify:  func(c *Config) { c.Container.Remote.RetentionCheckInterval = "not-a-duration" },
			wantErr: "invalid container remote retention_check_interval",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig()
			tt.modify(cfg)
			err := cfg.Validate()
			if err == nil {
				t.Fatal("expected validation error, got nil")
			}
			if err.Error() == "" {
				t.Fatal("error message should not be empty")
			}
		})
	}
}

func TestLoadMissingFile(t *testing.T) {
	_, err := Load("/nonexistent/path/config.yaml")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestLoadInvalidYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.yaml")
	if err := os.WriteFile(path, []byte("{{invalid yaml"), 0o644); err != nil {
		t.Fatalf("writing test file: %v", err)
	}

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestDefaultConfigJWTSecret(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Auth.JWTSecret == "" {
		t.Fatal("default config should have a non-empty JWT secret")
	}
	if len(cfg.Auth.JWTSecret) < 32 {
		t.Fatalf("JWT secret too short: got %d chars, expected >= 32", len(cfg.Auth.JWTSecret))
	}
}

func TestEnsureJWTSecret(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Auth.JWTSecret = ""
	if cfg.Auth.JWTSecret != "" {
		t.Fatal("JWT secret should be empty after clearing")
	}
	cfg.EnsureJWTSecret()
	if cfg.Auth.JWTSecret == "" {
		t.Fatal("EnsureJWTSecret should generate a secret")
	}
}

func TestEnsureJWTSecretPreservesExisting(t *testing.T) {
	cfg := DefaultConfig()
	original := cfg.Auth.JWTSecret
	cfg.EnsureJWTSecret()
	if cfg.Auth.JWTSecret != original {
		t.Fatal("EnsureJWTSecret should not overwrite existing secret")
	}
}

func TestLoadPreservesConfiguredJWTSecret(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/config.yaml"
	yamlContent := `server:
  port: 9090
  bind_addr: "0.0.0.0"
database:
  path: "/tmp/test.db"
storage:
  base_path: "/tmp/storage"
auth:
  jwt_secret: "my-custom-secret-12345"
crawler:
  max_concurrent: 2
  default_interval: "6h"
projects:
  - name: "prometheus"
    display_name: "Prometheus"
    source_type: "github"
    source_url: "https://github.com/prometheus/prometheus"
    crawl_interval: "6h"
    github_owner: "prometheus"
    github_repo: "prometheus"
`
	if err := os.WriteFile(path, []byte(yamlContent), 0o644); err != nil {
		t.Fatalf("writing test config: %v", err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Auth.JWTSecret != "my-custom-secret-12345" {
		t.Errorf("expected configured secret to be preserved, got %q", cfg.Auth.JWTSecret)
	}
}

func TestLoadGeneratesJWTSecretWhenMissing(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/config.yaml"
	yamlContent := `server:
  port: 9090
  bind_addr: "0.0.0.0"
database:
  path: "/tmp/test.db"
storage:
  base_path: "/tmp/storage"
crawler:
  max_concurrent: 2
  default_interval: "6h"
projects:
  - name: "prometheus"
    display_name: "Prometheus"
    source_type: "github"
    source_url: "https://github.com/prometheus/prometheus"
    crawl_interval: "6h"
    github_owner: "prometheus"
    github_repo: "prometheus"
`
	if err := os.WriteFile(path, []byte(yamlContent), 0o644); err != nil {
		t.Fatalf("writing test config: %v", err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Auth.JWTSecret == "" {
		t.Fatal("Load should generate JWT secret when missing from config")
	}
}

func TestSeedProjects(t *testing.T) {
	projects := SeedProjects()
	if len(projects) != 13 {
		t.Fatalf("expected 13 seed projects, got %d", len(projects))
	}
}

func TestValidateEmptyProjects(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Projects = nil
	if err := cfg.Validate(); err != nil {
		t.Fatalf("empty projects should validate: %v", err)
	}
}

func TestContainerConfigDefaults(t *testing.T) {
	cfg := DefaultConfig()
	if !cfg.Container.Local.Enabled {
		t.Errorf("expected Container.Local.Enabled == true, got %v", cfg.Container.Local.Enabled)
	}
	if cfg.Container.Local.DockerHost != "unix:///var/run/docker.sock" {
		t.Errorf("expected DockerHost unix:///var/run/docker.sock, got %q", cfg.Container.Local.DockerHost)
	}
	if !cfg.Container.Remote.Enabled {
		t.Errorf("expected Container.Remote.Enabled == true, got %v", cfg.Container.Remote.Enabled)
	}
	if cfg.Container.Remote.SyncConcurrency != 2 {
		t.Errorf("expected SyncConcurrency 2, got %d", cfg.Container.Remote.SyncConcurrency)
	}
	if cfg.Container.Remote.RetentionCheckInterval != "1h" {
		t.Errorf("expected RetentionCheckInterval 1h, got %q", cfg.Container.Remote.RetentionCheckInterval)
	}
}

func TestContainerConfigYAMLWithSection(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/config.yaml"
	yamlContent := "server:\n  port: 9090\n  bind_addr: \"0.0.0.0\"\ndatabase:\n  path: /tmp/test.db\nstorage:\n  base_path: /tmp/storage\ncontainer:\n  local:\n    enabled: false\n    docker_host: /custom/docker.sock\n  remote:\n    enabled: false\n    sync_concurrency: 4\n    retention_check_interval: 5m\nprojects: []\n"
	if err := os.WriteFile(path, []byte(yamlContent), 0o644); err != nil {
		t.Fatalf("writing test config: %v", err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Container.Local.Enabled {
		t.Errorf("expected Container.Local.Enabled == false, got %v", cfg.Container.Local.Enabled)
	}
	if cfg.Container.Local.DockerHost != "/custom/docker.sock" {
		t.Errorf("expected DockerHost /custom/docker.sock, got %q", cfg.Container.Local.DockerHost)
	}
	if cfg.Container.Remote.Enabled {
		t.Errorf("expected Container.Remote.Enabled == false, got %v", cfg.Container.Remote.Enabled)
	}
	if cfg.Container.Remote.SyncConcurrency != 4 {
		t.Errorf("expected SyncConcurrency 4, got %d", cfg.Container.Remote.SyncConcurrency)
	}
	if cfg.Container.Remote.RetentionCheckInterval != "5m" {
		t.Errorf("expected RetentionCheckInterval 5m, got %q", cfg.Container.Remote.RetentionCheckInterval)
	}
}

func TestContainerConfigYAMLWithoutSection(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/config.yaml"
	yamlContent := "server:\n  port: 9090\n  bind_addr: \"0.0.0.0\"\ndatabase:\n  path: /tmp/test.db\nstorage:\n  base_path: /tmp/storage\nprojects: []\n"
	if err := os.WriteFile(path, []byte(yamlContent), 0o644); err != nil {
		t.Fatalf("writing test config: %v", err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !cfg.Container.Local.Enabled {
		t.Errorf("expected Container.Local.Enabled == true (default), got %v", cfg.Container.Local.Enabled)
	}
	if cfg.Container.Local.DockerHost != "unix:///var/run/docker.sock" {
		t.Errorf("expected DockerHost default unix:///var/run/docker.sock, got %q", cfg.Container.Local.DockerHost)
	}
	if !cfg.Container.Remote.Enabled {
		t.Errorf("expected Container.Remote.Enabled == true (default), got %v", cfg.Container.Remote.Enabled)
	}
	if cfg.Container.Remote.SyncConcurrency != 2 {
		t.Errorf("expected SyncConcurrency 2 (default), got %d", cfg.Container.Remote.SyncConcurrency)
	}
	if cfg.Container.Remote.RetentionCheckInterval != "1h" {
		t.Errorf("expected RetentionCheckInterval 1h (default), got %q", cfg.Container.Remote.RetentionCheckInterval)
	}
}
