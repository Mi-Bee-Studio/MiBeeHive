package service

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"time"
)

// TestCreateShareLink tests creating a new share link.
func TestCreateShareLink(t *testing.T) {
	// This is a minimal integration test that uses an in-memory database.
	// Full unit tests with mocks would require more complex setup.

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory db: %v", err)
	}
	defer db.Close()

	// Create the share_links table
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS share_links (
			token TEXT PRIMARY KEY,
			file_id INTEGER NOT NULL REFERENCES files(id),
			expires_at DATETIME,
			max_downloads INTEGER,
			download_count INTEGER NOT NULL DEFAULT 0,
			note TEXT DEFAULT '',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	// Create a dummy file
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS files (id INTEGER PRIMARY KEY, status TEXT)`)
	if err != nil {
		t.Fatalf("failed to create files table: %v", err)
	}
	_, err = db.Exec(`INSERT INTO files (id, status) VALUES (123, 'available')`)
	if err != nil {
		t.Fatalf("failed to insert dummy file: %v", err)
	}

	svc := NewShareLinkService(db, db)
	ctx := context.Background()

	tests := []struct {
		name         string
		fileID       int64
		expiresAt    *time.Time
		maxDownloads int
		note         string
		expectError  bool
	}{
		{
			name:         "successful creation with expiry",
			fileID:       123,
			maxDownloads: 5,
			note:         "Test link",
		},
		{
			name:         "unlimited downloads (max_downloads=0)",
			fileID:       123,
			maxDownloads: 0,
			note:         "Unlimited link",
		},
		{
			name:         "no expiry (nil)",
			fileID:       123,
			expiresAt:    nil,
			maxDownloads: 10,
			note:         "No expiry link",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			link, err := svc.Create(ctx, tt.fileID, tt.expiresAt, tt.maxDownloads, tt.note)

			if tt.expectError {
				if err == nil {
					t.Error("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if link.FileID != tt.fileID {
				t.Errorf("expected file_id %d, got %d", tt.fileID, link.FileID)
			}
			if link.MaxDownloads != tt.maxDownloads {
				t.Errorf("expected max_downloads %d, got %d", tt.maxDownloads, link.MaxDownloads)
			}
			if link.Note != tt.note {
				t.Errorf("expected note %q, got %q", tt.note, link.Note)
			}
			if link.Token == "" {
				t.Error("expected non-empty token")
			}
			if len(link.Token) < 16 {
				t.Errorf("expected token length >= 16, got %d", len(link.Token))
			}
		})
	}
}

// TestValidate tests share token validation.
func TestValidate(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory db: %v", err)
	}
	defer db.Close()

	// Create tables
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS share_links (
			token TEXT PRIMARY KEY,
			file_id INTEGER NOT NULL REFERENCES files(id),
			expires_at DATETIME,
			max_downloads INTEGER,
			download_count INTEGER NOT NULL DEFAULT 0,
			note TEXT DEFAULT '',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS files (id INTEGER PRIMARY KEY, status TEXT)`)
	if err != nil {
		t.Fatalf("failed to create files table: %v", err)
	}

	svc := NewShareLinkService(db, db)
	ctx := context.Background()

	// Create a valid link
	now := time.Now()
	validLink, err := svc.Create(ctx, 123, nil, 0, "Valid link")
	if err != nil {
		t.Fatalf("failed to create valid link: %v", err)
	}

	// Create an expired link
	expired := now.Add(-1 * time.Hour)
	expiredLink, err := svc.Create(ctx, 456, &expired, 0, "Expired link")
	if err != nil {
		t.Fatalf("failed to create expired link: %v", err)
	}

	// Create a link with max downloads
	maxDownloadsLink, err := svc.Create(ctx, 789, nil, 3, "Max downloads link")
	if err != nil {
		t.Fatalf("failed to create max downloads link: %v", err)
	}
	// Exhaust it
	for i := 0; i < 3; i++ {
		_, _ = svc.Validate(ctx, maxDownloadsLink.Token)
	}

	tests := []struct {
		name        string
		token       string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "valid token",
			token:       validLink.Token,
		},
		{
			name:        "token not found",
			token:       "nonexistent",
			expectError: true,
			errorMsg:    "not found",
		},
		{
			name:        "expired token (410)",
			token:       expiredLink.Token,
			expectError: true,
			errorMsg:    "expired",
		},
		{
			name:        "max downloads exceeded (410)",
			token:       maxDownloadsLink.Token,
			expectError: true,
			errorMsg:    "max downloads exceeded",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fileID, err := svc.Validate(ctx, tt.token)

			if tt.expectError {
				if err == nil {
					t.Error("expected error but got none")
				}
				if tt.errorMsg != "" && err != nil {
					errMsg := err.Error()
					if !strings.Contains(errMsg, tt.errorMsg) {
						t.Errorf("error message should contain %q, got %q", tt.errorMsg, errMsg)
					}
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if fileID == 0 {
					t.Error("expected non-zero file_id")
				}
			}
		})
	}
}

