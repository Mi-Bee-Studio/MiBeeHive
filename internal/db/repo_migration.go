package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

const migrationTaskColumns = `id, module, old_path, new_path, status, progress, total_files, migrated_files, total_bytes, migrated_bytes, started_at, completed_at, error_message, created_at, updated_at`

// MigrationTask represents a storage migration task.
type MigrationTask struct {
	ID            int64
	Module        string
	OldPath       string
	NewPath       string
	Status        string
	Progress      int
	TotalFiles    int
	MigratedFiles int
	TotalBytes    int64
	MigratedBytes int64
	StartedAt     *time.Time
	CompletedAt   *time.Time
	ErrorMessage  string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// MigrationTaskRepo provides CRUD and status management for storage migration tasks.
type MigrationTaskRepo struct {
	db *sql.DB
}

// NewMigrationTaskRepo creates a new MigrationTaskRepo.
func NewMigrationTaskRepo(db *sql.DB) *MigrationTaskRepo {
	return &MigrationTaskRepo{db: db}
}

// Create inserts a new migration task with status 'pending' and returns the generated ID.
func (r *MigrationTaskRepo) Create(ctx context.Context, task *MigrationTask) (int64, error) {
	query := `INSERT INTO storage_migrations (module, old_path, new_path, status)
	          VALUES (?, ?, ?, ?)`
	result, err := r.db.ExecContext(ctx, query,
		task.Module, task.OldPath, task.NewPath, "pending")
	if err != nil {
		return 0, fmt.Errorf("inserting migration task: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("getting last insert id: %w", err)
	}
	return id, nil
}

// GetByID retrieves a migration task by its ID. Returns nil, nil if not found.
func (r *MigrationTaskRepo) GetByID(ctx context.Context, id int64) (*MigrationTask, error) {
	row := r.db.QueryRowContext(ctx, "SELECT "+migrationTaskColumns+" FROM storage_migrations WHERE id = ?", id)
	task, err := scanMigrationTask(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return task, nil
}

// ListByStatus returns all migration tasks with the given status.
func (r *MigrationTaskRepo) ListByStatus(ctx context.Context, status string) ([]*MigrationTask, error) {
	rows, err := r.db.QueryContext(ctx,
		"SELECT "+migrationTaskColumns+" FROM storage_migrations WHERE status = ? ORDER BY created_at ASC", status)
	if err != nil {
		return nil, fmt.Errorf("listing migration tasks by status %q: %w", status, err)
	}
	defer rows.Close()

	var tasks []*MigrationTask
	for rows.Next() {
		task, err := scanMigrationTask(rows)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}
	return tasks, rows.Err()
}

// UpdateStatus updates the status of a migration task.
func (r *MigrationTaskRepo) UpdateStatus(ctx context.Context, id int64, status string) error {
	_, err := r.db.ExecContext(ctx,
		"UPDATE storage_migrations SET status = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?",
		status, id)
	if err != nil {
		return fmt.Errorf("updating status for migration task %d: %w", id, err)
	}
	return nil
}

// UpdateProgress updates progress counters for a migration task.
func (r *MigrationTaskRepo) UpdateProgress(ctx context.Context, id int64, migratedFiles int, migratedBytes int64, progress int) error {
	_, err := r.db.ExecContext(ctx,
		"UPDATE storage_migrations SET migrated_files = ?, migrated_bytes = ?, progress = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?",
		migratedFiles, migratedBytes, progress, id)
	if err != nil {
		return fmt.Errorf("updating progress for migration task %d: %w", id, err)
	}
	return nil
}

// ListActive returns all migration tasks that are pending or running.
func (r *MigrationTaskRepo) ListActive(ctx context.Context) ([]*MigrationTask, error) {
	rows, err := r.db.QueryContext(ctx,
		"SELECT "+migrationTaskColumns+" FROM storage_migrations WHERE status IN ('pending','running') ORDER BY created_at ASC")
	if err != nil {
		return nil, fmt.Errorf("listing active migration tasks: %w", err)
	}
	defer rows.Close()

	var tasks []*MigrationTask
	for rows.Next() {
		task, err := scanMigrationTask(rows)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}
	return tasks, rows.Err()
}

// UpdateError sets a migration task as failed with an error message.
func (r *MigrationTaskRepo) UpdateError(ctx context.Context, id int64, errMsg string) error {
	_, err := r.db.ExecContext(ctx,
		"UPDATE storage_migrations SET status = 'failed', error_message = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?",
		errMsg, id)
	if err != nil {
		return fmt.Errorf("updating error for migration task %d: %w", id, err)
	}
	return nil
}

// SetStarted sets status to 'running' and records started_at.
func (r *MigrationTaskRepo) SetStarted(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx,
		"UPDATE storage_migrations SET status = 'running', started_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP WHERE id = ?",
		id)
	if err != nil {
		return fmt.Errorf("setting migration task %d as started: %w", id, err)
	}
	return nil
}

// SetCompleted sets status to 'completed' and records completed_at.
func (r *MigrationTaskRepo) SetCompleted(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx,
		"UPDATE storage_migrations SET status = 'completed', completed_at = CURRENT_TIMESTAMP, progress = 100, updated_at = CURRENT_TIMESTAMP WHERE id = ?",
		id)
	if err != nil {
		return fmt.Errorf("setting migration task %d as completed: %w", id, err)
	}
	return nil
}

// UpdateTotals sets total_files and total_bytes for a migration task.
func (r *MigrationTaskRepo) UpdateTotals(ctx context.Context, id int64, totalFiles int, totalBytes int64) error {
	_, err := r.db.ExecContext(ctx,
		"UPDATE storage_migrations SET total_files = ?, total_bytes = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?",
		totalFiles, totalBytes, id)
	if err != nil {
		return fmt.Errorf("updating totals for migration task %d: %w", id, err)
	}
	return nil
}

func scanMigrationTask(s interface{ Scan(dest ...any) error }) (*MigrationTask, error) {
	task := &MigrationTask{}
	var startedAt, completedAt sql.NullTime
	err := s.Scan(
		&task.ID, &task.Module, &task.OldPath, &task.NewPath,
		&task.Status, &task.Progress, &task.TotalFiles, &task.MigratedFiles,
		&task.TotalBytes, &task.MigratedBytes,
		&startedAt, &completedAt, &task.ErrorMessage,
		&task.CreatedAt, &task.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("scanning migration task: %w", err)
	}
	if startedAt.Valid {
		task.StartedAt = &startedAt.Time
	}
	if completedAt.Valid {
		task.CompletedAt = &completedAt.Time
	}
	return task, nil
}
