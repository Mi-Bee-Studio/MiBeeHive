package service

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/Mi-Bee-Studio/mibeehive/internal/db"
	"github.com/Mi-Bee-Studio/mibeehive/internal/model"
	"github.com/Mi-Bee-Studio/mibeehive/internal/registry"
)

// RegistrySyncClient defines the registry V2 operations needed for image sync.
type RegistrySyncClient interface {
	Manifest(ctx context.Context, repo, ref string) (*model.ManifestDetail, error)
	RawManifest(ctx context.Context, repo, ref string) ([]byte, string, string, error)
	PullBlob(ctx context.Context, repo, digest string, writer io.Writer) error
	PushBlob(ctx context.Context, repo string, reader io.Reader, digest string, size int64) error
	BlobExists(ctx context.Context, repo, digest string) (bool, error)
	PushManifest(ctx context.Context, repo, tag string, body []byte, mediaType string) error
}

// syncTaskStore defines the DB operations for sync tasks.
type syncTaskStore interface {
	Create(ctx context.Context, task *model.SyncTask) (int64, error)
	GetByID(ctx context.Context, id int64) (*model.SyncTask, error)
	Start(ctx context.Context, id int64) error
	UpdateProgress(ctx context.Context, id int64, progressBytes, totalBytes int64) error
	Complete(ctx context.Context, id int64) error
	Fail(ctx context.Context, id int64, errMsg string) error
	Cancel(ctx context.Context, id int64) error
	ListByStatus(ctx context.Context, status model.SyncTaskStatus) ([]model.SyncTask, error)
	GetActiveByTarget(ctx context.Context, registryID int64, repo, tag string) (*model.SyncTask, error)
}

// registryStore defines the DB operations for registries needed by sync.
type registryStore interface {
	GetByID(ctx context.Context, id int64) (*model.Registry, error)
	DecryptPassword(ctx context.Context, id int64) (string, error)
}

// SyncService handles cross-registry image synchronization.
// It copies images from one registry to another via temp files,
// transferring each layer blob and the image config before pushing the manifest.
type SyncService struct {
	taskRepo       syncTaskStore
	registryRepo   registryStore
	clientFactory  func(url string, creds *registry.Credentials) (RegistrySyncClient, error)
	maxConcurrency int
}

// NewSyncService creates a new SyncService.
// maxConcurrent limits the number of simultaneous sync operations (default 2 if < 1).
func NewSyncService(taskRepo *db.SyncTaskRepo, registryRepo *db.RegistryRepo, maxConcurrent int) *SyncService {
	if maxConcurrent < 1 {
		maxConcurrent = 2
	}
	return &SyncService{
		taskRepo:       taskRepo,
		registryRepo:   registryRepo,
		clientFactory:  defaultClientFactory,
		maxConcurrency: maxConcurrent,
	}
}

func defaultClientFactory(url string, creds *registry.Credentials) (RegistrySyncClient, error) {
	client, err := registry.NewClient(url, creds)
	if err != nil {
		return nil, err
	}
	return client, nil
}

// CreateSync validates registries and creates a new sync task in pending state.
// Returns an error if source/target registries don't exist or an active duplicate exists.
func (s *SyncService) CreateSync(ctx context.Context, req model.SyncRequest) (*model.SyncTask, error) {
	// Validate source registry exists.
	sourceReg, err := s.registryRepo.GetByID(ctx, req.SourceRegistryID)
	if err != nil {
		return nil, fmt.Errorf("checking source registry: %w", err)
	}
	if sourceReg == nil {
		return nil, fmt.Errorf("source registry %d not found", req.SourceRegistryID)
	}

	// Validate target registry exists.
	targetReg, err := s.registryRepo.GetByID(ctx, req.TargetRegistryID)
	if err != nil {
		return nil, fmt.Errorf("checking target registry: %w", err)
	}
	if targetReg == nil {
		return nil, fmt.Errorf("target registry %d not found", req.TargetRegistryID)
	}

	// Check for active duplicate task.
	active, err := s.taskRepo.GetActiveByTarget(ctx, req.TargetRegistryID, req.TargetRepo, req.TargetTag)
	if err != nil {
		return nil, fmt.Errorf("checking active tasks: %w", err)
	}
	if active != nil {
		return nil, fmt.Errorf("active sync already exists for %s:%s (task %d)", req.TargetRepo, req.TargetTag, active.ID)
	}

	task := &model.SyncTask{
		SourceRegistryID: req.SourceRegistryID,
		TargetRegistryID: req.TargetRegistryID,
		SourceRepo:       req.SourceRepo,
		SourceTag:        req.SourceTag,
		TargetRepo:       req.TargetRepo,
		TargetTag:        req.TargetTag,
	}

	id, err := s.taskRepo.Create(ctx, task)
	if err != nil {
		return nil, fmt.Errorf("creating sync task: %w", err)
	}
	slog.Info("sync task created", "task_id", id, "source_repo", req.SourceRepo, "target_repo", req.TargetRepo)

	// Fetch the full task to return.
	created, err := s.taskRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("fetching created sync task: %w", err)
	}
	return created, nil
}

