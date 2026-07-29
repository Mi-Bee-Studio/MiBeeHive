package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

const isoCatalogColumns = `
	id, name, distro, variant, arch, check_url, filename_pattern,
	base_url, version_dir_pattern, iso_path_template,
	current_url, auto_update, check_interval_hours, last_checked,
	last_error, status, download_status, sha256, created_at, updated_at
`

// ISOCatalogRepo implements CRUD operations for the iso_catalog table.
type ISOCatalogRepo struct {
	db *sql.DB
}

// NewISOCatalogRepo creates a new repository with database connection.
func NewISOCatalogRepo(db *sql.DB) *ISOCatalogRepo {
	return &ISOCatalogRepo{db: db}
}

// ISOCatalogDBEntry maps a row from the iso_catalog table.
type ISOCatalogDBEntry struct {
	ID                 int64
	Name               string
	Distro             string
	Variant            string
	Arch               string
	CheckURL           string
	FilenamePattern    string
	BaseURL            string
	VersionDirPattern  string
	ISOPathTemplate    string
	CurrentURL         string
	AutoUpdate         bool
	CheckIntervalHours int
	LastChecked        sql.NullString
	LastError          string
	Status             string
	DownloadStatus     string
	SHA256             string
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

func scanISOCatalog(scanner interface{ Scan(dest ...any) error }) (*ISOCatalogDBEntry, error) {
	var e ISOCatalogDBEntry
	err := scanner.Scan(
		&e.ID, &e.Name, &e.Distro, &e.Variant, &e.Arch,
		&e.CheckURL, &e.FilenamePattern, &e.BaseURL, &e.VersionDirPattern, &e.ISOPathTemplate,
		&e.CurrentURL,
		&e.AutoUpdate, &e.CheckIntervalHours, &e.LastChecked,
		&e.LastError, &e.Status, &e.DownloadStatus, &e.SHA256, &e.CreatedAt, &e.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &e, nil
}

// List returns all catalog entries ordered by distro and arch.
func (r *ISOCatalogRepo) List(ctx context.Context) ([]ISOCatalogDBEntry, error) {
	rows, err := r.db.QueryContext(ctx,
		"SELECT "+isoCatalogColumns+" FROM iso_catalog ORDER BY distro, arch")
	if err != nil {
		return nil, fmt.Errorf("listing iso_catalog: %w", err)
	}
	defer rows.Close()

	var entries []ISOCatalogDBEntry
	for rows.Next() {
		e, err := scanISOCatalog(rows)
		if err != nil {
			return nil, fmt.Errorf("scanning iso_catalog row: %w", err)
		}
		entries = append(entries, *e)
	}
	return entries, rows.Err()
}

// GetByID retrieves a catalog entry by ID. Returns nil, nil if not found.
func (r *ISOCatalogRepo) GetByID(ctx context.Context, id int64) (*ISOCatalogDBEntry, error) {
	row := r.db.QueryRowContext(ctx,
		"SELECT "+isoCatalogColumns+" FROM iso_catalog WHERE id = ?", id)
	e, err := scanISOCatalog(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("getting iso_catalog entry %d: %w", id, err)
	}
	return e, nil
}

// Create inserts a new catalog entry and returns its ID.
func (r *ISOCatalogRepo) Create(ctx context.Context, e *ISOCatalogDBEntry) (int64, error) {
	result, err := r.db.ExecContext(ctx,
		`INSERT INTO iso_catalog (name, distro, variant, arch, check_url, filename_pattern, base_url, version_dir_pattern, iso_path_template, auto_update, check_interval_hours, sha256, status)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		e.Name, e.Distro, e.Variant, e.Arch, e.CheckURL, e.FilenamePattern,
		e.BaseURL, e.VersionDirPattern, e.ISOPathTemplate,
		e.AutoUpdate, e.CheckIntervalHours, e.SHA256, e.Status)
	if err != nil {
		return 0, fmt.Errorf("creating iso_catalog entry: %w", err)
	}
	return result.LastInsertId()
}

func (r *ISOCatalogRepo) Update(ctx context.Context, id int64, e *ISOCatalogDBEntry) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE iso_catalog SET name=?, distro=?, variant=?, arch=?, check_url=?, filename_pattern=?, base_url=?, version_dir_pattern=?, iso_path_template=?,
		 auto_update=?, check_interval_hours=?, sha256=?, updated_at=CURRENT_TIMESTAMP WHERE id=?`,
		e.Name, e.Distro, e.Variant, e.Arch, e.CheckURL, e.FilenamePattern,
		e.BaseURL, e.VersionDirPattern, e.ISOPathTemplate,
		e.AutoUpdate, e.CheckIntervalHours, e.SHA256, id)
	if err != nil {
		return fmt.Errorf("updating iso_catalog entry %d: %w", id, err)
	}
	return nil
}

// Delete removes a catalog entry by ID.
func (r *ISOCatalogRepo) Delete(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM iso_catalog WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("deleting iso_catalog entry %d: %w", id, err)
	}
	return nil
}

// ListAutoUpdate returns all entries with auto_update enabled.
func (r *ISOCatalogRepo) ListAutoUpdate(ctx context.Context) ([]ISOCatalogDBEntry, error) {
	rows, err := r.db.QueryContext(ctx,
		"SELECT "+isoCatalogColumns+" FROM iso_catalog WHERE auto_update = 1")
	if err != nil {
		return nil, fmt.Errorf("listing auto-update iso_catalog: %w", err)
	}
	defer rows.Close()

	var entries []ISOCatalogDBEntry
	for rows.Next() {
		e, err := scanISOCatalog(rows)
		if err != nil {
			return nil, fmt.Errorf("scanning iso_catalog row: %w", err)
		}
		entries = append(entries, *e)
	}
	return entries, rows.Err()
}

// UpdateAfterCheck updates the current_url, status, last_error, and timestamps after a version check.
func (r *ISOCatalogRepo) UpdateAfterCheck(ctx context.Context, id int64, currentURL, status, lastError string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE iso_catalog SET current_url=?, status=?, last_error=?, last_checked=CURRENT_TIMESTAMP, updated_at=CURRENT_TIMESTAMP WHERE id=?`,
		currentURL, status, lastError, id)
	if err != nil {
		return fmt.Errorf("updating iso_catalog after check %d: %w", id, err)
	}
	return nil
}

// ListDownloadQueue returns entries with download_status IN ('pending', 'downloading') ordered by id.
func (r *ISOCatalogRepo) ListDownloadQueue(ctx context.Context) ([]ISOCatalogDBEntry, error) {
	rows, err := r.db.QueryContext(ctx,
		"SELECT "+isoCatalogColumns+" FROM iso_catalog WHERE download_status IN ('pending', 'downloading') ORDER BY id")
	if err != nil {
		return nil, fmt.Errorf("listing iso download queue: %w", err)
	}
	defer rows.Close()

	var entries []ISOCatalogDBEntry
	for rows.Next() {
		e, err := scanISOCatalog(rows)
		if err != nil {
			return nil, fmt.Errorf("scanning iso_catalog row: %w", err)
		}
		entries = append(entries, *e)
	}
	return entries, rows.Err()
}

// UpdateDownloadStatus sets the download_status for a catalog entry.
func (r *ISOCatalogRepo) UpdateDownloadStatus(ctx context.Context, id int64, status string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE iso_catalog SET download_status=?, updated_at=CURRENT_TIMESTAMP WHERE id=?`,
		status, id)
	if err != nil {
		return fmt.Errorf("updating iso_catalog download_status %d: %w", id, err)
	}
	return nil
}

// ListByDownloadStatuses returns entries matching any of the given download statuses.
func (r *ISOCatalogRepo) ListByDownloadStatuses(ctx context.Context, statuses []string) ([]ISOCatalogDBEntry, error) {
	if len(statuses) == 0 {
		return nil, nil
	}
	places := ""
	args := make([]any, len(statuses))
	for i, s := range statuses {
		if i > 0 {
			places += ","
		}
		places += "?"
		args[i] = s
	}
	rows, err := r.db.QueryContext(ctx,
		"SELECT "+isoCatalogColumns+" FROM iso_catalog WHERE download_status IN ("+places+") ORDER BY id", args...)
	if err != nil {
		return nil, fmt.Errorf("listing iso_catalog by download statuses: %w", err)
	}
	defer rows.Close()

	var entries []ISOCatalogDBEntry
	for rows.Next() {
		e, err := scanISOCatalog(rows)
		if err != nil {
			return nil, fmt.Errorf("scanning iso_catalog row: %w", err)
		}
		entries = append(entries, *e)
	}
	return entries, rows.Err()
}
