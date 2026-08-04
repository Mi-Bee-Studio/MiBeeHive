package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)


// AuditEntry is one row in virtual_audit_log.
type AuditEntry struct {
	ID         int64
	AdminUser  string
	Action     string // create, update, delete
	EntityType string // channel, view, node
	EntityID   int64
	EntityName string
	DiffJSON   string
	CreatedAt  time.Time
}

// AuditRepo reads/writes the virtual_audit_log table.
type AuditRepo struct {
	db *sql.DB
}

// NewAuditRepo creates an AuditRepo.
func NewAuditRepo(db *sql.DB) *AuditRepo { return &AuditRepo{db: db} }

// Log inserts an audit entry.
func (r *AuditRepo) Log(ctx context.Context, e *AuditEntry) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO virtual_audit_log (admin_user, action, entity_type, entity_id, entity_name, diff_json)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		e.AdminUser, e.Action, e.EntityType, e.EntityID, e.EntityName, e.DiffJSON)
	if err != nil {
		return fmt.Errorf("inserting audit log: %w", err)
	}
	return nil
}

// List returns recent audit entries, optionally filtered by entity type.
func (r *AuditRepo) List(ctx context.Context, entityType string, limit int) ([]*AuditEntry, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	q := `SELECT id, admin_user, action, entity_type, entity_id, entity_name, diff_json, created_at
	      FROM virtual_audit_log`
	args := []any{}
	if entityType != "" {
		q += " WHERE entity_type = ?"
		args = append(args, entityType)
	}
	q += " ORDER BY created_at DESC LIMIT ?"
	args = append(args, limit)

	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("querying audit log: %w", err)
	}
	defer rows.Close()

	var entries []*AuditEntry
	for rows.Next() {
		var e AuditEntry
		if err := rows.Scan(&e.ID, &e.AdminUser, &e.Action, &e.EntityType,
			&e.EntityID, &e.EntityName, &e.DiffJSON, &e.CreatedAt); err != nil {
			return nil, err
		}
		entries = append(entries, &e)
	}
	return entries, rows.Err()
}
