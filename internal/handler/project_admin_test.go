package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"

	"github.com/Mi-Bee-Studio/mibeehive/internal/config"
	"github.com/Mi-Bee-Studio/mibeehive/internal/crawler"
	"github.com/Mi-Bee-Studio/mibeehive/internal/db"
	"github.com/Mi-Bee-Studio/mibeehive/internal/model"
	"github.com/Mi-Bee-Studio/mibeehive/internal/service"
)

func setupProjectAdminHandler(t *testing.T, database *sql.DB) (*ProjectAdminHandler, *crawler.CrawlManager, *config.Config) {
	t.Helper()
	projectRepo := db.NewProjectRepo(database)
	fileRepo := db.NewFileRepo(database)
	tmpDir := t.TempDir()
	cfg := &config.Config{
		Auth: config.AuthConfig{
			PasswordHash: "$2a$10$abcdefghijklmnopqrstuvwxABCDEFGH",
			JWTSecret:    "test-jwt-secret-key-12345",
		},
		Storage: config.StorageConfig{BasePath: tmpDir},
		Crawler: config.CrawlerConfig{MaxConcurrent: 2, DefaultInterval: "6h"},
	}
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	fileService := service.NewFileService(database, service.NewStorageResolver(&config.Config{Storage: config.StorageConfig{BasePath: tmpDir}}), 2, nil)
	cm := crawler.NewCrawlManager(database, fileService, cfg, logger, nil)

	return NewProjectAdminHandler(projectRepo, fileRepo, cm, cfg), cm, cfg
}

func registerProjectAdminRoutes(mux *http.ServeMux, h *ProjectAdminHandler) {
	mux.HandleFunc("GET "+model.RouteAdminProjectsList, h.ListProjects)
	mux.HandleFunc("GET "+model.RouteAdminProjectsGet, h.GetProject)
	mux.HandleFunc("POST "+model.RouteAdminProjectsCreate, h.CreateProject)
	mux.HandleFunc("PUT "+model.RouteAdminProjectsUpdate, h.UpdateProject)
	mux.HandleFunc("DELETE "+model.RouteAdminProjectsDelete, h.DeleteProject)
	mux.HandleFunc("PATCH "+model.RouteAdminProjectsToggle, h.ToggleProject)
}

