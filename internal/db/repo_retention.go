package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/Mi-Bee-Studio/mibeehive/internal/model"
)

const retentionPolicyColumns = `id, registry_id, repo_pattern, keep_days, keep_count, keep_pattern, enabled, last_executed_at, created_at`

// RetentionPolicyRepo provides CRUD operations for retention policies.
type RetentionPolicyRepo struct {
	db *sql.DB
}

// NewRetentionPolicyRepo creates a new RetentionPolicyRepo.
func NewRetentionPolicyRepo(db *sql.DB) *RetentionPolicyRepo {
	return &RetentionPolicyRepo{db: db}
}

// List returns all retention policies ordered by id.
func (r *RetentionPolicyRepo) List(ctx context.Context) ([]model.RetentionPolicy, error) {
	rows, err := r.db.QueryContext(ctx, "SELECT "+retentionPolicyColumns+" FROM retention_policies ORDER BY id")
	if err != nil {
		return nil, fmt.Errorf("listing retention policies: %w", err)
	}
	defer rows.Close()

	var policies []model.RetentionPolicy
	for rows.Next() {
		p, err := scanRetentionPolicy(rows)
		if err != nil {
			return nil, err
		}
		policies = append(policies, p)
	}
	return policies, rows.Err()
}

// GetByID retrieves a retention policy by its ID. Returns nil, nil if not found.
func (r *RetentionPolicyRepo) GetByID(ctx context.Context, id int64) (*model.RetentionPolicy, error) {
	row := r.db.QueryRowContext(ctx, "SELECT "+retentionPolicyColumns+" FROM retention_policies WHERE id = ?", id)
	p, err := scanRetentionPolicy(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &p, nil
}

// ListEnabled returns only enabled retention policies for the scheduler.
func (r *RetentionPolicyRepo) ListEnabled(ctx context.Context) ([]model.RetentionPolicy, error) {
	rows, err := r.db.QueryContext(ctx,
		"SELECT "+retentionPolicyColumns+" FROM retention_policies WHERE enabled = 1 ORDER BY id")
	if err != nil {
		return nil, fmt.Errorf("listing enabled retention policies: %w", err)
	}
	defer rows.Close()

	var policies []model.RetentionPolicy
	for rows.Next() {
		p, err := scanRetentionPolicy(rows)
		if err != nil {
			return nil, err
		}
		policies = append(policies, p)
	}
	return policies, rows.Err()
}

// Create inserts a new retention policy and returns the generated ID.
func (r *RetentionPolicyRepo) Create(ctx context.Context, policy *model.RetentionPolicy) (int64, error) {
	query := `INSERT INTO retention_policies (registry_id, repo_pattern, keep_days, keep_count, keep_pattern, enabled)
	          VALUES (?, ?, ?, ?, ?, ?)`
	result, err := r.db.ExecContext(ctx, query,
		policy.RegistryID, policy.RepoPattern, policy.KeepDays, policy.KeepCount, policy.KeepPattern, policy.Enabled)
	if err != nil {
		return 0, fmt.Errorf("inserting retention policy: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("getting last insert id: %w", err)
	}
	return id, nil
}

// Update updates a retention policy.
func (r *RetentionPolicyRepo) Update(ctx context.Context, policy *model.RetentionPolicy) error {
	query := `UPDATE retention_policies SET registry_id = ?, repo_pattern = ?, keep_days = ?,
	          keep_count = ?, keep_pattern = ?, enabled = ? WHERE id = ?`
	_, err := r.db.ExecContext(ctx, query,
		policy.RegistryID, policy.RepoPattern, policy.KeepDays,
		policy.KeepCount, policy.KeepPattern, policy.Enabled, policy.ID)
	if err != nil {
		return fmt.Errorf("updating retention policy %d: %w", policy.ID, err)
	}
	return nil
}

// Delete removes a retention policy by ID.
func (r *RetentionPolicyRepo) Delete(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM retention_policies WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("deleting retention policy %d: %w", id, err)
	}
	return nil
}

// UpdateLastExecuted sets last_executed_at to the given time.
func (r *RetentionPolicyRepo) UpdateLastExecuted(ctx context.Context, id int64, t time.Time) error {
	_, err := r.db.ExecContext(ctx,
		"UPDATE retention_policies SET last_executed_at = ? WHERE id = ?", t.UTC(), id)
	if err != nil {
		return fmt.Errorf("updating last_executed_at for retention policy %d: %w", id, err)
	}
	return nil
}

func scanRetentionPolicy(s interface{ Scan(dest ...any) error }) (model.RetentionPolicy, error) {
	var p model.RetentionPolicy
	err := s.Scan(
		&p.ID, &p.RegistryID, &p.RepoPattern, &p.KeepDays, &p.KeepCount,
		&p.KeepPattern, &p.Enabled, &p.LastExecutedAt, &p.CreatedAt,
	)
	if err != nil {
		return p, fmt.Errorf("scanning retention policy: %w", err)
	}
	return p, nil
}
