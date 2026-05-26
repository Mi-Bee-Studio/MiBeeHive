package handler

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Mi-Bee-Studio/mibeehive/internal/model"
)

// === Mock: Container Lifecycle ===

// mockContainerService implements containerService for testing.
type mockContainerService struct {
	containers []model.Container
	created    *model.Container
	err        error // if set, all operations return this error
}

func (m *mockContainerService) List(ctx context.Context) ([]model.Container, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.containers, nil
}

func (m *mockContainerService) Create(ctx context.Context, req model.CreateContainerRequest) (*model.Container, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.created, nil
}

func (m *mockContainerService) Start(ctx context.Context, id string) error {
	return m.err
}

func (m *mockContainerService) Stop(ctx context.Context, id string, timeout int) error {
	return m.err
}

func (m *mockContainerService) Restart(ctx context.Context, id string, timeout int) error {
	return m.err
}

func (m *mockContainerService) Remove(ctx context.Context, id string, force bool) error {
	return m.err
}

// === Mock: Image Service ===

// mockImageService implements imageService for testing.
type mockImageService struct {
	images []model.Image
	err    error
}

func (m *mockImageService) ImageList(ctx context.Context) ([]model.Image, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.images, nil
}

func (m *mockImageService) ImagePull(ctx context.Context, imageName string) error {
	return m.err
}

func (m *mockImageService) ImageDelete(ctx context.Context, imageID string) error {
	return m.err
}

// === Mock: Container Log Service ===

// mockContainerLogService implements containerLogReader for testing.
type mockContainerLogService struct {
	stats *model.ContainerStats
	logs  io.ReadCloser
	err   error
}

func (m *mockContainerLogService) ContainerStats(ctx context.Context, id string) (*model.ContainerStats, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.stats, nil
}

func (m *mockContainerLogService) ContainerLogs(ctx context.Context, id string, tail string, since string) (io.ReadCloser, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.logs, nil
}

// === Docker Log Frame Helper ===

// dockerLogFrame builds a Docker multiplexed log frame for testing.
// streamType: 1=stdout, 2=stderr.
func dockerLogFrame(streamType byte, content string) []byte {
	payload := []byte(content + "\n")
	header := make([]byte, 8)
	header[0] = streamType
	binary.BigEndian.PutUint32(header[4:8], uint32(len(payload)))
	return append(header, payload...)
}

// === Setup Helpers ===

// setupContainerHandler creates a ContainerHandler with only lifecycle service (backward compat).
func setupContainerHandler(mock *mockContainerService) *ContainerHandler {
	return &ContainerHandler{
		svc:             mock,
		logger:          slog.Default(),
		dockerAvailable: true,
	}
}

func setupFullContainerHandler(containerMock *mockContainerService, imageMock *mockImageService, logMock *mockContainerLogService) *ContainerHandler {
	return &ContainerHandler{
		svc:             containerMock,
		imgSvc:          imageMock,
		logSvc:          logMock,
		logger:          slog.Default(),
		dockerAvailable: true,
	}
}

// registerContainerRoutes registers all container routes on the given mux.
func registerContainerRoutes(mux *http.ServeMux, h *ContainerHandler) {
	mux.HandleFunc("GET "+model.RouteAdminContainerList, h.HandleContainerList)
	mux.HandleFunc("POST "+model.RouteAdminContainerCreate, h.HandleContainerCreate)
	mux.HandleFunc("POST "+model.RouteAdminContainerStart, h.HandleContainerStart)
	mux.HandleFunc("POST "+model.RouteAdminContainerStop, h.HandleContainerStop)
	mux.HandleFunc("POST "+model.RouteAdminContainerRestart, h.HandleContainerRestart)
	mux.HandleFunc("DELETE "+model.RouteAdminContainerDelete, h.HandleContainerDelete)
}

