package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const fileColumns = `id, project_id, version, filename, os, arch, ext, size_bytes, download_url, local_path, checksum, status, error_message, retry_count, last_attempt_at, created_at`

// FileRepo provides CRUD operations for files.
type FileRepo struct {
	db *sql.DB
}

// NewFileRepo creates a new FileRepo.
func NewFileRepo(db *sql.DB) *FileRepo {
	return &FileRepo{db: db}
}

// Create inserts a new file and returns the generated ID.
func (r *FileRepo) Create(ctx context.Context, f *File) (int64, error) {
	query := `INSERT INTO files (project_id, version, filename, os, arch, ext,
	          size_bytes, download_url, local_path, checksum, status)
	          VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	result, err := r.db.ExecContext(ctx, query,
		f.ProjectID, f.Version, f.Filename, f.OS, f.Arch, f.Ext,
		f.SizeBytes, f.DownloadURL, f.LocalPath, f.Checksum, f.Status)
	if err != nil {
		return 0, fmt.Errorf("inserting file %q: %w", f.Filename, err)
	}
	return result.LastInsertId()
}

// GetByID retrieves a file by its ID.
func (r *FileRepo) GetByID(ctx context.Context, id int64) (*File, error) {
	row := r.db.QueryRowContext(ctx, "SELECT "+fileColumns+" FROM files WHERE id = ?", id)
	f, err := scanFileRow(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return f, nil
}

// ListByProject returns all files for a project ordered by version desc.
func (r *FileRepo) ListByProject(ctx context.Context, projectID int64) ([]*File, error) {
	rows, err := r.db.QueryContext(ctx,
		"SELECT "+fileColumns+" FROM files WHERE project_id = ? ORDER BY version DESC, filename", projectID)
	if err != nil {
		return nil, fmt.Errorf("listing files for project %d: %w", projectID, err)
	}
	defer rows.Close()

	var files []*File
	for rows.Next() {
		f, err := scanFile(rows)
		if err != nil {
			return nil, err
		}
		files = append(files, f)
	}
	return files, rows.Err()
}

// ListComplete returns all files whose status is "complete" (downloaded and
// verified), i.e. the files that are actually servable to external clients.
// Used by the supply layer to build a repository index. Results are ordered by
// filename for a stable listing. Pass a projectID of 0 to list across all
// projects.
func (r *FileRepo) ListComplete(ctx context.Context, projectID int64) ([]*File, error) {
	q := "SELECT " + fileColumns + " FROM files WHERE status = ?"
	args := []any{"complete"}
	if projectID > 0 {
		q += " AND project_id = ?"
		args = append(args, projectID)
	}
	q += " ORDER BY filename"
	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("listing complete files: %w", err)
	}
	defer rows.Close()

	var files []*File
	for rows.Next() {
		f, err := scanFile(rows)
		if err != nil {
			return nil, err
		}
		files = append(files, f)
	}
	return files, rows.Err()
}

// ListByProjectPaginated returns files for a project with pagination.
func (r *FileRepo) ListByProjectPaginated(ctx context.Context, projectID int64, limit, offset int) ([]*File, int, error) {
	// Count total files for the project.
	var total int
	err := r.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM files WHERE project_id = ?", projectID).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("counting files for project %d: %w", projectID, err)
	}

	rows, err := r.db.QueryContext(ctx,
		"SELECT "+fileColumns+" FROM files WHERE project_id = ? ORDER BY version DESC, filename LIMIT ? OFFSET ?",
		projectID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("listing files for project %d: %w", projectID, err)
	}
	defer rows.Close()

	var files []*File
	for rows.Next() {
		f, err := scanFile(rows)
		if err != nil {
			return nil, 0, err
		}
		files = append(files, f)
	}
	return files, total, rows.Err()
}

// FindExisting checks if a file already exists for a project+filename combination.
func (r *FileRepo) FindExisting(ctx context.Context, projectID int64, filename string) (*File, error) {
	row := r.db.QueryRowContext(ctx,
		"SELECT "+fileColumns+" FROM files WHERE project_id = ? AND filename = ?", projectID, filename)
	f, err := scanFileRow(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return f, nil
}

// UpdateStatus changes a file's status and optional error message.
func (r *FileRepo) UpdateStatus(ctx context.Context, id int64, status, errorMsg string) error {
	_, err := r.db.ExecContext(ctx,
		"UPDATE files SET status = ?, error_message = ? WHERE id = ?", status, errorMsg, id)
	if err != nil {
		return fmt.Errorf("updating status for file %d: %w", id, err)
	}
	return nil
}

// CountByProject returns the number of files for a project.
func (r *FileRepo) CountByProject(ctx context.Context, projectID int64) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM files WHERE project_id = ?", projectID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("counting files for project %d: %w", projectID, err)
	}
	return count, nil
}

// CountByProjects returns a map of project_id → file count for all projects.
func (r *FileRepo) CountByProjects(ctx context.Context) (map[int64]int, error) {
	rows, err := r.db.QueryContext(ctx, "SELECT project_id, COUNT(*) as cnt FROM files GROUP BY project_id")
	if err != nil {
		return nil, fmt.Errorf("counting files by project: %w", err)
	}
	defer rows.Close()

	counts := make(map[int64]int)
	for rows.Next() {
		var projectID int64
		var count int
		if err := rows.Scan(&projectID, &count); err != nil {
			return nil, fmt.Errorf("scanning file count: %w", err)
		}
		counts[projectID] = count
	}
	return counts, rows.Err()
}

// SearchByFilename returns files whose filename matches the given SQL LIKE pattern.
func (r *FileRepo) SearchByFilename(ctx context.Context, pattern string) ([]*File, error) {
	rows, err := r.db.QueryContext(ctx,
		"SELECT "+fileColumns+" FROM files WHERE filename LIKE ? ORDER BY filename LIMIT 100", pattern)
	if err != nil {
		return nil, fmt.Errorf("searching files by filename: %w", err)
	}
	defer rows.Close()

	var files []*File
	for rows.Next() {
		f, err := scanFile(rows)
		if err != nil {
			return nil, err
		}
		files = append(files, f)
	}
	return files, rows.Err()
}

// CountAll returns the total number of files.
func (r *FileRepo) CountAll(ctx context.Context) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM files").Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("counting all files: %w", err)
	}
	return count, nil
}

func scanFile(rows *sql.Rows) (*File, error) {
	return scanFileFromScanner(rows)
}

func scanFileRow(row *sql.Row) (*File, error) {
	return scanFileFromScanner(row)
}

func scanFileFromScanner(s interface{ Scan(dest ...any) error }) (*File, error) {
	f := &File{}
	err := s.Scan(
		&f.ID, &f.ProjectID, &f.Version, &f.Filename, &f.OS, &f.Arch, &f.Ext,
		&f.SizeBytes, &f.DownloadURL, &f.LocalPath, &f.Checksum,
		&f.Status, &f.ErrorMessage, &f.RetryCount, &f.LastAttemptAt, &f.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("scanning file: %w", err)
	}
	return f, nil
}

// ListQueue returns files with status pending or downloading, ordered by created_at ASC.
func (r *FileRepo) ListQueue(ctx context.Context) ([]*File, error) {
	rows, err := r.db.QueryContext(ctx,
		"SELECT "+fileColumns+" FROM files WHERE status IN ('pending', 'downloading') ORDER BY created_at ASC LIMIT 50")
	if err != nil {
		return nil, fmt.Errorf("listing file queue: %w", err)
	}
	defer rows.Close()

	var files []*File
	for rows.Next() {
		f, err := scanFile(rows)
		if err != nil {
			return nil, err
		}
		files = append(files, f)
	}
	return files, rows.Err()
}

// ResetZombieDownloads resets downloading→pending where file doesn't exist on disk.
// If the file already exists (download completed but status wasn't updated), it marks as complete.
// It also cleans up orphaned temp files (.download-*) from interrupted downloads.
func (r *FileRepo) ResetZombieDownloads(ctx context.Context) (int, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, local_path FROM files WHERE status = 'downloading'`)
	if err != nil {
		return 0, fmt.Errorf("querying zombie downloads: %w", err)
	}

	// Collect all zombie IDs first to avoid deadlock with MaxOpenConns(1).
	type zombie struct {
		id   int64
		path string
	}
	var zombies []zombie
	for rows.Next() {
		var z zombie
		if err := rows.Scan(&z.id, &z.path); err != nil {
			continue
		}
		zombies = append(zombies, z)
	}
	rows.Close()

	var resetCount int
	for _, z := range zombies {
		if info, err := os.Stat(z.path); err == nil && info.Size() > 0 {
			// File exists with content — download completed but status wasn't updated.
			if _, err := r.db.ExecContext(ctx,
				`UPDATE files SET status = 'complete', error_message = '', size_bytes = ? WHERE id = ?`, info.Size(), z.id); err == nil {
				resetCount++
			}
		} else {
			// File doesn't exist — reset to pending for retry.
			dir := filepath.Dir(z.path)
			filename := filepath.Base(z.path)
			os.Remove(filepath.Join(dir, ".download-"+filename))

			if _, err := r.db.ExecContext(ctx,
				`UPDATE files SET status = 'pending', error_message = '', retry_count = 0 WHERE id = ?`, z.id); err == nil {
				resetCount++
			}
		}
	}
	return resetCount, nil
}

