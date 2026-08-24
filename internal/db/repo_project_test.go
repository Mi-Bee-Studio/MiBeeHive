package db

import (
	"context"
	"testing"

	"github.com/Mi-Bee-Studio/mibeehive/internal/model"
)

func TestProjectRepoCreateWithSettings(t *testing.T) {
	db := testDB(t)
	repo := NewProjectRepo(db)
	ctx := context.Background()

	settings := model.ProjectSettings{
		CrawlInterval:  3600,
		GitHubOwner:    "prometheus",
		GitHubRepo:     "prometheus",
		FilterPatterns: []string{"linux-arm64", "linux-amd64"},
		StorageSubpath: "prometheus",
		DownloadAll:    true,
	}

	p, err := repo.CreateWithSettings(ctx, "prometheus", "Prometheus", "github", "https://github.com/prometheus/prometheus", settings)
	if err != nil {
		t.Fatalf("CreateWithSettings: %v", err)
	}
	if p.ID == 0 {
		t.Fatal("expected non-zero ID")
	}
	if !p.Enabled {
		t.Error("expected new project to be enabled by default")
	}

	// Verify settings round-trip.
	got, err := repo.GetSettings(ctx, p.ID)
	if err != nil {
		t.Fatalf("GetSettings: %v", err)
	}
	if got == nil {
		t.Fatal("GetSettings: expected non-nil settings")
	}
	if got.CrawlInterval != 3600 {
		t.Errorf("CrawlInterval: expected 3600, got %d", got.CrawlInterval)
	}
	if got.GitHubOwner != "prometheus" {
		t.Errorf("GitHubOwner: expected prometheus, got %q", got.GitHubOwner)
	}
	if got.GitHubRepo != "prometheus" {
		t.Errorf("GitHubRepo: expected prometheus, got %q", got.GitHubRepo)
	}
	if len(got.FilterPatterns) != 2 {
		t.Fatalf("FilterPatterns: expected 2, got %d", len(got.FilterPatterns))
	}
	if got.StorageSubpath != "prometheus" {
		t.Errorf("StorageSubpath: expected prometheus, got %q", got.StorageSubpath)
	}
	if !got.DownloadAll {
		t.Error("DownloadAll: expected true")
	}
}

