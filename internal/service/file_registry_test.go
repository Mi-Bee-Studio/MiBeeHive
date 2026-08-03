package service

import (
	"context"
	"database/sql"
	"testing"

	"github.com/Mi-Bee-Studio/mibeehive/internal/eventbus"
)

// mockDBChecker is a test double for DBChecker that always reports tokens don't exist.
type mockDBChecker struct {
	tokens map[string]bool
}

func (m *mockDBChecker) TokenExists(ctx context.Context, token string) (bool, error) {
	if m.tokens == nil {
		return false, nil
	}
	return m.tokens[token], nil
}

// testTokenChecker wraps a *sql.DB to implement DBChecker for in-memory tests.
type testTokenChecker struct {
	db *sql.DB
}

func (t *testTokenChecker) TokenExists(ctx context.Context, token string) (bool, error) {
	var one int
	err := t.db.QueryRowContext(ctx,
		"SELECT 1 FROM files WHERE public_token = ?", token).Scan(&one)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// setupFileRegistryDB creates an in-memory SQLite database with the files table for registry tests.
func setupFileRegistryDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("opening in-memory db: %v", err)
	}

	// Create files table with required schema
	_, err = db.Exec(`
		CREATE TABLE files (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			project_id INTEGER NOT NULL,
			version TEXT,
			filename TEXT,
			os TEXT,
			arch TEXT,
			ext TEXT,
			size_bytes INTEGER,
			download_url TEXT,
			local_path TEXT,
			checksum TEXT,
			status TEXT,
			error_message TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			retry_count INTEGER DEFAULT 0,
			last_attempt_at DATETIME,
			source_type TEXT,
			category TEXT,
			storage_subdir TEXT,
			public_token TEXT UNIQUE
		)
	`)
	if err != nil {
		db.Close()
		t.Fatalf("creating files table: %v", err)
	}

	t.Cleanup(func() { db.Close() })
	return db
}

// drainChannel drains any pending events from a channel to prevent blocking.
func drainChannel(ch <-chan eventbus.Event) {
	for {
		select {
		case <-ch:
		default:
			return
		}
	}
}

func TestRegisterFile(t *testing.T) {
	tests := []struct {
		name           string
		sourceType     string
		category       string
		storageSubdir  string
		localPath      string
		projectID      int64
		filename       string
		version        string
		sizeBytes      int64
		checksum       string
		wantErr        bool
		wantPublicToken bool
	}{
		{
			name:           "github source",
			sourceType:     "github",
			category:       "ops_tools",
			storageSubdir:  "github",
			localPath:      "/storage/github/test-tool-v1.0.0.tar.gz",
			projectID:      1,
			filename:       "test-tool-v1.0.0.tar.gz",
			version:        "v1.0.0",
			sizeBytes:      1024,
			checksum:       "abc123",
			wantErr:        false,
			wantPublicToken: true,
		},
		{
			name:           "iso source",
			sourceType:     "iso",
			category:       "os_images",
			storageSubdir:  "iso",
			localPath:      "/storage/iso/debian-12.iso",
			projectID:      0,
			filename:       "debian-12.iso",
			version:        "12.0.0",
			sizeBytes:      2048,
			checksum:       "def456",
			wantErr:        false,
			wantPublicToken: true,
		},
		{
			name:           "manual_upload source",
			sourceType:     "manual_upload",
			category:       "manual",
			storageSubdir:  "webdav",
			localPath:      "/storage/webdav/manual-file.txt",
			projectID:      0,
			filename:       "manual-file.txt",
			version:        "",
			sizeBytes:      512,
			checksum:       "ghi789",
			wantErr:        false,
			wantPublicToken: true,
		},
		{
			name:           "empty local_path",
			sourceType:     "github",
			category:       "ops_tools",
			storageSubdir:  "github",
			localPath:      "",
			projectID:      1,
			filename:       "test.tar.gz",
			version:        "v1.0.0",
			sizeBytes:      1024,
			checksum:       "abc123",
			wantErr:        true,
			wantPublicToken: false,
		},
		{
			name:           "empty source_type",
			sourceType:     "",
			category:       "ops_tools",
			storageSubdir:  "github",
			localPath:      "/storage/github/test.tar.gz",
			projectID:      1,
			filename:       "test.tar.gz",
			version:        "v1.0.0",
			sizeBytes:      1024,
			checksum:       "abc123",
			wantErr:        true,
			wantPublicToken: false,
		},
		{
			name:           "empty category",
			sourceType:     "github",
			category:       "",
			storageSubdir:  "github",
			localPath:      "/storage/github/test.tar.gz",
			projectID:      1,
			filename:       "test.tar.gz",
			version:        "v1.0.0",
			sizeBytes:      1024,
			checksum:       "abc123",
			wantErr:        true,
			wantPublicToken: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			db := setupFileRegistryDB(t)
			bus := eventbus.NewBus(10)
			defer bus.Close()

			// Subscribe to capture events
			eventCh := bus.Subscribe(eventbus.TagFilePublished)
			defer drainChannel(eventCh)

			fileID, err := RegisterFile(
				ctx, db, bus,
				tt.sourceType, tt.category, tt.storageSubdir, tt.localPath,
				tt.projectID, tt.filename, tt.version, tt.sizeBytes, tt.checksum,
			)

			if (err != nil) != tt.wantErr {
				t.Errorf("RegisterFile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if fileID == 0 {
					t.Error("RegisterFile() returned zero file ID on success")
					return
				}

				// Verify file was inserted with correct values
				var (
					gotID, gotProjectID, gotSize int64
					gotSourceType, gotCategory, gotStorageSubdir, gotLocalPath string
					gotFilename, gotVersion, gotChecksum, gotStatus string
					gotPublicToken sql.NullString
				)
				err := db.QueryRowContext(ctx, `
					SELECT id, project_id, source_type, category, storage_subdir,
						   local_path, filename, version, size_bytes, checksum,
						   status, public_token
					FROM files WHERE id = ?`, fileID).Scan(
					&gotID, &gotProjectID, &gotSourceType, &gotCategory, &gotStorageSubdir,
					&gotLocalPath, &gotFilename, &gotVersion, &gotSize, &gotChecksum,
					&gotStatus, &gotPublicToken,
				)
				if err != nil {
					t.Fatalf("querying inserted file: %v", err)
				}

				if gotSourceType != tt.sourceType {
					t.Errorf("source_type = %q, want %q", gotSourceType, tt.sourceType)
				}
				if gotCategory != tt.category {
					t.Errorf("category = %q, want %q", gotCategory, tt.category)
				}
				if gotStorageSubdir != tt.storageSubdir {
					t.Errorf("storage_subdir = %q, want %q", gotStorageSubdir, tt.storageSubdir)
				}
				if gotLocalPath != tt.localPath {
					t.Errorf("local_path = %q, want %q", gotLocalPath, tt.localPath)
				}
				if gotStatus != "complete" {
					t.Errorf("status = %q, want 'complete'", gotStatus)
				}

				if tt.wantPublicToken {
					if !gotPublicToken.Valid || gotPublicToken.String == "" {
						t.Error("public_token was not generated")
					}
				}

				// Verify FilePublished event was emitted
				select {
				case e := <-eventCh:
					if published, ok := e.(eventbus.FilePublished); ok {
						if published.FileID != fileID {
							t.Errorf("FilePublished.FileID = %d, want %d", published.FileID, fileID)
						}
					} else {
						t.Errorf("got wrong event type: %T", e)
					}
				default:
					t.Error("no FilePublished event emitted")
				}
			}
		})
	}
}

