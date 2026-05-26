package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Mi-Bee-Studio/mibeehive/internal/model"
)

// === Mock Services ===

type mockRegistryService struct {
	registries []model.Registry
	registry   *model.Registry
	tags       []string
	repos      []string
	tag        *model.RegistryTag
	manifest   *model.ManifestDetail
	err        error
}

func (m *mockRegistryService) ListRegistries(ctx context.Context) ([]model.Registry, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.registries, nil
}

func (m *mockRegistryService) CreateRegistry(ctx context.Context, req model.CreateRegistryRequest) (*model.Registry, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.registry != nil {
		return m.registry, nil
	}
	return &model.Registry{ID: 1, Name: req.Name, URL: req.URL, Type: req.Type}, nil
}

func (m *mockRegistryService) GetRegistry(ctx context.Context, id int64) (*model.Registry, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.registry != nil {
		return m.registry, nil
	}
	return &model.Registry{ID: id, Name: "test-registry"}, nil
}

func (m *mockRegistryService) UpdateRegistry(ctx context.Context, id int64, req model.UpdateRegistryRequest) (*model.Registry, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &model.Registry{ID: id, Name: req.Name, URL: req.URL}, nil
}

func (m *mockRegistryService) DeleteRegistry(ctx context.Context, id int64) error {
	return m.err
}

func (m *mockRegistryService) TestConnection(ctx context.Context, id int64) (*model.TestConnectionResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &model.TestConnectionResponse{Success: true, Version: "2.0", RegistryType: "dockerhub"}, nil
}

func (m *mockRegistryService) BrowseCatalog(ctx context.Context, id int64, n int, last string) ([]string, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.repos, nil
}

func (m *mockRegistryService) BrowseTags(ctx context.Context, id int64, repo string, n int, last string) ([]string, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.tags, nil
}

func (m *mockRegistryService) GetTagDetail(ctx context.Context, id int64, repo, tag string) (*model.RegistryTag, *model.ManifestDetail, error) {
	if m.err != nil {
		return nil, nil, m.err
	}
	return m.tag, m.manifest, nil
}

func (m *mockRegistryService) DeleteTag(ctx context.Context, id int64, repo, tag string) error {
	return m.err
}

type mockSyncService struct {
	tasks []model.SyncTask
	task  *model.SyncTask
	err   error
}

func (m *mockSyncService) CreateSync(ctx context.Context, req model.SyncRequest) (*model.SyncTask, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.task != nil {
		return m.task, nil
	}
	return &model.SyncTask{ID: 1, SourceRepo: req.SourceRepo, SourceTag: req.SourceTag, Status: model.SyncTaskPending}, nil
}

func (m *mockSyncService) ListSyncTasks(ctx context.Context, status string) ([]model.SyncTask, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.tasks, nil
}

func (m *mockSyncService) GetSyncTask(ctx context.Context, id int64) (*model.SyncTask, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.task != nil {
		return m.task, nil
	}
	return &model.SyncTask{ID: id, Status: model.SyncTaskRunning}, nil
}

func (m *mockSyncService) CancelSync(ctx context.Context, id int64) error {
	return m.err
}

type mockRetentionService struct {
	policies []model.RetentionPolicy
	policy   *model.RetentionPolicy
	deleted  int
	err      error
}

func (m *mockRetentionService) ListPolicies(ctx context.Context) ([]model.RetentionPolicy, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.policies, nil
}

func (m *mockRetentionService) CreatePolicy(ctx context.Context, req model.CreateRetentionPolicyRequest) (*model.RetentionPolicy, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.policy != nil {
		return m.policy, nil
	}
	return &model.RetentionPolicy{ID: 1, RepoPattern: req.RepoPattern, KeepDays: req.KeepDays}, nil
}

func (m *mockRetentionService) UpdatePolicy(ctx context.Context, id int64, req model.CreateRetentionPolicyRequest) (*model.RetentionPolicy, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &model.RetentionPolicy{ID: id, RepoPattern: req.RepoPattern}, nil
}

