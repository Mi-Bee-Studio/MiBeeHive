package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	dbrepo "github.com/Mi-Bee-Studio/mibeehive/internal/db"
	"github.com/Mi-Bee-Studio/mibeehive/internal/model"
)

func setupTemplateHandler(t *testing.T) (*AppTemplateHandler, http.Handler) {
	t.Helper()
	database := setupTestDB(t)
	t.Cleanup(func() { database.Close() })
	repo := dbrepo.NewAppTemplateRepo(database)
	h := NewAppTemplateHandler(repo)

	mux := http.NewServeMux()
	mux.HandleFunc("GET "+model.RouteAdminTemplateList, h.HandleTemplateList)
	mux.HandleFunc("GET "+model.RouteAdminTemplateGet, h.HandleTemplateGet)
	mux.HandleFunc("POST "+model.RouteAdminTemplateCreate, h.HandleTemplateCreate)
	mux.HandleFunc("DELETE "+model.RouteAdminTemplateDelete, h.HandleTemplateDelete)

	return h, wrapWithAuth(mux)
}

func TestTemplateList_WithSeeds(t *testing.T) {
	_, handler := setupTemplateHandler(t)

	req := authedRequest(http.MethodGet, model.RouteAdminTemplateList, nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp model.ApiResponse[[]model.AppTemplate]
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !resp.Success {
		t.Fatalf("expected success=true, got message: %s", resp.Message)
	}
	if len(resp.Data) != 3 {
		t.Fatalf("expected 3 seeded templates, got %d", len(resp.Data))
	}

	names := map[string]bool{}
	for _, tmpl := range resp.Data {
		names[tmpl.Name] = true
	}
	for _, n := range []string{"nginx", "redis", "postgres"} {
		if !names[n] {
			t.Errorf("expected template %q in list", n)
		}
	}
}

func TestTemplateList_Empty(t *testing.T) {
	database := setupTestDB(t)
	t.Cleanup(func() { database.Close() })

	// Delete all seeded templates to test empty list.
	database.Exec("DELETE FROM app_templates")

	repo := dbrepo.NewAppTemplateRepo(database)
	h := NewAppTemplateHandler(repo)

	mux := http.NewServeMux()
	mux.HandleFunc("GET "+model.RouteAdminTemplateList, h.HandleTemplateList)
	handler := wrapWithAuth(mux)

	req := authedRequest(http.MethodGet, model.RouteAdminTemplateList, nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp model.ApiResponse[[]model.AppTemplate]
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
		t.Fatalf("expected 0 templates, got %d", len(resp.Data))
	}
}

func TestTemplateGet_ValidID(t *testing.T) {
	_, handler := setupTemplateHandler(t)

	req := authedRequest(http.MethodGet, "/api/v1/admin/templates/1", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp model.ApiResponse[model.AppTemplate]
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !resp.Success {
		t.Fatalf("expected success=true, got message: %s", resp.Message)
	}
	if resp.Data.ID != 1 {
		t.Fatalf("expected id=1, got %d", resp.Data.ID)
	}
	if resp.Data.Name != "nginx" {
		t.Fatalf("expected name=nginx, got %q", resp.Data.Name)
	}
	if resp.Data.Image != "nginx:alpine" {
		t.Fatalf("expected image=nginx:alpine, got %q", resp.Data.Image)
	}
}

func TestTemplateGet_InvalidID(t *testing.T) {
	_, handler := setupTemplateHandler(t)

	req := authedRequest(http.MethodGet, "/api/v1/admin/templates/999", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestTemplateCreate_ValidRequest(t *testing.T) {
	_, handler := setupTemplateHandler(t)

	body := []byte(`{"name":"mysql","description":"MySQL Database","image":"mysql:8","ports":[{"host_port":3306,"container_port":3306,"protocol":"tcp"}],"env":{"MYSQL_ROOT_PASSWORD":"secret"},"category":"database"}`)
	req := authedRequest(http.MethodPost, model.RouteAdminTemplateCreate, body)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp model.ApiResponse[model.AppTemplate]
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !resp.Success {
		t.Fatalf("expected success=true, got message: %s", resp.Message)
	}
	if resp.Data.Name != "mysql" {
		t.Fatalf("expected name=mysql, got %q", resp.Data.Name)
	}
	if resp.Data.Image != "mysql:8" {
		t.Fatalf("expected image=mysql:8, got %q", resp.Data.Image)
	}
	if resp.Data.ID == 0 {
		t.Fatal("expected non-zero id after creation")
	}
}

func TestTemplateCreate_MissingName(t *testing.T) {
	_, handler := setupTemplateHandler(t)

	body := []byte(`{"image":"mysql:8"}`)
	req := authedRequest(http.MethodPost, model.RouteAdminTemplateCreate, body)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp model.ApiResponse[any]
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Success {
		t.Fatal("expected success=false for missing name")
	}
}

func TestTemplateDelete_ValidID(t *testing.T) {
	_, handler := setupTemplateHandler(t)

	req := authedRequest(http.MethodDelete, "/api/v1/admin/templates/1", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// Verify it's gone.
	req = authedRequest(http.MethodGet, "/api/v1/admin/templates/1", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 after delete, got %d", rec.Code)
	}
}

func TestTemplateDelete_InvalidID(t *testing.T) {
	_, handler := setupTemplateHandler(t)

	req := authedRequest(http.MethodDelete, "/api/v1/admin/templates/999", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}
