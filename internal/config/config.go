package config

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"strconv"
	"time"

	"gopkg.in/yaml.v3"
)

// SourceType defines the type of project source.
type SourceType string

const (
	SourceTypeGitHub    SourceType = "github"
	SourceTypeGo        SourceType = "go"
	SourceTypeHashiCorp SourceType = "hashicorp"
	SourceTypeGrafana   SourceType = "grafana"
	SourceTypeNPM       SourceType = "npm"
	SourceTypePyPI      SourceType = "pypi"
	SourceTypeCrates    SourceType = "crates"
)

// Config is the top-level configuration for MiBeeHive.
type Config struct {
	Server    ServerConfig    `yaml:"server"`
	Database  DatabaseConfig  `yaml:"database"`
	Storage   StorageConfig   `yaml:"storage"`
	Auth      AuthConfig      `yaml:"auth"`
	Crawler   CrawlerConfig   `yaml:"crawler"`
	Monitor   MonitorConfig   `yaml:"monitor"`
	Logging   LoggingConfig   `yaml:"logging"`
	Backup    BackupConfig    `yaml:"backup"`
	Container ContainerConfig `yaml:"container"`
	Projects  []ProjectConfig `yaml:"projects"`
}

// ServerConfig holds HTTP server settings.
type ServerConfig struct {
	Port           int      `yaml:"port"`
	BindAddr       string   `yaml:"bind_addr"`
	HTTPSPort      int      `yaml:"https_port"` // 0 = disabled
	CertPath       string   `yaml:"cert_path"`
	KeyPath        string   `yaml:"key_path"`
	TLSIPAddresses []string `yaml:"tls_ip_addresses"`
	TLSDNSNames    []string `yaml:"tls_dns_names"`
}

// DatabaseConfig holds SQLite database settings.
type DatabaseConfig struct {
	Path string `yaml:"path"`
}

// StorageConfig holds file storage settings.
type StorageConfig struct {
	BasePath string      `yaml:"base_path" json:"base_path"`
	Modules  ModulePaths `yaml:"modules" json:"modules"`
}

// ModulePaths holds per-module storage path overrides.
// Empty paths fall back to {BasePath}/{module} convention.
type ModulePaths struct {
	OSS       string `yaml:"oss" json:"oss"`
	OSInstall string `yaml:"os_install" json:"os_install"`
	ISO       string `yaml:"iso" json:"iso"`
}

// AuthConfig holds authentication settings.
type AuthConfig struct {
	PasswordHash      string `yaml:"password_hash"`
	JWTSecret         string `yaml:"jwt_secret"`
	PasswordChangedAt string `yaml:"password_changed_at"`
}

// GetPasswordChangedAt returns the password_changed_at time, or zero time if not set.
func (a *AuthConfig) GetPasswordChangedAt() time.Time {
	if a.PasswordChangedAt == "" {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339, a.PasswordChangedAt)
	if err != nil {
		return time.Time{}
	}
	return t
}

// defaultPasswordHash is the bcrypt hash of the default "admin" password.
const defaultPasswordHash = "$2a$10$rsBcy69QYKw.zW5UloqBoOMFPOk0pmRfuEBCESiEbYijCRBAst0DG"

// IsDefaultPassword checks if the current password hash matches the default.
func (a *AuthConfig) IsDefaultPassword() bool {
	return a.PasswordHash == defaultPasswordHash
}

// CrawlerConfig holds crawler concurrency and timing settings.
type CrawlerConfig struct {
	MaxConcurrent   int    `yaml:"max_concurrent"`
	DefaultInterval string `yaml:"default_interval"`

	// FetchTimeout bounds a single source's fetch+parse so one slow/hung source
	// can't stall a crawl cycle. Parses as a Go duration; "0" disables it (then
	// the shared HTTP client's 30s overall timeout is the only bound). Default "60s".
	FetchTimeout string `yaml:"fetch_timeout"`
	// MaxRetries is how many times a transient fetch error (timeout, connection
	// reset, 5xx) is retried with exponential backoff before being marked failed.
	// Non-transient errors (4xx, config, rate-limited) are never retried. Default 3.
	MaxRetries int `yaml:"max_retries"`
	// RetryInitialBackoff is the base delay before the first retry; subsequent
	// retries back off exponentially (×2, with light jitter). Default "2s".
	RetryInitialBackoff string `yaml:"retry_initial_backoff"`
}

