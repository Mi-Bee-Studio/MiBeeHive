package service

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Mi-Bee-Studio/mibeehive/internal/db"
	"github.com/Mi-Bee-Studio/mibeehive/internal/model"
	_ "modernc.org/sqlite"
)

const testSvcEncKey = "test-service-enc-key-1234567890ab"

func setupRegistryTestDB(t *testing.T) *sql.DB {
	t.Helper()
	d, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	if err := db.Migrate(d); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return d
}

func newTestRegistryService(t *testing.T) (*RegistryService, *sql.DB) {
	t.Helper()
	d := setupRegistryTestDB(t)
	repo := db.NewRegistryRepo(d, testSvcEncKey)
	svc := NewRegistryService(repo)
	return svc, d
}

func TestListRegistries(t *testing.T) {
	svc, _ := newTestRegistryService(t)
	ctx := context.Background()

	req1 := model.CreateRegistryRequest{
		Name: "DockerHub", URL: "https://registry-1.docker.io",
		Type: model.DockerHub, Username: "user1", Password: "pass1",
	}
	_, err := svc.CreateRegistry(ctx, req1)
	if err != nil {
		t.Fatalf("CreateRegistry: %v", err)
	}

	req2 := model.CreateRegistryRequest{
		Name: "GitHubCR", URL: "https://ghcr.io",
		Type: model.GHCR, Username: "user2", Password: "pass2",
	}
	_, err = svc.CreateRegistry(ctx, req2)
	if err != nil {
		t.Fatalf("CreateRegistry: %v", err)
	}

	registries, err := svc.ListRegistries(ctx)
	if err != nil {
		t.Fatalf("ListRegistries: %v", err)
	}
	if len(registries) != 2 {
		t.Fatalf("expected 2 registries, got %d", len(registries))
	}
}

func TestListRegistries_Empty(t *testing.T) {
	svc, _ := newTestRegistryService(t)
	registries, err := svc.ListRegistries(context.Background())
	if err != nil {
		t.Fatalf("ListRegistries: %v", err)
	}
	if len(registries) != 0 {
		t.Fatalf("expected 0 registries, got %d", len(registries))
	}
}

func TestCreateRegistry(t *testing.T) {
	svc, _ := newTestRegistryService(t)
	ctx := context.Background()

	req := model.CreateRegistryRequest{
		Name: "TestReg", URL: "https://test.io",
		Type: model.DockerHub, Username: "admin", Password: "secret",
	}
	created, err := svc.CreateRegistry(ctx, req)
	if err != nil {
		t.Fatalf("CreateRegistry: %v", err)
	}
	if created.ID <= 0 {
		t.Fatalf("expected positive ID, got %d", created.ID)
	}
	if created.Name != "TestReg" {
		t.Errorf("expected name=TestReg, got %q", created.Name)
	}
	if created.Type != model.DockerHub {
		t.Errorf("expected type=dockerhub, got %q", created.Type)
	}
	if !created.Enabled {
		t.Error("expected enabled=true")
	}
}

func TestGetRegistry(t *testing.T) {
	svc, _ := newTestRegistryService(t)
	ctx := context.Background()

	req := model.CreateRegistryRequest{
		Name: "MyReg", URL: "https://myreg.io",
		Type: model.GHCR, Username: "user", Password: "pass",
	}
	created, err := svc.CreateRegistry(ctx, req)
	if err != nil {
		t.Fatalf("CreateRegistry: %v", err)
	}

	got, err := svc.GetRegistry(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetRegistry: %v", err)
	}
	if got == nil {
		t.Fatal("GetRegistry: expected registry, got nil")
	}
	if got.Name != "MyReg" {
		t.Errorf("expected name=MyReg, got %q", got.Name)
	}
}

func TestGetRegistry_NotFound(t *testing.T) {
	svc, _ := newTestRegistryService(t)
	got, err := svc.GetRegistry(context.Background(), 999)
	if err != nil {
		t.Fatalf("GetRegistry: %v", err)
	}
	if got != nil {
		t.Fatal("expected nil for non-existent registry")
	}
}

func TestDeleteRegistry(t *testing.T) {
	svc, _ := newTestRegistryService(t)
	ctx := context.Background()

	req := model.CreateRegistryRequest{
		Name: "ToDelete", URL: "https://delete.io",
		Type: model.DockerHub, Username: "u", Password: "p",
	}
	created, err := svc.CreateRegistry(ctx, req)
	if err != nil {
		t.Fatalf("CreateRegistry: %v", err)
	}

	if err := svc.DeleteRegistry(ctx, created.ID); err != nil {
		t.Fatalf("DeleteRegistry: %v", err)
	}

	got, err := svc.GetRegistry(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetRegistry after delete: %v", err)
	}
	if got != nil {
		t.Error("expected nil after delete")
	}
}

func TestTestConnection(t *testing.T) {
	svc, _ := newTestRegistryService(t)
	ctx := context.Background()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v2/" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	req := model.CreateRegistryRequest{
		Name: "TestConn", URL: server.URL,
		Type: model.DockerHub, Username: "", Password: "",
	}
	created, err := svc.CreateRegistry(ctx, req)
	if err != nil {
		t.Fatalf("CreateRegistry: %v", err)
	}

	resp, err := svc.TestConnection(ctx, created.ID)
	if err != nil {
		t.Fatalf("TestConnection: %v", err)
	}
	if !resp.Success {
		t.Errorf("expected success, got error: %s", resp.ErrorMessage)
	}
}
