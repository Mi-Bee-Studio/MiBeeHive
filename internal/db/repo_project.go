package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/Mi-Bee-Studio/mibeehive/internal/model"
)

const projectColumns = `id, name, display_name, source_type, source_url, config, latest_version, last_crawled_at, created_at, enabled`

// ProjectRepo provides CRUD operations for projects.
type ProjectRepo struct {
	db *sql.DB
}

// NewProjectRepo creates a new ProjectRepo.
func NewProjectRepo(db *sql.DB) *ProjectRepo {
	return &ProjectRepo{db: db}
}

// Create inserts a new project and returns it with the generated ID.
func (r *ProjectRepo) Create(ctx context.Context, name, displayName, sourceType, sourceURL string) (*Project, error) {
	query := `INSERT INTO projects (name, display_name, source_type, source_url)
	          VALUES (?, ?, ?, ?)`
	result, err := r.db.ExecContext(ctx, query, name, displayName, sourceType, sourceURL)
	if err != nil {
		return nil, fmt.Errorf("inserting project %q: %w", name, err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("getting last insert id: %w", err)
	}

	return r.GetByID(ctx, id)
}

// GetByID retrieves a project by its ID.
func (r *ProjectRepo) GetByID(ctx context.Context, id int64) (*Project, error) {
	return r.getOne(ctx, "SELECT "+projectColumns+" FROM projects WHERE id = ?", id)
}

// GetByName retrieves a project by its unique name.
func (r *ProjectRepo) GetByName(ctx context.Context, name string) (*Project, error) {
	return r.getOne(ctx, "SELECT "+projectColumns+" FROM projects WHERE name = ?", name)
}
// List returns all projects ordered by name.
func (r *ProjectRepo) List(ctx context.Context) ([]*Project, error) {
	rows, err := r.db.QueryContext(ctx, "SELECT "+projectColumns+" FROM projects ORDER BY name")
	if err != nil {
		return nil, fmt.Errorf("listing projects: %w", err)
	}
	defer rows.Close()

	var projects []*Project
	for rows.Next() {
		p, err := scanProject(rows)
		if err != nil {
			return nil, err
		}
		projects = append(projects, p)
	}
	return projects, rows.Err()
}

// UpdateLatestVersion sets the latest_version for a project.
func (r *ProjectRepo) UpdateLatestVersion(ctx context.Context, id int64, version string) error {
	_, err := r.db.ExecContext(ctx,
		"UPDATE projects SET latest_version = ? WHERE id = ?", version, id)
	if err != nil {
		return fmt.Errorf("updating latest_version for project %d: %w", id, err)
	}
	return nil
}

// UpdateLastCrawledAt sets last_crawled_at to the current time.
func (r *ProjectRepo) UpdateLastCrawledAt(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx,
		"UPDATE projects SET last_crawled_at = CURRENT_TIMESTAMP WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("updating last_crawled_at for project %d: %w", id, err)
	}
	return nil
}

// CreateWithSettings inserts a new project with settings marshaled as JSON config.
func (r *ProjectRepo) CreateWithSettings(ctx context.Context, name, displayName, sourceType, sourceURL string, settings model.ProjectSettings) (*Project, error) {
	configJSON, err := json.Marshal(settings)
	if err != nil {
		return nil, fmt.Errorf("marshaling settings for project %q: %w", name, err)
	}
	query := `INSERT INTO projects (name, display_name, source_type, source_url, config)
		          VALUES (?, ?, ?, ?, ?)`
	result, err := r.db.ExecContext(ctx, query, name, displayName, sourceType, sourceURL, string(configJSON))
	if err != nil {
		return nil, fmt.Errorf("inserting project %q: %w", name, err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("getting last insert id: %w", err)
	}
	return r.GetByID(ctx, id)
}

// UpdateProject updates a project's metadata and marshals settings to the config JSON column.
func (r *ProjectRepo) UpdateProject(ctx context.Context, id int64, name, displayName, sourceType, sourceURL string, settings model.ProjectSettings) error {
	configJSON, err := json.Marshal(settings)
	if err != nil {
		return fmt.Errorf("marshaling settings for project %d: %w", id, err)
	}
	query := `UPDATE projects SET name = ?, display_name = ?, source_type = ?, source_url = ?, config = ? WHERE id = ?`
	_, err = r.db.ExecContext(ctx, query, name, displayName, sourceType, sourceURL, string(configJSON), id)
	if err != nil {
		return fmt.Errorf("updating project %d: %w", id, err)
	}
	return nil
}

// GetSettings unmarshals the config JSON column into ProjectSettings.
func (r *ProjectRepo) GetSettings(ctx context.Context, projectID int64) (*model.ProjectSettings, error) {
	var configStr string
	err := r.db.QueryRowContext(ctx, "SELECT config FROM projects WHERE id = ?", projectID).Scan(&configStr)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("querying config for project %d: %w", projectID, err)
	}
	var settings model.ProjectSettings
	if err := json.Unmarshal([]byte(configStr), &settings); err != nil {
		return nil, fmt.Errorf("unmarshaling config for project %d: %w", projectID, err)
	}
	return &settings, nil
}