// MonitorConfig holds system monitoring settings.
type MonitorConfig struct {
	SampleInterval      int    `yaml:"sample_interval"`       // seconds, default 30
	RetentionDays       int    `yaml:"retention_days"`        // default 7, max 30
	NodeExporterURL     string `yaml:"node_exporter_url"`     // node_exporter metrics URL
	DiskWarningPercent  int    `yaml:"disk_warning_percent"`  // default 90
	DiskCriticalPercent int    `yaml:"disk_critical_percent"` // default 95
	DiskCheckEnabled    bool   `yaml:"disk_check_enabled"`    // default true
}

// LoggingConfig holds log rotation settings.
type LoggingConfig struct {
	Filename   string `yaml:"filename"`    // log file path
	MaxSize    int    `yaml:"max_size"`    // MB before rotation
	MaxBackups int    `yaml:"max_backups"` // number of old logs to keep
	MaxAge     int    `yaml:"max_age"`     // days to keep old logs
	Compress   bool   `yaml:"compress"`    // compress rotated logs
	LocalTime  bool   `yaml:"local_time"`  // use local time in filenames
}

// BackupConfig holds backup settings.
type BackupConfig struct {
	Enabled        bool   `yaml:"enabled"`
	Schedule       string `yaml:"schedule"`   // HH:MM format
	Retention      int    `yaml:"retention"`  // number of backups to keep
	LocalPath      string `yaml:"local_path"` // local backup directory
	RemoteURL      string `yaml:"remote_url"` // remote backup destination
	RemoteUsername string `yaml:"remote_username"`
	RemotePassword string `yaml:"remote_password"`
}

// ContainerConfig holds dual-mode container management settings.
type ContainerConfig struct {
	Local  LocalContainerConfig  `yaml:"local"`
	Remote RemoteContainerConfig `yaml:"remote"`
}

// LocalContainerConfig holds settings for local Docker container management.
type LocalContainerConfig struct {
	Enabled    bool   `yaml:"enabled"`
	DockerHost string `yaml:"docker_host"`
}

// RemoteContainerConfig holds settings for remote container management.
type RemoteContainerConfig struct {
	Enabled                bool   `yaml:"enabled"`
	SyncConcurrency        int    `yaml:"sync_concurrency"`
	RetentionCheckInterval string `yaml:"retention_check_interval"`
}

// ProjectConfig defines a single project to crawl and serve.
type ProjectConfig struct {
	Name           string     `yaml:"name"`
	DisplayName    string     `yaml:"display_name"`
	SourceType     SourceType `yaml:"source_type"`
	SourceURL      string     `yaml:"source_url"`
	CrawlInterval  string     `yaml:"crawl_interval"`
	GitHubOwner    string     `yaml:"github_owner"`
	GitHubRepo     string     `yaml:"github_repo"`
	FilterPatterns []string   `yaml:"filter_patterns"`
	StorageSubdir  string     `yaml:"storage_subdir"`
}

// DefaultConfig returns a Config populated with sensible defaults and all 13 projects.
func DefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Port:      9090,
			BindAddr:  "0.0.0.0",
			HTTPSPort: 9443,
		},
		Database: DatabaseConfig{
			Path: "./mibeehive.db",
		},
		Storage: StorageConfig{
			BasePath: "./data",
			Modules:  ModulePaths{},
		},
		Auth: AuthConfig{
			PasswordHash:      defaultPasswordHash,
			JWTSecret:         generateJWTSecret(),
			PasswordChangedAt: "", // do NOT default to time.Now() — that would
			// invalidate all existing tokens on every restart. Only set when the
			// password is actually changed (GetPasswordChangedAt returns zero →
			// middleware skips the iat check).
		},
		Crawler: CrawlerConfig{
			MaxConcurrent:      2,
			DefaultInterval:    "6h",
			FetchTimeout:       "60s",
			MaxRetries:         3,
			RetryInitialBackoff: "2s",
		},
		Monitor: MonitorConfig{
			SampleInterval:      30,
			RetentionDays:       7,
			NodeExporterURL:     "http://localhost:9100/metrics",
			DiskWarningPercent:  90,
			DiskCriticalPercent: 95,
			DiskCheckEnabled:    true,
		},
		Logging: LoggingConfig{
			Filename:   "./mibeehive.log",
			MaxSize:    10,
			MaxBackups: 3,
			MaxAge:     30,
			Compress:   true,
			LocalTime:  true,
		},
		Backup: BackupConfig{
			Enabled:   false,
			Schedule:  "03:00",
			Retention: 5,
			LocalPath: "./backups",
		},
		Container: ContainerConfig{
			Local: LocalContainerConfig{
				Enabled:    true,
				DockerHost: "unix:///var/run/docker.sock",
			},
			Remote: RemoteContainerConfig{
				Enabled:                true,
				SyncConcurrency:        2,
				RetentionCheckInterval: "1h",
			},
		},
		Projects: []ProjectConfig{},
	}
}

