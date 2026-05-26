package db

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/Mi-Bee-Studio/mibeehive/internal/config"
	"github.com/Mi-Bee-Studio/mibeehive/internal/model"
)

func TestSeedProjectsFromConfig(t *testing.T) {
	db := testDB(t)
	repo := NewProjectRepo(db)
	ctx := context.Background()

	projects := []config.ProjectConfig{
		{
			Name:          "prometheus",
			DisplayName:   "Prometheus",
			SourceType:    config.SourceTypeGitHub,
			SourceURL:     "https://github.com/prometheus/prometheus",
			CrawlInterval: "6h",
			GitHubOwner:   "prometheus",
			GitHubRepo:    "prometheus",
			FilterPatterns: []string{"linux-arm64"},
		},
		{
			Name:          "consul",
			DisplayName:   "Consul",
			SourceType:    config.SourceTypeHashiCorp,
			SourceURL:     "https://releases.hashicorp.com/consul/",
			CrawlInterval: "12h",
		},
		{
			Name:          "golang",
			DisplayName:   "Go",
			SourceType:    config.SourceTypeGo,
			SourceURL:     "https://go.dev/dl/",
			CrawlInterval: "24h",
		},
	}

	// Seed into empty DB.
	count, err := SeedProjectsFromConfig(ctx, repo, projects)
	if err != nil {
		t.Fatalf("SeedProjectsFromConfig: %v", err)
	}
	if count != 3 {
		t.Fatalf("expected 3 seeded projects, got %d", count)
	}

	// Verify projects were created.
	all, err := repo.ListAll(ctx)
	if err != nil {
		t.Fatalf("ListAll: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("expected 3 projects in DB, got %d", len(all))
	}

	// Verify settings for prometheus (has GitHub fields + filter patterns).
	p, err := repo.GetByName(ctx, "prometheus")
	if err != nil {
		t.Fatalf("GetByName prometheus: %v", err)
	}
	if p == nil {
		t.Fatal("prometheus project not found")
	}
	settings, err := repo.GetSettings(ctx, p.ID)
	if err != nil {
		t.Fatalf("GetSettings prometheus: %v", err)
	}
	if settings.CrawlInterval != 360 {
		t.Errorf("expected crawl_interval=360 (6h), got %d", settings.CrawlInterval)
	}
	if settings.GitHubOwner != "prometheus" {
		t.Errorf("expected github_owner=prometheus, got %q", settings.GitHubOwner)
	}
	if settings.GitHubRepo != "prometheus" {
		t.Errorf("expected github_repo=prometheus, got %q", settings.GitHubRepo)
	}
	if len(settings.FilterPatterns) != 1 || settings.FilterPatterns[0] != "linux-arm64" {
		t.Errorf("expected filter_patterns=[linux-arm64], got %v", settings.FilterPatterns)
	}

	// Verify settings for consul (crawl_interval only).
	p2, err := repo.GetByName(ctx, "consul")
	if err != nil {
		t.Fatalf("GetByName consul: %v", err)
	}
	settings2, err := repo.GetSettings(ctx, p2.ID)
	if err != nil {
		t.Fatalf("GetSettings consul: %v", err)
	}
	if settings2.CrawlInterval != 720 {
		t.Errorf("expected crawl_interval=720 (12h), got %d", settings2.CrawlInterval)
	}

	// Second seed call should be a no-op.
	count2, err := SeedProjectsFromConfig(ctx, repo, projects)
	if err != nil {
		t.Fatalf("SeedProjectsFromConfig second call: %v", err)
	}
	if count2 != 0 {
		t.Errorf("expected 0 on second seed (no-op), got %d", count2)
	}

	// DB should still have exactly 3 projects.
	all2, err := repo.ListAll(ctx)
	if err != nil {
		t.Fatalf("ListAll after second seed: %v", err)
	}
	if len(all2) != 3 {
		t.Errorf("expected 3 projects after no-op seed, got %d", len(all2))
	}
}

func TestSeedProjectsFromConfig_InvalidInterval(t *testing.T) {
	db := testDB(t)
	repo := NewProjectRepo(db)
	ctx := context.Background()

	projects := []config.ProjectConfig{
		{
			Name:          "prometheus",
			DisplayName:   "Prometheus",
			SourceType:    config.SourceTypeGitHub,
			SourceURL:     "https://github.com/prometheus/prometheus",
			CrawlInterval: "not-a-duration",
		},
		{
			Name:          "consul",
			DisplayName:   "Consul",
			SourceType:    config.SourceTypeHashiCorp,
			SourceURL:     "https://releases.hashicorp.com/consul/",
			CrawlInterval: "6h",
		},
	}

	count, err := SeedProjectsFromConfig(ctx, repo, projects)
	if err != nil {
		t.Fatalf("SeedProjectsFromConfig: %v", err)
	}
	// prometheus should be skipped due to invalid interval, consul should be seeded.
	if count != 1 {
		t.Fatalf("expected 1 seeded project (consul), got %d", count)
	}
}

func TestSeedProjectsFromConfig_FullSeedList(t *testing.T) {
	db := testDB(t)
	repo := NewProjectRepo(db)
	ctx := context.Background()

	// Use the real SeedProjects list (13 projects).
	projects := config.SeedProjects()

	count, err := SeedProjectsFromConfig(ctx, repo, projects)
	if err != nil {
		t.Fatalf("SeedProjectsFromConfig: %v", err)
	}
	if count != 13 {
		t.Fatalf("expected 13 seeded projects, got %d", count)
	}

	// Spot-check: verify config JSON is valid model.ProjectSettings for each.
	all, err := repo.ListAll(ctx)
	if err != nil {
		t.Fatalf("ListAll: %v", err)
	}
	for _, p := range all {
		var settings model.ProjectSettings
		if err := json.Unmarshal([]byte(p.Config), &settings); err != nil {
			t.Errorf("invalid config JSON for project %q: %v", p.Name, err)
		}
		// All seed projects should have a non-zero crawl_interval.
		if settings.CrawlInterval <= 0 {
			t.Errorf("project %q: expected crawl_interval > 0, got %d", p.Name, settings.CrawlInterval)
		}
	}
}
