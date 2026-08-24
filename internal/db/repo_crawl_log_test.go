package db

import (
	"context"
	"testing"
	"time"
)

func TestCrawlLogLatestPerProject(t *testing.T) {
	db := testDB(t)
	repo := NewCrawlLogRepo(db)
	ctx := context.Background()

	p1, err := NewProjectRepo(db).Create(ctx, "proj1", "Proj1", "github", "https://example.com/1")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	// Two logs for p1: an old failure, then a newer success.
	id1, err := repo.Create(ctx, &CrawlLog{ProjectID: p1.ID, StartedAt: time.Now().Add(-time.Hour), Status: "running"})
	if err != nil {
		t.Fatalf("create log 1: %v", err)
	}
	if err := repo.UpdateFinished(ctx, id1, "error", 0, 0, "HashiCorp API token required"); err != nil {
		t.Fatalf("finish log 1: %v", err)
	}
	id2, err := repo.Create(ctx, &CrawlLog{ProjectID: p1.ID, StartedAt: time.Now(), Status: "running"})
	if err != nil {
		t.Fatalf("create log 2: %v", err)
	}
	if err := repo.UpdateFinished(ctx, id2, "success", 3, 2, ""); err != nil {
		t.Fatalf("finish log 2: %v", err)
	}

	logs, err := repo.LatestPerProject(ctx)
	if err != nil {
		t.Fatalf("LatestPerProject: %v", err)
	}
	latest, ok := logs[p1.ID]
	if !ok {
		t.Fatal("expected a latest log for proj1")
	}
	if latest.ID != id2 {
		t.Errorf("latest.ID = %d, want %d (the newest log)", latest.ID, id2)
	}
	if latest.Status != "success" || latest.ErrorMessage != "" {
		t.Errorf("latest = %+v, want status=success with no error", latest)
	}
}