// IncrementRetryCount increments retry_count, sets status='error', updates last_attempt_at.
func (r *FileRepo) IncrementRetryCount(ctx context.Context, fileID int64, lastError string) (int, error) {
	_, err := r.db.ExecContext(ctx,
		`UPDATE files SET retry_count = retry_count + 1, status = 'error', error_message = ?, last_attempt_at = CURRENT_TIMESTAMP WHERE id = ?`,
		lastError, fileID)
	if err != nil {
		return 0, fmt.Errorf("incrementing retry count for file %d: %w", fileID, err)
	}
	var retryCount int
	err = r.db.QueryRowContext(ctx, `SELECT retry_count FROM files WHERE id = ?`, fileID).Scan(&retryCount)
	if err != nil {
		return 0, fmt.Errorf("reading retry count for file %d: %w", fileID, err)
	}
	return retryCount, nil
}

// MarkFailedPermanent sets status='failed_permanent'.
func (r *FileRepo) MarkFailedPermanent(ctx context.Context, fileID int64) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE files SET status = 'failed_permanent', error_message = 'exceeded max retry limit' WHERE id = ?`, fileID)
	if err != nil {
		return fmt.Errorf("marking file %d as failed permanent: %w", fileID, err)
	}
	return nil
}

// ResetRetry resets a file's status to pending, clears error_message and retry_count.
func (r *FileRepo) ResetRetry(ctx context.Context, fileID int64) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE files SET status = 'pending', retry_count = 0, error_message = '' WHERE id = ?`, fileID)
	if err != nil {
		return fmt.Errorf("resetting retry for file %d: %w", fileID, err)
	}
	return nil
}