// ExecuteSync runs a sync task: pulls blobs from source, pushes to target, pushes manifest.
// It transfers each layer blob and the config blob via temp files, then pushes the manifest.
// If the target already has the same manifest, the sync is a no-op (idempotent).
func (s *SyncService) ExecuteSync(ctx context.Context, taskID int64) error {
	// Get task.
	task, err := s.taskRepo.GetByID(ctx, taskID)
	if err != nil {
		return fmt.Errorf("getting sync task: %w", err)
	}
	if task == nil {
		return fmt.Errorf("sync task %d not found", taskID)
	}

	// Start task.
	if err := s.taskRepo.Start(ctx, taskID); err != nil {
		return fmt.Errorf("starting sync task: %w", err)
	}

	// Helper to mark task as failed.
	failTask := func(msg string) {
		if failErr := s.taskRepo.Fail(ctx, taskID, msg); failErr != nil {
			slog.Error("failed to mark task as failed", "task_id", taskID, "error", failErr)
		}
	}

	// Create source and target clients.
	sourceClient, err := s.createClient(ctx, task.SourceRegistryID)
	if err != nil {
		failTask(err.Error())
		return fmt.Errorf("creating source client: %w", err)
	}

	targetClient, err := s.createClient(ctx, task.TargetRegistryID)
	if err != nil {
		failTask(err.Error())
		return fmt.Errorf("creating target client: %w", err)
	}

	// Get raw manifest from source (for push to target and idempotency check).
	rawBody, sourceDigest, mediaType, err := sourceClient.RawManifest(ctx, task.SourceRepo, task.SourceTag)
	if err != nil {
		failTask(err.Error())
		return fmt.Errorf("fetching raw manifest: %w", err)
	}

	// Idempotency check: if target already has the same manifest body, skip sync.
	targetRaw, _, _, targetErr := targetClient.RawManifest(ctx, task.TargetRepo, task.TargetTag)
	if targetErr == nil && bytes.Equal(targetRaw, rawBody) {
		slog.Info("target already has same manifest, skipping sync", "task_id", taskID, "digest", sourceDigest)
		if err := s.taskRepo.Complete(ctx, taskID); err != nil {
			return fmt.Errorf("completing idempotent sync: %w", err)
		}
		return nil
	}

	// Get parsed manifest for layer info.
	manifest, err := sourceClient.Manifest(ctx, task.SourceRepo, task.SourceTag)
	if err != nil {
		failTask(err.Error())
		return fmt.Errorf("fetching source manifest: %w", err)
	}

	// Calculate total size from layers.
	var totalBytes int64
	for _, layer := range manifest.Layers {
		totalBytes += layer.Size
	}

	if err := s.taskRepo.UpdateProgress(ctx, taskID, 0, totalBytes); err != nil {
		slog.Warn("failed to update total progress", "task_id", taskID, "error", err)
	}

	// Transfer each layer blob.
	var bytesDone int64
	for _, layer := range manifest.Layers {
		// Check if blob already exists in target.
		exists, err := targetClient.BlobExists(ctx, task.TargetRepo, layer.Digest)
		if err != nil {
			failTask(fmt.Sprintf("checking blob %s: %v", layer.Digest, err))
			return fmt.Errorf("checking blob existence: %w", err)
		}
		if exists {
			bytesDone += layer.Size
			slog.Debug("blob exists in target, skipping", "digest", layer.Digest)
			if err := s.taskRepo.UpdateProgress(ctx, taskID, bytesDone, totalBytes); err != nil {
				slog.Warn("failed to update progress", "task_id", taskID, "error", err)
			}
			continue
		}

		// Transfer blob via temp file.
		if _, err := s.transferBlob(ctx, sourceClient, targetClient, task.SourceRepo, task.TargetRepo, layer.Digest); err != nil {
			failTask(err.Error())
			return err
		}

		bytesDone += layer.Size
		if err := s.taskRepo.UpdateProgress(ctx, taskID, bytesDone, totalBytes); err != nil {
			slog.Warn("failed to update progress", "task_id", taskID, "error", err)
		}
	}

	// Transfer config blob (required by manifest, typically small).
	if manifest.Config.Digest != "" {
		exists, err := targetClient.BlobExists(ctx, task.TargetRepo, manifest.Config.Digest)
		if err != nil {
			slog.Warn("failed to check config blob existence", "digest", manifest.Config.Digest, "error", err)
		}
		if err == nil && !exists {
			if _, err := s.transferBlob(ctx, sourceClient, targetClient, task.SourceRepo, task.TargetRepo, manifest.Config.Digest); err != nil {
				failTask(err.Error())
				return err
			}
		}
	}

	// Push manifest to target.
	if err := targetClient.PushManifest(ctx, task.TargetRepo, task.TargetTag, rawBody, mediaType); err != nil {
		failTask(fmt.Sprintf("pushing manifest: %v", err))
		return fmt.Errorf("pushing manifest: %w", err)
	}

	// Complete task.
	if err := s.taskRepo.Complete(ctx, taskID); err != nil {
		return fmt.Errorf("completing sync task: %w", err)
	}

	slog.Info("sync completed", "task_id", taskID, "source_digest", sourceDigest)
	return nil
}

