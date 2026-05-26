package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/Mi-Bee-Studio/mibeehive/internal/model"
)

// registryService defines the interface for registry CRUD and browsing operations.
type registryService interface {
	ListRegistries(ctx context.Context) ([]model.Registry, error)
	CreateRegistry(ctx context.Context, req model.CreateRegistryRequest) (*model.Registry, error)
	GetRegistry(ctx context.Context, id int64) (*model.Registry, error)
	UpdateRegistry(ctx context.Context, id int64, req model.UpdateRegistryRequest) (*model.Registry, error)
	DeleteRegistry(ctx context.Context, id int64) error
	TestConnection(ctx context.Context, id int64) (*model.TestConnectionResponse, error)
	BrowseCatalog(ctx context.Context, id int64, n int, last string) ([]string, error)
	BrowseTags(ctx context.Context, id int64, repo string, n int, last string) ([]string, error)
	GetTagDetail(ctx context.Context, id int64, repo, tag string) (*model.RegistryTag, *model.ManifestDetail, error)
	DeleteTag(ctx context.Context, id int64, repo, tag string) error
}

// syncService defines the interface for sync task operations.
type syncService interface {
	CreateSync(ctx context.Context, req model.SyncRequest) (*model.SyncTask, error)
	ListSyncTasks(ctx context.Context, status string) ([]model.SyncTask, error)
	GetSyncTask(ctx context.Context, id int64) (*model.SyncTask, error)
	CancelSync(ctx context.Context, id int64) error
}

// retentionService defines the interface for retention policy operations.
type retentionService interface {
	ListPolicies(ctx context.Context) ([]model.RetentionPolicy, error)
	CreatePolicy(ctx context.Context, req model.CreateRetentionPolicyRequest) (*model.RetentionPolicy, error)
	UpdatePolicy(ctx context.Context, id int64, req model.CreateRetentionPolicyRequest) (*model.RetentionPolicy, error)
	DeletePolicy(ctx context.Context, id int64) error
	ExecutePolicy(ctx context.Context, id int64) (int, error)
}

// RegistryHandler handles all registry management HTTP endpoints.
type RegistryHandler struct {
	registrySvc   registryService
	syncSvc       syncService
	retentionSvc  retentionService
	remoteEnabled bool
}

// NewRegistryHandler creates a new RegistryHandler.
func NewRegistryHandler(registrySvc registryService, syncSvc syncService, retentionSvc retentionService, remoteEnabled bool) *RegistryHandler {
	return &RegistryHandler{
		registrySvc:   registrySvc,
		syncSvc:       syncSvc,
		retentionSvc:  retentionSvc,
		remoteEnabled: remoteEnabled,
	}
}

// checkRemote returns false and writes 404 if remote registry management is disabled.
func (h *RegistryHandler) checkRemote(w http.ResponseWriter) bool {
	if !h.remoteEnabled {
		writeJSON(w, http.StatusNotFound, model.ApiResponse[any]{
			Success: false,
			Message: "Remote registry management is disabled",
		})
		return false
	}
	return true
}

// parseIntID extracts and parses an int64 path parameter.
func parseIntID(r *http.Request, param string) (int64, bool) {
	s := r.PathValue(param)
	if s == "" {
		return 0, false
	}
	id, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, false
	}
	return id, true
}

// writeIDError writes a bad request response for invalid ID.
func writeIDError(w http.ResponseWriter, param string) {
	writeJSON(w, http.StatusBadRequest, model.ApiResponse[any]{
		Success: false,
		Message: fmt.Sprintf("invalid %s", param),
	})
}

// === Registry CRUD Handlers ===

// ListRegistries handles GET /api/v1/admin/registries.
func (h *RegistryHandler) ListRegistries(w http.ResponseWriter, r *http.Request) {
	if !h.checkRemote(w) {
		return
	}

	registries, err := h.registrySvc.ListRegistries(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ApiResponse[any]{
			Success: false,
			Message: fmt.Sprintf("failed to list registries: %v", err),
		})
		return
	}

	if registries == nil {
		registries = []model.Registry{}
	}

	for i := range registries {
		registries[i].EncryptedPassword = ""
	}

	writeJSON(w, http.StatusOK, model.ApiResponse[[]model.Registry]{
		Success: true,
		Data:    registries,
	})
}