func (m *mockRetentionService) DeletePolicy(ctx context.Context, id int64) error {
	return m.err
}

func (m *mockRetentionService) ExecutePolicy(ctx context.Context, id int64) (int, error) {
	if m.err != nil {
		return 0, m.err
	}
	return m.deleted, nil
}

// === Setup Helpers ===

func setupRegistryHandler(registrySvc *mockRegistryService, syncSvc *mockSyncService, retentionSvc *mockRetentionService) *RegistryHandler {
	if registrySvc == nil {
		registrySvc = &mockRegistryService{}
	}
	if syncSvc == nil {
		syncSvc = &mockSyncService{}
	}
	if retentionSvc == nil {
		retentionSvc = &mockRetentionService{}
	}
	return NewRegistryHandler(registrySvc, syncSvc, retentionSvc, true)
}

func setupDisabledRegistryHandler() *RegistryHandler {
	return NewRegistryHandler(&mockRegistryService{}, &mockSyncService{}, &mockRetentionService{}, false)
}

func registerRegistryRoutes(mux *http.ServeMux, h *RegistryHandler) {
	mux.HandleFunc(model.RouteRegistryList, h.ListRegistries)
	mux.HandleFunc(model.RouteRegistryCreate, h.CreateRegistry)
	mux.HandleFunc(model.RouteRegistryGet, h.GetRegistry)
	mux.HandleFunc(model.RouteRegistryUpdate, h.UpdateRegistry)
	mux.HandleFunc(model.RouteRegistryDelete, h.DeleteRegistry)
	mux.HandleFunc(model.RouteRegistryTestConnection, h.TestConnection)
	mux.HandleFunc(model.RouteRegistryCatalog, h.BrowseCatalog)
	mux.HandleFunc(model.RouteRegistryTags, h.BrowseTags)
	mux.HandleFunc(model.RouteRegistryTagDetail, h.GetTagDetail)
	mux.HandleFunc(model.RouteRegistryTagDelete, h.DeleteTag)
	mux.HandleFunc(model.RouteSyncCreate, h.CreateSync)
	mux.HandleFunc(model.RouteSyncTaskList, h.ListSyncTasks)
	mux.HandleFunc(model.RouteSyncTaskGet, h.GetSyncTask)
	mux.HandleFunc(model.RouteSyncTaskCancel, h.CancelSync)
	mux.HandleFunc(model.RouteRetentionList, h.ListPolicies)
	mux.HandleFunc(model.RouteRetentionCreate, h.CreatePolicy)
	mux.HandleFunc(model.RouteRetentionUpdate, h.UpdatePolicy)
	mux.HandleFunc(model.RouteRetentionDelete, h.DeletePolicy)
	mux.HandleFunc(model.RouteRetentionExecute, h.ExecutePolicy)
}

// === Registry CRUD Tests ===

