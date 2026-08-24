package db

import (
	"context"
	"database/sql"
	"fmt"
)

// CrawlLogRepo provides CRUD operations for crawl logs.
type CrawlLogRepo struct {
	db *sql.DB
}

// NewCrawlLogRepo creates a new CrawlLogRepo.
func NewCrawlLogRepo(db *sql.DB) *CrawlLogRepo {
	return &CrawlLogRepo{db: db}
}

// Create inserts a new crawl log and returns the generated ID.
func (r *CrawlLogRepo) Create(ctx context.Context, log *CrawlLog) (int64, error) {
	query := `INSERT INTO crawl_logs (project_id, started_at, status)
	          VALUES (?, ?, ?)`
	result, err := r.db.ExecContext(ctx, query, log.ProjectID, log.StartedAt, log.Status)
	if err != nil {
		return 0, fmt.Errorf("inserting crawl log: %w", err)
	}
	return result.LastInsertId()
}

// UpdateFinished marks a crawl log as finished with results.
func (r *CrawlLogRepo) UpdateFinished(ctx context.Context, id int64, status string, versionsFound, filesDownloaded int, errorMsg string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE crawl_logs
		SET finished_at = CURRENT_TIMESTAMP, status = ?,
		    versions_found = ?, files_downloaded = ?, error_message = ?
		WHERE id = ?`, status, versionsFound, filesDownloaded, errorMsg, id)
	if err != nil {
		return fmt.Errorf("finishing crawl log %d: %w", id, err)
	}
	return nil
}

// ListByProject returns recent crawl logs for a project, most recent first.
func (r *CrawlLogRepo) ListByProject(ctx context.Context, projectID int64, limit int) ([]*CrawlLog, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := r.db.QueryContext(ctx,
		"SELECT id, project_id, started_at, finished_at, status, versions_found, files_downloaded, error_message FROM crawl_logs WHERE project_id = ? ORDER BY id DESC LIMIT ?",
		projectID, limit)
	if err != nil {
		return nil, fmt.Errorf("listing crawl logs for project %d: %w", projectID, err)
	}
	defer rows.Close()

	var logs []*CrawlLog
	for rows.Next() {
		l := &CrawlLog{}
		err := rows.Scan(
			&l.ID, &l.ProjectID, &l.StartedAt, &l.FinishedAt,
			&l.Status, &l.VersionsFound, &l.FilesDownloaded, &l.ErrorMessage,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning crawl log: %w", err)
		}
		logs = append(logs, l)
	}
	return logs, rows.Err()
}

// ListRecent returns the most recent crawl logs across all projects.
func (r *CrawlLogRepo) ListRecent(ctx context.Context, limit int) ([]*CrawlLog, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	rows, err := r.db.QueryContext(ctx,
		"SELECT id, project_id, started_at, finished_at, status, versions_found, files_downloaded, error_message FROM crawl_logs ORDER BY started_at DESC LIMIT ?", limit)
	if err != nil {
		return nil, fmt.Errorf("listing recent crawl logs: %w", err)
	}
	defer rows.Close()
	var logs []*CrawlLog
	for rows.Next() {
		l := &CrawlLog{}
		err := rows.Scan(&l.ID, &l.ProjectID, &l.StartedAt, &l.FinishedAt, &l.Status, &l.VersionsFound, &l.FilesDownloaded, &l.ErrorMessage)
		if err != nil {
			return nil, fmt.Errorf("scanning crawl log: %w", err)
		}
		logs = append(logs, l)
	}
	return logs, rows.Err()
}

// LatestPerProject returns the most recent crawl log for each project,
// keyed by project ID. Used to surface per-project crawl health (last
// status / last error) in the UI instead of a silent "0 files, never".
func (r *CrawlLogRepo) LatestPerProject(ctx context.Context) (map[int64]*CrawlLog, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT l.id, l.project_id, l.started_at, l.finished_at, l.status, l.versions_found, l.files_downloaded, l.error_message
		 FROM crawl_logs l
		 JOIN (SELECT project_id, MAX(id) AS max_id FROM crawl_logs GROUP BY project_id) m
		   ON l.id = m.max_id`)
	if err != nil {
		return nil, fmt.Errorf("listing latest crawl logs per project: %w", err)
	}
	defer rows.Close()

	logs := make(map[int64]*CrawlLog)
	for rows.Next() {
		l := &CrawlLog{}
		if err := rows.Scan(&l.ID, &l.ProjectID, &l.StartedAt, &l.FinishedAt, &l.Status, &l.VersionsFound, &l.FilesDownloaded, &l.ErrorMessage); err != nil {
			return nil, fmt.Errorf("scanning latest crawl log: %w", err)
		}
		logs[l.ProjectID] = l
	}
	return logs, rows.Err()
}