// CreateRegistry handles POST /api/v1/admin/registries.
func (h *RegistryHandler) CreateRegistry(w http.ResponseWriter, r *http.Request) {
	if !h.checkRemote(w) {
		return
	}

	var req model.CreateRegistryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, model.ApiResponse[any]{
			Success: false,
			Message: "invalid request body",
		})
		return
	}

	if err := req.Validate(); err != nil {
		writeJSON(w, http.StatusBadRequest, model.ApiResponse[any]{
			Success: false,
			Message: err.Error(),
		})
		return
	}

	registry, err := h.registrySvc.CreateRegistry(r.Context(), req)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ApiResponse[any]{
			Success: false,
			Message: fmt.Sprintf("failed to create registry: %v", err),
		})
		return
	}

	registry.EncryptedPassword = ""

	writeJSON(w, http.StatusCreated, model.ApiResponse[model.Registry]{
		Success: true,
		Data:    *registry,
	})
}

// GetRegistry handles GET /api/v1/admin/registries/{id}.
func (h *RegistryHandler) GetRegistry(w http.ResponseWriter, r *http.Request) {
	if !h.checkRemote(w) {
		return
	}

	id, ok := parseIntID(r, "id")
	if !ok {
		writeIDError(w, "registry id")
		return
	}

	registry, err := h.registrySvc.GetRegistry(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, model.ApiResponse[any]{
			Success: false,
			Message: fmt.Sprintf("registry not found: %v", err),
		})
		return
	}

	registry.EncryptedPassword = ""

	writeJSON(w, http.StatusOK, model.ApiResponse[model.Registry]{
		Success: true,
		Data:    *registry,
	})
}

// UpdateRegistry handles PUT /api/v1/admin/registries/{id}.
func (h *RegistryHandler) UpdateRegistry(w http.ResponseWriter, r *http.Request) {
	if !h.checkRemote(w) {
		return
	}

	id, ok := parseIntID(r, "id")
	if !ok {
		writeIDError(w, "registry id")
		return
	}

	var req model.UpdateRegistryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, model.ApiResponse[any]{
			Success: false,
			Message: "invalid request body",
		})
		return
	}
	req.ID = id

	registry, err := h.registrySvc.UpdateRegistry(r.Context(), id, req)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ApiResponse[any]{
			Success: false,
			Message: fmt.Sprintf("failed to update registry: %v", err),
		})
		return
	}

	registry.EncryptedPassword = ""

	writeJSON(w, http.StatusOK, model.ApiResponse[model.Registry]{
		Success: true,
		Data:    *registry,
	})
}

// DeleteRegistry handles DELETE /api/v1/admin/registries/{id}.
func (h *RegistryHandler) DeleteRegistry(w http.ResponseWriter, r *http.Request) {
	if !h.checkRemote(w) {
		return
	}

	id, ok := parseIntID(r, "id")
	if !ok {
		writeIDError(w, "registry id")
		return
	}

	if err := h.registrySvc.DeleteRegistry(r.Context(), id); err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ApiResponse[any]{
			Success: false,
			Message: fmt.Sprintf("failed to delete registry: %v", err),
		})
		return
	}

	writeJSON(w, http.StatusOK, model.ApiResponse[any]{
		Success: true,
		Message: "registry deleted",
	})
}

// === Test Connection ===