// ListRetryable returns pending and error files with retry_count < maxRetries.
// Pending files (never attempted) are prioritized over error files (failed previously).
func (r *FileRepo) ListRetryable(ctx context.Context, maxRetries int) ([]*File, error) {
	rows, err := r.db.QueryContext(ctx,
		"SELECT "+fileColumns+" FROM files WHERE status IN ('pending', 'error') AND retry_count < ? ORDER BY CASE WHEN status = 'pending' THEN 0 ELSE 1 END, last_attempt_at ASC LIMIT 20", maxRetries)
	if err != nil {
		return nil, fmt.Errorf("listing retryable files: %w", err)
	}
	defer rows.Close()
	var files []*File
	for rows.Next() {
		f, err := scanFile(rows)
		if err != nil {
			return nil, err
		}
		files = append(files, f)
	}
	return files, rows.Err()
}

// GetQueueStats returns file counts grouped by status.
func (r *FileRepo) GetQueueStats(ctx context.Context) (*QueueStats, error) {
	stats := &QueueStats{}
	rows, err := r.db.QueryContext(ctx, `SELECT status, COUNT(*) FROM files GROUP BY status`)
	if err != nil {
		return nil, fmt.Errorf("getting queue stats: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return nil, fmt.Errorf("scanning queue stats: %w", err)
		}
		switch status {
		case "pending":
			stats.Pending = count
		case "downloading":
			stats.Downloading = count
		case "complete":
			stats.Complete = count
		case "error":
			stats.Error = count
		case "failed_permanent":
			stats.FailedPermanent = count
		}
	}
	return stats, rows.Err()
}

