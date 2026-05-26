package db

import (
	"context"
	"encoding/json"
	"testing"
)

func TestOsInstallConfigRepo(t *testing.T) {
	db := testDB(t)
	repo := NewOsInstallConfigRepo(db)
	ctx := context.Background()

	// Test Create
	configJSON, _ := json.Marshal(map[string]string{"key": "value"})
	created, err := repo.Create(ctx, "test-config", "Test Config", "debian", string(configJSON))
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if created.ID == 0 {
		t.Fatal("Create returned ID 0")
	}
	if created.Name != "test-config" {
		t.Errorf("Name = %q, want %q", created.Name, "test-config")
	}
	if !created.Enabled {
		t.Error("Enabled should be true after create")
	}

	// Test GetByID
	got, err := repo.GetByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}
	if got.Name != "test-config" {
		t.Errorf("GetByID Name = %q, want %q", got.Name, "test-config")
	}

	// Test GetByID not found
	got, err = repo.GetByID(ctx, 99999)
	if err != nil {
		t.Fatalf("GetByID not-found should not return error, got: %v", err)
	}
	if got != nil {
		t.Error("GetByID not-found should return nil")
	}

	// Test List
	all, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(all) < 1 {
		t.Fatal("List should return at least 1 config")
	}

	// Test ListEnabled
	enabled, err := repo.ListEnabled(ctx)
	if err != nil {
		t.Fatalf("ListEnabled failed: %v", err)
	}
	if len(enabled) < 1 {
		t.Fatal("ListEnabled should return at least 1 config")
	}

	// Test Update
	err = repo.Update(ctx, created.ID, "updated-name", "Updated Config", "ubuntu", string(configJSON))
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	updated, err := repo.GetByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetByID after update failed: %v", err)
	}
	if updated.Name != "updated-name" {
		t.Errorf("Name after update = %q, want %q", updated.Name, "updated-name")
	}
	if updated.OsType != "ubuntu" {
		t.Errorf("OsType after update = %q, want %q", updated.OsType, "ubuntu")
	}

	// Test Delete (soft delete)
	err = repo.Delete(ctx, created.ID)
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	// After soft delete, GetByID should still find it but ListEnabled should not
	enabledAfterDelete, err := repo.ListEnabled(ctx)
	if err != nil {
		t.Fatalf("ListEnabled after delete failed: %v", err)
	}
	for _, c := range enabledAfterDelete {
		if c.ID == created.ID {
			t.Error("soft-deleted config should not appear in ListEnabled")
		}
	}
}
