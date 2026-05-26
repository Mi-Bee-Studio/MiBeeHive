package service

import (
	"context"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/Mi-Bee-Studio/mibeehive/internal/model"
	"github.com/Mi-Bee-Studio/mibeehive/internal/registry"
)

// ---- Mock implementations ----

// mockSyncTaskRepo stores sync tasks in memory for testing.
type mockSyncTaskRepo struct {
	tasks  map[int64]*model.SyncTask
	nextID int64
}

func newMockSyncTaskRepo() *mockSyncTaskRepo {
	return &mockSyncTaskRepo{
		tasks:  make(map[int64]*model.SyncTask),
		nextID: 1,
	}
}

func (m *mockSyncTaskRepo) Create(ctx context.Context, task *model.SyncTask) (int64, error) {
	id := m.nextID
	m.nextID++
	task.ID = id
	task.Status = model.SyncTaskPending
	m.tasks[id] = task
	return id, nil
}

func (m *mockSyncTaskRepo) GetByID(ctx context.Context, id int64) (*model.SyncTask, error) {
	t, ok := m.tasks[id]
	if !ok {
		return nil, nil
	}
	return t, nil
}

func (m *mockSyncTaskRepo) Start(ctx context.Context, id int64) error {
	t, ok := m.tasks[id]
	if !ok {
		return fmt.Errorf("task %d not found", id)
	}
	t.Status = model.SyncTaskRunning
	return nil
}

func (m *mockSyncTaskRepo) UpdateProgress(ctx context.Context, id int64, progressBytes, totalBytes int64) error {
	t, ok := m.tasks[id]
	if !ok {
		return fmt.Errorf("task %d not found", id)
	}
	t.ProgressBytes = progressBytes
	t.TotalBytes = totalBytes
	return nil
}

func (m *mockSyncTaskRepo) Complete(ctx context.Context, id int64) error {
	t, ok := m.tasks[id]
	if !ok {
		return fmt.Errorf("task %d not found", id)
	}
	t.Status = model.SyncTaskCompleted
	return nil
}

func (m *mockSyncTaskRepo) Fail(ctx context.Context, id int64, errMsg string) error {
	t, ok := m.tasks[id]
	if !ok {
		return fmt.Errorf("task %d not found", id)
	}
	t.Status = model.SyncTaskFailed
	t.ErrorMessage = errMsg
	return nil
}

func (m *mockSyncTaskRepo) Cancel(ctx context.Context, id int64) error {
	t, ok := m.tasks[id]
	if !ok {
		return fmt.Errorf("task %d not found", id)
	}
	t.Status = model.SyncTaskFailed
	t.ErrorMessage = "cancelled"
	return nil
}

func (m *mockSyncTaskRepo) ListByStatus(ctx context.Context, status model.SyncTaskStatus) ([]model.SyncTask, error) {
	var result []model.SyncTask
	for _, t := range m.tasks {
		if string(status) == "" || t.Status == status {
			result = append(result, *t)
		}
	}
	return result, nil
}

func (m *mockSyncTaskRepo) GetActiveByTarget(ctx context.Context, registryID int64, repo, tag string) (*model.SyncTask, error) {
	for _, t := range m.tasks {
		if t.TargetRegistryID == registryID && t.TargetRepo == repo && t.TargetTag == tag &&
			(t.Status == model.SyncTaskPending || t.Status == model.SyncTaskRunning) {
			return t, nil
		}
	}
	return nil, nil
}

// mockRegistryStore stores registries in memory for testing.
type mockRegistryStore struct {
	registries map[int64]*model.Registry
	passwords  map[int64]string
}

func newMockRegistryStore() *mockRegistryStore {
	return &mockRegistryStore{
		registries: make(map[int64]*model.Registry),
		passwords:  make(map[int64]string),
	}
}

func (m *mockRegistryStore) GetByID(ctx context.Context, id int64) (*model.Registry, error) {
	r, ok := m.registries[id]
	if !ok {
		return nil, nil
	}
	return r, nil
}

func (m *mockRegistryStore) DecryptPassword(ctx context.Context, id int64) (string, error) {
	return m.passwords[id], nil
}

