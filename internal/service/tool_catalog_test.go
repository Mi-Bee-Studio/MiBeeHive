package service

import (
	"context"
	"errors"
	"testing"

	"github.com/Mi-Bee-Studio/mibeehive/internal/db"
)

func testToolCatalogDB(t *testing.T) *db.ProjectRepo {
	t.Helper()
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("Open(:memory:): %v", err)
	}
	t.Cleanup(func() { database.Close() })
	if err := db.Migrate(database); err != nil {
		t.Fatalf("Migrate(): %v", err)
	}
	return db.NewProjectRepo(database)
}

func TestListCatalog(t *testing.T) {
	svc := NewToolCatalogService()
	catalog := svc.ListCatalog()

	if len(catalog) == 0 {
		t.Fatal("expected non-empty catalog")
	}
	if len(catalog) > 30 {
		t.Fatalf("expected ≤30 seed items, got %d", len(catalog))
	}

	// Every entry must have a unique slug and a valid source type.
	seen := make(map[string]bool, len(catalog))
	for _, e := range catalog {
		if e.Slug == "" {
			t.Error("found entry with empty slug")
		}
		if seen[e.Slug] {
			t.Errorf("duplicate slug %q", e.Slug)
		}
		seen[e.Slug] = true
		if e.SourceType == "" {
			t.Errorf("entry %q has empty source_type", e.Slug)
		}
		if e.SourceURL == "" {
			t.Errorf("entry %q has empty source_url", e.Slug)
		}
	}

	// Spot-check a few expected slugs.
	for _, want := range []string{"prometheus", "grafana", "nginx", "terraform", "docker"} {
		if !seen[want] {
			t.Errorf("expected slug %q in catalog", want)
		}
	}
}

func TestEnableTool(t *testing.T) {
	repo := testToolCatalogDB(t)
	svc := NewToolCatalogService()
	ctx := context.Background()

	project, err := svc.EnableTool(ctx, repo, "prometheus")
	if err != nil {
		t.Fatalf("EnableTool(prometheus): %v", err)
	}
	if project == nil {
		t.Fatal("expected non-nil project")
	}
	if project.Name != "prometheus" {
		t.Errorf("expected name=prometheus, got %q", project.Name)
	}
	if project.SourceType != "github" {
		t.Errorf("expected source_type=github, got %q", project.SourceType)
	}
	if project.SourceURL != "https://github.com/prometheus/prometheus" {
		t.Errorf("unexpected source_url %q", project.SourceURL)
	}
	if project.StorageSubdir != "oss/prometheus" {
		t.Errorf("expected storage_subdir=oss/prometheus, got %q", project.StorageSubdir)
	}

	// Verify it was persisted.
	got, err := repo.GetByName(ctx, "prometheus")
	if err != nil {
		t.Fatalf("GetByName: %v", err)
	}
	if got == nil {
		t.Fatal("expected project to exist in DB")
	}
}

func TestEnableToolIdempotent(t *testing.T) {
	repo := testToolCatalogDB(t)
	svc := NewToolCatalogService()
	ctx := context.Background()

	first, err := svc.EnableTool(ctx, repo, "grafana")
	if err != nil {
		t.Fatalf("EnableTool first: %v", err)
	}
	second, err := svc.EnableTool(ctx, repo, "grafana")
	if err != nil {
		t.Fatalf("EnableTool second: %v", err)
	}

	if first.ID != second.ID {
		t.Errorf("expected same project ID on second enable, got %d vs %d", first.ID, second.ID)
	}

	// Only one project row should exist.
	projects, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(projects) != 1 {
		t.Errorf("expected 1 project, got %d", len(projects))
	}
}

func TestEnableToolUnknownSlug(t *testing.T) {
	repo := testToolCatalogDB(t)
	svc := NewToolCatalogService()
	ctx := context.Background()

	_, err := svc.EnableTool(ctx, repo, "does-not-exist")
	if err == nil {
		t.Fatal("expected error for unknown slug")
	}
	if !errors.Is(err, ErrToolNotFound) {
		t.Errorf("expected ErrToolNotFound, got %v", err)
	}
}