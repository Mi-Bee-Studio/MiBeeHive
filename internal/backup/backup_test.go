package backup

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

// testDB creates an in-memory SQLite database with a small test table.
func testDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("opening test db: %v", err)
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		t.Fatalf("setting WAL: %v", err)
	}
	if _, err := db.Exec("CREATE TABLE test_items (id INTEGER PRIMARY KEY, name TEXT)"); err != nil {
		t.Fatalf("creating test table: %v", err)
	}
	if _, err := db.Exec("INSERT INTO test_items (name) VALUES ('hello'), ('world')"); err != nil {
		t.Fatalf("inserting test data: %v", err)
	}
	return db
}

func TestCreateBackup(t *testing.T) {
	db := testDB(t)
	defer db.Close()

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "mibeehive.db")
	cfgPath := filepath.Join(tmpDir, "config.yaml")

	// Write a config file to copy.
	if err := os.WriteFile(cfgPath, []byte("server:\n  port: 9090\n"), 0o644); err != nil {
		t.Fatalf("writing config: %v", err)
	}

	svc := NewBackupService(db, dbPath, cfgPath, Config{
		LocalPath: filepath.Join(tmpDir, "backups"),
		Retention: 5,
		Schedule:  "03:00",
	})

	if err := svc.CreateBackup(context.Background()); err != nil {
		t.Fatalf("CreateBackup: %v", err)
	}

	// Verify backup DB file exists.
	files, err := filepath.Glob(filepath.Join(svc.localPath, "mibeehive-*.db"))
	if err != nil {
		t.Fatalf("globbing backup files: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 backup file, got %d", len(files))
	}

	// Verify config backup exists.
	cfgFiles, err := filepath.Glob(filepath.Join(svc.localPath, "config-*.yaml"))
	if err != nil {
		t.Fatalf("globbing config backups: %v", err)
	}
	if len(cfgFiles) != 1 {
		t.Fatalf("expected 1 config backup, got %d", len(cfgFiles))
	}

	// Verify the backup DB contains our data.
	backupDB, err := sql.Open("sqlite", files[0])
	if err != nil {
		t.Fatalf("opening backup db: %v", err)
	}
	defer backupDB.Close()

	var count int
	if err := backupDB.QueryRow("SELECT COUNT(*) FROM test_items").Scan(&count); err != nil {
		t.Fatalf("querying backup: %v", err)
	}
	if count != 2 {
		t.Errorf("expected 2 rows in backup, got %d", count)
	}
}