// mockRegistryClient uses function fields for flexible per-test behavior.
type mockRegistryClient struct {
	manifestFunc     func(ctx context.Context, repo, ref string) (*model.ManifestDetail, error)
	rawManifestFunc  func(ctx context.Context, repo, ref string) ([]byte, string, string, error)
	pullBlobFunc     func(ctx context.Context, repo, digest string, writer io.Writer) error
	pushBlobFunc     func(ctx context.Context, repo string, reader io.Reader, digest string, size int64) error
	blobExistsFunc   func(ctx context.Context, repo, digest string) (bool, error)
	pushManifestFunc func(ctx context.Context, repo, tag string, body []byte, mediaType string) error
}

func (m *mockRegistryClient) Manifest(ctx context.Context, repo, ref string) (*model.ManifestDetail, error) {
	return m.manifestFunc(ctx, repo, ref)
}

func (m *mockRegistryClient) RawManifest(ctx context.Context, repo, ref string) ([]byte, string, string, error) {
	return m.rawManifestFunc(ctx, repo, ref)
}

func (m *mockRegistryClient) PullBlob(ctx context.Context, repo, digest string, writer io.Writer) error {
	return m.pullBlobFunc(ctx, repo, digest, writer)
}

func (m *mockRegistryClient) PushBlob(ctx context.Context, repo string, reader io.Reader, digest string, size int64) error {
	return m.pushBlobFunc(ctx, repo, reader, digest, size)
}

func (m *mockRegistryClient) BlobExists(ctx context.Context, repo, digest string) (bool, error) {
	return m.blobExistsFunc(ctx, repo, digest)
}

func (m *mockRegistryClient) PushManifest(ctx context.Context, repo, tag string, body []byte, mediaType string) error {
	return m.pushManifestFunc(ctx, repo, tag, body, mediaType)
}

// ---- Test helpers ----

const (
	testRawManifest = `{"schemaVersion":2,"mediaType":"application/vnd.docker.distribution.manifest.v2+json","layers":[]}`
	testMediaType   = "application/vnd.docker.distribution.manifest.v2+json"
	testDigest      = "sha256:abcdef1234567890"
)

func makeTestManifest() *model.ManifestDetail {
	return &model.ManifestDetail{
		SchemaVersion: 2,
		MediaType:     testMediaType,
		Layers: []model.LayerInfo{
			{Digest: "sha256:aaa111", MediaType: "application/vnd.docker.image.rootfs.diff.tar.gzip", Size: 100},
			{Digest: "sha256:bbb222", MediaType: "application/vnd.docker.image.rootfs.diff.tar.gzip", Size: 200},
			{Digest: "sha256:ccc333", MediaType: "application/vnd.docker.image.rootfs.diff.tar.gzip", Size: 300},
		},
		Config: struct {
			Digest string `json:"digest"`
		}{
			Digest: "sha256:cfg999",
		},
	}
}

func setupTestService(
	taskRepo *mockSyncTaskRepo,
	regRepo *mockRegistryStore,
	sourceClient *mockRegistryClient,
	targetClient *mockRegistryClient,
) *SyncService {
	return &SyncService{
		taskRepo:     taskRepo,
		registryRepo: regRepo,
		clientFactory: func(url string, creds *registry.Credentials) (RegistrySyncClient, error) {
			if strings.Contains(url, "source") {
				return sourceClient, nil
			}
			return targetClient, nil
		},
		maxConcurrency: 2,
	}
}

// ---- Tests ----

func TestCreateSyncTask_Validation(t *testing.T) {
	regRepo := newMockRegistryStore()
	taskRepo := newMockSyncTaskRepo()
	svc := &SyncService{taskRepo: taskRepo, registryRepo: regRepo, maxConcurrency: 2}

	// Neither source nor target registry exists.
	_, err := svc.CreateSync(context.Background(), model.SyncRequest{
		SourceRegistryID: 1,
		TargetRegistryID: 2,
		SourceRepo:       "library/nginx",
		SourceTag:        "latest",
		TargetRepo:       "mirror/nginx",
		TargetTag:        "latest",
	})
	if err == nil {
		t.Fatal("expected error for missing source registry")
	}
	if !strings.Contains(err.Error(), "source registry") {
		t.Errorf("expected source registry error, got: %v", err)
	}

	// Add source registry, target still missing.
	regRepo.registries[1] = &model.Registry{ID: 1, URL: "https://source.example.com", Username: "user"}
	_, err = svc.CreateSync(context.Background(), model.SyncRequest{
		SourceRegistryID: 1,
		TargetRegistryID: 2,
		SourceRepo:       "library/nginx",
		SourceTag:        "latest",
		TargetRepo:       "mirror/nginx",
		TargetTag:        "latest",
	})
	if err == nil {
		t.Fatal("expected error for missing target registry")
	}
	if !strings.Contains(err.Error(), "target registry") {
		t.Errorf("expected target registry error, got: %v", err)
	}
}