func TestAdminListProjects(t *testing.T) {
	database := setupAdminTestDB(t)
	defer database.Close()

	// Seed a project.
	projectRepo := db.NewProjectRepo(database)
	_, err := projectRepo.CreateWithSettings(context.Background(),
		"testproj", "Test Project", "github", "https://github.com/test/test",
		model.ProjectSettings{GitHubOwner: "test", GitHubRepo: "test"})
	if err != nil {
		t.Fatalf("failed to seed project: %v", err)
	}

	h, _, _ := setupProjectAdminHandler(t, database)
	mux := http.NewServeMux()
	registerProjectAdminRoutes(mux, h)
	handler := wrapWithAuth(mux)

	req := authedRequest(http.MethodGet, "/api/v1/admin/projects", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp model.ApiResponse[[]model.AdminProjectResponse]
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !resp.Success {
		t.Fatal("expected success=true")
	}
	if len(resp.Data) < 1 {
		t.Fatalf("expected at least 1 project, got %d", len(resp.Data))
	}
	if resp.Data[0].Name != "testproj" {
		t.Fatalf("expected project name 'testproj', got %q", resp.Data[0].Name)
	}
}

func TestAdminCreateProject(t *testing.T) {
	database := setupAdminTestDB(t)
	defer database.Close()

	h, _, _ := setupProjectAdminHandler(t, database)
	mux := http.NewServeMux()
	registerProjectAdminRoutes(mux, h)
	handler := wrapWithAuth(mux)

	body, _ := json.Marshal(model.CreateProjectRequest{
		Name:        "newproj",
		DisplayName: "New Project",
		SourceType:  model.SourceTypeGo,
		SourceURL:   "https://go.dev/dl/",
		Settings:    model.ProjectSettings{},
	})
	req := authedRequest(http.MethodPost, "/api/v1/admin/projects", body)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp model.ApiResponse[model.AdminProjectResponse]
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !resp.Success {
		t.Fatal("expected success=true")
	}
	if resp.Data.Name != "newproj" {
		t.Fatalf("expected name 'newproj', got %q", resp.Data.Name)
	}
	if resp.Data.SourceType != "go" {
		t.Fatalf("expected source_type 'go', got %q", resp.Data.SourceType)
	}
}

func TestAdminCreateProject_MissingFields(t *testing.T) {
	database := setupAdminTestDB(t)
	defer database.Close()

	h, _, _ := setupProjectAdminHandler(t, database)
	mux := http.NewServeMux()
	registerProjectAdminRoutes(mux, h)
	handler := wrapWithAuth(mux)

	body, _ := json.Marshal(model.CreateProjectRequest{
		DisplayName: "No Name",
	})
	req := authedRequest(http.MethodPost, "/api/v1/admin/projects", body)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminDeleteProject(t *testing.T) {
	database := setupAdminTestDB(t)
	defer database.Close()

	projectRepo := db.NewProjectRepo(database)
	proj, err := projectRepo.CreateWithSettings(context.Background(),
		"delproj", "Delete Me", "github", "https://github.com/test/del",
		model.ProjectSettings{})
	if err != nil {
		t.Fatalf("failed to create project: %v", err)
	}

	h, _, _ := setupProjectAdminHandler(t, database)
	mux := http.NewServeMux()
	registerProjectAdminRoutes(mux, h)
	handler := wrapWithAuth(mux)

	req := authedRequest(http.MethodDelete, "/api/v1/admin/projects/"+strconv.FormatInt(proj.ID, 10), nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// Verify project is hard-deleted (GetByID returns nil).
	updated, err := projectRepo.GetByID(context.Background(), proj.ID)
	if err != nil {
		t.Fatalf("failed to get project: %v", err)
	}
	if updated != nil {
		t.Fatal("expected project to be nil after hard delete")
	}
}

func TestAdminToggleProject(t *testing.T) {
	database := setupAdminTestDB(t)
	defer database.Close()

	projectRepo := db.NewProjectRepo(database)
	proj, err := projectRepo.CreateWithSettings(context.Background(),
		"toggleproj", "Toggle Me", "github", "https://github.com/test/toggle",
		model.ProjectSettings{})
	if err != nil {
		t.Fatalf("failed to create project: %v", err)
	}

	h, _, _ := setupProjectAdminHandler(t, database)
	mux := http.NewServeMux()
	registerProjectAdminRoutes(mux, h)
	handler := wrapWithAuth(mux)

	// Toggle: should disable.
	req := authedRequest(http.MethodPatch, "/api/v1/admin/projects/"+strconv.FormatInt(proj.ID, 10)+"/toggle", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp model.ApiResponse[map[string]bool]
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Data["enabled"] {
		t.Fatal("expected enabled=false after toggle")
	}
}

func TestAdminUpdateProject(t *testing.T) {
	database := setupAdminTestDB(t)
	defer database.Close()

	projectRepo := db.NewProjectRepo(database)
	proj, err := projectRepo.CreateWithSettings(context.Background(),
		"updproj", "Update Me", "github", "https://github.com/test/upd",
		model.ProjectSettings{GitHubOwner: "test", GitHubRepo: "upd"})
	if err != nil {
		t.Fatalf("failed to create project: %v", err)
	}

	h, _, _ := setupProjectAdminHandler(t, database)
	mux := http.NewServeMux()
	registerProjectAdminRoutes(mux, h)
	handler := wrapWithAuth(mux)

	body, _ := json.Marshal(model.UpdateProjectRequest{
		Name:        "updproj",
		DisplayName: "Updated Name",
		SourceType:  model.SourceTypeGitHub,
		SourceURL:   "https://github.com/test/upd",
		Settings:    model.ProjectSettings{GitHubOwner: "test", GitHubRepo: "upd", CrawlInterval: 3600},
	})
	req := authedRequest(http.MethodPut, "/api/v1/admin/projects/"+strconv.FormatInt(proj.ID, 10), body)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// Verify settings were updated.
	settings, err := projectRepo.GetSettings(context.Background(), proj.ID)
	if err != nil {
		t.Fatalf("failed to get settings: %v", err)
	}
	if settings.CrawlInterval != 3600 {
		t.Fatalf("expected crawl_interval=3600, got %d", settings.CrawlInterval)
	}
}
func TestAdminListProjects_FileCountReturned(t *testing.T) {
	database := setupAdminTestDB(t)
	defer database.Close()

	projectRepo := db.NewProjectRepo(database)
	fileRepo := db.NewFileRepo(database)

	proj, err := projectRepo.CreateWithSettings(context.Background(),
		"testproj", "Test Project", "github", "https://github.com/test/test",
		model.ProjectSettings{})
	if err != nil {
		t.Fatalf("failed to seed project: %v", err)
	}

	// Create 3 files for the project.
	for i := 0; i < 3; i++ {
		f := &db.File{
			ProjectID: proj.ID,
			Filename:  fmt.Sprintf("file%d.tar.gz", i),
			Status:    "complete",
		}
		_, err := fileRepo.Create(context.Background(), f)
		if err != nil {
			t.Fatalf("failed to create file %d: %v", i, err)
		}
	}

	h, _, _ := setupProjectAdminHandler(t, database)
	mux := http.NewServeMux()
	registerProjectAdminRoutes(mux, h)
	handler := wrapWithAuth(mux)

	req := authedRequest(http.MethodGet, "/api/v1/admin/projects", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp model.ApiResponse[[]model.AdminProjectResponse]
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !resp.Success {
		t.Fatal("expected success=true")
	}
	if len(resp.Data) < 1 {
		t.Fatalf("expected at least 1 project, got %d", len(resp.Data))
	}
	if resp.Data[0].FileCount != 3 {
		t.Fatalf("expected FileCount=3, got %d", resp.Data[0].FileCount)
	}
}

func TestAdminListProjects_FileCountZeroWhenNoFiles(t *testing.T) {
	database := setupAdminTestDB(t)
	defer database.Close()

	projectRepo := db.NewProjectRepo(database)
	_, err := projectRepo.CreateWithSettings(context.Background(),
		"nofiles", "No Files", "github", "https://github.com/test/nofiles",
		model.ProjectSettings{})
	if err != nil {
		t.Fatalf("failed to seed project: %v", err)
	}

	h, _, _ := setupProjectAdminHandler(t, database)
	mux := http.NewServeMux()
	registerProjectAdminRoutes(mux, h)
	handler := wrapWithAuth(mux)

	req := authedRequest(http.MethodGet, "/api/v1/admin/projects", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp model.ApiResponse[[]model.AdminProjectResponse]
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !resp.Success {
		t.Fatal("expected success=true")
	}
	if resp.Data[0].FileCount != 0 {
		t.Fatalf("expected FileCount=0, got %d", resp.Data[0].FileCount)
	}
}

func TestAdminGetProject_FileCountReturned(t *testing.T) {
	database := setupAdminTestDB(t)
	defer database.Close()

	projectRepo := db.NewProjectRepo(database)
	fileRepo := db.NewFileRepo(database)

	proj, err := projectRepo.CreateWithSettings(context.Background(),
		"getproj", "Get Project", "github", "https://github.com/test/getproj",
		model.ProjectSettings{})
	if err != nil {
		t.Fatalf("failed to seed project: %v", err)
	}

	// Create 2 files for the project.
	for i := 0; i < 2; i++ {
		f := &db.File{
			ProjectID: proj.ID,
			Filename:  fmt.Sprintf("file%d.tar.gz", i),
			Status:    "complete",
		}
		_, err := fileRepo.Create(context.Background(), f)
		if err != nil {
			t.Fatalf("failed to create file %d: %v", i, err)
		}
	}

	h, _, _ := setupProjectAdminHandler(t, database)
	mux := http.NewServeMux()
	registerProjectAdminRoutes(mux, h)
	handler := wrapWithAuth(mux)

	req := authedRequest(http.MethodGet, "/api/v1/admin/projects/"+strconv.FormatInt(proj.ID, 10), nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp model.ApiResponse[model.AdminProjectResponse]
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !resp.Success {
		t.Fatal("expected success=true")
	}
	if resp.Data.FileCount != 2 {
		t.Fatalf("expected FileCount=2, got %d", resp.Data.FileCount)
	}
}