func TestBackupRotation(t *testing.T) {
	db := testDB(t)
	defer db.Close()

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "mibeehive.db")
	cfgPath := filepath.Join(tmpDir, "config.yaml")

	svc := NewBackupService(db, dbPath, cfgPath, Config{
		LocalPath: filepath.Join(tmpDir, "backups"),
		Retention: 2,
		Schedule:  "03:00",
	})

	// Create 4 backups with distinct timestamps.
	for i := 0; i < 4; i++ {
		ts := time.Date(2026, 1, i+1, 3, 0, 0, 0, time.UTC)
		name := "mibeehive-" + ts.Format("20060102-1504") + ".db"
		path := filepath.Join(svc.localPath, name)
		if err := os.MkdirAll(svc.localPath, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("fake"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	if err := svc.rotate(); err != nil {
		t.Fatalf("rotate: %v", err)
	}

	files, err := filepath.Glob(filepath.Join(svc.localPath, "mibeehive-*.db"))
	if err != nil {
		t.Fatalf("globbing: %v", err)
	}
	if len(files) != 2 {
		t.Errorf("expected 2 files after rotation, got %d: %v", len(files), files)
	}

	// The two newest should remain (Jan 3, Jan 4).
	for _, f := range files {
		base := filepath.Base(f)
		if base != "mibeehive-20260103-0300.db" && base != "mibeehive-20260104-0300.db" {
			t.Errorf("unexpected file after rotation: %s", base)
		}
	}
}

func TestBackupIntegrityCheck(t *testing.T) {
	db := testDB(t)
	defer db.Close()

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "mibeehive.db")
	cfgPath := filepath.Join(tmpDir, "config.yaml")

	svc := NewBackupService(db, dbPath, cfgPath, Config{
		LocalPath: filepath.Join(tmpDir, "backups"),
		Retention: 5,
		Schedule:  "03:00",
	})

	if err := svc.CreateBackup(context.Background()); err != nil {
		t.Fatalf("CreateBackup: %v", err)
	}

	files, _ := filepath.Glob(filepath.Join(svc.localPath, "mibeehive-*.db"))
	if len(files) != 1 {
		t.Fatalf("expected 1 backup file")
	}

	// verifyIntegrity should succeed on a valid backup.
	if err := verifyIntegrity(files[0]); err != nil {
		t.Errorf("verifyIntegrity on valid backup: %v", err)
	}

	// verifyIntegrity should fail on a corrupt file.
	corruptPath := filepath.Join(tmpDir, "corrupt.db")
	if err := os.WriteFile(corruptPath, []byte("not a sqlite db"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := verifyIntegrity(corruptPath); err == nil {
		t.Error("expected integrity check to fail on corrupt file")
	}
}

func TestConcurrentBackup(t *testing.T) {
	db := testDB(t)
	defer db.Close()

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "mibeehive.db")
	cfgPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(cfgPath, []byte("test: true\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	svc := NewBackupService(db, dbPath, cfgPath, Config{
		LocalPath: filepath.Join(tmpDir, "backups"),
		Retention: 10,
		Schedule:  "03:00",
	})

	var wg sync.WaitGroup
	var successes atomic.Int32
	var errors atomic.Int32

	// Launch 3 concurrent backups. They should serialize via mutex.
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := svc.CreateBackup(context.Background()); err != nil {
				t.Logf("backup error: %v", err)
				errors.Add(1)
			} else {
				successes.Add(1)
			}
		}()
	}
	wg.Wait()

	s := successes.Load()
	e := errors.Load()
	if s != 3 {
		t.Errorf("expected 3 successes, got %d (errors: %d)", s, e)
	}

	// All 3 backups should exist.
	files, _ := filepath.Glob(filepath.Join(svc.localPath, "mibeehive-*.db"))
	if len(files) != 3 {
		t.Errorf("expected 3 backup files, got %d", len(files))
	}
}

func TestScheduleParsing(t *testing.T) {
	tests := []struct {
		input   string
		wantH   int
		wantM   int
		wantErr bool
	}{
		{"03:00", 3, 0, false},
		{"23:59", 23, 59, false},
		{"00:00", 0, 0, false},
		{"12:30", 12, 30, false},
		{"24:00", 0, 0, true},  // hour out of range
		{"12:60", 0, 0, true},  // minute out of range
		{"3:00", 0, 0, true},   // wrong format
		{"", 0, 0, true},       // empty
		{"ab:cd", 0, 0, true},  // non-numeric
		{"12-30", 0, 0, true},  // wrong separator
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			h, m, err := parseSchedule(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if h != tt.wantH || m != tt.wantM {
				t.Errorf("got (%d, %d), want (%d, %d)", h, m, tt.wantH, tt.wantM)
			}
		})
	}
}

func TestNextTrigger(t *testing.T) {
	// Test that nextTrigger returns a future time.
	now := time.Now()
	trigger := nextTrigger(3, 0)
	if !trigger.After(now) {
		t.Errorf("nextTrigger should be in the future, got %v (now %v)", trigger, now)
	}
	if trigger.Hour() != 3 || trigger.Minute() != 0 {
		t.Errorf("nextTrigger should be 03:00, got %02d:%02d", trigger.Hour(), trigger.Minute())
	}
}

