package db

import (
	"context"
	"testing"
	"time"
)

func TestMigrationTaskRepo_Create(t *testing.T) {
	db := testDB(t)
	repo := NewMigrationTaskRepo(db)
	ctx := context.Background()

	now := time.Now().UTC().Truncate(time.Second)

	task := &MigrationTask{
		Module:  "oss",
		OldPath: "/old/path/oss",
		NewPath: "/new/path/oss",
	}
	id, err := repo.Create(ctx, task)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if id == 0 {
		t.Fatal("expected non-zero ID")
	}

	got, err := repo.GetByID(ctx, id)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got == nil {
		t.Fatal("GetByID: expected task, got nil")
	}
	if got.Module != "oss" {
		t.Errorf("expected module=oss, got %q", got.Module)
	}
	if got.OldPath != "/old/path/oss" {
		t.Errorf("expected old_path=/old/path/oss, got %q", got.OldPath)
	}
	if got.NewPath != "/new/path/oss" {
		t.Errorf("expected new_path=/new/path/oss, got %q", got.NewPath)
	}
	if got.Status != "pending" {
		t.Errorf("expected status=pending, got %q", got.Status)
	}
	if got.CreatedAt.Before(now) {
		t.Error("expected created_at to be set")
	}
	if got.UpdatedAt.Before(now) {
		t.Error("expected updated_at to be set")
	}
}

func TestMigrationTaskRepo_UpdateStatus(t *testing.T) {
	db := testDB(t)
	repo := NewMigrationTaskRepo(db)
	ctx := context.Background()

	task := &MigrationTask{
		Module:  "os_install",
		OldPath: "/old/path/os-install",
		NewPath: "/new/path/os-install",
	}
	id, err := repo.Create(ctx, task)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	err = repo.UpdateStatus(ctx, id, "running")
	if err != nil {
		t.Fatalf("UpdateStatus: %v", err)
	}

	got, err := repo.GetByID(ctx, id)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got == nil {
		t.Fatal("GetByID: expected task, got nil")
	}
	if got.Status != "running" {
		t.Errorf("expected status=running, got %q", got.Status)
	}
}

func TestMigrationTaskRepo_UpdateProgress(t *testing.T) {
	db := testDB(t)
	repo := NewMigrationTaskRepo(db)
	ctx := context.Background()

	task := &MigrationTask{
		Module:  "iso",
		OldPath: "/old/path/iso",
		NewPath: "/new/path/iso",
	}
	id, err := repo.Create(ctx, task)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	err = repo.UpdateProgress(ctx, id, 5, 1024, 50)
	if err != nil {
		t.Fatalf("UpdateProgress: %v", err)
	}

	got, err := repo.GetByID(ctx, id)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got == nil {
		t.Fatal("GetByID: expected task, got nil")
	}
	if got.MigratedFiles != 5 {
		t.Errorf("expected migrated_files=5, got %d", got.MigratedFiles)
	}
	if got.MigratedBytes != 1024 {
		t.Errorf("expected migrated_bytes=1024, got %d", got.MigratedBytes)
	}
	if got.Progress != 50 {
		t.Errorf("expected progress=50, got %d", got.Progress)
	}
}

func TestMigrationTaskRepo_ListActive(t *testing.T) {
	db := testDB(t)
	repo := NewMigrationTaskRepo(db)
	ctx := context.Background()

	// Create tasks with different statuses.
	t1 := &MigrationTask{Module: "oss", OldPath: "/old/a", NewPath: "/new/a"}
	t1ID, err := repo.Create(ctx, t1)
	if err != nil {
		t.Fatalf("Create t1: %v", err)
	}

	t2 := &MigrationTask{Module: "os_install", OldPath: "/old/b", NewPath: "/new/b"}
	t2ID, err := repo.Create(ctx, t2)
	if err != nil {
		t.Fatalf("Create t2: %v", err)
	}

	t3 := &MigrationTask{Module: "iso", OldPath: "/old/c", NewPath: "/new/c"}
	t3ID, err := repo.Create(ctx, t3)
	if err != nil {
		t.Fatalf("Create t3: %v", err)
	}

	// Set t2 to running, t3 to completed.
	if err := repo.UpdateStatus(ctx, t2ID, "running"); err != nil {
		t.Fatalf("UpdateStatus t2: %v", err)
	}
	if err := repo.UpdateStatus(ctx, t3ID, "completed"); err != nil {
		t.Fatalf("UpdateStatus t3: %v", err)
	}

	active, err := repo.ListActive(ctx)
	if err != nil {
		t.Fatalf("ListActive: %v", err)
	}

	if len(active) != 2 {
		t.Fatalf("ListActive: expected 2 (pending + running), got %d", len(active))
	}
	// Should include pending (t1) and running (t2) but not completed (t3).
	ids := map[int64]bool{t1ID: true, t2ID: true}
	for _, a := range active {
		if !ids[a.ID] {
			t.Errorf("unexpected active task ID: %d (expected %d or %d)", a.ID, t1ID, t2ID)
		}
		delete(ids, a.ID)
	}
	if len(ids) > 0 {
		t.Errorf("missing active tasks: %v", ids)
	}
}

