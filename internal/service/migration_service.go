package service

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"

	"github.com/Mi-Bee-Studio/mibeehive/internal/db"
	"github.com/Mi-Bee-Studio/mibeehive/internal/diskutil"
)

// MigrationService handles background file migration between storage paths.
// It copies files from old path to new path using streaming I/O, updates DB
// records atomically per-file, and tracks progress.
type MigrationService struct {
	repo     *db.MigrationTaskRepo
	fileRepo *db.FileRepo
	logger   *slog.Logger
	mu       sync.Mutex
	active   sync.Map // taskID (int64) → context.CancelFunc
}

// NewMigrationService creates a new MigrationService.
func NewMigrationService(
	repo *db.MigrationTaskRepo,
	fileRepo *db.FileRepo,
	logger *slog.Logger,
) *MigrationService {
	return &MigrationService{
		repo:     repo,
		fileRepo: fileRepo,
		logger:   logger,
	}
}

// EnqueueMigration validates parameters, checks disk space, and creates a pending migration task.
func (s *MigrationService) EnqueueMigration(ctx context.Context, module, oldPath, newPath string) (int64, error) {
	if oldPath == "" || newPath == "" {
		return 0, fmt.Errorf("old_path and new_path must be non-empty")
	}
	if oldPath == newPath {
		return 0, fmt.Errorf("old_path and new_path must be different")
	}

	var totalBytes int64
	err := filepath.Walk(oldPath, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			totalBytes += info.Size()
		}
		return nil
	})
	if err != nil {
		return 0, fmt.Errorf("calculating source size: %w", err)
	}

	if err := s.checkDiskSpace(newPath, totalBytes); err != nil {
		return 0, fmt.Errorf("disk space check: %w", err)
	}

	task := &db.MigrationTask{
		Module:  module,
		OldPath: oldPath,
		NewPath: newPath,
	}
	id, err := s.repo.Create(ctx, task)
	if err != nil {
		return 0, fmt.Errorf("creating migration task: %w", err)
	}

	s.logger.Info("migration task enqueued",
		"task_id", id,
		"module", module,
		"old_path", oldPath,
		"new_path", newPath,
		"total_bytes", totalBytes,
	)
	return id, nil
}

// StartMigration begins a background migration for the given task.
// It returns immediately; the migration runs in a background goroutine.
func (s *MigrationService) StartMigration(ctx context.Context, taskID int64) error {
	task, err := s.repo.GetByID(ctx, taskID)
	if err != nil {
		return fmt.Errorf("getting migration task: %w", err)
	}
	if task == nil {
		return fmt.Errorf("migration task %d not found", taskID)
	}
	if task.Status != "pending" {
		return fmt.Errorf("migration task %d has status %q, expected 'pending'", taskID, task.Status)
	}

	if err := s.repo.SetStarted(ctx, taskID); err != nil {
		return fmt.Errorf("starting migration task: %w", err)
	}

	migCtx, cancel := context.WithCancel(context.Background())

	s.mu.Lock()
	s.active.Store(taskID, cancel)
	s.mu.Unlock()

	go s.runMigration(migCtx, taskID, task.OldPath, task.NewPath)

	s.logger.Info("migration started", "task_id", taskID)
	return nil
}

// CancelMigration cancels an active migration task.
func (s *MigrationService) CancelMigration(taskID int64) error {
	val, ok := s.active.Load(taskID)
	if !ok {
		return fmt.Errorf("no active migration for task %d", taskID)
	}

	cancel := val.(context.CancelFunc)
	cancel()
	s.active.Delete(taskID)

	ctx := context.Background()
	if err := s.repo.UpdateStatus(ctx, taskID, "cancelled"); err != nil {
		return fmt.Errorf("cancelling migration task %d: %w", taskID, err)
	}

	s.logger.Info("migration cancelled", "task_id", taskID)
	return nil
}

// GetProgress returns the current state of a migration task.
func (s *MigrationService) GetProgress(ctx context.Context, taskID int64) (*db.MigrationTask, error) {
	task, err := s.repo.GetByID(ctx, taskID)
	if err != nil {
		return nil, fmt.Errorf("getting migration progress: %w", err)
	}
	return task, nil
}

// ListActive returns all pending and running migration tasks.
func (s *MigrationService) ListActive(ctx context.Context) ([]*db.MigrationTask, error) {
	return s.repo.ListActive(ctx)
}

// ResetStaleMigrations resets any 'running' tasks back to 'pending' for retry.
// Call this on startup to handle interrupted migrations.
func (s *MigrationService) ResetStaleMigrations(ctx context.Context) error {
	running, err := s.repo.ListByStatus(ctx, "running")
	if err != nil {
		return fmt.Errorf("listing stale migrations: %w", err)
	}

	for _, task := range running {
		if err := s.repo.UpdateStatus(ctx, task.ID, "pending"); err != nil {
			s.logger.Error("failed to reset stale migration",
				"task_id", task.ID,
				"error", err,
			)
			continue
		}
		s.logger.Info("reset stale migration to pending", "task_id", task.ID)
	}
	return nil
}

