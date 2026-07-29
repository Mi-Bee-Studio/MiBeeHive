package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/Mi-Bee-Studio/mibeehive/internal/model"
)

const osInstallConfigColumns = `id, name, config_name, os_type, config, enabled, created_at, updated_at`

// OsInstallConfigRepo implements CRUD operations for OS install configurations.
type OsInstallConfigRepo struct {
	db *sql.DB
}

// NewOsInstallConfigRepo creates a new repository with database connection.
func NewOsInstallConfigRepo(db *sql.DB) *OsInstallConfigRepo {
	return &OsInstallConfigRepo{db: db}
}

// scanOsInstallConfigFromScanner maps database rows to OsInstallConfig struct.
func scanOsInstallConfigFromScanner(s interface{ Scan(dest ...any) error }) (*model.OsInstallConfig, error) {
	var c model.OsInstallConfig
	if err := s.Scan(&c.ID, &c.Name, &c.ConfigName, &c.OsType, &c.Config, &c.Enabled, &c.CreatedAt, &c.UpdatedAt); err != nil {
		return nil, err
	}
	return &c, nil
}

// List returns all OS install configs ordered by name.
func (r *OsInstallConfigRepo) List(ctx context.Context) ([]*model.OsInstallConfig, error) {
	rows, err := r.db.QueryContext(ctx, "SELECT "+osInstallConfigColumns+" FROM os_install_configs ORDER BY name")
	if err != nil {
		return nil, fmt.Errorf("listing os install configs: %w", err)
	}
	defer rows.Close()

	var configs []*model.OsInstallConfig
	for rows.Next() {
		c, err := scanOsInstallConfigFromScanner(rows)
		if err != nil {
			return nil, err
		}
		configs = append(configs, c)
	}
	return configs, rows.Err()
}

// ListEnabled returns only enabled configurations.
func (r *OsInstallConfigRepo) ListEnabled(ctx context.Context) ([]*model.OsInstallConfig, error) {
	rows, err := r.db.QueryContext(ctx, "SELECT "+osInstallConfigColumns+" FROM os_install_configs WHERE enabled = 1")
	if err != nil {
		return nil, fmt.Errorf("listing enabled os install configs: %w", err)
	}
	defer rows.Close()

	var configs []*model.OsInstallConfig
	for rows.Next() {
		c, err := scanOsInstallConfigFromScanner(rows)
		if err != nil {
			return nil, err
		}
		configs = append(configs, c)
	}
	return configs, rows.Err()
}

// GetByID retrieves a configuration by ID. Returns nil, nil if not found.
func (r *OsInstallConfigRepo) GetByID(ctx context.Context, id int64) (*model.OsInstallConfig, error) {
	row := r.db.QueryRowContext(ctx, "SELECT "+osInstallConfigColumns+" FROM os_install_configs WHERE id = ?", id)
	c, err := scanOsInstallConfigFromScanner(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("getting os install config %d: %w", id, err)
	}
	return c, nil
}

// GetByName retrieves a configuration by name. Returns nil, nil if not found.
func (r *OsInstallConfigRepo) GetByName(ctx context.Context, name string) (*model.OsInstallConfig, error) {
	row := r.db.QueryRowContext(ctx, "SELECT "+osInstallConfigColumns+" FROM os_install_configs WHERE name = ?", name)
	c, err := scanOsInstallConfigFromScanner(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("getting os install config %q: %w", name, err)
	}
	return c, nil
}

// Create inserts a new configuration and returns the created instance.
func (r *OsInstallConfigRepo) Create(ctx context.Context, name, configName, osType, config string) (*model.OsInstallConfig, error) {
	result, err := r.db.ExecContext(ctx,
		`INSERT INTO os_install_configs (name, config_name, os_type, config, enabled, created_at, updated_at) VALUES (?, ?, ?, ?, 1, datetime('now'), datetime('now'))`,
		name, configName, osType, config)
	if err != nil {
		return nil, fmt.Errorf("inserting os install config %q: %w", name, err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("getting last insert id: %w", err)
	}
	return r.GetByID(ctx, id)
}

// Update modifies an existing configuration.
func (r *OsInstallConfigRepo) Update(ctx context.Context, id int64, name, configName, osType, config string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE os_install_configs SET name = ?, config_name = ?, os_type = ?, config = ?, updated_at = datetime('now') WHERE id = ?`,
		name, configName, osType, config, id)
	if err != nil {
		return fmt.Errorf("updating os install config %d: %w", id, err)
	}
	return nil
}

// Delete performs soft delete by setting enabled=0.
func (r *OsInstallConfigRepo) Delete(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `UPDATE os_install_configs SET enabled = 0 WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("soft-deleting os install config %d: %w", id, err)
	}
	return nil
}