// SetEnabled updates the enabled column for a project.
func (r *ProjectRepo) SetEnabled(ctx context.Context, projectID int64, enabled bool) error {
	_, err := r.db.ExecContext(ctx,
		"UPDATE projects SET enabled = ? WHERE id = ?", enabled, projectID)
	if err != nil {
		return fmt.Errorf("setting enabled=%v for project %d: %w", enabled, projectID, err)
	}
	return nil
}

// Delete permanently removes a project and its associated file/crawl_log records from the database.
func (r *ProjectRepo) Delete(ctx context.Context, projectID int64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback()

	// Delete child records in FK dependency order.
	// download_logs may not exist in all deployments; ignore "no such table" errors.
	if _, err := tx.ExecContext(ctx, "DELETE FROM download_logs WHERE project_id = ?", projectID); err != nil {
		if !strings.Contains(err.Error(), "no such table") {
			return fmt.Errorf("deleting download_logs for project %d: %w", projectID, err)
		}
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM files WHERE project_id = ?", projectID); err != nil {
		return fmt.Errorf("deleting files for project %d: %w", projectID, err)
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM crawl_logs WHERE project_id = ?", projectID); err != nil {
		return fmt.Errorf("deleting crawl_logs for project %d: %w", projectID, err)
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM projects WHERE id = ?", projectID); err != nil {
		return fmt.Errorf("deleting project %d: %w", projectID, err)
	}

	return tx.Commit()
}

// ListEnabled returns only enabled projects ordered by name.
func (r *ProjectRepo) ListEnabled(ctx context.Context) ([]*Project, error) {
	rows, err := r.db.QueryContext(ctx, "SELECT "+projectColumns+" FROM projects WHERE enabled = 1 ORDER BY name")
	if err != nil {
		return nil, fmt.Errorf("listing enabled projects: %w", err)
	}
	defer rows.Close()
	var projects []*Project
	for rows.Next() {
		p, err := scanProject(rows)
		if err != nil {
			return nil, err
		}
		projects = append(projects, p)
	}
	return projects, rows.Err()
}

// ListAll returns all projects including disabled ones, ordered by name.
func (r *ProjectRepo) ListAll(ctx context.Context) ([]*Project, error) {
	return r.List(ctx)
}

func (r *ProjectRepo) getOne(ctx context.Context, query string, args ...any) (*Project, error) {
	row := r.db.QueryRowContext(ctx, query, args...)
	p, err := scanProjectRow(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return p, nil
}

func scanProject(rows *sql.Rows) (*Project, error) {
	return scanProjectFromScanner(rows)
}

func scanProjectRow(row *sql.Row) (*Project, error) {
	return scanProjectFromScanner(row)
}

func scanProjectFromScanner(s interface{ Scan(dest ...any) error }) (*Project, error) {
	p := &Project{}
	err := s.Scan(
		&p.ID, &p.Name, &p.DisplayName, &p.SourceType, &p.SourceURL,
		&p.Config, &p.LatestVersion, &p.LastCrawledAt, &p.CreatedAt, &p.Enabled,
	)
	if err != nil {
		return nil, fmt.Errorf("scanning project: %w", err)
	}
	return p, nil
}
