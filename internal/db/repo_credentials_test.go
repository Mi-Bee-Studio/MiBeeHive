package db

import (
	"context"
	"testing"
)

func TestSourceCredentialRepoGetBySourceType(t *testing.T) {
	db := testDB(t)
	repo := NewSourceCredentialRepo(db)
	ctx := context.Background()

	// Non-existent returns nil.
	got, err := repo.GetBySourceType(ctx, "github")
	if err != nil {
		t.Fatalf("GetBySourceType(nonexistent): %v", err)
	}
	if got != nil {
		t.Errorf("expected nil, got %v", got)
	}

	// Insert and retrieve.
	if err := repo.Upsert(ctx, "github", "ghp_test123"); err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	got, err = repo.GetBySourceType(ctx, "github")
	if err != nil {
		t.Fatalf("GetBySourceType(github): %v", err)
	}
	if got == nil || got.Token != "ghp_test123" {
		t.Errorf("expected token=ghp_test123, got %v", got)
	}
	if got.SourceType != "github" {
		t.Errorf("expected source_type=github, got %q", got.SourceType)
	}
	if got.ID == 0 {
		t.Error("expected non-zero ID")
	}
	if got.CreatedAt == "" {
		t.Error("expected non-empty created_at")
	}
	if got.UpdatedAt == "" {
		t.Error("expected non-empty updated_at")
	}
}

func TestSourceCredentialRepoUpsert(t *testing.T) {
	db := testDB(t)
	repo := NewSourceCredentialRepo(db)
	ctx := context.Background()

	// Insert new.
	if err := repo.Upsert(ctx, "hashicorp", "hc_initial"); err != nil {
		t.Fatalf("Upsert (insert): %v", err)
	}
	got, err := repo.GetBySourceType(ctx, "hashicorp")
	if err != nil {
		t.Fatalf("GetBySourceType: %v", err)
	}
	if got == nil || got.Token != "hc_initial" {
		t.Errorf("expected token=hc_initial, got %v", got)
	}

	// Update existing (same source_type).
	if err := repo.Upsert(ctx, "hashicorp", "hc_updated"); err != nil {
		t.Fatalf("Upsert (update): %v", err)
	}
	got, err = repo.GetBySourceType(ctx, "hashicorp")
	if err != nil {
		t.Fatalf("GetBySourceType after update: %v", err)
	}
	if got == nil || got.Token != "hc_updated" {
		t.Errorf("expected token=hc_updated, got %v", got)
	}

	// Verify ID didn't change (update, not insert).
	originalID := got.ID
	if err := repo.Upsert(ctx, "hashicorp", "hc_third"); err != nil {
		t.Fatalf("Upsert (third): %v", err)
	}
	got, err = repo.GetBySourceType(ctx, "hashicorp")
	if err != nil {
		t.Fatalf("GetBySourceType after third: %v", err)
	}
	if got.ID != originalID {
		t.Errorf("expected same ID %d, got %d", originalID, got.ID)
	}

	// Verify total count is still 1.
	creds, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(creds) != 1 {
		t.Errorf("expected 1 credential, got %d", len(creds))
	}
}

func TestSourceCredentialRepoList(t *testing.T) {
	db := testDB(t)
	repo := NewSourceCredentialRepo(db)
	ctx := context.Background()

	// Empty list.
	creds, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("List (empty): %v", err)
	}
	if len(creds) != 0 {
		t.Errorf("expected 0 credentials, got %d", len(creds))
	}

	// Insert multiple.
	for _, pair := range []struct {
		src, tok string
	}{
		{"github", "ghp_abc"},
		{"hashicorp", "hcp_xyz"},
		{"grafana", "gf_123"},
	} {
		if err := repo.Upsert(ctx, pair.src, pair.tok); err != nil {
			t.Fatalf("Upsert %s: %v", pair.src, err)
		}
	}

	creds, err = repo.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(creds) != 3 {
		t.Fatalf("expected 3 credentials, got %d", len(creds))
	}

	// Should be ordered by source_type alphabetically.
	expected := []string{"github", "grafana", "hashicorp"}
	for i, e := range expected {
		if creds[i].SourceType != e {
			t.Errorf("position %d: expected %q, got %q", i, e, creds[i].SourceType)
		}
	}
}