func TestExecuteSync_Success(t *testing.T) {
	regRepo := newMockRegistryStore()
	regRepo.registries[1] = &model.Registry{ID: 1, URL: "https://source.example.com", Username: "user"}
	regRepo.registries[2] = &model.Registry{ID: 2, URL: "https://target.example.com", Username: "user"}
	regRepo.passwords[1] = "pass1"
	regRepo.passwords[2] = "pass2"

	taskRepo := newMockSyncTaskRepo()
	manifest := makeTestManifest()

	var pulledBlobs, pushedBlobs []string

	sourceClient := &mockRegistryClient{
		manifestFunc: func(ctx context.Context, repo, ref string) (*model.ManifestDetail, error) {
			return manifest, nil
		},
		rawManifestFunc: func(ctx context.Context, repo, ref string) ([]byte, string, string, error) {
			return []byte(testRawManifest), testDigest, testMediaType, nil
		},
		pullBlobFunc: func(ctx context.Context, repo, digest string, writer io.Writer) error {
			pulledBlobs = append(pulledBlobs, digest)
			writer.Write([]byte("blob-data-for-" + digest))
			return nil
		},
	}

	targetClient := &mockRegistryClient{
		rawManifestFunc: func(ctx context.Context, repo, ref string) ([]byte, string, string, error) {
			return nil, "", "", fmt.Errorf("tag not found")
		},
		blobExistsFunc: func(ctx context.Context, repo, digest string) (bool, error) {
			return false, nil
		},
		pushBlobFunc: func(ctx context.Context, repo string, reader io.Reader, digest string, size int64) error {
			pushedBlobs = append(pushedBlobs, digest)
			io.Copy(io.Discard, reader)
			return nil
		},
		pushManifestFunc: func(ctx context.Context, repo, tag string, body []byte, mediaType string) error {
			return nil
		},
	}

	svc := setupTestService(taskRepo, regRepo, sourceClient, targetClient)

	// Create task.
	created, err := svc.CreateSync(context.Background(), model.SyncRequest{
		SourceRegistryID: 1,
		TargetRegistryID: 2,
		SourceRepo:       "library/nginx",
		SourceTag:        "latest",
		TargetRepo:       "mirror/nginx",
		TargetTag:        "latest",
	})
	if err != nil {
		t.Fatalf("CreateSync: %v", err)
	}

	// Execute sync.
	if err := svc.ExecuteSync(context.Background(), created.ID); err != nil {
		t.Fatalf("ExecuteSync: %v", err)
	}

	// All 3 layers + 1 config = 4 blobs pulled and pushed.
	if len(pulledBlobs) != 4 {
		t.Errorf("expected 4 blobs pulled, got %d: %v", len(pulledBlobs), pulledBlobs)
	}
	if len(pushedBlobs) != 4 {
		t.Errorf("expected 4 blobs pushed, got %d: %v", len(pushedBlobs), pushedBlobs)
	}

	// Task should be completed.
	task, _ := taskRepo.GetByID(context.Background(), created.ID)
	if task.Status != model.SyncTaskCompleted {
		t.Errorf("expected status completed, got %s", task.Status)
	}
	if task.ProgressBytes != 600 { // 100+200+300
		t.Errorf("expected progress 600, got %d", task.ProgressBytes)
	}
}

