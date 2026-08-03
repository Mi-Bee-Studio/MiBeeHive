package service

import (
	"context"
	"crypto/rand"
	"database/sql"
	"fmt"
	"log/slog"
	"time"
)

// ShareLink represents a share link in the database.
type ShareLink struct {
	Token         string
	FileID        int64
	ExpiresAt     *time.Time
	MaxDownloads  int // 0 = unlimited
	DownloadCount int
	Note          string
	CreatedAt     time.Time
}

// ShareLinkService provides business logic for share link management.
type ShareLinkService struct {
	db     *sql.DB
	readDB *sql.DB
}

// NewShareLinkService creates a new ShareLinkService.
func NewShareLinkService(db, readDB *sql.DB) *ShareLinkService {
	return &ShareLinkService{
		db:     db,
		readDB: readDB,
	}
}

// Create creates a new share link for the given file.
// Returns the created share link with a generated token.
func (s *ShareLinkService) Create(ctx context.Context, fileID int64, expiresAt *time.Time, maxDownloads int, note string) (*ShareLink, error) {
	// Generate a unique token
	token, err := generateShareToken(ctx, s.db)
	if err != nil {
		return nil, fmt.Errorf("generating share token: %w", err)
	}

	now := time.Now()
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO share_links (token, file_id, expires_at, max_downloads, note, created_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		token, fileID, expiresAt, maxDownloads, note, now)
	if err != nil {
		return nil, fmt.Errorf("creating share link: %w", err)
	}

	return &ShareLink{
		Token:         token,
		FileID:        fileID,
		ExpiresAt:     expiresAt,
		MaxDownloads:  maxDownloads,
		DownloadCount: 0,
		Note:          note,
		CreatedAt:     now,
	}, nil
}

// Validate checks if a share token is valid and returns the file ID.
// It atomically increments the download count and checks expiry and max_downloads.
// Returns an error if:
// - Token not found (404)
// - Token expired (410 Gone)
// - Max downloads exceeded (410 Gone)
func (s *ShareLinkService) Validate(ctx context.Context, token string) (int64, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback()

	var fileID int64
	var expiresAt *time.Time
	var maxDownloads, downloadCount int

	err = tx.QueryRowContext(ctx,
		`SELECT file_id, expires_at, max_downloads, download_count
		 FROM share_links
		 WHERE token = ?`,
		token).Scan(&fileID, &expiresAt, &maxDownloads, &downloadCount)
	if err != nil {
		if err == sql.ErrNoRows {
			return 0, fmt.Errorf("share token not found")
		}
		return 0, fmt.Errorf("querying share link: %w", err)
	}

	// Check expiry
	if expiresAt != nil && time.Now().After(*expiresAt) {
		return 0, fmt.Errorf("share token expired")
	}

	// Check max downloads (0 = unlimited)
	if maxDownloads > 0 && downloadCount >= maxDownloads {
		return 0, fmt.Errorf("max downloads exceeded")
	}

	// Increment download count atomically
	_, err = tx.ExecContext(ctx,
		`UPDATE share_links SET download_count = download_count + 1 WHERE token = ?`,
		token)
	if err != nil {
		return 0, fmt.Errorf("incrementing download count: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("committing transaction: %w", err)
	}

	slog.Info("share link validated", "token", token, "file_id", fileID, "download_count", downloadCount+1)
	return fileID, nil
}

// Revoke deletes a share link by token.
func (s *ShareLinkService) Revoke(ctx context.Context, token string) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM share_links WHERE token = ?`, token)
	if err != nil {
		return fmt.Errorf("deleting share link: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("share link not found")
	}
	slog.Info("share link revoked", "token", token)
	return nil
}

// List returns all share links.
func (s *ShareLinkService) List(ctx context.Context) ([]*ShareLink, error) {
	rows, err := s.readDB.QueryContext(ctx,
		`SELECT token, file_id, expires_at, max_downloads, download_count, note, created_at
		 FROM share_links
		 ORDER BY created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("listing share links: %w", err)
	}
	defer rows.Close()

	var links []*ShareLink
	for rows.Next() {
		var link ShareLink
		if err := rows.Scan(
			&link.Token,
			&link.FileID,
			&link.ExpiresAt,
			&link.MaxDownloads,
			&link.DownloadCount,
			&link.Note,
			&link.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning share link: %w", err)
		}
		links = append(links, &link)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating share links: %w", err)
	}
	return links, nil
}

// GetByToken retrieves a share link by its token.
func (s *ShareLinkService) GetByToken(ctx context.Context, token string) (*ShareLink, error) {
	var link ShareLink
	err := s.readDB.QueryRowContext(ctx,
		`SELECT token, file_id, expires_at, max_downloads, download_count, note, created_at
		 FROM share_links
		 WHERE token = ?`,
		token).Scan(
		&link.Token,
		&link.FileID,
		&link.ExpiresAt,
		&link.MaxDownloads,
		&link.DownloadCount,
		&link.Note,
		&link.CreatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("share link not found")
		}
		return nil, fmt.Errorf("querying share link: %w", err)
	}
	return &link, nil
}

// generateShareToken generates a cryptographically random share token using
// base58 encoding, 22 characters (~128 bits of entropy). It checks the database
// for UNIQUE collisions and retries up to maxTokenRetries times.
func generateShareToken(ctx context.Context, db *sql.DB) (string, error) {
	const maxRetries = 5
	for attempt := 0; attempt < maxRetries; attempt++ {
		token, err := randomShareToken()
		if err != nil {
			return "", err
		}
		var exists bool
		err = db.QueryRowContext(ctx,
			"SELECT 1 FROM share_links WHERE token = ?", token).Scan(&exists)
		if err == sql.ErrNoRows {
			return token, nil
		}
		if err != nil {
			return "", fmt.Errorf("checking token uniqueness: %w", err)
		}
	}
	return "", fmt.Errorf("failed to generate unique share token after %d attempts", maxRetries)
}

// randomShareToken reads 16 bytes from crypto/rand, encodes them to base58, and
// returns the first 22 characters.
func randomShareToken() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("reading random bytes: %w", err)
	}
	encoded := base58Encode(buf)
	if len(encoded) > 22 {
		encoded = encoded[:22]
	}
	return encoded, nil
}
