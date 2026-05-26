package service

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/Mi-Bee-Studio/mibeehive/internal/model"
)

// Task type constants.
const (
	TaskTypeCrawl     = "crawl"
	TaskTypeDownload  = "download"
	TaskTypeBackup    = "backup"
	TaskTypeISOCheck  = "iso_check"
)

// TaskService aggregates background tasks from multiple sources into a unified view.
type TaskService struct {
	db *sql.DB
}

// NewTaskService creates a new TaskService.
func NewTaskService(db *sql.DB) *TaskService {
	return &TaskService{db: db}
}

// GetAllTasks returns combined task list from all sources: crawl schedules, downloads, ISO checks, and backups.
func (s *TaskService) GetAllTasks(ctx context.Context) ([]model.Task, error) {
	var tasks []model.Task

	crawlTasks, err := s.getCrawlTasks(ctx)
	if err != nil {
		return nil, fmt.Errorf("getting crawl tasks: %w", err)
	}
	tasks = append(tasks, crawlTasks...)

	downloadTasks, err := s.getDownloadTasks(ctx)
	if err != nil {
		return nil, fmt.Errorf("getting download tasks: %w", err)
	}
	tasks = append(tasks, downloadTasks...)

	isoTasks, err := s.getISOTasks(ctx)
	if err != nil {
		return nil, fmt.Errorf("getting ISO tasks: %w", err)
	}
	tasks = append(tasks, isoTasks...)

	backupTasks, err := s.getBackupTasks(ctx)
	if err != nil {
		return nil, fmt.Errorf("getting backup tasks: %w", err)
	}
	tasks = append(tasks, backupTasks...)

	return tasks, nil
}

// getCrawlTasks returns enabled projects as scheduled crawl tasks.
func (s *TaskService) getCrawlTasks(ctx context.Context) ([]model.Task, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, display_name, config, last_crawled_at
		 FROM projects WHERE enabled = 1 ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("querying enabled projects: %w", err)
	}
	defer rows.Close()

	var tasks []model.Task
	for rows.Next() {
		var id int64
		var name, displayName, configJSON string
		var lastCrawledAt sql.NullString
		if err := rows.Scan(&id, &name, &displayName, &configJSON, &lastCrawledAt); err != nil {
			return nil, fmt.Errorf("scanning project row: %w", err)
		}

		task := model.Task{
			ID:     fmt.Sprintf("crawl-%d", id),
			Name:   displayName,
			Type:   TaskTypeCrawl,
			Status: "scheduled",
		}

		if lastCrawledAt.Valid {
			task.LastRunAt = lastCrawledAt.String
		}

		tasks = append(tasks, task)
	}
	return tasks, rows.Err()
}

// getDownloadTasks returns pending and downloading files as download tasks.
func (s *TaskService) getDownloadTasks(ctx context.Context) ([]model.Task, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT f.id, f.filename, f.status, f.size_bytes, f.download_url,
		        (SELECT SUM(CASE WHEN f2.status = 'complete' THEN 1 ELSE 0 END) * 100.0 / COUNT(*)
		         FROM files f2 WHERE f2.project_id = f.project_id) AS progress
		 FROM files f
		 WHERE f.status IN ('pending', 'downloading')
		 ORDER BY f.created_at ASC`)
	if err != nil {
		return nil, fmt.Errorf("querying pending files: %w", err)
	}
	defer rows.Close()

	var tasks []model.Task
	for rows.Next() {
		var id int64
		var filename, status, downloadURL string
		var sizeBytes int64
		var progress float64
		if err := rows.Scan(&id, &filename, &status, &sizeBytes, &downloadURL, &progress); err != nil {
			return nil, fmt.Errorf("scanning file row: %w", err)
		}

		task := model.Task{
			ID:       fmt.Sprintf("download-%d", id),
			Name:     filename,
			Type:     TaskTypeDownload,
			Status:   status,
			Progress: progress,
		}

		tasks = append(tasks, task)
	}
	return tasks, rows.Err()
}

// getISOTasks returns enabled ISO catalog entries with auto_update as ISO check tasks.
func (s *TaskService) getISOTasks(ctx context.Context) ([]model.Task, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, check_interval_hours, last_checked, last_error, status, download_status
		 FROM iso_catalog
		 WHERE auto_update = 1
		 ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("querying ISO catalog: %w", err)
	}
	defer rows.Close()

	var tasks []model.Task
	for rows.Next() {
		var id int64
		var name string
		var checkIntervalHours int
		var lastChecked, lastError sql.NullString
		var status, downloadStatus string
		if err := rows.Scan(&id, &name, &checkIntervalHours, &lastChecked, &lastError, &status, &downloadStatus); err != nil {
			return nil, fmt.Errorf("scanning ISO catalog row: %w", err)
		}

		task := model.Task{
			ID:       fmt.Sprintf("iso-%d", id),
			Name:     name,
			Type:     TaskTypeISOCheck,
			Status:   status,
			Schedule: fmt.Sprintf("every %dh", checkIntervalHours),
		}

		if lastChecked.Valid {
			task.LastRunAt = lastChecked.String
		}
		if lastError.Valid && lastError.String != "" {
			task.LastResult = lastError.String
		}
		if downloadStatus != "" {
			task.LastResult = downloadStatus
		}

		tasks = append(tasks, task)
	}
	return tasks, rows.Err()
}

// getBackupTasks returns backup task placeholder.
// Backup scheduling requires external config not yet modeled in DB.
func (s *TaskService) getBackupTasks(_ context.Context) ([]model.Task, error) {
	// Placeholder: backup service uses external config, not DB.
	// Will be populated when backup scheduling is implemented.
	return nil, nil
}
