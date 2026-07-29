package db

import (
	"context"
	"testing"
)

func TestBackfillEmptyVersions_FillsFromFilenames(t *testing.T) {
	db := testDB(t)
	pRepo := NewProjectRepo(db)
	fRepo := NewFileRepo(db)
	ctx := context.Background()

	p, err := pRepo.Create(ctx, "consul", "Consul", "hashicorp", "https://releases.hashicorp.com/consul/")
	if err != nil {
		t.Fatalf("Create project: %v", err)
	}

	filenames := []string{
		"consul_1.22.2_linux_amd64.zip",
		"prometheus-3.11.3.linux-arm64.tar.gz",
		"grafana-11.1.0.darwin-amd64.tar.gz",
	}
	for _, fn := range filenames {
		_, err := fRepo.Create(ctx, &File{
			ProjectID: p.ID,
			Version:   "",
			Filename:  fn,
			OS:        "linux",
			Arch:      "amd64",
			Status:    "complete",
		})
		if err != nil {
			t.Fatalf("Create file %q: %v", fn, err)
		}
	}

	count, err := fRepo.BackfillEmptyVersions(ctx)
	if err != nil {
		t.Fatalf("BackfillEmptyVersions: %v", err)
	}
	if count != 3 {
		t.Errorf("expected count=3, got %d", count)
	}

	files, err := fRepo.ListByProject(ctx, p.ID)
	if err != nil {
		t.Fatalf("ListByProject: %v", err)
	}
	expectedVersions := map[string]string{
		"consul_1.22.2_linux_amd64.zip":        "1.22.2",
		"prometheus-3.11.3.linux-arm64.tar.gz": "3.11.3",
		"grafana-11.1.0.darwin-amd64.tar.gz":   "11.1.0",
	}
	for _, f := range files {
		want, ok := expectedVersions[f.Filename]
		if !ok {
			continue
		}
		if f.Version != want {
			t.Errorf("file %q: expected version=%q, got %q", f.Filename, want, f.Version)
		}
	}
}

func TestBackfillEmptyVersions_Idempotent(t *testing.T) {
	db := testDB(t)
	pRepo := NewProjectRepo(db)
	fRepo := NewFileRepo(db)
	ctx := context.Background()

	p, err := pRepo.Create(ctx, "consul", "Consul", "hashicorp", "https://releases.hashicorp.com/consul/")
	if err != nil {
		t.Fatalf("Create project: %v", err)
	}

	_, err = fRepo.Create(ctx, &File{
		ProjectID: p.ID,
		Version:   "",
		Filename:  "consul_1.22.2_linux_amd64.zip",
		OS:        "linux",
		Arch:      "amd64",
		Status:    "complete",
	})
	if err != nil {
		t.Fatalf("Create file: %v", err)
	}

	count1, err := fRepo.BackfillEmptyVersions(ctx)
	if err != nil {
		t.Fatalf("First backfill: %v", err)
	}
	if count1 != 1 {
		t.Errorf("first call: expected count=1, got %d", count1)
	}

	count2, err := fRepo.BackfillEmptyVersions(ctx)
	if err != nil {
		t.Fatalf("Second backfill: %v", err)
	}
	if count2 != 0 {
		t.Errorf("second call: expected count=0, got %d", count2)
	}
}

func TestBackfillEmptyVersions_SkipsNoVersion(t *testing.T) {
	db := testDB(t)
	pRepo := NewProjectRepo(db)
	fRepo := NewFileRepo(db)
	ctx := context.Background()

	p, err := pRepo.Create(ctx, "test", "Test", "github", "https://example.com")
	if err != nil {
		t.Fatalf("Create project: %v", err)
	}

	_, err = fRepo.Create(ctx, &File{
		ProjectID: p.ID,
		Version:   "",
		Filename:  "config.yaml",
		OS:        "linux",
		Arch:      "amd64",
		Status:    "complete",
	})
	if err != nil {
		t.Fatalf("Create file: %v", err)
	}

	count, err := fRepo.BackfillEmptyVersions(ctx)
	if err != nil {
		t.Fatalf("BackfillEmptyVersions: %v", err)
	}
	if count != 0 {
		t.Errorf("expected count=0, got %d", count)
	}

	files, err := fRepo.ListByProject(ctx, p.ID)
	if err != nil {
		t.Fatalf("ListByProject: %v", err)
	}
	if len(files) != 1 || files[0].Version != "" {
		t.Errorf("expected empty version, got %q", files[0].Version)
	}
}

func TestFileRepo_UpdateLocalPath(t *testing.T) {
	db := testDB(t)
	pRepo := NewProjectRepo(db)
	fRepo := NewFileRepo(db)
	ctx := context.Background()

	p, err := pRepo.Create(ctx, "test", "Test", "github", "https://example.com")
	if err != nil {
		t.Fatalf("Create project: %v", err)
	}

	initialPath := "/old/path/file.tar.gz"
	id, err := fRepo.Create(ctx, &File{
		ProjectID: p.ID,
		Filename:  "file.tar.gz",
		Status:    "complete",
		LocalPath: initialPath,
	})
	if err != nil {
		t.Fatalf("Create file: %v", err)
	}

	newPath := "/new/path/file.tar.gz"
	err = fRepo.UpdateLocalPath(ctx, id, newPath)
	if err != nil {
		t.Fatalf("UpdateLocalPath: %v", err)
	}

	got, err := fRepo.GetByID(ctx, id)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.LocalPath != newPath {
		t.Errorf("UpdateLocalPath: expected local_path=%q, got %q", newPath, got.LocalPath)
	}
}

func TestFileRepo_ListComplete(t *testing.T) {
	db := testDB(t)
	pRepo := NewProjectRepo(db)
	fRepo := NewFileRepo(db)
	ctx := context.Background()

	p, err := pRepo.Create(ctx, "test", "Test", "github", "https://example.com")
	if err != nil {
		t.Fatalf("Create project: %v", err)
	}

	// Three files: two complete (servable), one pending (not yet servable).
	mk := func(name, status string) {
		t.Helper()
		if _, err := fRepo.Create(ctx, &File{ProjectID: p.ID, Filename: name, Status: status, LocalPath: "/p/" + name}); err != nil {
			t.Fatalf("Create %s: %v", name, err)
		}
	}
	mk("complete-a.tar.gz", "complete")
	mk("complete-b.tar.gz", "complete")
	mk("pending-c.tar.gz", "pending")

	// ListComplete across all projects (projectID 0).
	got, err := fRepo.ListComplete(ctx, 0)
	if err != nil {
		t.Fatalf("ListComplete: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 complete files, got %d", len(got))
	}
	for _, f := range got {
		if f.Status != "complete" {
			t.Errorf("ListComplete returned non-complete file %q (status=%q)", f.Filename, f.Status)
		}
	}

	// Scoped to the project: still 2.
	got, err = fRepo.ListComplete(ctx, p.ID)
	if err != nil {
		t.Fatalf("ListComplete scoped: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("scoped: want 2, got %d", len(got))
	}
}
