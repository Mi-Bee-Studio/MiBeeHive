package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/Mi-Bee-Studio/mibeehive/internal/model"
)

const syncTaskColumns = `id, source_registry_id, target_registry_id, source_repo, source_tag, target_repo, target_tag, status, progress_bytes, total_bytes, error_message, created_at, updated_at`

// SyncTaskRepo provides CRUD and status transition operations for sync tasks.
type SyncTaskRepo struct {
	db *sql.DB
}

// NewSyncTaskRepo creates a new SyncTaskRepo.
func NewSyncTaskRepo(db *sql.DB) *SyncTaskRepo {
	return &SyncTaskRepo{db: db}
}

// Create inserts a new sync task with status 'pending' and returns the generated ID.
func (r *SyncTaskRepo) Create(ctx context.Context, task *model.SyncTask) (int64, error) {
	query := `INSERT INTO sync_tasks (source_registry_id, target_registry_id, source_repo, source_tag,
	          target_repo, target_tag, status)
	          VALUES (?, ?, ?, ?, ?, ?, ?)`
	result, err := r.db.ExecContext(ctx, query,
		task.SourceRegistryID, task.TargetRegistryID,
		task.SourceRepo, task.SourceTag, task.TargetRepo, task.TargetTag,
		model.SyncTaskPending)
	if err != nil {
		return 0, fmt.Errorf("inserting sync task: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("getting last insert id: %w", err)
	}
	return id, nil
}

// GetByID retrieves a sync task by its ID. Returns nil, nil if not found.
func (r *SyncTaskRepo) GetByID(ctx context.Context, id int64) (*model.SyncTask, error) {
	row := r.db.QueryRowContext(ctx, "SELECT "+syncTaskColumns+" FROM sync_tasks WHERE id = ?", id)
	task, err := scanSyncTask(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return task, nil
}

// ListByStatus returns all sync tasks with the given status.
func (r *SyncTaskRepo) ListByStatus(ctx context.Context, status model.SyncTaskStatus) ([]model.SyncTask, error) {
	rows, err := r.db.QueryContext(ctx,
		"SELECT "+syncTaskColumns+" FROM sync_tasks WHERE status = ? ORDER BY created_at ASC", status)
	if err != nil {
		return nil, fmt.Errorf("listing sync tasks by status %q: %w", status, err)
	}
	defer rows.Close()

	var tasks []model.SyncTask
	for rows.Next() {
		task, err := scanSyncTask(rows)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, *task)
	}
	return tasks, rows.Err()
}

// GetActiveByTarget finds an active (pending or running) task matching the target.
// Returns nil, nil if no active task found.
func (r *SyncTaskRepo) GetActiveByTarget(ctx context.Context, registryID int64, repo, tag string) (*model.SyncTask, error) {
	query := "SELECT " + syncTaskColumns +
		" FROM sync_tasks WHERE target_registry_id = ? AND target_repo = ? AND target_tag = ?" +
		" AND status IN ('pending', 'running') LIMIT 1"
	row := r.db.QueryRowContext(ctx, query, registryID, repo, tag)
	task, err := scanSyncTask(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return task, nil
}

// Start transitions a task from pending to running.
func (r *SyncTaskRepo) Start(ctx context.Context, id int64) error {
	return r.transitionStatus(ctx, id, model.SyncTaskPending, model.SyncTaskRunning, "")
}

// UpdateProgress updates progress_bytes and total_bytes for a running task.
func (r *SyncTaskRepo) UpdateProgress(ctx context.Context, id int64, progressBytes, totalBytes int64) error {
	_, err := r.db.ExecContext(ctx,
		"UPDATE sync_tasks SET progress_bytes = ?, total_bytes = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?",
		progressBytes, totalBytes, id)
	if err != nil {
		return fmt.Errorf("updating progress for sync task %d: %w", id, err)
	}
	return nil
}

// Complete transitions a task from running to completed.
func (r *SyncTaskRepo) Complete(ctx context.Context, id int64) error {
	return r.transitionStatus(ctx, id, model.SyncTaskRunning, model.SyncTaskCompleted, "")
}

// Fail transitions a task from running or pending to failed with an error message.
func (r *SyncTaskRepo) Fail(ctx context.Context, id int64, errMsg string) error {
	// Check current status.
	var currentStatus string
	err := r.db.QueryRowContext(ctx, "SELECT status FROM sync_tasks WHERE id = ?", id).Scan(&currentStatus)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("sync task %d not found", id)
		}
		return fmt.Errorf("checking status for sync task %d: %w", id, err)
	}

	if currentStatus != string(model.SyncTaskRunning) && currentStatus != string(model.SyncTaskPending) {
		return fmt.Errorf("cannot fail task %d in status %q (must be pending or running)", id, currentStatus)
	}

	_, err = r.db.ExecContext(ctx,
		"UPDATE sync_tasks SET status = ?, error_message = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?",
		model.SyncTaskFailed, errMsg, id)
	if err != nil {
		return fmt.Errorf("failing sync task %d: %w", id, err)
	}
	return nil
}

// Cancel transitions a task from pending or running to failed with "cancelled" message.
func (r *SyncTaskRepo) Cancel(ctx context.Context, id int64) error {
	return r.Fail(ctx, id, "cancelled")
}

// transitionStatus validates and executes a status transition.
func (r *SyncTaskRepo) transitionStatus(ctx context.Context, id int64, from, to model.SyncTaskStatus, errMsg string) error {
	result, err := r.db.ExecContext(ctx,
		"UPDATE sync_tasks SET status = ?, error_message = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ? AND status = ?",
		to, errMsg, id, from)
	if err != nil {
		return fmt.Errorf("transitioning sync task %d from %s to %s: %w", id, from, to, err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking rows affected for sync task %d: %w", id, err)
	}
	if affected == 0 {
		return fmt.Errorf("sync task %d not in expected status %q for transition to %q", id, from, to)
	}
	return nil
}

func scanSyncTask(s interface{ Scan(dest ...any) error }) (*model.SyncTask, error) {
	var task model.SyncTask
	var status string
	err := s.Scan(
		&task.ID, &task.SourceRegistryID, &task.TargetRegistryID,
		&task.SourceRepo, &task.SourceTag, &task.TargetRepo, &task.TargetTag,
		&status, &task.ProgressBytes, &task.TotalBytes, &task.ErrorMessage,
		&task.CreatedAt, &task.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("scanning sync task: %w", err)
	}
	task.Status = model.SyncTaskStatus(status)
	return &task, nil
}