func TestExecuteSync_SkipExistingBlobs(t *testing.T) {
	regRepo := newMockRegistryStore()
	regRepo.registries[1] = &model.Registry{ID: 1, URL: "https://source.example.com", Username: "user"}
	regRepo.registries[2] = &model.Registry{ID: 2, URL: "https://target.example.com", Username: "user"}

	taskRepo := newMockSyncTaskRepo()
	manifest := makeTestManifest()

	var pulledBlobs, pushedBlobs []string

	sourceClient := &mockRegistryClient{
		manifestFunc: func(ctx context.Context, repo, ref string) (*model.ManifestDetail, error) {
			return manifest, nil
		},
		rawManifestFunc: func(ctx context.Context, repo, ref string) ([]byte, string, string, error) {
			return []byte(testRawManifest), testDigest, testMediaType, nil
		},
		pullBlobFunc: func(ctx context.Context, repo, digest string, writer io.Writer) error {
			pulledBlobs = append(pulledBlobs, digest)
			writer.Write([]byte("blob-data"))
			return nil
		},
	}

	// Layer 2 (sha256:bbb222) already exists in target.
	targetClient := &mockRegistryClient{
		rawManifestFunc: func(ctx context.Context, repo, ref string) ([]byte, string, string, error) {
			return nil, "", "", fmt.Errorf("tag not found")
		},
		blobExistsFunc: func(ctx context.Context, repo, digest string) (bool, error) {
			return digest == "sha256:bbb222", nil
		},
		pushBlobFunc: func(ctx context.Context, repo string, reader io.Reader, digest string, size int64) error {
			pushedBlobs = append(pushedBlobs, digest)
			io.Copy(io.Discard, reader)
			return nil
		},
		pushManifestFunc: func(ctx context.Context, repo, tag string, body []byte, mediaType string) error {
			return nil
		},
	}

	svc := setupTestService(taskRepo, regRepo, sourceClient, targetClient)

	created, err := svc.CreateSync(context.Background(), model.SyncRequest{
		SourceRegistryID: 1,
		TargetRegistryID: 2,
		SourceRepo:       "test/repo",
		SourceTag:        "v1",
		TargetRepo:       "mirror/repo",
		TargetTag:        "v1",
	})
	if err != nil {
		t.Fatalf("CreateSync: %v", err)
	}

	if err := svc.ExecuteSync(context.Background(), created.ID); err != nil {
		t.Fatalf("ExecuteSync: %v", err)
	}

	// Layer 2 should NOT be pulled (skipped because it exists).
	for _, d := range pulledBlobs {
		if d == "sha256:bbb222" {
			t.Error("existing blob should not have been pulled")
		}
	}
	for _, d := range pushedBlobs {
		if d == "sha256:bbb222" {
			t.Error("existing blob should not have been pushed")
		}
	}

	// Should have 2 layers + 1 config = 3 pulled/pushed (layer 2 skipped).
	if len(pulledBlobs) != 3 {
		t.Errorf("expected 3 blobs pulled (layer 2 skipped), got %d: %v", len(pulledBlobs), pulledBlobs)
	}
	if len(pushedBlobs) != 3 {
		t.Errorf("expected 3 blobs pushed (layer 2 skipped), got %d: %v", len(pushedBlobs), pushedBlobs)
	}

	// Progress should include all 3 layer sizes (including skipped one).
	task, _ := taskRepo.GetByID(context.Background(), created.ID)
	if task.Status != model.SyncTaskCompleted {
		t.Errorf("expected completed, got %s", task.Status)
	}
	if task.ProgressBytes != 600 {
		t.Errorf("expected progress 600, got %d", task.ProgressBytes)
	}
}

func TestExecuteSync_Failure(t *testing.T) {
	regRepo := newMockRegistryStore()
	regRepo.registries[1] = &model.Registry{ID: 1, URL: "https://source.example.com", Username: "user"}
	regRepo.registries[2] = &model.Registry{ID: 2, URL: "https://target.example.com", Username: "user"}

	taskRepo := newMockSyncTaskRepo()

	sourceClient := &mockRegistryClient{
		rawManifestFunc: func(ctx context.Context, repo, ref string) ([]byte, string, string, error) {
			return []byte(testRawManifest), testDigest, testMediaType, nil
		},
		manifestFunc: func(ctx context.Context, repo, ref string) (*model.ManifestDetail, error) {
			return makeTestManifest(), nil
		},
		pullBlobFunc: func(ctx context.Context, repo, digest string, writer io.Writer) error {
			return fmt.Errorf("network error pulling %s", digest)
		},
	}

	targetClient := &mockRegistryClient{
		rawManifestFunc: func(ctx context.Context, repo, ref string) ([]byte, string, string, error) {
			return nil, "", "", fmt.Errorf("tag not found")
		},
		blobExistsFunc: func(ctx context.Context, repo, digest string) (bool, error) {
			return false, nil
		},
	}

	svc := setupTestService(taskRepo, regRepo, sourceClient, targetClient)

	created, err := svc.CreateSync(context.Background(), model.SyncRequest{
		SourceRegistryID: 1,
		TargetRegistryID: 2,
		SourceRepo:       "test/repo",
		SourceTag:        "v1",
		TargetRepo:       "mirror/repo",
		TargetTag:        "v1",
	})
	if err != nil {
		t.Fatalf("CreateSync: %v", err)
	}

	err = svc.ExecuteSync(context.Background(), created.ID)
	if err == nil {
		t.Fatal("expected error from failed blob pull")
	}

	// Task should be failed.
	task, _ := taskRepo.GetByID(context.Background(), created.ID)
	if task.Status != model.SyncTaskFailed {
		t.Errorf("expected status failed, got %s", task.Status)
	}
	if task.ErrorMessage == "" {
		t.Error("expected non-empty error message")
	}
}