func SeedProjects() []ProjectConfig {
	return []ProjectConfig{
		// 7 GitHub projects
		{
			Name:          "prometheus",
			DisplayName:   "Prometheus",
			SourceType:    SourceTypeGitHub,
			SourceURL:     "https://github.com/prometheus/prometheus",
			CrawlInterval: "6h",
			GitHubOwner:   "prometheus",
			GitHubRepo:    "prometheus",
		},
		{
			Name:          "node-exporter",
			DisplayName:   "Node Exporter",
			SourceType:    SourceTypeGitHub,
			SourceURL:     "https://github.com/prometheus/node_exporter",
			CrawlInterval: "6h",
			GitHubOwner:   "prometheus",
			GitHubRepo:    "node_exporter",
		},
		{
			Name:          "blackbox-exporter",
			DisplayName:   "Blackbox Exporter",
			SourceType:    SourceTypeGitHub,
			SourceURL:     "https://github.com/prometheus/blackbox_exporter",
			CrawlInterval: "6h",
			GitHubOwner:   "prometheus",
			GitHubRepo:    "blackbox_exporter",
		},
		{
			Name:          "mysqld-exporter",
			DisplayName:   "MySQLd Exporter",
			SourceType:    SourceTypeGitHub,
			SourceURL:     "https://github.com/prometheus/mysqld_exporter",
			CrawlInterval: "6h",
			GitHubOwner:   "prometheus",
			GitHubRepo:    "mysqld_exporter",
		},
		{
			Name:          "victoriametrics",
			DisplayName:   "VictoriaMetrics",
			SourceType:    SourceTypeGitHub,
			SourceURL:     "https://github.com/VictoriaMetrics/VictoriaMetrics",
			CrawlInterval: "6h",
			GitHubOwner:   "VictoriaMetrics",
			GitHubRepo:    "VictoriaMetrics",
		},
		{
			Name:          "victorialogs",
			DisplayName:   "VictoriaLogs",
			SourceType:    SourceTypeGitHub,
			SourceURL:     "https://github.com/VictoriaMetrics/VictoriaLogs",
			CrawlInterval: "6h",
			GitHubOwner:   "VictoriaMetrics",
			GitHubRepo:    "VictoriaLogs",
		},
		{
			Name:          "vmagent",
			DisplayName:   "VMAgent",
			SourceType:    SourceTypeGitHub,
			SourceURL:     "https://github.com/VictoriaMetrics/vmagent",
			CrawlInterval: "6h",
			GitHubOwner:   "VictoriaMetrics",
			GitHubRepo:    "vmagent",
		},
		// 1 Go official
		{
			Name:          "golang",
			DisplayName:   "Go",
			SourceType:    SourceTypeGo,
			SourceURL:     "https://go.dev/dl/",
			CrawlInterval: "24h",
		},
		// 4 HashiCorp
		{
			Name:          "consul",
			DisplayName:   "Consul",
			SourceType:    SourceTypeHashiCorp,
			SourceURL:     "https://releases.hashicorp.com/consul/",
			CrawlInterval: "6h",
		},
		{
			Name:          "packer",
			DisplayName:   "Packer",
			SourceType:    SourceTypeHashiCorp,
			SourceURL:     "https://releases.hashicorp.com/packer/",
			CrawlInterval: "6h",
		},
		{
			Name:          "vagrant",
			DisplayName:   "Vagrant",
			SourceType:    SourceTypeHashiCorp,
			SourceURL:     "https://releases.hashicorp.com/vagrant/",
			CrawlInterval: "6h",
		},
		{
			Name:          "nomad",
			DisplayName:   "Nomad",
			SourceType:    SourceTypeHashiCorp,
			SourceURL:     "https://releases.hashicorp.com/nomad/",
			CrawlInterval: "6h",
		},
		// 1 Grafana
		{
			Name:          "grafana",
			DisplayName:   "Grafana",
			SourceType:    SourceTypeGrafana,
			SourceURL:     "https://github.com/grafana/grafana",
			CrawlInterval: "6h",
			GitHubOwner:   "grafana",
			GitHubRepo:    "grafana",
		},
	}
}

