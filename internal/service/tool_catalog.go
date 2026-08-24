package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/Mi-Bee-Studio/mibeehive/internal/db"
	"github.com/Mi-Bee-Studio/mibeehive/internal/model"
)

// ErrToolNotFound is returned when EnableTool is called with an unknown slug.
var ErrToolNotFound = errors.New("tool not found in catalog")

// ToolCatalogEntry is a pre-filled source configuration template for a common
// ops tool. Enabling an entry creates a crawl project from its template.
type ToolCatalogEntry struct {
	Slug          string `json:"slug"`
	Name          string `json:"name"`
	DisplayName   string `json:"display_name"`
	Category      string `json:"category"`
	SourceType    string `json:"source_type"`
	SourceURL     string `json:"source_url"`
	Description   string `json:"description"`
	ConfigTemplate string `json:"config_template,omitempty"`
}

// ToolCatalogService provides access to the built-in tool catalog and the
// one-click enable flow that materializes a catalog entry as a project.
type ToolCatalogService struct{}

// NewToolCatalogService creates a ToolCatalogService.
func NewToolCatalogService() *ToolCatalogService {
	return &ToolCatalogService{}
}

// ListCatalog returns all seed catalog entries.
func (s *ToolCatalogService) ListCatalog() []ToolCatalogEntry {
	return DefaultCatalog()
}

// EnableTool creates a project from the catalog template for the given slug.
// It is idempotent: if a project with the same name (slug) already exists, it
// is returned without creating a duplicate.
func (s *ToolCatalogService) EnableTool(ctx context.Context, repo *db.ProjectRepo, slug string) (*db.Project, error) {
	entry, ok := s.findEntry(slug)
	if !ok {
		return nil, fmt.Errorf("enabling tool %q: %w", slug, ErrToolNotFound)
	}

	// Idempotency: a project named after the slug already exists.
	existing, err := repo.GetByName(ctx, entry.Slug)
	if err != nil {
		return nil, fmt.Errorf("checking existing tool %q: %w", slug, err)
	}
	if existing != nil {
		return existing, nil
	}

	settings := model.ProjectSettings{}
	if entry.ConfigTemplate != "" {
		if err := json.Unmarshal([]byte(entry.ConfigTemplate), &settings); err != nil {
			return nil, fmt.Errorf("parsing config template for %q: %w", slug, err)
		}
	}
	if settings.StorageSubdir == "" {
		settings.StorageSubdir = DefaultStorageSubdir(entry.Slug)
	}

	project, err := repo.CreateWithSettings(ctx, entry.Slug, entry.DisplayName, entry.SourceType, entry.SourceURL, settings)
	if err != nil {
		return nil, fmt.Errorf("creating project from tool %q: %w", slug, err)
	}
	return project, nil
}

func (s *ToolCatalogService) findEntry(slug string) (ToolCatalogEntry, bool) {
	for _, e := range DefaultCatalog() {
		if e.Slug == slug {
			return e, true
		}
	}
	return ToolCatalogEntry{}, false
}