func TestExecuteSync_Idempotent(t *testing.T) {
	regRepo := newMockRegistryStore()
	regRepo.registries[1] = &model.Registry{ID: 1, URL: "https://source.example.com", Username: "user"}
	regRepo.registries[2] = &model.Registry{ID: 2, URL: "https://target.example.com", Username: "user"}

	taskRepo := newMockSyncTaskRepo()
	rawBody := []byte(testRawManifest)

	sourceClient := &mockRegistryClient{
		rawManifestFunc: func(ctx context.Context, repo, ref string) ([]byte, string, string, error) {
			return rawBody, testDigest, testMediaType, nil
		},
		pullBlobFunc: func(ctx context.Context, repo, digest string, writer io.Writer) error {
			t.Error("pullBlob should not be called for idempotent sync")
			return nil
		},
		pushBlobFunc: func(ctx context.Context, repo string, reader io.Reader, digest string, size int64) error {
			t.Error("pushBlob should not be called for idempotent sync")
			return nil
		},
	}

	// Target already has the same manifest.
	targetClient := &mockRegistryClient{
		rawManifestFunc: func(ctx context.Context, repo, ref string) ([]byte, string, string, error) {
			return rawBody, testDigest, testMediaType, nil
		},
		blobExistsFunc: func(ctx context.Context, repo, digest string) (bool, error) {
			return false, nil
		},
		pushManifestFunc: func(ctx context.Context, repo, tag string, body []byte, mediaType string) error {
			t.Error("pushManifest should not be called for idempotent sync")
			return nil
		},
	}

	svc := setupTestService(taskRepo, regRepo, sourceClient, targetClient)

	created, err := svc.CreateSync(context.Background(), model.SyncRequest{
		SourceRegistryID: 1,
		TargetRegistryID: 2,
		SourceRepo:       "test/repo",
		SourceTag:        "v1",
		TargetRepo:       "mirror/repo",
		TargetTag:        "v1",
	})
	if err != nil {
		t.Fatalf("CreateSync: %v", err)
	}

	if err := svc.ExecuteSync(context.Background(), created.ID); err != nil {
		t.Fatalf("ExecuteSync: %v", err)
	}

	// Task completed without any blob transfers.
	task, _ := taskRepo.GetByID(context.Background(), created.ID)
	if task.Status != model.SyncTaskCompleted {
		t.Errorf("expected completed, got %s", task.Status)
	}
}

func TestCancelSync(t *testing.T) {
	taskRepo := newMockSyncTaskRepo()
	regRepo := newMockRegistryStore()

	svc := &SyncService{taskRepo: taskRepo, registryRepo: regRepo, maxConcurrency: 2}

	// Create a pending task.
	task := &model.SyncTask{
		SourceRegistryID: 1,
		TargetRegistryID: 2,
		SourceRepo:       "test/repo",
		SourceTag:        "v1",
		TargetRepo:       "mirror/repo",
		TargetTag:        "v1",
	}
	id, _ := taskRepo.Create(context.Background(), task)

	if err := svc.CancelSync(context.Background(), id); err != nil {
		t.Fatalf("CancelSync: %v", err)
	}

	result, _ := taskRepo.GetByID(context.Background(), id)
	if result.Status != model.SyncTaskFailed {
		t.Errorf("expected status failed, got %s", result.Status)
	}
	if result.ErrorMessage != "cancelled" {
		t.Errorf("expected error message 'cancelled', got %q", result.ErrorMessage)
	}
}