func TestConfigCopyMissing(t *testing.T) {
	db := testDB(t)
	defer db.Close()

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "mibeehive.db")
	cfgPath := filepath.Join(tmpDir, "nonexistent.yaml")

	svc := NewBackupService(db, dbPath, cfgPath, Config{
		LocalPath: filepath.Join(tmpDir, "backups"),
		Retention: 5,
		Schedule:  "03:00",
	})

	// Should succeed — config copy is non-critical.
	if err := svc.CreateBackup(context.Background()); err != nil {
		t.Fatalf("CreateBackup with missing config should not fail: %v", err)
	}

	files, _ := filepath.Glob(filepath.Join(svc.localPath, "mibeehive-*.db"))
	if len(files) != 1 {
		t.Errorf("expected 1 backup file, got %d", len(files))
	}
}

func TestStartCancellation(t *testing.T) {
	db := testDB(t)
	defer db.Close()

	tmpDir := t.TempDir()
	svc := NewBackupService(db, tmpDir, "", Config{
		LocalPath: filepath.Join(tmpDir, "backups"),
		Retention: 5,
		Schedule:  "23:59", // far future so it blocks
	})

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	err := svc.Start(ctx)
	if err == nil {
		t.Error("expected context cancellation error")
	}
	if ctx.Err() == nil {
		t.Error("expected context to be cancelled")
	}
}

func TestVacuumIntoProducesConsistentSnapshot(t *testing.T) {
	db := testDB(t)
	defer db.Close()

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "mibeehive.db")
	cfgPath := filepath.Join(tmpDir, "config.yaml")

	svc := NewBackupService(db, dbPath, cfgPath, Config{
		LocalPath: filepath.Join(tmpDir, "backups"),
		Retention: 5,
		Schedule:  "03:00",
	})

	if err := svc.CreateBackup(context.Background()); err != nil {
		t.Fatalf("CreateBackup: %v", err)
	}

	// Insert new data in the source DB.
	if _, err := db.Exec("INSERT INTO test_items (name) VALUES ('after-backup')"); err != nil {
		t.Fatal(err)
	}

	// Verify the backup does NOT contain the new row (was a snapshot).
	files, _ := filepath.Glob(filepath.Join(svc.localPath, "mibeehive-*.db"))
	if len(files) != 1 {
		t.Fatal("expected 1 backup file")
	}

	backupDB, err := sql.Open("sqlite", files[0])
	if err != nil {
		t.Fatal(err)
	}
	defer backupDB.Close()

	var count int
	if err := backupDB.QueryRow("SELECT COUNT(*) FROM test_items").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Errorf("backup should have 2 rows (snapshot), got %d", count)
	}

	// Verify source DB has 3 rows.
	if err := db.QueryRow("SELECT COUNT(*) FROM test_items").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 3 {
		t.Errorf("source should have 3 rows, got %d", count)
	}
}

func TestRotationAlsoDeletesConfigBackup(t *testing.T) {
	tmpDir := t.TempDir()
	backupDir := filepath.Join(tmpDir, "backups")
	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Create paired backup files.
	for i := 1; i <= 4; i++ {
		ts := fmt.Sprintf("202601%02d-0300", i)
		dbFile := filepath.Join(backupDir, "mibeehive-"+ts+".db")
		cfgFile := filepath.Join(backupDir, "config-"+ts+".yaml")
		os.WriteFile(dbFile, []byte("db"), 0o644)
		os.WriteFile(cfgFile, []byte("cfg"), 0o644)
	}

	db := testDB(t)
	defer db.Close()

	svc := NewBackupService(db, "", "", Config{
		LocalPath: backupDir,
		Retention: 2,
		Schedule:  "03:00",
	})

	if err := svc.rotate(); err != nil {
		t.Fatalf("rotate: %v", err)
	}

	// Should have 2 db and 2 yaml files remaining.
	dbFiles, _ := filepath.Glob(filepath.Join(backupDir, "mibeehive-*.db"))
	cfgFiles, _ := filepath.Glob(filepath.Join(backupDir, "config-*.yaml"))

	if len(dbFiles) != 2 {
		t.Errorf("expected 2 db files, got %d", len(dbFiles))
	}
	if len(cfgFiles) != 2 {
		t.Errorf("expected 2 config files, got %d", len(cfgFiles))
	}
}