func TestProjectRepoUpdateProject(t *testing.T) {
	db := testDB(t)
	repo := NewProjectRepo(db)
	ctx := context.Background()

	p, err := repo.Create(ctx, "consul", "Consul", "hashicorp", "https://releases.hashicorp.com/consul/")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	newSettings := model.ProjectSettings{
		CrawlInterval: 7200,
		GitHubOwner:   "hashicorp",
		GitHubRepo:    "consul",
	}
	err = repo.UpdateProject(ctx, p.ID, "consul", "Consul Updated", "hashicorp", "https://example.com/consul", newSettings)
	if err != nil {
		t.Fatalf("UpdateProject: %v", err)
	}

	// Verify updated fields.
	got, err := repo.GetByID(ctx, p.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.DisplayName != "Consul Updated" {
		t.Errorf("DisplayName: expected 'Consul Updated', got %q", got.DisplayName)
	}
	if got.SourceURL != "https://example.com/consul" {
		t.Errorf("SourceURL: expected updated URL, got %q", got.SourceURL)
	}

	// Verify settings round-trip.
	settings, err := repo.GetSettings(ctx, p.ID)
	if err != nil {
		t.Fatalf("GetSettings: %v", err)
	}
	if settings.CrawlInterval != 7200 {
		t.Errorf("CrawlInterval: expected 7200, got %d", settings.CrawlInterval)
	}
	if settings.GitHubOwner != "hashicorp" {
		t.Errorf("GitHubOwner: expected hashicorp, got %q", settings.GitHubOwner)
	}
}

func TestProjectRepoGetSettingsEmpty(t *testing.T) {
	db := testDB(t)
	repo := NewProjectRepo(db)
	ctx := context.Background()

	// Create without settings (default {} config).
	p, err := repo.Create(ctx, "grafana", "Grafana", "grafana", "https://github.com/grafana/grafana")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := repo.GetSettings(ctx, p.ID)
	if err != nil {
		t.Fatalf("GetSettings: %v", err)
	}
	if got == nil {
		t.Fatal("GetSettings: expected non-nil for empty JSON object")
	}
	if got.CrawlInterval != 0 {
		t.Errorf("CrawlInterval: expected 0, got %d", got.CrawlInterval)
	}
}

func TestProjectRepoGetSettingsNonExistent(t *testing.T) {
	db := testDB(t)
	repo := NewProjectRepo(db)
	ctx := context.Background()

	got, err := repo.GetSettings(ctx, 9999)
	if err != nil {
		t.Fatalf("GetSettings(non-existent): %v", err)
	}
	if got != nil {
		t.Errorf("GetSettings(non-existent): expected nil, got %v", got)
	}
}

func TestProjectRepoSetEnabled(t *testing.T) {
	db := testDB(t)
	repo := NewProjectRepo(db)
	ctx := context.Background()

	p, err := repo.Create(ctx, "prometheus", "Prometheus", "github", "https://github.com/prometheus/prometheus")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// New projects should be enabled by default.
	if !p.Enabled {
		t.Error("expected new project to be enabled")
	}

	// Disable.
	err = repo.SetEnabled(ctx, p.ID, false)
	if err != nil {
		t.Fatalf("SetEnabled(false): %v", err)
	}
	got, err := repo.GetByID(ctx, p.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Enabled {
		t.Error("expected project to be disabled")
	}

	// Re-enable.
	err = repo.SetEnabled(ctx, p.ID, true)
	if err != nil {
		t.Fatalf("SetEnabled(true): %v", err)
	}
	got, err = repo.GetByID(ctx, p.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if !got.Enabled {
		t.Error("expected project to be re-enabled")
	}
}

func TestProjectRepoSoftDelete(t *testing.T) {
	db := testDB(t)
	repo := NewProjectRepo(db)
	ctx := context.Background()

	p, err := repo.Create(ctx, "prometheus", "Prometheus", "github", "https://github.com/prometheus/prometheus")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Hard delete.
	err = repo.Delete(ctx, p.ID)
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}

	// Project should no longer exist.
	got, err := repo.GetByID(ctx, p.ID)
	if err != nil {
		t.Fatalf("GetByID after delete: %v", err)
	}
	if got != nil {
		t.Fatal("GetByID: expected project to be nil after hard delete")
	}
}

func TestProjectRepoListEnabled(t *testing.T) {
	db := testDB(t)
	repo := NewProjectRepo(db)
	ctx := context.Background()

	// Create 3 projects.
	_, _ = repo.Create(ctx, "alpha", "Alpha", "github", "https://github.com/a/a")
	pBeta, _ := repo.Create(ctx, "beta", "Beta", "github", "https://github.com/b/b")
	_, _ = repo.Create(ctx, "gamma", "Gamma", "github", "https://github.com/c/c")

	// Disable beta.
	_ = repo.SetEnabled(ctx, pBeta.ID, false)

	// ListEnabled should only return alpha and gamma.
	enabled, err := repo.ListEnabled(ctx)
	if err != nil {
		t.Fatalf("ListEnabled: %v", err)
	}
	if len(enabled) != 2 {
		t.Fatalf("ListEnabled: expected 2, got %d", len(enabled))
	}
	if enabled[0].Name != "alpha" || enabled[1].Name != "gamma" {
		t.Errorf("ListEnabled: expected [alpha, gamma], got [%s, %s]", enabled[0].Name, enabled[1].Name)
	}
}

func TestProjectRepoListAll(t *testing.T) {
	db := testDB(t)
	repo := NewProjectRepo(db)
	ctx := context.Background()

	// Create 3 projects.
	_, _ = repo.Create(ctx, "alpha", "Alpha", "github", "https://github.com/a/a")
	pBeta, _ := repo.Create(ctx, "beta", "Beta", "github", "https://github.com/b/b")
	_, _ = repo.Create(ctx, "gamma", "Gamma", "github", "https://github.com/c/c")

	// Disable beta.
	_ = repo.SetEnabled(ctx, pBeta.ID, false)

	// ListAll should return all 3.
	all, err := repo.ListAll(ctx)
	if err != nil {
		t.Fatalf("ListAll: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("ListAll: expected 3, got %d", len(all))
	}

	// List (alias) should also return all 3.
	list, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 3 {
		t.Fatalf("List: expected 3, got %d", len(list))
	}
}

func TestProjectRepoMaxLastCrawledAt(t *testing.T) {
	db := testDB(t)
	repo := NewProjectRepo(db)
	ctx := context.Background()

	// No projects crawled yet → nil.
	if ts, err := repo.MaxLastCrawledAt(ctx); err != nil || ts != nil {
		t.Fatalf("empty DB: got (%v, %v), want (nil, nil)", ts, err)
	}

	p1, err := repo.Create(ctx, "proj1", "Proj1", "github", "https://example.com/1")
	if err != nil {
		t.Fatalf("create p1: %v", err)
	}
	p2, err := repo.Create(ctx, "proj2", "Proj2", "github", "https://example.com/2")
	if err != nil {
		t.Fatalf("create p2: %v", err)
	}

	if err := repo.UpdateLastCrawledAt(ctx, p1.ID); err != nil {
		t.Fatalf("update p1 last_crawled_at: %v", err)
	}
	ts, err := repo.MaxLastCrawledAt(ctx)
	if err != nil {
		t.Fatalf("MaxLastCrawledAt: %v", err)
	}
	if ts == nil {
		t.Fatal("expected non-nil timestamp after crawling p1")
	}
	first := *ts

	// Crawling the second project later must move the max forward.
	if err := repo.UpdateLastCrawledAt(ctx, p2.ID); err != nil {
		t.Fatalf("update p2 last_crawled_at: %v", err)
	}
	ts, err = repo.MaxLastCrawledAt(ctx)
	if err != nil {
		t.Fatalf("MaxLastCrawledAt: %v", err)
	}
	if ts == nil || ts.Before(first) {
		t.Errorf("max timestamp did not advance: first=%v now=%v", first, ts)
	}
}