// transferBlob copies a single blob from source to target via a temp file.
// The temp file is always cleaned up (closed and removed) on return.
func (s *SyncService) transferBlob(ctx context.Context, source, target RegistrySyncClient, sourceRepo, targetRepo, digest string) (int64, error) {
	tmpFile, err := os.CreateTemp("", "registry-sync-")
	if err != nil {
		return 0, fmt.Errorf("creating temp file: %w", err)
	}
	tmpName := tmpFile.Name()
	defer func() {
		tmpFile.Close()
		os.Remove(tmpName)
	}()

	if err := source.PullBlob(ctx, sourceRepo, digest, tmpFile); err != nil {
		return 0, fmt.Errorf("pulling blob %s: %w", digest, err)
	}

	if _, err := tmpFile.Seek(0, 0); err != nil {
		return 0, fmt.Errorf("seeking temp file: %w", err)
	}

	info, err := tmpFile.Stat()
	if err != nil {
		return 0, fmt.Errorf("stat temp file: %w", err)
	}
	blobSize := info.Size()

	if err := target.PushBlob(ctx, targetRepo, tmpFile, digest, blobSize); err != nil {
		return 0, fmt.Errorf("pushing blob %s: %w", digest, err)
	}

	return blobSize, nil
}

// ListSyncTasks returns sync tasks filtered by status.
// Returns an empty slice (never nil) if no tasks match.
func (s *SyncService) ListSyncTasks(ctx context.Context, status string) ([]model.SyncTask, error) {
	taskStatus := model.SyncTaskStatus(status)
	tasks, err := s.taskRepo.ListByStatus(ctx, taskStatus)
	if err != nil {
		return nil, fmt.Errorf("listing sync tasks: %w", err)
	}
	if tasks == nil {
		tasks = []model.SyncTask{}
	}
	return tasks, nil
}

// GetSyncTask returns a single sync task by ID.
// Returns nil, nil if not found.
func (s *SyncService) GetSyncTask(ctx context.Context, id int64) (*model.SyncTask, error) {
	task, err := s.taskRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("getting sync task: %w", err)
	}
	return task, nil
}

// CancelSync cancels a pending or running sync task.
func (s *SyncService) CancelSync(ctx context.Context, id int64) error {
	if err := s.taskRepo.Cancel(ctx, id); err != nil {
		return fmt.Errorf("cancelling sync task: %w", err)
	}
	slog.Info("sync task cancelled", "task_id", id)
	return nil
}

// createClient creates a registry client for the given registry ID.
func (s *SyncService) createClient(ctx context.Context, registryID int64) (RegistrySyncClient, error) {
	reg, err := s.registryRepo.GetByID(ctx, registryID)
	if err != nil {
		return nil, fmt.Errorf("getting registry %d: %w", registryID, err)
	}
	if reg == nil {
		return nil, fmt.Errorf("registry %d not found", registryID)
	}

	password, err := s.registryRepo.DecryptPassword(ctx, registryID)
	if err != nil {
		return nil, fmt.Errorf("decrypting password for registry %d: %w", registryID, err)
	}

	var creds *registry.Credentials
	if reg.Username != "" {
		creds = &registry.Credentials{Username: reg.Username, Password: password}
	}

	client, err := s.clientFactory(reg.URL, creds)
	if err != nil {
		return nil, fmt.Errorf("creating client for %s: %w", reg.URL, err)
	}
	return client, nil
}