// semverFromFilename extracts a semver-like version (X.Y.Z or X.Y) from a filename.
var semverFromFilename = regexp.MustCompile(`(\d+\.\d+(?:\.\d+)?(?:(?:alpha|beta|rc|dev|pre|patch|build|post)\.?[a-zA-Z0-9.]*)?)`)

// extractVersionFromFilename extracts a version string from a filename.
// Returns empty string if no version found.
func extractVersionFromFilename(filename string) string {
	stripped := filename
	if strings.HasPrefix(filename, "go") && len(filename) > 2 && filename[2] >= '0' && filename[2] <= '9' {
		stripped = filename[2:]
	}
	matches := semverFromFilename.FindStringSubmatch(stripped)
	if len(matches) < 2 {
		return ""
	}
	version := matches[1]
	if !strings.Contains(version, ".") {
		return ""
	}
	return version
}

// BackfillEmptyVersions populates empty version fields by extracting versions from filenames.
// Returns the count of updated rows. Safe to call multiple times (idempotent).
func (r *FileRepo) BackfillEmptyVersions(ctx context.Context) (int, error) {
	rows, err := r.db.QueryContext(ctx, "SELECT id, filename FROM files WHERE version = '' OR version IS NULL")
	if err != nil {
		return 0, fmt.Errorf("querying files with empty versions: %w", err)
	}
	defer rows.Close()

	type idFile struct {
		id       int64
		filename string
	}
	var collected []idFile
	for rows.Next() {
		var item idFile
		if err := rows.Scan(&item.id, &item.filename); err != nil {
			continue
		}
		collected = append(collected, item)
	}
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("iterating files with empty versions: %w", err)
	}

	count := 0
	for _, item := range collected {
		version := extractVersionFromFilename(item.filename)
		if version == "" {
			continue
		}
		result, err := r.db.ExecContext(ctx,
			"UPDATE files SET version = ? WHERE id = ? AND (version = '' OR version IS NULL)",
			version, item.id)
		if err != nil {
			continue
		}
		if n, _ := result.RowsAffected(); n > 0 {
			count++
		}
	}
	return count, nil
}

// UpdateLocalPath updates the local_path of a file record. Used during storage migration.
func (r *FileRepo) UpdateLocalPath(ctx context.Context, id int64, newPath string) error {
	_, err := r.db.ExecContext(ctx,
		"UPDATE files SET local_path = ? WHERE id = ?", newPath, id)
	if err != nil {
		return fmt.Errorf("updating local_path for file %d: %w", id, err)
	}
	return nil
}

// FindByLocalPath finds a file record by its local_path. Returns nil, nil if not found.
func (r *FileRepo) FindByLocalPath(ctx context.Context, localPath string) (*File, error) {
	row := r.db.QueryRowContext(ctx,
		"SELECT "+fileColumns+" FROM files WHERE local_path = ?", localPath)
	f, err := scanFileRow(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return f, nil
}