// DefaultCatalog returns the built-in seed catalog (≤25 items) grouped by
// category. Each entry carries a pre-filled source config template so a user
// can enable a tool with one click.
func DefaultCatalog() []ToolCatalogEntry {
	return []ToolCatalogEntry{
		// Monitoring
		{
			Slug: "prometheus", Name: "prometheus", DisplayName: "Prometheus",
			Category: "monitoring", SourceType: "github",
			SourceURL:     "https://github.com/prometheus/prometheus",
			Description:   "Open-source systems monitoring and alerting toolkit.",
			ConfigTemplate: `{"github_owner":"prometheus","github_repo":"prometheus","version_pattern":"v[0-9]+\\.[0-9]+\\.[0-9]+"}`,
		},
		{
			Slug: "grafana", Name: "grafana", DisplayName: "Grafana",
			Category: "monitoring", SourceType: "grafana",
			SourceURL:     "https://github.com/grafana/grafana",
			Description:   "Open-source analytics and interactive visualization web application.",
			ConfigTemplate: `{"github_owner":"grafana","github_repo":"grafana"}`,
		},
		{
			Slug: "node_exporter", Name: "node_exporter", DisplayName: "Node Exporter",
			Category: "monitoring", SourceType: "github",
			SourceURL:     "https://github.com/prometheus/node_exporter",
			Description:   "Prometheus exporter for hardware and OS metrics.",
			ConfigTemplate: `{"github_owner":"prometheus","github_repo":"node_exporter","version_pattern":"v[0-9]+\\.[0-9]+\\.[0-9]+"}`,
		},
		{
			Slug: "alertmanager", Name: "alertmanager", DisplayName: "Alertmanager",
			Category: "monitoring", SourceType: "github",
			SourceURL:     "https://github.com/prometheus/alertmanager",
			Description:   "Handles alerts sent by client applications such as the Prometheus server.",
			ConfigTemplate: `{"github_owner":"prometheus","github_repo":"alertmanager","version_pattern":"v[0-9]+\\.[0-9]+\\.[0-9]+"}`,
		},
		{
			Slug: "loki", Name: "loki", DisplayName: "Loki",
			Category: "monitoring", SourceType: "github",
			SourceURL:     "https://github.com/grafana/loki",
			Description:   "Horizontally-scalable, highly-available log aggregation system.",
			ConfigTemplate: `{"github_owner":"grafana","github_repo":"loki","version_pattern":"v[0-9]+\\.[0-9]+\\.[0-9]+"}`,
		},
		// Database
		{
			Slug: "postgres", Name: "postgres", DisplayName: "PostgreSQL",
			Category: "database", SourceType: "github",
			SourceURL:     "https://github.com/postgres/postgres",
			Description:   "Advanced open-source relational database.",
			ConfigTemplate: `{"github_owner":"postgres","github_repo":"postgres"}`,
		},
		{
			Slug: "mysql", Name: "mysql", DisplayName: "MySQL",
			Category: "database", SourceType: "github",
			SourceURL:     "https://github.com/mysql/mysql-server",
			Description:   "Open-source relational database management system.",
			ConfigTemplate: `{"github_owner":"mysql","github_repo":"mysql-server"}`,
		},
		{
			Slug: "redis", Name: "redis", DisplayName: "Redis",
			Category: "database", SourceType: "github",
			SourceURL:     "https://github.com/redis/redis",
			Description:   "In-memory data structure store used as a database and cache.",
			ConfigTemplate: `{"github_owner":"redis","github_repo":"redis"}`,
		},
		{
			Slug: "mongodb", Name: "mongodb", DisplayName: "MongoDB",
			Category: "database", SourceType: "github",
			SourceURL:     "https://github.com/mongodb/mongo",
			Description:   "Source-available cross-platform document-oriented database.",
			ConfigTemplate: `{"github_owner":"mongodb","github_repo":"mongo"}`,
		},
		// Runtime
		{
			Slug: "go", Name: "go", DisplayName: "Go",
			Category: "runtime", SourceType: "go",
			SourceURL:     "https://go.dev/dl/",
			Description:   "Open-source programming language and toolchain.",
			ConfigTemplate: `{"version_pattern":"go[0-9]+\\.[0-9]+(\\.[0-9]+)?"}`,
		},
		{
			Slug: "nodejs", Name: "nodejs", DisplayName: "Node.js",
			Category: "runtime", SourceType: "github",
			SourceURL:     "https://github.com/nodejs/node",
			Description:   "JavaScript runtime built on Chrome's V8 JavaScript engine.",
			ConfigTemplate: `{"github_owner":"nodejs","github_repo":"node","version_pattern":"v[0-9]+\\.[0-9]+\\.[0-9]+"}`,
		},
		{
			Slug: "python", Name: "python", DisplayName: "Python",
			Category: "runtime", SourceType: "github",
			SourceURL:     "https://github.com/python/cpython",
			Description:   "High-level, general-purpose programming language.",
			ConfigTemplate: `{"github_owner":"python","github_repo":"cpython","version_pattern":"v[0-9]+\\.[0-9]+\\.[0-9]+"}`,
		},
		// Infrastructure
		{
			Slug: "nginx", Name: "nginx", DisplayName: "Nginx",
			Category: "infrastructure", SourceType: "github",
			SourceURL:     "https://github.com/nginx/nginx",
			Description:   "High-performance HTTP server and reverse proxy.",
			ConfigTemplate: `{"github_owner":"nginx","github_repo":"nginx"}`,
		},
		{
			Slug: "haproxy", Name: "haproxy", DisplayName: "HAProxy",
			Category: "infrastructure", SourceType: "github",
			SourceURL:     "https://github.com/haproxy/haproxy",
			Description:   "Reliable, high-performance TCP/HTTP load balancer.",
			ConfigTemplate: `{"github_owner":"haproxy","github_repo":"haproxy","version_pattern":"v[0-9]+\\.[0-9]+\\.[0-9]+"}`,
		},
		{
			Slug: "consul", Name: "consul", DisplayName: "Consul",
			Category: "infrastructure", SourceType: "hashicorp",
			SourceURL:     "https://releases.hashicorp.com/consul/",
			Description:   "Service networking solution to connect and secure services.",
			ConfigTemplate: `{"github_owner":"consul","version_pattern":"[0-9]+\\.[0-9]+\\.[0-9]+"}`,
		},
		{
			Slug: "vault", Name: "vault", DisplayName: "Vault",
			Category: "infrastructure", SourceType: "hashicorp",
			SourceURL:     "https://releases.hashicorp.com/vault/",
			Description:   "Tool for securely accessing secrets.",
			ConfigTemplate: `{"github_owner":"vault","version_pattern":"[0-9]+\\.[0-9]+\\.[0-9]+"}`,
		},
		{
			Slug: "traefik", Name: "traefik", DisplayName: "Traefik",
			Category: "infrastructure", SourceType: "github",
			SourceURL:     "https://github.com/traefik/traefik",
			Description:   "Modern HTTP reverse proxy and load balancer.",
			ConfigTemplate: `{"github_owner":"traefik","github_repo":"traefik","version_pattern":"v[0-9]+\\.[0-9]+\\.[0-9]+"}`,
		},
		// Container
		{
			Slug: "docker", Name: "docker", DisplayName: "Docker",
			Category: "container", SourceType: "github",
			SourceURL:     "https://github.com/moby/moby",
			Description:   "Open platform for developing, shipping, and running applications.",
			ConfigTemplate: `{"github_owner":"moby","github_repo":"moby","version_pattern":"v[0-9]+\\.[0-9]+\\.[0-9]+"}`,
		},
		{
			Slug: "containerd", Name: "containerd", DisplayName: "containerd",
			Category: "container", SourceType: "github",
			SourceURL:     "https://github.com/containerd/containerd",
			Description:   "Industry-standard container runtime.",
			ConfigTemplate: `{"github_owner":"containerd","github_repo":"containerd","version_pattern":"v[0-9]+\\.[0-9]+\\.[0-9]+"}`,
		},
		{
			Slug: "runc", Name: "runc", DisplayName: "runc",
			Category: "container", SourceType: "github",
			SourceURL:     "https://github.com/opencontainers/runc",
			Description:   "CLI tool for spawning and running containers.",
			ConfigTemplate: `{"github_owner":"opencontainers","github_repo":"runc","version_pattern":"v[0-9]+\\.[0-9]+\\.[0-9]+"}`,
		},
		{
			Slug: "podman", Name: "podman", DisplayName: "Podman",
			Category: "container", SourceType: "github",
			SourceURL:     "https://github.com/containers/podman",
			Description:   "Daemonless container engine for developing and managing OCI containers.",
			ConfigTemplate: `{"github_owner":"containers","github_repo":"podman","version_pattern":"v[0-9]+\\.[0-9]+\\.[0-9]+"}`,
		},
		// Other
		{
			Slug: "terraform", Name: "terraform", DisplayName: "Terraform",
			Category: "other", SourceType: "hashicorp",
			SourceURL:     "https://releases.hashicorp.com/terraform/",
			Description:   "Infrastructure as code software tool.",
			ConfigTemplate: `{"github_owner":"terraform","version_pattern":"[0-9]+\\.[0-9]+\\.[0-9]+"}`,
		},
		{
			Slug: "packer", Name: "packer", DisplayName: "Packer",
			Category: "other", SourceType: "hashicorp",
			SourceURL:     "https://releases.hashicorp.com/packer/",
			Description:   "Tool for creating identical machine images for multiple platforms.",
			ConfigTemplate: `{"github_owner":"packer","version_pattern":"[0-9]+\\.[0-9]+\\.[0-9]+"}`,
		},
		{
			Slug: "ansible", Name: "ansible", DisplayName: "Ansible",
			Category: "other", SourceType: "github",
			SourceURL:     "https://github.com/ansible/ansible",
			Description:   "Radically simple IT automation engine.",
			ConfigTemplate: `{"github_owner":"ansible","github_repo":"ansible"}`,
		},
		{
			Slug: "kubectl", Name: "kubectl", DisplayName: "kubectl",
			Category: "other", SourceType: "github",
			SourceURL:     "https://github.com/kubernetes/kubernetes",
			Description:   "Command-line tool for controlling Kubernetes clusters.",
			ConfigTemplate: `{"github_owner":"kubernetes","github_repo":"kubernetes","version_pattern":"v[0-9]+\\.[0-9]+\\.[0-9]+"}`,
		},
	}
}