// registerAllContainerRoutes registers container + image + stats + logs routes.
func registerAllContainerRoutes(mux *http.ServeMux, h *ContainerHandler) {
	registerContainerRoutes(mux, h)
	mux.HandleFunc("GET "+model.RouteAdminImageList, h.HandleImageList)
	mux.HandleFunc("POST "+model.RouteAdminImagePull, h.HandleImagePull)
	mux.HandleFunc("DELETE "+model.RouteAdminImageDelete, h.HandleImageDelete)
	mux.HandleFunc("GET "+model.RouteAdminContainerStats, h.HandleContainerStats)
	mux.HandleFunc("GET "+model.RouteAdminContainerLogs, h.HandleContainerLogs)
}

// === Container Lifecycle Tests ===

func TestContainerList_Empty(t *testing.T) {
	mock := &mockContainerService{containers: []model.Container{}}
	h := setupContainerHandler(mock)

	mux := http.NewServeMux()
	registerContainerRoutes(mux, h)
	handler := wrapWithAuth(mux)

	req := authedRequest(http.MethodGet, "/api/v1/admin/containers", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp model.ApiResponse[[]model.Container]
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !resp.Success {
		t.Fatalf("expected success=true, got message: %s", resp.Message)
	}
	if len(resp.Data) != 0 {
		t.Fatalf("expected 0 containers, got %d", len(resp.Data))
	}
}

func TestContainerList_WithContainers(t *testing.T) {
	mock := &mockContainerService{
		containers: []model.Container{
			{Name: "nginx", Image: "nginx:latest", Status: "running", ContainerID: "abc123"},
			{Name: "redis", Image: "redis:7", Status: "exited", ContainerID: "def456"},
		},
	}
	h := setupContainerHandler(mock)

	mux := http.NewServeMux()
	registerContainerRoutes(mux, h)
	handler := wrapWithAuth(mux)

	req := authedRequest(http.MethodGet, "/api/v1/admin/containers", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp model.ApiResponse[[]model.Container]
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !resp.Success {
		t.Fatalf("expected success=true, got message: %s", resp.Message)
	}
	if len(resp.Data) != 2 {
		t.Fatalf("expected 2 containers, got %d", len(resp.Data))
	}
	if resp.Data[0].Name != "nginx" {
		t.Fatalf("expected first container name 'nginx', got %q", resp.Data[0].Name)
	}
	if resp.Data[1].Status != "exited" {
		t.Fatalf("expected second container status 'exited', got %q", resp.Data[1].Status)
	}
}

func TestContainerList_XTotalCount(t *testing.T) {
	mock := &mockContainerService{
		containers: []model.Container{
			{Name: "nginx", Image: "nginx:latest", Status: "running", ContainerID: "abc123"},
			{Name: "redis", Image: "redis:7", Status: "exited", ContainerID: "def456"},
		},
	}
	h := setupContainerHandler(mock)

	mux := http.NewServeMux()
	registerContainerRoutes(mux, h)
	handler := wrapWithAuth(mux)

	req := authedRequest(http.MethodGet, "/api/v1/admin/containers", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	totalHeader := rec.Header().Get("X-Total-Count")
	if totalHeader != "2" {
		t.Fatalf("expected X-Total-Count=2, got %q", totalHeader)
	}
}

func TestContainerList_Pagination(t *testing.T) {
	mock := &mockContainerService{
		containers: []model.Container{
			{Name: "a", Image: "img", Status: "running", ContainerID: "1"},
			{Name: "b", Image: "img", Status: "running", ContainerID: "2"},
			{Name: "c", Image: "img", Status: "running", ContainerID: "3"},
		},
	}
	h := setupContainerHandler(mock)

	mux := http.NewServeMux()
	registerContainerRoutes(mux, h)
	handler := wrapWithAuth(mux)

	// limit=1 — should get 1 item but total=3.
	req := authedRequest(http.MethodGet, "/api/v1/admin/containers?limit=1", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	totalHeader := rec.Header().Get("X-Total-Count")
	if totalHeader != "3" {
		t.Fatalf("expected X-Total-Count=3, got %q", totalHeader)
	}
	var resp model.ApiResponse[[]model.Container]
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(resp.Data) != 1 {
		t.Fatalf("expected 1 container with limit=1, got %d", len(resp.Data))
	}
}

// === Create Tests ===

func TestContainerCreate_ValidRequest(t *testing.T) {
	created := &model.Container{
		Name:        "nginx",
		Image:       "nginx:latest",
		Status:      "created",
		ContainerID: "new123",
	}
	mock := &mockContainerService{created: created}
	h := setupContainerHandler(mock)

	mux := http.NewServeMux()
	registerContainerRoutes(mux, h)
	handler := wrapWithAuth(mux)

	body := []byte(`{"name":"nginx","image":"nginx:latest"}`)
	req := authedRequest(http.MethodPost, "/api/v1/admin/containers", body)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp model.ApiResponse[model.Container]
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !resp.Success {
		t.Fatalf("expected success=true, got message: %s", resp.Message)
	}
	if resp.Data.Name != "nginx" {
		t.Fatalf("expected container name 'nginx', got %q", resp.Data.Name)
	}
	if resp.Data.ContainerID != "new123" {
		t.Fatalf("expected container_id 'new123', got %q", resp.Data.ContainerID)
	}
}

func TestContainerCreate_MissingName(t *testing.T) {
	mock := &mockContainerService{}
	h := setupContainerHandler(mock)

	mux := http.NewServeMux()
	registerContainerRoutes(mux, h)
	handler := wrapWithAuth(mux)

	body := []byte(`{"image":"nginx:latest"}`)
	req := authedRequest(http.MethodPost, "/api/v1/admin/containers", body)
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
		t.Fatal("expected success=false")
	}
	if resp.Message != "name is required" {
		t.Fatalf("expected 'name is required' message, got %q", resp.Message)
	}
}

func TestContainerCreate_MissingImage(t *testing.T) {
	mock := &mockContainerService{}
	h := setupContainerHandler(mock)

	mux := http.NewServeMux()
	registerContainerRoutes(mux, h)
	handler := wrapWithAuth(mux)

	body := []byte(`{"name":"nginx"}`)
	req := authedRequest(http.MethodPost, "/api/v1/admin/containers", body)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp model.ApiResponse[any]
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Message != "image is required" {
		t.Fatalf("expected 'image is required' message, got %q", resp.Message)
	}
}

func TestContainerCreate_InvalidBody(t *testing.T) {
	mock := &mockContainerService{}
	h := setupContainerHandler(mock)

	mux := http.NewServeMux()
	registerContainerRoutes(mux, h)
	handler := wrapWithAuth(mux)

	body := []byte(`{invalid json`)
	req := authedRequest(http.MethodPost, "/api/v1/admin/containers", body)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

// === Start Tests ===

func TestContainerStart_ValidID(t *testing.T) {
	mock := &mockContainerService{}
	h := setupContainerHandler(mock)

	mux := http.NewServeMux()
	registerContainerRoutes(mux, h)
	handler := wrapWithAuth(mux)

	req := authedRequest(http.MethodPost, "/api/v1/admin/containers/abc123/start", nil)
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

func TestContainerStart_InvalidID(t *testing.T) {
	mock := &mockContainerService{}
	h := setupContainerHandler(mock)

	mux := http.NewServeMux()
	registerContainerRoutes(mux, h)
	handler := wrapWithAuth(mux)

	req := authedRequest(http.MethodPost, "/api/v1/admin/containers/nonexistent/start", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// With no error set, this should succeed (mock returns nil error).
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

// === Stop Tests ===

func TestContainerStop_ValidID(t *testing.T) {
	mock := &mockContainerService{}
	h := setupContainerHandler(mock)

	mux := http.NewServeMux()
	registerContainerRoutes(mux, h)
	handler := wrapWithAuth(mux)

	req := authedRequest(http.MethodPost, "/api/v1/admin/containers/abc123/stop", nil)
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

// === Restart Tests ===

func TestContainerRestart_ValidID(t *testing.T) {
	mock := &mockContainerService{}
	h := setupContainerHandler(mock)

	mux := http.NewServeMux()
	registerContainerRoutes(mux, h)
	handler := wrapWithAuth(mux)

	req := authedRequest(http.MethodPost, "/api/v1/admin/containers/abc123/restart", nil)
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

// === Delete Tests ===

func TestContainerDelete_ValidID(t *testing.T) {
	mock := &mockContainerService{}
	h := setupContainerHandler(mock)

	mux := http.NewServeMux()
	registerContainerRoutes(mux, h)
	handler := wrapWithAuth(mux)

	req := authedRequest(http.MethodDelete, "/api/v1/admin/containers/abc123", nil)
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

// === No Docker Available Tests ===

func TestContainerEndpoints_NoDocker(t *testing.T) {
	dockerErr := errors.New("docker daemon not available")
	mock := &mockContainerService{err: dockerErr}
	h := setupContainerHandler(mock)

	tests := []struct {
		name   string
		method string
		path   string
		body   []byte
	}{
		{"list", http.MethodGet, "/api/v1/admin/containers", nil},
		{"create", http.MethodPost, "/api/v1/admin/containers", []byte(`{"name":"test","image":"nginx"}`)},
		{"start", http.MethodPost, "/api/v1/admin/containers/abc/start", nil},
		{"stop", http.MethodPost, "/api/v1/admin/containers/abc/stop", nil},
		{"restart", http.MethodPost, "/api/v1/admin/containers/abc/restart", nil},
		{"delete", http.MethodDelete, "/api/v1/admin/containers/abc", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mux := http.NewServeMux()
			registerContainerRoutes(mux, h)
			handler := wrapWithAuth(mux)

			req := authedRequest(tt.method, tt.path, tt.body)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusServiceUnavailable {
				t.Fatalf("%s: expected 503, got %d: %s", tt.name, rec.Code, rec.Body.String())
			}

			var resp model.ApiResponse[any]
			if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}
			if resp.Success {
				t.Fatalf("%s: expected success=false", tt.name)
			}
		})
	}
}

// TestContainerEndpoints_NilDocker verifies that NewContainerHandler with nil services
// returns 503 for all endpoints (the actual production nil-pointer panic fix).
func TestContainerEndpoints_NilDocker(t *testing.T) {
	h := NewContainerHandler(nil, nil, nil, slog.Default())

	tests := []struct {
		name   string
		method string
		path   string
		body   []byte
	}{
		{"list", http.MethodGet, "/api/v1/admin/containers", nil},
		{"create", http.MethodPost, "/api/v1/admin/containers", []byte(`{"name":"test","image":"nginx"}`)},
		{"start", http.MethodPost, "/api/v1/admin/containers/abc/start", nil},
		{"stop", http.MethodPost, "/api/v1/admin/containers/abc/stop", nil},
		{"restart", http.MethodPost, "/api/v1/admin/containers/abc/restart", nil},
		{"delete", http.MethodDelete, "/api/v1/admin/containers/abc", nil},
		{"images", http.MethodGet, "/api/v1/admin/images", nil},
		{"pull", http.MethodPost, "/api/v1/admin/images/pull", []byte(`{"image":"nginx"}`)},
		{"image-delete", http.MethodDelete, "/api/v1/admin/images/abc", nil},
		{"stats", http.MethodGet, "/api/v1/admin/containers/abc123/stats", nil},
		{"logs", http.MethodGet, "/api/v1/admin/containers/abc123/logs", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mux := http.NewServeMux()
			registerAllContainerRoutes(mux, h)
			handler := wrapWithAuth(mux)

			req := authedRequest(tt.method, tt.path, tt.body)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusServiceUnavailable {
				t.Fatalf("%s: expected 503, got %d: %s", tt.name, rec.Code, rec.Body.String())
			}

			var resp model.ApiResponse[any]
			if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}
			if resp.Success {
				t.Fatalf("%s: expected success=false", tt.name)
			}
			if resp.Message != "Docker is not available on this server" {
				t.Fatalf("%s: unexpected message: %s", tt.name, resp.Message)
			}
		})
	}
}

// === Image List Tests ===

func TestImageList_Empty(t *testing.T) {
	h := setupFullContainerHandler(
		&mockContainerService{},
		&mockImageService{images: []model.Image{}},
		&mockContainerLogService{},
	)

	mux := http.NewServeMux()
	registerAllContainerRoutes(mux, h)
	handler := wrapWithAuth(mux)

	req := authedRequest(http.MethodGet, "/api/v1/admin/images", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp model.ApiResponse[[]model.Image]
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !resp.Success {
		t.Fatalf("expected success=true, got message: %s", resp.Message)
	}
	if len(resp.Data) != 0 {
		t.Fatalf("expected 0 images, got %d", len(resp.Data))
	}
}

func TestImageList_WithImages(t *testing.T) {
	h := setupFullContainerHandler(
		&mockContainerService{},
		&mockImageService{
			images: []model.Image{
				{ID: "sha256:abc123", RepoTags: []string{"nginx:latest"}, SizeMB: 187.5},
				{ID: "sha256:def456", RepoTags: []string{"redis:7", "redis:latest"}, SizeMB: 130.2},
			},
		},
		&mockContainerLogService{},
	)

	mux := http.NewServeMux()
	registerAllContainerRoutes(mux, h)
	handler := wrapWithAuth(mux)

	req := authedRequest(http.MethodGet, "/api/v1/admin/images", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp model.ApiResponse[[]model.Image]
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !resp.Success {
		t.Fatalf("expected success=true, got message: %s", resp.Message)
	}
	if len(resp.Data) != 2 {
		t.Fatalf("expected 2 images, got %d", len(resp.Data))
	}
	if resp.Data[0].RepoTags[0] != "nginx:latest" {
		t.Fatalf("expected first image tag 'nginx:latest', got %q", resp.Data[0].RepoTags[0])
	}
}

// === Image Pull Tests ===

func TestImagePull_ValidRequest(t *testing.T) {
	h := setupFullContainerHandler(
		&mockContainerService{},
		&mockImageService{},
		&mockContainerLogService{},
	)

	mux := http.NewServeMux()
	registerAllContainerRoutes(mux, h)
	handler := wrapWithAuth(mux)

	body := []byte(`{"image":"nginx:latest"}`)
	req := authedRequest(http.MethodPost, "/api/v1/admin/images/pull", body)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp model.ApiResponse[any]
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !resp.Success {
		t.Fatalf("expected success=true, got message: %s", resp.Message)
	}
}

func TestImagePull_MissingImage(t *testing.T) {
	h := setupFullContainerHandler(
		&mockContainerService{},
		&mockImageService{},
		&mockContainerLogService{},
	)

	mux := http.NewServeMux()
	registerAllContainerRoutes(mux, h)
	handler := wrapWithAuth(mux)

	body := []byte(`{"image":""}`)
	req := authedRequest(http.MethodPost, "/api/v1/admin/images/pull", body)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp model.ApiResponse[any]
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Message != "image is required" {
		t.Fatalf("expected 'image is required' message, got %q", resp.Message)
	}
}

// === Image Delete Tests ===

func TestImageDelete_ValidID(t *testing.T) {
	h := setupFullContainerHandler(
		&mockContainerService{},
		&mockImageService{},
		&mockContainerLogService{},
	)

	mux := http.NewServeMux()
	registerAllContainerRoutes(mux, h)
	handler := wrapWithAuth(mux)

	req := authedRequest(http.MethodDelete, "/api/v1/admin/images/sha256%3Aabc123", nil)
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

// === Container Stats Tests ===

func TestContainerStats_ValidID(t *testing.T) {
	stats := &model.ContainerStats{
		CPUUsagePercent: 12.5,
		MemoryUsageMB:   256.0,
		MemoryLimitMB:   512.0,
		NetworkRxBytes:  1024000,
		NetworkTxBytes:  512000,
	}
	h := setupFullContainerHandler(
		&mockContainerService{},
		&mockImageService{},
		&mockContainerLogService{stats: stats},
	)

	mux := http.NewServeMux()
	registerAllContainerRoutes(mux, h)
	handler := wrapWithAuth(mux)

	req := authedRequest(http.MethodGet, "/api/v1/admin/containers/abc123/stats", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp model.ApiResponse[model.ContainerStats]
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !resp.Success {
		t.Fatalf("expected success=true, got message: %s", resp.Message)
	}
	if resp.Data.CPUUsagePercent != 12.5 {
		t.Fatalf("expected cpu_usage_percent=12.5, got %f", resp.Data.CPUUsagePercent)
	}
	if resp.Data.MemoryUsageMB != 256.0 {
		t.Fatalf("expected memory_usage_mb=256.0, got %f", resp.Data.MemoryUsageMB)
	}
}

func TestContainerStats_InvalidID(t *testing.T) {
	dockerErr := errors.New("container not found")
	h := setupFullContainerHandler(
		&mockContainerService{},
		&mockImageService{},
		&mockContainerLogService{err: dockerErr},
	)

	mux := http.NewServeMux()
	registerAllContainerRoutes(mux, h)
	handler := wrapWithAuth(mux)

	req := authedRequest(http.MethodGet, "/api/v1/admin/containers/nonexistent/stats", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp model.ApiResponse[any]
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Success {
		t.Fatal("expected success=false")
	}
}

// === Container Logs Tests ===

func TestContainerLogs_ValidID(t *testing.T) {
	logData := append(
		dockerLogFrame(1, "server started on port 8080"),
		dockerLogFrame(2, "connection refused")...,
	)
	h := setupFullContainerHandler(
		&mockContainerService{},
		&mockImageService{},
		&mockContainerLogService{logs: io.NopCloser(bytes.NewReader(logData))},
	)

	mux := http.NewServeMux()
	registerAllContainerRoutes(mux, h)
	handler := wrapWithAuth(mux)

	req := authedRequest(http.MethodGet, "/api/v1/admin/containers/abc123/logs", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp model.ApiResponse[[]model.ContainerLogEntry]
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !resp.Success {
		t.Fatalf("expected success=true, got message: %s", resp.Message)
	}
	if len(resp.Data) != 2 {
		t.Fatalf("expected 2 log entries, got %d", len(resp.Data))
	}
	if resp.Data[0].Stream != "stdout" {
		t.Fatalf("expected first entry stream 'stdout', got %q", resp.Data[0].Stream)
	}
	if resp.Data[0].Content != "server started on port 8080" {
		t.Fatalf("unexpected first entry content: %q", resp.Data[0].Content)
	}
	if resp.Data[1].Stream != "stderr" {
		t.Fatalf("expected second entry stream 'stderr', got %q", resp.Data[1].Stream)
	}
}

func TestContainerLogs_WithTail(t *testing.T) {
	logData := dockerLogFrame(1, "line 1")
	h := setupFullContainerHandler(
		&mockContainerService{},
		&mockImageService{},
		&mockContainerLogService{logs: io.NopCloser(bytes.NewReader(logData))},
	)

	mux := http.NewServeMux()
	registerAllContainerRoutes(mux, h)
	handler := wrapWithAuth(mux)

	req := authedRequest(http.MethodGet, "/api/v1/admin/containers/abc123/logs?tail=50", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp model.ApiResponse[[]model.ContainerLogEntry]
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !resp.Success {
		t.Fatalf("expected success=true, got message: %s", resp.Message)
	}
}

func TestContainerLogs_WithSince(t *testing.T) {
	logData := dockerLogFrame(1, "recent log line")
	h := setupFullContainerHandler(
		&mockContainerService{},
		&mockImageService{},
		&mockContainerLogService{logs: io.NopCloser(bytes.NewReader(logData))},
	)

	mux := http.NewServeMux()
	registerAllContainerRoutes(mux, h)
	handler := wrapWithAuth(mux)

	req := authedRequest(http.MethodGet, "/api/v1/admin/containers/abc123/logs?since=2024-01-01", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp model.ApiResponse[[]model.ContainerLogEntry]
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(resp.Data) != 1 {
		t.Fatalf("expected 1 log entry, got %d", len(resp.Data))
	}
}

func TestContainerLogs_Empty(t *testing.T) {
	h := setupFullContainerHandler(
		&mockContainerService{},
		&mockImageService{},
		&mockContainerLogService{logs: io.NopCloser(bytes.NewReader(nil))},
	)

	mux := http.NewServeMux()
	registerAllContainerRoutes(mux, h)
	handler := wrapWithAuth(mux)

	req := authedRequest(http.MethodGet, "/api/v1/admin/containers/abc123/logs", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp model.ApiResponse[[]model.ContainerLogEntry]
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !resp.Success {
		t.Fatalf("expected success=true, got message: %s", resp.Message)
	}
	if len(resp.Data) != 0 {
		t.Fatalf("expected 0 log entries for empty stream, got %d", len(resp.Data))
	}
}

func TestContainerCreate_MemoryExceedsCapacity(t *testing.T) {
	created := &model.Container{
		Name: "test", Image: "nginx:latest", Status: "created", ContainerID: "c1",
	}
	mock := &mockContainerService{created: created}
	h := setupContainerHandler(mock)
	h.totalMemBytes = 500 * 1024 * 1024 // 500 MB

	mux := http.NewServeMux()
	registerContainerRoutes(mux, h)
	handler := wrapWithAuth(mux)

	body := []byte(`{"name":"test","image":"nginx:latest","memory_limit":"2g"}`)
	req := authedRequest(http.MethodPost, "/api/v1/admin/containers", body)
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
		t.Fatal("expected success=false")
	}
	if !strings.Contains(resp.Message, "memory limit exceeds") {
		t.Fatalf("expected memory limit error, got: %s", resp.Message)
	}
}

func TestContainerCreate_CPUOutOfRange(t *testing.T) {
	mock := &mockContainerService{}
	h := setupContainerHandler(mock)

	mux := http.NewServeMux()
	registerContainerRoutes(mux, h)
	handler := wrapWithAuth(mux)

	body := []byte(`{"name":"test","image":"nginx:latest","cpu_limit":8.0}`)
	req := authedRequest(http.MethodPost, "/api/v1/admin/containers", body)
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
		t.Fatal("expected success=false")
	}
	if !strings.Contains(resp.Message, "CPU limit must be between") {
		t.Fatalf("expected CPU limit range error, got: %s", resp.Message)
	}
}

func TestContainerCreate_ValidLimits(t *testing.T) {
	created := &model.Container{
		Name: "test", Image: "nginx:latest", Status: "created", ContainerID: "c1",
	}
	mock := &mockContainerService{created: created}
	h := setupContainerHandler(mock)
	h.totalMemBytes = 8 * 1024 * 1024 * 1024 // 8 GB

	mux := http.NewServeMux()
	registerContainerRoutes(mux, h)
	handler := wrapWithAuth(mux)

	body := []byte(`{"name":"test","image":"nginx:latest","memory_limit":"512m","cpu_limit":2.0}`)
	req := authedRequest(http.MethodPost, "/api/v1/admin/containers", body)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp model.ApiResponse[model.Container]
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !resp.Success {
		t.Fatalf("expected success=true, got message: %s", resp.Message)
	}
}