// TestRevoke tests revoking a share link.
func TestRevoke(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory db: %v", err)
	}
	defer db.Close()

	// Create tables
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS share_links (
			token TEXT PRIMARY KEY,
			file_id INTEGER NOT NULL REFERENCES files(id),
			expires_at DATETIME,
			max_downloads INTEGER,
			download_count INTEGER NOT NULL DEFAULT 0,
			note TEXT DEFAULT '',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS files (id INTEGER PRIMARY KEY, status TEXT)`)
	if err != nil {
		t.Fatalf("failed to create files table: %v", err)
	}

	svc := NewShareLinkService(db, db)
	ctx := context.Background()

	// Create a link to revoke
	link, err := svc.Create(ctx, 123, nil, 0, "To revoke")
	if err != nil {
		t.Fatalf("failed to create link: %v", err)
	}

	tests := []struct {
		name        string
		token       string
		expectError bool
	}{
		{
			name:  "successful revoke",
			token: link.Token,
		},
		{
			name:        "token not found (404)",
			token:       "nonexistent",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := svc.Revoke(ctx, tt.token)

			if tt.expectError && err == nil {
				t.Error("expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			// Verify the link is actually deleted
			if !tt.expectError {
				_, err := svc.GetByToken(ctx, tt.token)
				if err == nil {
					t.Error("expected link to be deleted but GetByToken succeeded")
				}
			}
		})
	}
}

// TestList tests listing all share links.
func TestList(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory db: %v", err)
	}
	defer db.Close()

	// Create tables
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS share_links (
			token TEXT PRIMARY KEY,
			file_id INTEGER NOT NULL REFERENCES files(id),
			expires_at DATETIME,
			max_downloads INTEGER,
			download_count INTEGER NOT NULL DEFAULT 0,
			note TEXT DEFAULT '',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS files (id INTEGER PRIMARY KEY, status TEXT)`)
	if err != nil {
		t.Fatalf("failed to create files table: %v", err)
	}

	svc := NewShareLinkService(db, db)
	ctx := context.Background()

	// Create some links
	link1, _ := svc.Create(ctx, 123, nil, 0, "Link 1")
	link2, _ := svc.Create(ctx, 456, nil, 5, "Link 2")

	links, err := svc.List(ctx)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Verify we have at least 2 links
	if len(links) < 2 {
		t.Errorf("expected at least 2 links, got %d", len(links))
	}

	// Find our links
	found1, found2 := false, false
	for _, link := range links {
		if link.Token == link1.Token {
			found1 = true
		}
		if link.Token == link2.Token {
			found2 = true
		}
	}

	if !found1 {
		t.Error("link1 not found in list")
	}
	if !found2 {
		t.Error("link2 not found in list")
	}
}

// TestGetByToken tests retrieving a share link by token.
func TestGetByToken(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory db: %v", err)
	}
	defer db.Close()

	// Create tables
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS share_links (
			token TEXT PRIMARY KEY,
			file_id INTEGER NOT NULL REFERENCES files(id),
			expires_at DATETIME,
			max_downloads INTEGER,
			download_count INTEGER NOT NULL DEFAULT 0,
			note TEXT DEFAULT '',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS files (id INTEGER PRIMARY KEY, status TEXT)`)
	if err != nil {
		t.Fatalf("failed to create files table: %v", err)
	}

	svc := NewShareLinkService(db, db)
	ctx := context.Background()

	// Create a link
	link, err := svc.Create(ctx, 123, nil, 0, "Test link")
	if err != nil {
		t.Fatalf("failed to create link: %v", err)
	}

	tests := []struct {
		name        string
		token       string
		expectError bool
	}{
		{
			name:  "found",
			token: link.Token,
		},
		{
			name:        "not found",
			token:       "nonexistent",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			found, err := svc.GetByToken(ctx, tt.token)

			if tt.expectError && err == nil {
				t.Error("expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if !tt.expectError && found != nil {
				if found.Token != tt.token {
					t.Errorf("expected token %q, got %q", tt.token, found.Token)
				}
			}
		})
	}
}