func TestMigrationTaskRepo_UpdateError(t *testing.T) {
	db := testDB(t)
	repo := NewMigrationTaskRepo(db)
	ctx := context.Background()

	task := &MigrationTask{
		Module:  "oss",
		OldPath: "/old/path/oss",
		NewPath: "/new/path/oss",
	}
	id, err := repo.Create(ctx, task)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	err = repo.UpdateError(ctx, id, "disk full")
	if err != nil {
		t.Fatalf("UpdateError: %v", err)
	}

	got, err := repo.GetByID(ctx, id)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got == nil {
		t.Fatal("GetByID: expected task, got nil")
	}
	if got.Status != "failed" {
		t.Errorf("expected status=failed, got %q", got.Status)
	}
	if got.ErrorMessage != "disk full" {
		t.Errorf("expected error_message='disk full', got %q", got.ErrorMessage)
	}
}

func TestMigrationTaskRepo_ListByStatus(t *testing.T) {
	db := testDB(t)
	repo := NewMigrationTaskRepo(db)
	ctx := context.Background()

	t1 := &MigrationTask{Module: "oss", OldPath: "/old/a", NewPath: "/new/a"}
	t1ID, err := repo.Create(ctx, t1)
	if err != nil {
		t.Fatalf("Create t1: %v", err)
	}

	t2 := &MigrationTask{Module: "os_install", OldPath: "/old/b", NewPath: "/new/b"}
	t2ID, err := repo.Create(ctx, t2)
	if err != nil {
		t.Fatalf("Create t2: %v", err)
	}

	_ = t2ID // used at status change

	t3 := &MigrationTask{Module: "iso", OldPath: "/old/c", NewPath: "/new/c"}
	_, err = repo.Create(ctx, t3)
	if err != nil {
		t.Fatalf("Create t3: %v", err)
	}

	// Mark t2 as completed.
	if err := repo.UpdateStatus(ctx, t2ID, "completed"); err != nil {
		t.Fatalf("UpdateStatus t2: %v", err)
	}

	// List pending tasks.
	pending, err := repo.ListByStatus(ctx, "pending")
	if err != nil {
		t.Fatalf("ListByStatus(pending): %v", err)
	}
	// t1 and t3 are pending; t2 is completed.
	if len(pending) != 2 {
		t.Fatalf("ListByStatus(pending): expected 2, got %d", len(pending))
	}

	// List completed tasks.
	completed, err := repo.ListByStatus(ctx, "completed")
	if err != nil {
		t.Fatalf("ListByStatus(completed): %v", err)
	}
	if len(completed) != 1 {
		t.Fatalf("ListByStatus(completed): expected 1, got %d", len(completed))
	}
	if completed[0].ID != t1ID {
		// t2 should be completed, but we need t2's actual ID.
		if completed[0].ID != t2ID {
			t.Errorf("expected completed task ID %d, got %d", t2ID, completed[0].ID)
		}
	}
}

func TestMigrationTaskRepo_GetByID_NotFound(t *testing.T) {
	db := testDB(t)
	repo := NewMigrationTaskRepo(db)
	ctx := context.Background()

	got, err := repo.GetByID(ctx, 9999)
	if err != nil {
		t.Fatalf("GetByID(9999): %v", err)
	}
	if got != nil {
		t.Errorf("GetByID(9999): expected nil, got %v", got)
	}
}

func TestMigrationTaskRepo_EmptyList(t *testing.T) {
	db := testDB(t)
	repo := NewMigrationTaskRepo(db)
	ctx := context.Background()

	active, err := repo.ListActive(ctx)
	if err != nil {
		t.Fatalf("ListActive: %v", err)
	}
	if len(active) != 0 {
		t.Errorf("ListActive: expected 0, got %d", len(active))
	}

	pending, err := repo.ListByStatus(ctx, "pending")
	if err != nil {
		t.Fatalf("ListByStatus(pending): %v", err)
	}
	if len(pending) != 0 {
		t.Errorf("ListByStatus(pending): expected 0, got %d", len(pending))
	}
}