// TestConnection handles POST /api/v1/admin/registries/test-connection.
func (h *RegistryHandler) TestConnection(w http.ResponseWriter, r *http.Request) {
	if !h.checkRemote(w) {
		return
	}

	// Read registry_id from JSON body since route has no {id} param.
	var body struct {
		RegistryID int64 `json:"registry_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, model.ApiResponse[any]{
			Success: false,
			Message: "invalid request body",
		})
		return
	}

	if body.RegistryID == 0 {
		writeJSON(w, http.StatusBadRequest, model.ApiResponse[any]{
			Success: false,
			Message: "registry_id is required",
		})
		return
	}

	result, err := h.registrySvc.TestConnection(r.Context(), body.RegistryID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ApiResponse[any]{
			Success: false,
			Message: fmt.Sprintf("connection test failed: %v", err),
		})
		return
	}

	writeJSON(w, http.StatusOK, model.ApiResponse[model.TestConnectionResponse]{
		Success: true,
		Data:    *result,
	})
}

// === Browsing Handlers ===

// BrowseCatalog handles GET /api/v1/admin/registries/{id}/catalog.
func (h *RegistryHandler) BrowseCatalog(w http.ResponseWriter, r *http.Request) {
	if !h.checkRemote(w) {
		return
	}

	id, ok := parseIntID(r, "id")
	if !ok {
		writeIDError(w, "registry id")
		return
	}

	n := 0
	if ns := r.URL.Query().Get("n"); ns != "" {
		if v, err := strconv.Atoi(ns); err == nil && v > 0 {
			n = v
		}
	}
	last := r.URL.Query().Get("last")

	repos, err := h.registrySvc.BrowseCatalog(r.Context(), id, n, last)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ApiResponse[any]{
			Success: false,
			Message: fmt.Sprintf("failed to browse catalog: %v", err),
		})
		return
	}

	if repos == nil {
		repos = []string{}
	}

	writeJSON(w, http.StatusOK, model.ApiResponse[[]string]{
		Success: true,
		Data:    repos,
	})
}

// BrowseTags handles GET /api/v1/admin/registries/{id}/tags.
func (h *RegistryHandler) BrowseTags(w http.ResponseWriter, r *http.Request) {
	if !h.checkRemote(w) {
		return
	}

	id, ok := parseIntID(r, "id")
	if !ok {
		writeIDError(w, "registry id")
		return
	}

	repo := r.URL.Query().Get("repo")
	if repo == "" {
		writeJSON(w, http.StatusBadRequest, model.ApiResponse[any]{
			Success: false,
			Message: "repo query parameter is required",
		})
		return
	}

	n := 0
	if ns := r.URL.Query().Get("n"); ns != "" {
		if v, err := strconv.Atoi(ns); err == nil && v > 0 {
			n = v
		}
	}
	last := r.URL.Query().Get("last")

	tags, err := h.registrySvc.BrowseTags(r.Context(), id, repo, n, last)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ApiResponse[any]{
			Success: false,
			Message: fmt.Sprintf("failed to browse tags: %v", err),
		})
		return
	}

	if tags == nil {
		tags = []string{}
	}

	writeJSON(w, http.StatusOK, model.ApiResponse[[]string]{
		Success: true,
		Data:    tags,
	})
}

// tagDetailResponse combines tag and manifest detail for the API response.
type tagDetailResponse struct {
	Tag    model.RegistryTag    `json:"tag"`
	Detail model.ManifestDetail `json:"detail"`
}

// GetTagDetail handles GET /api/v1/admin/registries/{id}/tags/{tag}.
func (h *RegistryHandler) GetTagDetail(w http.ResponseWriter, r *http.Request) {
	if !h.checkRemote(w) {
		return
	}

	id, ok := parseIntID(r, "id")
	if !ok {
		writeIDError(w, "registry id")
		return
	}

	tag := r.PathValue("tag")
	if tag == "" {
		writeJSON(w, http.StatusBadRequest, model.ApiResponse[any]{
			Success: false,
			Message: "tag is required",
		})
		return
	}

	repo := r.URL.Query().Get("repo")
	if repo == "" {
		writeJSON(w, http.StatusBadRequest, model.ApiResponse[any]{
			Success: false,
			Message: "repo query parameter is required",
		})
		return
	}

	registryTag, manifest, err := h.registrySvc.GetTagDetail(r.Context(), id, repo, tag)
	if err != nil {
		writeJSON(w, http.StatusNotFound, model.ApiResponse[any]{
			Success: false,
			Message: fmt.Sprintf("failed to get tag detail: %v", err),
		})
		return
	}

	writeJSON(w, http.StatusOK, model.ApiResponse[tagDetailResponse]{
		Success: true,
		Data: tagDetailResponse{
			Tag:    *registryTag,
			Detail: *manifest,
		},
	})
}

// DeleteTag handles DELETE /api/v1/admin/registries/{id}/tags/{tag}.
func (h *RegistryHandler) DeleteTag(w http.ResponseWriter, r *http.Request) {
	if !h.checkRemote(w) {
		return
	}

	id, ok := parseIntID(r, "id")
	if !ok {
		writeIDError(w, "registry id")
		return
	}

	tag := r.PathValue("tag")
	if tag == "" {
		writeJSON(w, http.StatusBadRequest, model.ApiResponse[any]{
			Success: false,
			Message: "tag is required",
		})
		return
	}

	repo := r.URL.Query().Get("repo")
	if repo == "" {
		writeJSON(w, http.StatusBadRequest, model.ApiResponse[any]{
			Success: false,
			Message: "repo query parameter is required",
		})
		return
	}

	if err := h.registrySvc.DeleteTag(r.Context(), id, repo, tag); err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ApiResponse[any]{
			Success: false,
			Message: fmt.Sprintf("failed to delete tag: %v", err),
		})
		return
	}

	writeJSON(w, http.StatusOK, model.ApiResponse[any]{
		Success: true,
		Message: fmt.Sprintf("tag %q deleted", tag),
	})
}

// === Sync Handlers ===

// CreateSync handles POST /api/v1/admin/sync.
func (h *RegistryHandler) CreateSync(w http.ResponseWriter, r *http.Request) {
	if !h.checkRemote(w) {
		return
	}

	var req model.SyncRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, model.ApiResponse[any]{
			Success: false,
			Message: "invalid request body",
		})
		return
	}

	if req.SourceRegistryID == 0 {
		writeJSON(w, http.StatusBadRequest, model.ApiResponse[any]{
			Success: false,
			Message: "source_registry_id is required",
		})
		return
	}
	if req.TargetRegistryID == 0 {
		writeJSON(w, http.StatusBadRequest, model.ApiResponse[any]{
			Success: false,
			Message: "target_registry_id is required",
		})
		return
	}
	if req.SourceRepo == "" {
		writeJSON(w, http.StatusBadRequest, model.ApiResponse[any]{
			Success: false,
			Message: "source_repo is required",
		})
		return
	}
	if req.SourceTag == "" {
		writeJSON(w, http.StatusBadRequest, model.ApiResponse[any]{
			Success: false,
			Message: "source_tag is required",
		})
		return
	}

	task, err := h.syncSvc.CreateSync(r.Context(), req)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ApiResponse[any]{
			Success: false,
			Message: fmt.Sprintf("failed to create sync task: %v", err),
		})
		return
	}

	writeJSON(w, http.StatusCreated, model.ApiResponse[model.SyncTask]{
		Success: true,
		Data:    *task,
	})
}

// ListSyncTasks handles GET /api/v1/admin/sync.
func (h *RegistryHandler) ListSyncTasks(w http.ResponseWriter, r *http.Request) {
	if !h.checkRemote(w) {
		return
	}

	status := r.URL.Query().Get("status")

	tasks, err := h.syncSvc.ListSyncTasks(r.Context(), status)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ApiResponse[any]{
			Success: false,
			Message: fmt.Sprintf("failed to list sync tasks: %v", err),
		})
		return
	}

	if tasks == nil {
		tasks = []model.SyncTask{}
	}

	writeJSON(w, http.StatusOK, model.ApiResponse[[]model.SyncTask]{
		Success: true,
		Data:    tasks,
	})
}

// GetSyncTask handles GET /api/v1/admin/sync/{id}.
func (h *RegistryHandler) GetSyncTask(w http.ResponseWriter, r *http.Request) {
	if !h.checkRemote(w) {
		return
	}

	id, ok := parseIntID(r, "id")
	if !ok {
		writeIDError(w, "sync task id")
		return
	}

	task, err := h.syncSvc.GetSyncTask(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, model.ApiResponse[any]{
			Success: false,
			Message: fmt.Sprintf("sync task not found: %v", err),
		})
		return
	}

	writeJSON(w, http.StatusOK, model.ApiResponse[model.SyncTask]{
		Success: true,
		Data:    *task,
	})
}

// CancelSync handles POST /api/v1/admin/sync/{id}/cancel.
func (h *RegistryHandler) CancelSync(w http.ResponseWriter, r *http.Request) {
	if !h.checkRemote(w) {
		return
	}

	id, ok := parseIntID(r, "id")
	if !ok {
		writeIDError(w, "sync task id")
		return
	}

	if err := h.syncSvc.CancelSync(r.Context(), id); err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ApiResponse[any]{
			Success: false,
			Message: fmt.Sprintf("failed to cancel sync task: %v", err),
		})
		return
	}

	writeJSON(w, http.StatusOK, model.ApiResponse[any]{
		Success: true,
		Message: fmt.Sprintf("sync task %d cancelled", id),
	})
}

// === Retention Handlers ===

// ListPolicies handles GET /api/v1/admin/retention.
func (h *RegistryHandler) ListPolicies(w http.ResponseWriter, r *http.Request) {
	if !h.checkRemote(w) {
		return
	}

	policies, err := h.retentionSvc.ListPolicies(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ApiResponse[any]{
			Success: false,
			Message: fmt.Sprintf("failed to list retention policies: %v", err),
		})
		return
	}

	if policies == nil {
		policies = []model.RetentionPolicy{}
	}

	writeJSON(w, http.StatusOK, model.ApiResponse[[]model.RetentionPolicy]{
		Success: true,
		Data:    policies,
	})
}

// CreatePolicy handles POST /api/v1/admin/retention.
func (h *RegistryHandler) CreatePolicy(w http.ResponseWriter, r *http.Request) {
	if !h.checkRemote(w) {
		return
	}

	var req model.CreateRetentionPolicyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, model.ApiResponse[any]{
			Success: false,
			Message: "invalid request body",
		})
		return
	}

	if err := req.Validate(); err != nil {
		writeJSON(w, http.StatusBadRequest, model.ApiResponse[any]{
			Success: false,
			Message: err.Error(),
		})
		return
	}

	policy, err := h.retentionSvc.CreatePolicy(r.Context(), req)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ApiResponse[any]{
			Success: false,
			Message: fmt.Sprintf("failed to create retention policy: %v", err),
		})
		return
	}

	writeJSON(w, http.StatusCreated, model.ApiResponse[model.RetentionPolicy]{
		Success: true,
		Data:    *policy,
	})
}

// UpdatePolicy handles PUT /api/v1/admin/retention/{id}.
func (h *RegistryHandler) UpdatePolicy(w http.ResponseWriter, r *http.Request) {
	if !h.checkRemote(w) {
		return
	}

	id, ok := parseIntID(r, "id")
	if !ok {
		writeIDError(w, "policy id")
		return
	}

	var req model.CreateRetentionPolicyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, model.ApiResponse[any]{
			Success: false,
			Message: "invalid request body",
		})
		return
	}

	if err := req.Validate(); err != nil {
		writeJSON(w, http.StatusBadRequest, model.ApiResponse[any]{
			Success: false,
			Message: err.Error(),
		})
		return
	}

	policy, err := h.retentionSvc.UpdatePolicy(r.Context(), id, req)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ApiResponse[any]{
			Success: false,
			Message: fmt.Sprintf("failed to update retention policy: %v", err),
		})
		return
	}

	writeJSON(w, http.StatusOK, model.ApiResponse[model.RetentionPolicy]{
		Success: true,
		Data:    *policy,
	})
}

// DeletePolicy handles DELETE /api/v1/admin/retention/{id}.
func (h *RegistryHandler) DeletePolicy(w http.ResponseWriter, r *http.Request) {
	if !h.checkRemote(w) {
		return
	}

	id, ok := parseIntID(r, "id")
	if !ok {
		writeIDError(w, "policy id")
		return
	}

	if err := h.retentionSvc.DeletePolicy(r.Context(), id); err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ApiResponse[any]{
			Success: false,
			Message: fmt.Sprintf("failed to delete retention policy: %v", err),
		})
		return
	}

	writeJSON(w, http.StatusOK, model.ApiResponse[any]{
		Success: true,
		Message: "retention policy deleted",
	})
}

// ExecutePolicy handles POST /api/v1/admin/retention/{id}/execute.
func (h *RegistryHandler) ExecutePolicy(w http.ResponseWriter, r *http.Request) {
	if !h.checkRemote(w) {
		return
	}

	id, ok := parseIntID(r, "id")
	if !ok {
		writeIDError(w, "policy id")
		return
	}

	deleted, err := h.retentionSvc.ExecutePolicy(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ApiResponse[any]{
			Success: false,
			Message: fmt.Sprintf("failed to execute retention policy: %v", err),
		})
		return
	}

	writeJSON(w, http.StatusOK, model.ApiResponse[any]{
		Success: true,
		Message: fmt.Sprintf("deleted %d tags", deleted),
	})
}