// Load reads and parses a YAML config file.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	cfg := DefaultConfig()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parsing config YAML: %w", err)
	}

	// Ensure JWT secret is set even if missing from config file.
	cfg.EnsureJWTSecret()
	// Ensure password hash falls back to the default when empty (default
	// config ships `password_hash: ""` meaning "use default admin password").
	cfg.EnsurePasswordHash()

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// GenerateDefault writes the default config to the given path.
func GenerateDefault(path string) error {
	cfg := DefaultConfig()

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshaling default config: %w", err)
	}

	if err := os.MkdirAll(parentDir(path), 0o755); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}

	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("writing config file: %w", err)
	}

	return nil
}

// Validate checks required fields and returns an error if invalid.
func (c *Config) Validate() error {
	if c.Server.Port <= 0 || c.Server.Port > 65535 {
		return fmt.Errorf("invalid server port: %d", c.Server.Port)
	}
	if c.Server.BindAddr == "" {
		return fmt.Errorf("server bind_addr is required")
	}
	if c.Database.Path == "" {
		return fmt.Errorf("database path is required")
	}
	if c.Storage.BasePath == "" {
		return fmt.Errorf("storage base_path is required")
	}

	// Validate module storage paths
	if err := validateModulePaths(c.Storage.Modules); err != nil {
		return fmt.Errorf("storage modules: %w", err)
	}
	if c.Crawler.MaxConcurrent < 1 {
		return fmt.Errorf("crawler max_concurrent must be >= 1")
	}
	if _, err := time.ParseDuration(c.Crawler.DefaultInterval); err != nil {
		return fmt.Errorf("invalid crawler default_interval %q: %w", c.Crawler.DefaultInterval, err)
	}
	if c.Crawler.FetchTimeout == "" {
		c.Crawler.FetchTimeout = "60s"
	}
	if _, err := time.ParseDuration(c.Crawler.FetchTimeout); err != nil {
		return fmt.Errorf("invalid crawler fetch_timeout %q: %w", c.Crawler.FetchTimeout, err)
	}
	if c.Crawler.RetryInitialBackoff == "" {
		c.Crawler.RetryInitialBackoff = "2s"
	}
	if _, err := time.ParseDuration(c.Crawler.RetryInitialBackoff); err != nil {
		return fmt.Errorf("invalid crawler retry_initial_backoff %q: %w", c.Crawler.RetryInitialBackoff, err)
	}
	if c.Crawler.MaxRetries < 0 {
		return fmt.Errorf("crawler max_retries must be >= 0")
	}
	// Validate monitor config
	if c.Monitor.RetentionDays <= 0 {
		c.Monitor.RetentionDays = 7
	}
	if c.Monitor.RetentionDays > 30 {
		c.Monitor.RetentionDays = 30
	}
	if c.Monitor.SampleInterval <= 0 {
		c.Monitor.SampleInterval = 30
	}

	// Validate logging config
	if c.Logging.MaxSize <= 0 {
		return fmt.Errorf("logging max_size must be > 0")
	}
	if c.Logging.MaxBackups < 0 {
		return fmt.Errorf("logging max_backups must be >= 0")
	}
	if c.Logging.MaxAge < 0 {
		return fmt.Errorf("logging max_age must be >= 0")
	}

	// Validate backup config
	if c.Backup.Enabled {
		if c.Backup.Retention <= 0 {
			return fmt.Errorf("backup retention must be > 0 when backup is enabled")
		}
		if err := validateSchedule(c.Backup.Schedule); err != nil {
			return fmt.Errorf("backup schedule: %w", err)
		}
	}

	// Validate disk monitor thresholds
	if c.Monitor.DiskWarningPercent < 1 || c.Monitor.DiskWarningPercent > 100 {
		return fmt.Errorf("monitor disk_warning_percent must be between 1 and 100")
	}
	if c.Monitor.DiskCriticalPercent < 1 || c.Monitor.DiskCriticalPercent > 100 {
		return fmt.Errorf("monitor disk_critical_percent must be between 1 and 100")
	}
	if c.Monitor.DiskCriticalPercent <= c.Monitor.DiskWarningPercent {
		return fmt.Errorf("monitor disk_critical_percent must be greater than disk_warning_percent")
	}

	// Validate container config
	if c.Container.Local.DockerHost == "" {
		return fmt.Errorf("container local docker_host is required")
	}
	if c.Container.Remote.SyncConcurrency < 1 {
		return fmt.Errorf("container remote sync_concurrency must be >= 1")
	}
	if _, err := time.ParseDuration(c.Container.Remote.RetentionCheckInterval); err != nil {
		return fmt.Errorf("invalid container remote retention_check_interval %q: %w", c.Container.Remote.RetentionCheckInterval, err)
	}

	return nil
}