func TestListRegistries_Empty(t *testing.T) {
	h := setupRegistryHandler(&mockRegistryService{registries: []model.Registry{}}, nil, nil)

	mux := http.NewServeMux()
	registerRegistryRoutes(mux, h)
	handler := wrapWithAuth(mux)

	req := authedRequest(http.MethodGet, "/api/v1/admin/registries", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp model.ApiResponse[[]model.Registry]
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !resp.Success {
		t.Fatalf("expected success=true, got message: %s", resp.Message)
	}
	if len(resp.Data) != 0 {
		t.Fatalf("expected 0 registries, got %d", len(resp.Data))
	}
}

func TestListRegistries_WithData(t *testing.T) {
	registries := []model.Registry{
		{ID: 1, Name: "dockerhub", URL: "https://registry.hub.docker.com", Type: model.DockerHub, Enabled: true},
		{ID: 2, Name: "ghcr", URL: "https://ghcr.io", Type: model.GHCR, Enabled: true},
	}
	h := setupRegistryHandler(&mockRegistryService{registries: registries}, nil, nil)

	mux := http.NewServeMux()
	registerRegistryRoutes(mux, h)
	handler := wrapWithAuth(mux)

	req := authedRequest(http.MethodGet, "/api/v1/admin/registries", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp model.ApiResponse[[]model.Registry]
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(resp.Data) != 2 {
		t.Fatalf("expected 2 registries, got %d", len(resp.Data))
	}
	if resp.Data[0].Name != "dockerhub" {
		t.Fatalf("expected first registry name 'dockerhub', got %q", resp.Data[0].Name)
	}
}

func TestCreateRegistry_Success(t *testing.T) {
	created := &model.Registry{ID: 1, Name: "dockerhub", URL: "https://registry.hub.docker.com", Type: model.DockerHub}
	h := setupRegistryHandler(&mockRegistryService{registry: created}, nil, nil)

	mux := http.NewServeMux()
	registerRegistryRoutes(mux, h)
	handler := wrapWithAuth(mux)

	body := []byte(`{"name":"dockerhub","url":"https://registry.hub.docker.com","type":"dockerhub","username":"user","password":"pass"}`)
	req := authedRequest(http.MethodPost, "/api/v1/admin/registries", body)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp model.ApiResponse[model.Registry]
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !resp.Success {
		t.Fatalf("expected success=true, got message: %s", resp.Message)
	}
	if resp.Data.Name != "dockerhub" {
		t.Fatalf("expected registry name 'dockerhub', got %q", resp.Data.Name)
	}
}

func TestCreateRegistry_InvalidInput(t *testing.T) {
	h := setupRegistryHandler(nil, nil, nil)

	mux := http.NewServeMux()
	registerRegistryRoutes(mux, h)
	handler := wrapWithAuth(mux)

	tests := []struct {
		name    string
		body    string
		wantMsg string
	}{
		{"empty name", `{"url":"https://example.com","type":"dockerhub","username":"u","password":"p"}`, "name is required"},
		{"empty url", `{"name":"test","type":"dockerhub","username":"u","password":"p"}`, "url is required"},
		{"empty type", `{"name":"test","url":"https://example.com","username":"u","password":"p"}`, "type is required"},
		{"empty username", `{"name":"test","url":"https://example.com","type":"dockerhub","password":"p"}`, "username is required"},
		{"empty password", `{"name":"test","url":"https://example.com","type":"dockerhub","username":"u"}`, "password is required"},
		{"invalid json", `{invalid}`, "invalid request body"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := authedRequest(http.MethodPost, "/api/v1/admin/registries", []byte(tt.body))
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
			}

			var resp model.ApiResponse[any]
			if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}
			if resp.Message != tt.wantMsg {
				t.Fatalf("expected message %q, got %q", tt.wantMsg, resp.Message)
			}
		})
	}
}

