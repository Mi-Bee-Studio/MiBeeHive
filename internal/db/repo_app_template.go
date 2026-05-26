package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/Mi-Bee-Studio/mibeehive/internal/model"
)

const appTemplateColumns = `id, name, description, image, command, env, ports, volumes, restart_policy, category, icon, enabled, created_at, updated_at`

// AppTemplateRepo implements CRUD operations for application templates.
type AppTemplateRepo struct {
	db *sql.DB
}

// NewAppTemplateRepo creates a new repository with database connection.
func NewAppTemplateRepo(db *sql.DB) *AppTemplateRepo {
	return &AppTemplateRepo{db: db}
}

// scanAppTemplate maps database rows to model.AppTemplate.
func scanAppTemplate(scanner interface{ Scan(dest ...any) error }) (*model.AppTemplate, error) {
	var t model.AppTemplate
	var envJSON, portsJSON, volumesJSON string
	var enabled int

	err := scanner.Scan(
		&t.ID, &t.Name, &t.Description, &t.Image, &t.Command,
		&envJSON, &portsJSON, &volumesJSON,
		&t.RestartPolicy, &t.Category, &t.Icon, &enabled,
		&t.CreatedAt, &t.UpdatedAt,
	)
	// Skip missing rows
	if err != nil {
		return nil, err
	}

	t.Enabled = enabled == 1

	// Decode JSON columns
	if err := json.Unmarshal([]byte(envJSON), &t.Env); err != nil {
		t.Env = map[string]string{}
	}
	if err := json.Unmarshal([]byte(portsJSON), &t.Ports); err != nil {
		t.Ports = nil
	}
	if err := json.Unmarshal([]byte(volumesJSON), &t.Volumes); err != nil {
		t.Volumes = nil
	}

	return &t, nil
}

// List returns all enabled application templates ordered by category and name.
func (r *AppTemplateRepo) List(ctx context.Context) ([]model.AppTemplate, error) {
	rows, err := r.db.QueryContext(ctx, "SELECT "+appTemplateColumns+" FROM app_templates WHERE enabled = 1 ORDER BY category, name")
	if err != nil {
		return nil, fmt.Errorf("listing app templates: %w", err)
	}
	defer rows.Close()

	var templates []model.AppTemplate
	for rows.Next() {
		t, err := scanAppTemplate(rows)
		if err != nil {
			return nil, err
		}
		templates = append(templates, *t)
	}
	return templates, rows.Err()
}

// ListAll returns all application templates (including disabled) ordered by category and name.
func (r *AppTemplateRepo) ListAll(ctx context.Context) ([]model.AppTemplate, error) {
	rows, err := r.db.QueryContext(ctx, "SELECT "+appTemplateColumns+" FROM app_templates ORDER BY category, name")
	if err != nil {
		return nil, fmt.Errorf("listing all app templates: %w", err)
	}
	defer rows.Close()

	var templates []model.AppTemplate
	for rows.Next() {
		t, err := scanAppTemplate(rows)
		if err != nil {
			return nil, err
		}
		templates = append(templates, *t)
	}
	return templates, rows.Err()
}

// GetByID retrieves a template by ID. Returns nil, nil if not found.
func (r *AppTemplateRepo) GetByID(ctx context.Context, id int64) (*model.AppTemplate, error) {
	row := r.db.QueryRowContext(ctx, "SELECT "+appTemplateColumns+" FROM app_templates WHERE id = ?", id)
	t, err := scanAppTemplate(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("getting app template %d: %w", id, err)
	}
	return t, nil
}

// Create inserts a new application template.
func (r *AppTemplateRepo) Create(ctx context.Context, t *model.AppTemplate) error {
	envJSON, err := json.Marshal(t.Env)
	if err != nil {
		return fmt.Errorf("marshaling env: %w", err)
	}
	portsJSON, err := json.Marshal(t.Ports)
	if err != nil {
		return fmt.Errorf("marshaling ports: %w", err)
	}
	volumesJSON, err := json.Marshal(t.Volumes)
	if err != nil {
		return fmt.Errorf("marshaling volumes: %w", err)
	}

	enabled := 0
	if t.Enabled {
		enabled = 1
	}

	result, err := r.db.ExecContext(ctx,
		`INSERT INTO app_templates (name, description, image, command, env, ports, volumes, restart_policy, category, icon, enabled, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, datetime('now'), datetime('now'))`,
		t.Name, t.Description, t.Image, t.Command, string(envJSON), string(portsJSON), string(volumesJSON),
		t.RestartPolicy, t.Category, t.Icon, enabled)
	if err != nil {
		return fmt.Errorf("inserting app template %q: %w", t.Name, err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("getting last insert id: %w", err)
	}
	t.ID = id
	return nil
}

// Delete removes an application template by ID.
func (r *AppTemplateRepo) Delete(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM app_templates WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("deleting app template %d: %w", id, err)
	}
	return nil
}
