package handler

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Mi-Bee-Studio/mibeehive/internal/db"
	"github.com/Mi-Bee-Studio/mibeehive/internal/model"
	"github.com/Mi-Bee-Studio/mibeehive/internal/service"
)

func TestTaskHandler_HandleTaskList(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	// The seeded project (prometheus, enabled=1) should appear as a crawl task.
	// The seeded pending file should appear as a download task.
	taskSvc := service.NewTaskService(database)
	h := NewTaskHandler(taskSvc)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/admin/tasks", h.HandleTaskList)
	handler := wrapWithAuth(mux)

	req := authedRequest(http.MethodGet, "/api/v1/admin/tasks", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp model.ApiResponse[[]model.Task]
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !resp.Success {
		t.Fatalf("expected success=true, got message: %s", resp.Message)
	}

	// Should have at least 1 crawl task (prometheus project) and 1 download task (pending file).
	hasCrawl := false
	hasDownload := false
	for _, task := range resp.Data {
		if task.Type == "crawl" {
			hasCrawl = true
		}
		if task.Type == "download" {
			hasDownload = true
		}
	}
	if !hasCrawl {
		t.Fatal("expected at least one crawl task from seeded project")
	}
	if !hasDownload {
		t.Fatal("expected at least one download task from seeded pending file")
	}
}

func TestTaskHandler_HandleTaskList_Empty(t *testing.T) {
	database := setupEmptyTestDB(t)
	defer database.Close()

	taskSvc := service.NewTaskService(database)
	h := NewTaskHandler(taskSvc)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/admin/tasks", h.HandleTaskList)
	handler := wrapWithAuth(mux)

	req := authedRequest(http.MethodGet, "/api/v1/admin/tasks", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp model.ApiResponse[[]model.Task]
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !resp.Success {
		t.Fatalf("expected success=true, got message: %s", resp.Message)
	}
	if resp.Data == nil {
		t.Fatal("expected non-nil data (empty slice)")
	}
	if len(resp.Data) != 0 {
		t.Fatalf("expected 0 tasks from empty DB, got %d", len(resp.Data))
	}
}

// setupEmptyTestDB creates a migrated in-memory DB without seed data.
func setupEmptyTestDB(t *testing.T) *sql.DB {
	t.Helper()
	tmpDir := t.TempDir()
	dbPath := tmpDir + "/empty.db"
	database, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open empty db: %v", err)
	}
	if err := db.Migrate(database); err != nil {
		t.Fatalf("failed to migrate empty db: %v", err)
	}
	database.SetMaxOpenConns(1)
	return database
}
