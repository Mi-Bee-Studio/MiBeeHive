package service

import (
	"context"
	"log/slog"
	"testing"

	"github.com/Mi-Bee-Studio/mibeehive/internal/db"
	"github.com/Mi-Bee-Studio/mibeehive/internal/model"
)

func testAppTemplateDB(t *testing.T) *db.AppTemplateRepo {
	t.Helper()
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("Open(:memory:): %v", err)
	}
	t.Cleanup(func() { database.Close() })
	if err := db.Migrate(database); err != nil {
		t.Fatalf("Migrate(): %v", err)
	}
	return db.NewAppTemplateRepo(database)
}

func TestAppTemplateService_ListTemplates(t *testing.T) {
	repo := testAppTemplateDB(t)
	svc := NewAppTemplateService(repo, slog.Default())
	ctx := context.Background()

	templates, err := svc.ListTemplates(ctx)
	if err != nil {
		t.Fatalf("ListTemplates: %v", err)
	}

	// Seed data should provide 3 templates (nginx, redis, postgres)
	if len(templates) < 3 {
		t.Errorf("expected at least 3 seed templates, got %d", len(templates))
	}

	// Verify all returned templates are enabled
	for _, tmpl := range templates {
		if !tmpl.Enabled {
			t.Errorf("ListTemplates returned disabled template: %s", tmpl.Name)
		}
	}

	// Find nginx by name
	found := false
	for _, tmpl := range templates {
		if tmpl.Name == "nginx" {
			found = true
			if tmpl.Image != "nginx:alpine" {
				t.Errorf("nginx image: expected nginx:alpine, got %s", tmpl.Image)
			}
			if tmpl.Category != "web" {
				t.Errorf("nginx category: expected web, got %s", tmpl.Category)
			}
			if len(tmpl.Ports) != 1 || tmpl.Ports[0].HostPort != 80 {
				t.Errorf("nginx ports: expected [{host_port:80}], got %v", tmpl.Ports)
			}
		}
	}
	if !found {
		t.Error("nginx template not found in seed data")
	}
}

func TestAppTemplateService_GetTemplate(t *testing.T) {
	repo := testAppTemplateDB(t)
	svc := NewAppTemplateService(repo, slog.Default())
	ctx := context.Background()

	// Get all first to find an ID
	templates, _ := svc.ListTemplates(ctx)
	if len(templates) == 0 {
		t.Fatal("no templates available")
	}

	// Get by existing ID
	tmpl, err := svc.GetTemplate(ctx, templates[0].ID)
	if err != nil {
		t.Fatalf("GetTemplate: %v", err)
	}
	if tmpl == nil {
		t.Fatal("expected template, got nil")
	}
	if tmpl.Name != templates[0].Name {
		t.Errorf("expected name=%s, got %s", templates[0].Name, tmpl.Name)
	}

	// Get by non-existent ID
	tmpl, err = svc.GetTemplate(ctx, 99999)
	if err != nil {
		t.Fatalf("GetTemplate(99999): %v", err)
	}
	if tmpl != nil {
		t.Errorf("expected nil for non-existent ID, got %v", tmpl)
	}
}

func TestAppTemplateService_CreateFromTemplate(t *testing.T) {
	repo := testAppTemplateDB(t)
	svc := NewAppTemplateService(repo, slog.Default())
	ctx := context.Background()

	// Find the postgres template
	templates, _ := svc.ListTemplates(ctx)
	var postgresID int64
	for _, tmpl := range templates {
		if tmpl.Name == "postgres" {
			postgresID = tmpl.ID
		}
	}
	if postgresID == 0 {
		t.Fatal("postgres template not found")
	}

	// Create from template without overrides
	req, err := svc.CreateFromTemplate(ctx, postgresID, nil)
	if err != nil {
		t.Fatalf("CreateFromTemplate: %v", err)
	}
	if req == nil {
		t.Fatal("expected request, got nil")
	}
	if req.Image != "postgres:alpine" {
		t.Errorf("expected image=postgres:alpine, got %s", req.Image)
	}
	if req.Env["POSTGRES_PASSWORD"] != "changeme" {
		t.Errorf("expected env POSTGRES_PASSWORD=changeme, got %v", req.Env)
	}
	if len(req.Ports) != 1 || req.Ports[0].HostPort != 5432 {
		t.Errorf("expected ports [{host_port:5432}], got %v", req.Ports)
	}

	// Create from template with overrides
	overrides := &model.CreateContainerRequest{
		Name:  "my-postgres",
		Env:   map[string]string{"POSTGRES_PASSWORD": "secret123"},
		Ports: []model.PortMapping{{HostPort: 5433, ContainerPort: 5432, Protocol: "tcp"}},
	}
	req, err = svc.CreateFromTemplate(ctx, postgresID, overrides)
	if err != nil {
		t.Fatalf("CreateFromTemplate with overrides: %v", err)
	}
	if req.Name != "my-postgres" {
		t.Errorf("expected name=my-postgres, got %s", req.Name)
	}
	if req.Env["POSTGRES_PASSWORD"] != "secret123" {
		t.Errorf("expected overridden env, got %v", req.Env)
	}
	if len(req.Ports) != 1 || req.Ports[0].HostPort != 5433 {
		t.Errorf("expected overridden ports, got %v", req.Ports)
	}

	// Non-existent template
	req, err = svc.CreateFromTemplate(ctx, 99999, nil)
	if err != nil {
		t.Fatalf("CreateFromTemplate(99999): %v", err)
	}
	if req != nil {
		t.Errorf("expected nil for non-existent template, got %v", req)
	}
}

