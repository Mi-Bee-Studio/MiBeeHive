package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

const credentialColumns = `id, source_type, token, created_at, updated_at`

// SourceCredentialRepo provides CRUD operations for source credentials.
type SourceCredentialRepo struct {
	db *sql.DB
}

// NewSourceCredentialRepo creates a new SourceCredentialRepo.
func NewSourceCredentialRepo(db *sql.DB) *SourceCredentialRepo {
	return &SourceCredentialRepo{db: db}
}

// GetBySourceType retrieves a credential by its source type.
func (r *SourceCredentialRepo) GetBySourceType(ctx context.Context, sourceType string) (*SourceCredential, error) {
	row := r.db.QueryRowContext(ctx,
		"SELECT "+credentialColumns+" FROM source_credentials WHERE source_type = ?", sourceType)
	c, err := scanCredentialFromScanner(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("getting credential for source %q: %w", sourceType, err)
	}
	return c, nil
}

// Upsert inserts or updates a credential for a source type.
func (r *SourceCredentialRepo) Upsert(ctx context.Context, sourceType, token string) error {
	query := `INSERT INTO source_credentials (source_type, token)
	          VALUES (?, ?)
	          ON CONFLICT(source_type) DO UPDATE SET token = ?, updated_at = CURRENT_TIMESTAMP`
	_, err := r.db.ExecContext(ctx, query, sourceType, token, token)
	if err != nil {
		return fmt.Errorf("upserting credential for source %q: %w", sourceType, err)
	}
	return nil
}

// List returns all credentials.
func (r *SourceCredentialRepo) List(ctx context.Context) ([]*SourceCredential, error) {
	rows, err := r.db.QueryContext(ctx, "SELECT "+credentialColumns+" FROM source_credentials ORDER BY source_type")
	if err != nil {
		return nil, fmt.Errorf("listing credentials: %w", err)
	}
	defer rows.Close()

	var creds []*SourceCredential
	for rows.Next() {
		c, err := scanCredentialFromScanner(rows)
		if err != nil {
			return nil, err
		}
		creds = append(creds, c)
	}
	return creds, rows.Err()
}

func scanCredentialFromScanner(s interface{ Scan(dest ...any) error }) (*SourceCredential, error) {
	c := &SourceCredential{}
	err := s.Scan(&c.ID, &c.SourceType, &c.Token, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("scanning credential: %w", err)
	}
	return c, nil
}