func TestGetRegistry_Success(t *testing.T) {
	h := setupRegistryHandler(&mockRegistryService{registry: &model.Registry{ID: 1, Name: "test-registry"}}, nil, nil)

	mux := http.NewServeMux()
	registerRegistryRoutes(mux, h)
	handler := wrapWithAuth(mux)

	req := authedRequest(http.MethodGet, "/api/v1/admin/registries/1", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp model.ApiResponse[model.Registry]
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Data.ID != 1 {
		t.Fatalf("expected registry id 1, got %d", resp.Data.ID)
	}
}

func TestGetRegistry_InvalidID(t *testing.T) {
	h := setupRegistryHandler(nil, nil, nil)

	mux := http.NewServeMux()
	registerRegistryRoutes(mux, h)
	handler := wrapWithAuth(mux)

	req := authedRequest(http.MethodGet, "/api/v1/admin/registries/abc", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestGetRegistry_NotFound(t *testing.T) {
	h := setupRegistryHandler(&mockRegistryService{err: errors.New("not found")}, nil, nil)

	mux := http.NewServeMux()
	registerRegistryRoutes(mux, h)
	handler := wrapWithAuth(mux)

	req := authedRequest(http.MethodGet, "/api/v1/admin/registries/999", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestUpdateRegistry_Success(t *testing.T) {
	h := setupRegistryHandler(nil, nil, nil)

	mux := http.NewServeMux()
	registerRegistryRoutes(mux, h)
	handler := wrapWithAuth(mux)

	body := []byte(`{"name":"updated","url":"https://updated.example.com","type":"dockerhub","username":"user","password":"newpass"}`)
	req := authedRequest(http.MethodPut, "/api/v1/admin/registries/1", body)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp model.ApiResponse[model.Registry]
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Data.Name != "updated" {
		t.Fatalf("expected name 'updated', got %q", resp.Data.Name)
	}
}

func TestDeleteRegistry_Success(t *testing.T) {
	h := setupRegistryHandler(nil, nil, nil)

	mux := http.NewServeMux()
	registerRegistryRoutes(mux, h)
	handler := wrapWithAuth(mux)

	req := authedRequest(http.MethodDelete, "/api/v1/admin/registries/1", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp model.ApiResponse[any]
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !resp.Success {
		t.Fatalf("expected success=true, got message: %s", resp.Message)
	}
}

// === Test Connection ===

func TestTestConnection_Success(t *testing.T) {
	h := setupRegistryHandler(nil, nil, nil)

	mux := http.NewServeMux()
	registerRegistryRoutes(mux, h)
	handler := wrapWithAuth(mux)

	body := []byte(`{"registry_id":1}`)
	req := authedRequest(http.MethodPost, "/api/v1/admin/registries/test-connection", body)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp model.ApiResponse[model.TestConnectionResponse]
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !resp.Success {
		t.Fatalf("expected success=true, got message: %s", resp.Message)
	}
	if !resp.Data.Success {
		t.Fatal("expected connection success=true")
	}
	if resp.Data.RegistryType != "dockerhub" {
		t.Fatalf("expected registry_type 'dockerhub', got %q", resp.Data.RegistryType)
	}
}

func TestTestConnection_MissingRegistryID(t *testing.T) {
	h := setupRegistryHandler(nil, nil, nil)

	mux := http.NewServeMux()
	registerRegistryRoutes(mux, h)
	handler := wrapWithAuth(mux)

	body := []byte(`{}`)
	req := authedRequest(http.MethodPost, "/api/v1/admin/registries/test-connection", body)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp model.ApiResponse[any]
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Message != "registry_id is required" {
		t.Fatalf("expected 'registry_id is required', got %q", resp.Message)
	}
}

// === Browsing Tests ===

func TestBrowseCatalog_Success(t *testing.T) {
	h := setupRegistryHandler(&mockRegistryService{repos: []string{"nginx", "redis", "alpine"}}, nil, nil)

	mux := http.NewServeMux()
	registerRegistryRoutes(mux, h)
	handler := wrapWithAuth(mux)

	req := authedRequest(http.MethodGet, "/api/v1/admin/registries/1/catalog?n=10", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp model.ApiResponse[[]string]
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(resp.Data) != 3 {
		t.Fatalf("expected 3 repos, got %d", len(resp.Data))
	}
	if resp.Data[0] != "nginx" {
		t.Fatalf("expected first repo 'nginx', got %q", resp.Data[0])
	}
}

func TestBrowseCatalog_Empty(t *testing.T) {
	h := setupRegistryHandler(&mockRegistryService{repos: []string{}}, nil, nil)

	mux := http.NewServeMux()
	registerRegistryRoutes(mux, h)
	handler := wrapWithAuth(mux)

	req := authedRequest(http.MethodGet, "/api/v1/admin/registries/1/catalog", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp model.ApiResponse[[]string]
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(resp.Data) != 0 {
		t.Fatalf("expected 0 repos, got %d", len(resp.Data))
	}
}

func TestBrowseTags_Success(t *testing.T) {
	h := setupRegistryHandler(&mockRegistryService{tags: []string{"latest", "1.0", "1.1"}}, nil, nil)

	mux := http.NewServeMux()
	registerRegistryRoutes(mux, h)
	handler := wrapWithAuth(mux)

	req := authedRequest(http.MethodGet, "/api/v1/admin/registries/1/tags?repo=nginx", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp model.ApiResponse[[]string]
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(resp.Data) != 3 {
		t.Fatalf("expected 3 tags, got %d", len(resp.Data))
	}
}

func TestBrowseTags_MissingRepo(t *testing.T) {
	h := setupRegistryHandler(nil, nil, nil)

	mux := http.NewServeMux()
	registerRegistryRoutes(mux, h)
	handler := wrapWithAuth(mux)

	req := authedRequest(http.MethodGet, "/api/v1/admin/registries/1/tags", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestGetTagDetail_Success(t *testing.T) {
	now := time.Now()
	tag := &model.RegistryTag{Name: "latest", Digest: "sha256:abc123", Size: 1024, CreatedAt: now, MediaType: "application/vnd.docker.distribution.manifest.v2+json"}
	manifest := &model.ManifestDetail{SchemaVersion: 2, MediaType: "application/vnd.docker.distribution.manifest.v2+json"}

	h := setupRegistryHandler(&mockRegistryService{tag: tag, manifest: manifest}, nil, nil)

	mux := http.NewServeMux()
	registerRegistryRoutes(mux, h)
	handler := wrapWithAuth(mux)

	req := authedRequest(http.MethodGet, "/api/v1/admin/registries/1/tags/latest?repo=nginx", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp model.ApiResponse[tagDetailResponse]
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Data.Tag.Name != "latest" {
		t.Fatalf("expected tag name 'latest', got %q", resp.Data.Tag.Name)
	}
}

func TestDeleteTag_Success(t *testing.T) {
	h := setupRegistryHandler(nil, nil, nil)

	mux := http.NewServeMux()
	registerRegistryRoutes(mux, h)
	handler := wrapWithAuth(mux)

	req := authedRequest(http.MethodDelete, "/api/v1/admin/registries/1/tags/sha256%3Aabc123?repo=nginx", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp model.ApiResponse[any]
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !resp.Success {
		t.Fatalf("expected success=true, got message: %s", resp.Message)
	}
}

func TestDeleteTag_MissingRepo(t *testing.T) {
	h := setupRegistryHandler(nil, nil, nil)

	mux := http.NewServeMux()
	registerRegistryRoutes(mux, h)
	handler := wrapWithAuth(mux)

	req := authedRequest(http.MethodDelete, "/api/v1/admin/registries/1/tags/sha256%3Aabc123", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

// === Sync Tests ===

func TestCreateSync_Success(t *testing.T) {
	task := &model.SyncTask{ID: 1, SourceRegistryID: 1, TargetRegistryID: 2, SourceRepo: "nginx", SourceTag: "latest", Status: model.SyncTaskPending}
	h := setupRegistryHandler(nil, &mockSyncService{task: task}, nil)

	mux := http.NewServeMux()
	registerRegistryRoutes(mux, h)
	handler := wrapWithAuth(mux)

	body := []byte(`{"source_registry_id":1,"target_registry_id":2,"source_repo":"nginx","source_tag":"latest","target_repo":"nginx","target_tag":"latest"}`)
	req := authedRequest(http.MethodPost, "/api/v1/admin/sync", body)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp model.ApiResponse[model.SyncTask]
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Data.SourceRepo != "nginx" {
		t.Fatalf("expected source_repo 'nginx', got %q", resp.Data.SourceRepo)
	}
}

func TestCreateSync_MissingFields(t *testing.T) {
	h := setupRegistryHandler(nil, nil, nil)

	mux := http.NewServeMux()
	registerRegistryRoutes(mux, h)
	handler := wrapWithAuth(mux)

	tests := []struct {
		name    string
		body    string
		wantMsg string
	}{
		{"missing source id", `{"target_registry_id":2,"source_repo":"nginx","source_tag":"latest"}`, "source_registry_id is required"},
		{"missing target id", `{"source_registry_id":1,"source_repo":"nginx","source_tag":"latest"}`, "target_registry_id is required"},
		{"missing source repo", `{"source_registry_id":1,"target_registry_id":2,"source_tag":"latest"}`, "source_repo is required"},
		{"missing source tag", `{"source_registry_id":1,"target_registry_id":2,"source_repo":"nginx"}`, "source_tag is required"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := authedRequest(http.MethodPost, "/api/v1/admin/sync", []byte(tt.body))
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
			}

			var resp model.ApiResponse[any]
			if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}
			if resp.Message != tt.wantMsg {
				t.Fatalf("expected message %q, got %q", tt.wantMsg, resp.Message)
			}
		})
	}
}

func TestListSyncTasks_Empty(t *testing.T) {
	h := setupRegistryHandler(nil, &mockSyncService{tasks: []model.SyncTask{}}, nil)

	mux := http.NewServeMux()
	registerRegistryRoutes(mux, h)
	handler := wrapWithAuth(mux)

	req := authedRequest(http.MethodGet, "/api/v1/admin/sync", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp model.ApiResponse[[]model.SyncTask]
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(resp.Data) != 0 {
		t.Fatalf("expected 0 tasks, got %d", len(resp.Data))
	}
}

func TestGetSyncTask_Success(t *testing.T) {
	h := setupRegistryHandler(nil, nil, nil)

	mux := http.NewServeMux()
	registerRegistryRoutes(mux, h)
	handler := wrapWithAuth(mux)

	req := authedRequest(http.MethodGet, "/api/v1/admin/sync/1", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp model.ApiResponse[model.SyncTask]
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Data.ID != 1 {
		t.Fatalf("expected task id 1, got %d", resp.Data.ID)
	}
}

func TestCancelSync_Success(t *testing.T) {
	h := setupRegistryHandler(nil, nil, nil)

	mux := http.NewServeMux()
	registerRegistryRoutes(mux, h)
	handler := wrapWithAuth(mux)

	req := authedRequest(http.MethodPost, "/api/v1/admin/sync/1/cancel", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp model.ApiResponse[any]
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !resp.Success {
		t.Fatalf("expected success=true, got message: %s", resp.Message)
	}
}

// === Retention Tests ===

func TestListPolicies_Empty(t *testing.T) {
	h := setupRegistryHandler(nil, nil, &mockRetentionService{policies: []model.RetentionPolicy{}})

	mux := http.NewServeMux()
	registerRegistryRoutes(mux, h)
	handler := wrapWithAuth(mux)

	req := authedRequest(http.MethodGet, "/api/v1/admin/retention", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp model.ApiResponse[[]model.RetentionPolicy]
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(resp.Data) != 0 {
		t.Fatalf("expected 0 policies, got %d", len(resp.Data))
	}
}

func TestCreatePolicy_Success(t *testing.T) {
	policy := &model.RetentionPolicy{ID: 1, RegistryID: 1, RepoPattern: ".*", KeepDays: 30, KeepCount: 10, Enabled: true}
	h := setupRegistryHandler(nil, nil, &mockRetentionService{policy: policy})

	mux := http.NewServeMux()
	registerRegistryRoutes(mux, h)
	handler := wrapWithAuth(mux)

	body := []byte(`{"registry_id":1,"repo_pattern":".*","keep_days":30,"keep_count":10,"enabled":true}`)
	req := authedRequest(http.MethodPost, "/api/v1/admin/retention", body)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp model.ApiResponse[model.RetentionPolicy]
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Data.KeepDays != 30 {
		t.Fatalf("expected keep_days=30, got %d", resp.Data.KeepDays)
	}
}

func TestCreatePolicy_InvalidInput(t *testing.T) {
	h := setupRegistryHandler(nil, nil, nil)

	mux := http.NewServeMux()
	registerRegistryRoutes(mux, h)
	handler := wrapWithAuth(mux)

	tests := []struct {
		name    string
		body    string
		wantMsg string
	}{
		{"zero keep_days", `{"registry_id":1,"repo_pattern":".*","keep_days":0,"keep_count":10}`, "keep_days must be >= 1"},
		{"zero keep_count", `{"registry_id":1,"repo_pattern":".*","keep_days":30,"keep_count":0}`, "keep_count must be >= 1"},
		{"empty repo_pattern", `{"registry_id":1,"repo_pattern":"","keep_days":30,"keep_count":10}`, "repo_pattern is required"},
		{"invalid keep_pattern", `{"registry_id":1,"repo_pattern":".*","keep_days":30,"keep_count":10,"keep_pattern":"[invalid"}`, "keep_pattern"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := authedRequest(http.MethodPost, "/api/v1/admin/retention", []byte(tt.body))
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
			}

			var resp model.ApiResponse[any]
			if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}
		})
	}
}

func TestUpdatePolicy_Success(t *testing.T) {
	h := setupRegistryHandler(nil, nil, nil)

	mux := http.NewServeMux()
	registerRegistryRoutes(mux, h)
	handler := wrapWithAuth(mux)

	body := []byte(`{"registry_id":1,"repo_pattern":"nginx-*","keep_days":14,"keep_count":5}`)
	req := authedRequest(http.MethodPut, "/api/v1/admin/retention/1", body)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp model.ApiResponse[model.RetentionPolicy]
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !resp.Success {
		t.Fatalf("expected success=true, got message: %s", resp.Message)
	}
}

func TestDeletePolicy_Success(t *testing.T) {
	h := setupRegistryHandler(nil, nil, nil)

	mux := http.NewServeMux()
	registerRegistryRoutes(mux, h)
	handler := wrapWithAuth(mux)

	req := authedRequest(http.MethodDelete, "/api/v1/admin/retention/1", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestExecutePolicy_Success(t *testing.T) {
	h := setupRegistryHandler(nil, nil, &mockRetentionService{deleted: 5})

	mux := http.NewServeMux()
	registerRegistryRoutes(mux, h)
	handler := wrapWithAuth(mux)

	req := authedRequest(http.MethodPost, "/api/v1/admin/retention/1/execute", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp model.ApiResponse[any]
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Message != "deleted 5 tags" {
		t.Fatalf("expected 'deleted 5 tags', got %q", resp.Message)
	}
}

// === Remote Disabled Tests ===

func TestHandlers_RemoteDisabled(t *testing.T) {
	h := setupDisabledRegistryHandler()

	tests := []struct {
		name   string
		method string
		path   string
		body   []byte
	}{
		{"list registries", http.MethodGet, "/api/v1/admin/registries", nil},
		{"create registry", http.MethodPost, "/api/v1/admin/registries", []byte(`{"name":"test"}`)},
		{"get registry", http.MethodGet, "/api/v1/admin/registries/1", nil},
		{"update registry", http.MethodPut, "/api/v1/admin/registries/1", []byte(`{"name":"test"}`)},
		{"delete registry", http.MethodDelete, "/api/v1/admin/registries/1", nil},
		{"test connection", http.MethodPost, "/api/v1/admin/registries/test-connection", []byte(`{"registry_id":1}`)},
		{"browse catalog", http.MethodGet, "/api/v1/admin/registries/1/catalog", nil},
		{"browse tags", http.MethodGet, "/api/v1/admin/registries/1/tags?repo=nginx", nil},
		{"tag detail", http.MethodGet, "/api/v1/admin/registries/1/tags/latest?repo=nginx", nil},
		{"delete tag", http.MethodDelete, "/api/v1/admin/registries/1/tags/latest?repo=nginx", nil},
		{"create sync", http.MethodPost, "/api/v1/admin/sync", []byte(`{"source_registry_id":1,"target_registry_id":2,"source_repo":"n","source_tag":"l"}`)},
		{"list sync", http.MethodGet, "/api/v1/admin/sync", nil},
		{"get sync", http.MethodGet, "/api/v1/admin/sync/1", nil},
		{"cancel sync", http.MethodPost, "/api/v1/admin/sync/1/cancel", nil},
		{"list retention", http.MethodGet, "/api/v1/admin/retention", nil},
		{"create retention", http.MethodPost, "/api/v1/admin/retention", []byte(`{"registry_id":1,"repo_pattern":".*","keep_days":30,"keep_count":10}`)},
		{"update retention", http.MethodPut, "/api/v1/admin/retention/1", []byte(`{"registry_id":1,"repo_pattern":".*","keep_days":30,"keep_count":10}`)},
		{"delete retention", http.MethodDelete, "/api/v1/admin/retention/1", nil},
		{"execute retention", http.MethodPost, "/api/v1/admin/retention/1/execute", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mux := http.NewServeMux()
			registerRegistryRoutes(mux, h)
			handler := wrapWithAuth(mux)

			req := authedRequest(tt.method, tt.path, tt.body)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusNotFound {
				t.Fatalf("%s: expected 404, got %d: %s", tt.name, rec.Code, rec.Body.String())
			}

			var resp model.ApiResponse[any]
			if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}
			if resp.Success {
				t.Fatalf("%s: expected success=false", tt.name)
			}
			if resp.Message != "Remote registry management is disabled" {
				t.Fatalf("%s: unexpected message: %s", tt.name, resp.Message)
			}
		})
	}
}

// === Service Error Tests ===

func TestListRegistries_ServiceError(t *testing.T) {
	h := setupRegistryHandler(&mockRegistryService{err: errors.New("db error")}, nil, nil)

	mux := http.NewServeMux()
	registerRegistryRoutes(mux, h)
	handler := wrapWithAuth(mux)

	req := authedRequest(http.MethodGet, "/api/v1/admin/registries", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestCreateRegistry_ServiceError(t *testing.T) {
	h := setupRegistryHandler(&mockRegistryService{err: errors.New("db error")}, nil, nil)

	mux := http.NewServeMux()
	registerRegistryRoutes(mux, h)
	handler := wrapWithAuth(mux)

	body := []byte(`{"name":"test","url":"https://example.com","type":"dockerhub","username":"u","password":"p"}`)
	req := authedRequest(http.MethodPost, "/api/v1/admin/registries", body)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestDeleteTag_ServiceError(t *testing.T) {
	h := setupRegistryHandler(&mockRegistryService{err: errors.New("delete failed")}, nil, nil)

	mux := http.NewServeMux()
	registerRegistryRoutes(mux, h)
	handler := wrapWithAuth(mux)

	req := authedRequest(http.MethodDelete, "/api/v1/admin/registries/1/tags/latest?repo=nginx", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestSyncTask_ServiceError(t *testing.T) {
	h := setupRegistryHandler(nil, &mockSyncService{err: errors.New("sync error")}, nil)

	mux := http.NewServeMux()
	registerRegistryRoutes(mux, h)
	handler := wrapWithAuth(mux)

	req := authedRequest(http.MethodGet, "/api/v1/admin/sync/1", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestExecutePolicy_ServiceError(t *testing.T) {
	h := setupRegistryHandler(nil, nil, &mockRetentionService{err: errors.New("execution error")})

	mux := http.NewServeMux()
	registerRegistryRoutes(mux, h)
	handler := wrapWithAuth(mux)

	req := authedRequest(http.MethodPost, "/api/v1/admin/retention/1/execute", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", rec.Code, rec.Body.String())
	}
}