func TestRegisterFileExistingPath(t *testing.T) {
	ctx := context.Background()
	db := setupFileRegistryDB(t)
	bus := eventbus.NewBus(10)
	defer bus.Close()

	localPath := "/storage/github/test-existing.tar.gz"

	// First registration should succeed
	fileID1, err := RegisterFile(
		ctx, db, bus,
		"github", "ops_tools", "github", localPath,
		1, "test-existing.tar.gz", "v1.0.0", 1024, "abc123",
	)
	if err != nil {
		t.Fatalf("first RegisterFile() failed: %v", err)
	}

	// Second registration with same local_path should return existing ID (idempotent)
	fileID2, err := RegisterFile(
		ctx, db, bus,
		"github", "ops_tools", "github", localPath,
		1, "test-existing.tar.gz", "v1.0.0", 1024, "abc123",
	)
	if err != nil {
		t.Fatalf("second RegisterFile() failed: %v", err)
	}

	if fileID2 != fileID1 {
		t.Errorf("RegisterFile() idempotency: got ID %d, want existing ID %d", fileID2, fileID1)
	}

	// Verify only one row exists
	var count int
	err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM files WHERE local_path = ?", localPath).Scan(&count)
	if err != nil {
		t.Fatalf("counting files: %v", err)
	}
	if count != 1 {
		t.Errorf("file count = %d, want 1", count)
	}
}

func TestSoftDelete(t *testing.T) {
	ctx := context.Background()
	db := setupFileRegistryDB(t)
	bus := eventbus.NewBus(10)
	defer bus.Close()

	// Register a file first
	fileID, err := RegisterFile(
		ctx, db, bus,
		"github", "ops_tools", "github", "/storage/github/test-softdelete.tar.gz",
		1, "test-softdelete.tar.gz", "v1.0.0", 1024, "abc123",
	)
	if err != nil {
		t.Fatalf("RegisterFile() failed: %v", err)
	}

	// Subscribe to capture FileRemoved event
	eventCh := bus.Subscribe(eventbus.TagFileRemoved)
	defer drainChannel(eventCh)

	// Soft delete the file
	err = SoftDelete(ctx, db, bus, fileID)
	if err != nil {
		t.Fatalf("SoftDelete() failed: %v", err)
	}

	// Verify status was changed to 'deleted'
	var status string
	err = db.QueryRowContext(ctx, "SELECT status FROM files WHERE id = ?", fileID).Scan(&status)
	if err != nil {
		t.Fatalf("querying file status: %v", err)
	}
	if status != "deleted" {
		t.Errorf("status = %q, want 'deleted'", status)
	}

	// Verify FileRemoved event was emitted
	select {
	case e := <-eventCh:
		if removed, ok := e.(eventbus.FileRemoved); ok {
			if removed.FileID != fileID {
				t.Errorf("FileRemoved.FileID = %d, want %d", removed.FileID, fileID)
			}
		} else {
			t.Errorf("got wrong event type: %T", e)
		}
	default:
		t.Error("no FileRemoved event emitted")
	}
}

func TestSoftDeleteNotFound(t *testing.T) {
	ctx := context.Background()
	db := setupFileRegistryDB(t)
	bus := eventbus.NewBus(10)
	defer bus.Close()

	// Try to delete non-existent file
	err := SoftDelete(ctx, db, bus, 99999)
	if err == nil {
		t.Error("SoftDelete() should return error for non-existent file")
	}
}