// validateModulePaths checks per-module storage path overrides.
func validateModulePaths(mp ModulePaths) error {
	paths := map[string]string{
		"oss":        mp.OSS,
		"os_install": mp.OSInstall,
		"iso":        mp.ISO,
	}
	for name, p := range paths {
		if p == "" {
			continue // empty = use fallback
		}
		if p[0] != '/' {
			return fmt.Errorf("%s path %q is not absolute (must start with /)", name, p)
		}
		// If dir exists, verify it's writable
		if fi, err := os.Stat(p); err == nil {
			if !fi.IsDir() {
				return fmt.Errorf("%s path %q exists but is not a directory", name, p)
			}
			// Check write permission by creating a temp file
			tmp := p + "/.mibeehive_write_test"
			if err := os.WriteFile(tmp, []byte{}, 0o644); err != nil {
				return fmt.Errorf("%s path %q exists but is not writable: %w", name, p, err)
			}
			os.Remove(tmp)
		}
	}
	return nil
}

// generateJWTSecret generates a random 32-byte hex string for JWT signing.
func generateJWTSecret() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		// Fallback should never happen with crypto/rand on modern systems.
		return "fallback-secret-must-replace-" + fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

// EnsureJWTSecret generates a random JWT secret if one is not configured.
func (c *Config) EnsureJWTSecret() {
	if c.Auth.JWTSecret == "" {
		c.Auth.JWTSecret = generateJWTSecret()
	}
}

// EnsurePasswordHash falls back to the default "admin" password hash when the
// configured hash is empty. The default config file ships with
// `password_hash: ""` and a comment "default password is admin"; without this
// fallback the empty value (set by yaml.Unmarshal over DefaultConfig) would
// make every login fail with "invalid password". Mirrors EnsureJWTSecret.
func (c *Config) EnsurePasswordHash() {
	if c.Auth.PasswordHash == "" {
		c.Auth.PasswordHash = defaultPasswordHash
	}
}

func parentDir(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' {
			return path[:i]
		}
	}
	return "."
}

// validateSchedule checks that the schedule format is valid HH:MM.
func validateSchedule(schedule string) error {
	if len(schedule) != 5 {
		return fmt.Errorf("invalid schedule format %q, expected HH:MM", schedule)
	}
	if schedule[2] != ':' {
		return fmt.Errorf("invalid schedule format %q, expected HH:MM", schedule)
	}
	h, err := strconv.Atoi(schedule[0:2])
	if err != nil || h < 0 || h > 23 {
		return fmt.Errorf("invalid hour in schedule %q, expected 00-23", schedule)
	}
	m, err := strconv.Atoi(schedule[3:5])
	if err != nil || m < 0 || m > 59 {
		return fmt.Errorf("invalid minute in schedule %q, expected 00-59", schedule)
	}
	return nil
}