func TestAppTemplateRepo_CRUD(t *testing.T) {
	repo := testAppTemplateDB(t)
	ctx := context.Background()

	// Create a custom template
	tmpl := &model.AppTemplate{
		Name:          "custom-app",
		Description:   "Custom Application",
		Image:         "custom:latest",
		Command:       "/bin/run",
		Env:           map[string]string{"KEY": "value"},
		Ports:         []model.PortMapping{{HostPort: 8080, ContainerPort: 8080, Protocol: "tcp"}},
		Volumes:       []model.VolumeMount{{HostPath: "/data", ContainerPath: "/app/data", Mode: "rw"}},
		RestartPolicy: "always",
		Category:      "custom",
		Enabled:       true,
	}
	err := repo.Create(ctx, tmpl)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if tmpl.ID == 0 {
		t.Fatal("expected non-zero ID after Create")
	}

	// GetByID
	got, err := repo.GetByID(ctx, tmpl.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got == nil {
		t.Fatal("expected template, got nil")
	}
	if got.Name != "custom-app" {
		t.Errorf("expected name=custom-app, got %s", got.Name)
	}
	if got.Image != "custom:latest" {
		t.Errorf("expected image=custom:latest, got %s", got.Image)
	}
	if got.Env["KEY"] != "value" {
		t.Errorf("expected env KEY=value, got %v", got.Env)
	}
	if len(got.Ports) != 1 || got.Ports[0].HostPort != 8080 {
		t.Errorf("expected ports [{host_port:8080}], got %v", got.Ports)
	}
	if len(got.Volumes) != 1 || got.Volumes[0].HostPath != "/data" {
		t.Errorf("expected volumes [{host_path:/data}], got %v", got.Volumes)
	}
	if !got.Enabled {
		t.Error("expected enabled=true")
	}

	// List should include seed data + custom
	all, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(all) < 4 {
		t.Errorf("expected at least 4 enabled templates (3 seed + 1 custom), got %d", len(all))
	}

	// Delete
	err = repo.Delete(ctx, tmpl.ID)
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}
	got, err = repo.GetByID(ctx, tmpl.ID)
	if err != nil {
		t.Fatalf("GetByID after delete: %v", err)
	}
	if got != nil {
		t.Error("expected nil after delete")
	}

	// ListAll should still show seed data
	allTemplates, err := repo.ListAll(ctx)
	if err != nil {
		t.Fatalf("ListAll: %v", err)
	}
	if len(allTemplates) < 3 {
		t.Errorf("expected at least 3 seed templates, got %d", len(allTemplates))
	}
}

func TestAppTemplateRepo_GetByID_NotFound(t *testing.T) {
	repo := testAppTemplateDB(t)
	ctx := context.Background()

	got, err := repo.GetByID(ctx, 99999)
	if err != nil {
		t.Fatalf("GetByID(99999): %v", err)
	}
	if got != nil {
		t.Errorf("expected nil for non-existent ID, got %v", got)
	}
}

func TestAppTemplateRepo_CreateDuplicate(t *testing.T) {
	repo := testAppTemplateDB(t)
	ctx := context.Background()

	tmpl := &model.AppTemplate{
		Name:   "nginx", // duplicate of seed data
		Image:  "nginx:test",
		Enabled: true,
	}
	err := repo.Create(ctx, tmpl)
	if err == nil {
		t.Error("expected error on duplicate template name")
	}
}
