package service

import (
	"context"
	"log/slog"
	"time"

	"github.com/Mi-Bee-Studio/mibeehive/internal/db"
)

// migrationAdapter wraps *MigrationService to satisfy the handler's MigrationService interface.
// It bridges the gap between the handler's simplified interface (no context params,
// MigrationTaskInfo returns) and the concrete service's full signatures.
type migrationAdapter struct {
	svc    *MigrationService
	logger *slog.Logger
}

// NewMigrationAdapter creates an adapter that wraps *MigrationService to satisfy
// the handler's MigrationService interface.
func NewMigrationAdapter(svc *MigrationService, logger *slog.Logger) *migrationAdapter {
	return &migrationAdapter{svc: svc, logger: logger}
}

// Enqueue enqueues a migration and immediately starts it in the background.
func (a *migrationAdapter) Enqueue(module, oldPath, newPath string) (int64, error) {
	ctx := context.Background()
	id, err := a.svc.EnqueueMigration(ctx, module, oldPath, newPath)
	if err != nil {
		return 0, err
	}
	if err := a.svc.StartMigration(ctx, id); err != nil {
		a.logger.Warn("failed to auto-start migration", "task_id", id, "error", err)
	}
	return id, nil
}

// List returns all active migration tasks as MigrationTaskInfo.
func (a *migrationAdapter) List() ([]MigrationTaskInfo, error) {
	tasks, err := a.svc.ListActive(context.Background())
	if err != nil {
		return nil, err
	}
	result := make([]MigrationTaskInfo, 0, len(tasks))
	for _, t := range tasks {
		result = append(result, taskToInfo(t))
	}
	return result, nil
}

// Get returns a single migration task by ID.
func (a *migrationAdapter) Get(id int64) (*MigrationTaskInfo, error) {
	task, err := a.svc.GetProgress(context.Background(), id)
	if err != nil {
		return nil, err
	}
	if task == nil {
		return nil, nil
	}
	info := taskToInfo(task)
	return &info, nil
}

// Cancel cancels an active migration task.
func (a *migrationAdapter) Cancel(id int64) error {
	return a.svc.CancelMigration(id)
}

// taskToInfo converts a db.MigrationTask to a MigrationTaskInfo.
func taskToInfo(t *db.MigrationTask) MigrationTaskInfo {
	info := MigrationTaskInfo{
		ID:            t.ID,
		Module:        t.Module,
		OldPath:       t.OldPath,
		NewPath:       t.NewPath,
		Status:        t.Status,
		Progress:      t.Progress,
		TotalFiles:    t.TotalFiles,
		MigratedFiles: t.MigratedFiles,
		TotalBytes:    t.TotalBytes,
		MigratedBytes: t.MigratedBytes,
		ErrorMessage:  t.ErrorMessage,
		CreatedAt:     t.CreatedAt.Format(time.RFC3339),
	}
	if t.StartedAt != nil {
		s := t.StartedAt.Format(time.RFC3339)
		info.StartedAt = &s
	}
	if t.CompletedAt != nil {
		s := t.CompletedAt.Format(time.RFC3339)
		info.CompletedAt = &s
	}
	return info
}