// runMigration is the background goroutine that performs the actual file migration.
func (s *MigrationService) runMigration(ctx context.Context, taskID int64, oldPath, newPath string) {
	defer func() {
		s.active.Delete(taskID)
	}()

	bgCtx := context.Background()

	// Phase 1: Walk old directory to count files and total bytes.
	type fileEntry struct {
		relPath string
		size    int64
	}
	var files []fileEntry
	var totalBytes int64

	err := filepath.Walk(oldPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(oldPath, path)
		if err != nil {
			return fmt.Errorf("getting relative path: %w", err)
		}
		files = append(files, fileEntry{relPath: rel, size: info.Size()})
		totalBytes += info.Size()
		return nil
	})
	if err != nil {
		s.failTask(bgCtx, taskID, fmt.Sprintf("walking source directory: %v", err))
		return
	}

	if err := s.repo.UpdateTotals(bgCtx, taskID, len(files), totalBytes); err != nil {
		s.logger.Error("failed to update totals", "task_id", taskID, "error", err)
	}

	s.logger.Info("migration scan complete",
		"task_id", taskID,
		"total_files", len(files),
		"total_bytes", totalBytes,
	)

	// Phase 2: Copy each file atomically.
	var migratedFiles int
	var migratedBytes int64

	for _, f := range files {
		select {
		case <-ctx.Done():
			s.logger.Info("migration cancelled", "task_id", taskID)
			return
		default:
		}

		src := filepath.Join(oldPath, f.relPath)
		dst := filepath.Join(newPath, f.relPath)

		if err := s.copyFile(src, dst); err != nil {
			s.failTask(bgCtx, taskID, fmt.Sprintf("copying %s: %v", f.relPath, err))
			return
		}

		srcInfo, err := os.Stat(src)
		if err != nil {
			s.failTask(bgCtx, taskID, fmt.Sprintf("stat source %s: %v", f.relPath, err))
			return
		}
		dstInfo, err := os.Stat(dst)
		if err != nil {
			s.failTask(bgCtx, taskID, fmt.Sprintf("stat destination %s: %v", f.relPath, err))
			return
		}
		if srcInfo.Size() != dstInfo.Size() {
			os.Remove(dst)
			s.failTask(bgCtx, taskID, fmt.Sprintf("size mismatch for %s: src=%d dst=%d", f.relPath, srcInfo.Size(), dstInfo.Size()))
			return
		}

		// Step c: Find file record in DB by local_path and update.
		dbFile, err := s.fileRepo.FindByLocalPath(bgCtx, src)
		if err != nil {
			s.logger.Warn("failed to find file record in DB",
				"task_id", taskID,
				"path", src,
				"error", err,
			)
			// Non-fatal: file may not be tracked in DB (e.g. ISO images).
		} else if dbFile != nil {
			if err := s.fileRepo.UpdateLocalPath(bgCtx, dbFile.ID, dst); err != nil {
				s.failTask(bgCtx, taskID, fmt.Sprintf("updating DB path for %s: %v", f.relPath, err))
				return
			}
		}

		// Step d: Delete source file (only after successful copy + DB update).
		if err := os.Remove(src); err != nil {
			s.logger.Warn("failed to delete source file",
				"task_id", taskID,
				"path", src,
				"error", err,
			)
			// Non-fatal: migration can continue even if source deletion fails.
		}

		// Step e: Update progress.
		migratedFiles++
		migratedBytes += f.size
		progress := 0
		if totalBytes > 0 {
			progress = int(float64(migratedBytes) / float64(totalBytes) * 100)
		}
		if err := s.repo.UpdateProgress(bgCtx, taskID, migratedFiles, migratedBytes, progress); err != nil {
			s.logger.Warn("failed to update progress",
				"task_id", taskID,
				"error", err,
			)
		}
	}

	// Phase 3: Mark completed.
	if err := s.repo.SetCompleted(bgCtx, taskID); err != nil {
		s.logger.Error("failed to mark migration as completed",
			"task_id", taskID,
			"error", err,
		)
		return
	}

	s.logger.Info("migration completed",
		"task_id", taskID,
		"migrated_files", migratedFiles,
		"migrated_bytes", migratedBytes,
	)
}

// failTask marks a migration task as failed with the given error message.
func (s *MigrationService) failTask(ctx context.Context, taskID int64, errMsg string) {
	s.logger.Error("migration failed", "task_id", taskID, "error", errMsg)
	if err := s.repo.UpdateError(ctx, taskID, errMsg); err != nil {
		s.logger.Error("failed to update error for migration task",
			"task_id", taskID,
			"error", err,
		)
	}
}

// copyFile copies src to dst using io.Copy (memory-efficient streaming).
func (s *MigrationService) copyFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return fmt.Errorf("creating directory: %w", err)
	}

	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("opening source: %w", err)
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("creating destination: %w", err)
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return fmt.Errorf("copying %s to %s: %w", src, dst, err)
	}

	return out.Sync()
}

// checkDiskSpace verifies that the filesystem has at least 110% of the
// required bytes available (10% safety margin).
func (s *MigrationService) checkDiskSpace(path string, requiredBytes int64) error {
	if err := os.MkdirAll(path, 0755); err != nil {
		return fmt.Errorf("creating path for disk check: %w", err)
	}

	_, _, available, err := diskutil.Usage(path)
	if err != nil {
		return fmt.Errorf("checking disk space: %w", err)
	}

	needed := int64(float64(requiredBytes) * 1.1)
	if int64(available) < needed {
		return fmt.Errorf("insufficient disk space: need %d bytes, have %d available", needed, available)
	}
	return nil
}
