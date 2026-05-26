package backup

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Config holds backup configuration. Defined locally to avoid importing
// the project's config package (prevents circular dependencies).
type Config struct {
	LocalPath      string
	Retention      int
	Schedule       string // "HH:MM"
	RemoteURL      string // optional WebDAV URL for remote backup
	RemoteUsername string
	RemotePassword string
}

// BackupService creates and manages local database backups.
type BackupService struct {
	db             *sql.DB
	dbPath         string // absolute path to the live SQLite database
	configPath     string // absolute path to config.yaml
	localPath      string // backup output directory
	retention      int    // number of backups to keep
	schedule       string // "HH:MM"
	remoteURL      string // optional WebDAV URL
	remoteUsername string
	remotePassword string
	mu             sync.Mutex
}

// NewBackupService creates a new BackupService.
func NewBackupService(db *sql.DB, dbPath, configPath string, cfg Config) *BackupService {
	return &BackupService{
		db:             db,
		dbPath:         dbPath,
		configPath:     configPath,
		localPath:      cfg.LocalPath,
		retention:      cfg.Retention,
		schedule:       cfg.Schedule,
		remoteURL:      cfg.RemoteURL,
		remoteUsername: cfg.RemoteUsername,
		remotePassword: cfg.RemotePassword,
	}
}

// CreateBackup performs a single backup: VACUUM INTO the database, copy the
// config file, verify integrity, and rotate old backups.
func (b *BackupService) CreateBackup(ctx context.Context) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	start := time.Now()

	// 1. Ensure backup directory exists.
	if err := os.MkdirAll(b.localPath, 0o755); err != nil {
		return fmt.Errorf("creating backup directory %s: %w", b.localPath, err)
	}

	// 2. Generate unique timestamped filenames.
	now := time.Now()
	ts := now.Format("20060102-150405")
	backupDB := filepath.Join(b.localPath, "mibeehive-"+ts+".db")
	backupCfg := filepath.Join(b.localPath, "config-"+ts+".yaml")

	// If file already exists (e.g. rapid successive calls), append nanoseconds.
	if _, err := os.Stat(backupDB); err == nil {
		ts = now.Format("20060102-150405.000000000")
		backupDB = filepath.Join(b.localPath, "mibeehive-"+ts+".db")
		backupCfg = filepath.Join(b.localPath, "config-"+ts+".yaml")
	}

	// 3. VACUUM INTO — produces a consistent snapshot even under WAL mode.
	if _, err := b.db.ExecContext(ctx, "VACUUM INTO ?", backupDB); err != nil {
		return fmt.Errorf("vacuum into %s: %w", backupDB, err)
	}
	slog.Info("backup: database vacuumed", "path", backupDB)

	// 4. Copy config file (non-critical — log warning on failure).
	if err := copyFile(b.configPath, backupCfg); err != nil {
		slog.Warn("backup: config copy failed", "src", b.configPath, "dst", backupCfg, "error", err)
	} else {
		slog.Info("backup: config copied", "path", backupCfg)
	}

	// 5. Verify backup integrity.
	if err := verifyIntegrity(backupDB); err != nil {
		return fmt.Errorf("integrity check failed for %s: %w", backupDB, err)
	}
	slog.Info("backup: integrity check passed", "path", backupDB)

	// 6. Rotate old backups.
	if err := b.rotate(); err != nil {
		slog.Warn("backup: rotation failed", "error", err)
	}

	// 6.5. Remote upload (if configured).
	if b.remoteURL != "" {
		remoteFileURL := b.remoteURL
		if !strings.HasSuffix(remoteFileURL, "/") {
			remoteFileURL += "/"
		}
		remoteFileURL += filepath.Base(backupDB)
		if err := UploadBackup(ctx, backupDB, remoteFileURL, b.remoteUsername, b.remotePassword); err != nil {
			slog.Error("backup: remote upload failed", "error", err)
			// Non-fatal — local backup is still valid
		}
	}

	// 7. Report size and duration.
	var size int64
	if fi, err := os.Stat(backupDB); err == nil {
		size = fi.Size()
	}
	slog.Info("backup: complete",
		"db_path", backupDB,
		"size_bytes", size,
		"duration", time.Since(start).Round(time.Millisecond),
	)

	return nil
}

// Start runs the scheduled backup loop. It blocks until ctx is cancelled.
func (b *BackupService) Start(ctx context.Context) error {
	hour, minute, err := parseSchedule(b.schedule)
	if err != nil {
		return fmt.Errorf("parsing backup schedule: %w", err)
	}

	slog.Info("backup: scheduler started", "schedule", b.schedule)

	for {
		next := nextTrigger(hour, minute)
		delay := time.Until(next)

		slog.Info("backup: next run scheduled", "at", next.Format(time.RFC3339))

		select {
		case <-ctx.Done():
			slog.Info("backup: scheduler stopped")
			return ctx.Err()
		case <-time.After(delay):
			if err := b.CreateBackup(ctx); err != nil {
				slog.Error("backup: scheduled backup failed", "error", err)
			}
		}
	}
}

// rotate removes the oldest backup files when count exceeds retention.
// It matches files named "mibeehive-*.db" and deletes the oldest ones.
func (b *BackupService) rotate() error {
	if b.retention <= 0 {
		return nil
	}

	entries, err := filepath.Glob(filepath.Join(b.localPath, "mibeehive-*.db"))
	if err != nil {
		return fmt.Errorf("listing backup files: %w", err)
	}

	// Sort oldest-first (names contain timestamps so lexicographic works).
	sort.Strings(entries)

	if len(entries) <= b.retention {
		return nil
	}

	toDelete := entries[:len(entries)-b.retention]
	for _, path := range toDelete {
		if err := os.Remove(path); err != nil {
			slog.Warn("backup: failed to delete old backup", "path", path, "error", err)
			continue
		}
		// Also remove the corresponding config backup if it exists.
		cfgPath := strings.Replace(path, "mibeehive-", "config-", 1)
		cfgPath = strings.TrimSuffix(cfgPath, ".db") + ".yaml"
		if err := os.Remove(cfgPath); err != nil && !os.IsNotExist(err) {
			slog.Warn("backup: failed to delete old config backup", "path", cfgPath, "error", err)
		}
		slog.Info("backup: rotated old backup", "path", path)
	}

	return nil
}

// parseSchedule parses "HH:MM" into hour and minute components.
func parseSchedule(s string) (int, int, error) {
	if len(s) != 5 || s[2] != ':' {
		return 0, 0, fmt.Errorf("invalid schedule format %q, expected HH:MM", s)
	}
	h, err := strconv.Atoi(s[0:2])
	if err != nil || h < 0 || h > 23 {
		return 0, 0, fmt.Errorf("invalid hour in schedule %q", s)
	}
	m, err := strconv.Atoi(s[3:5])
	if err != nil || m < 0 || m > 59 {
		return 0, 0, fmt.Errorf("invalid minute in schedule %q", s)
	}
	return h, m, nil
}

// nextTrigger returns the next time the backup should fire.
func nextTrigger(hour, minute int) time.Time {
	now := time.Now()
	target := time.Date(now.Year(), now.Month(), now.Day(), hour, minute, 0, 0, now.Location())
	if !target.After(now) {
		target = target.Add(24 * time.Hour)
	}
	return target
}

// copyFile copies src to dst using io.Copy (memory-efficient streaming).
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("opening source %s: %w", src, err)
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("creating destination %s: %w", dst, err)
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return fmt.Errorf("copying %s to %s: %w", src, dst, err)
	}
	return out.Sync()
}

// verifyIntegrity opens the backup database and runs PRAGMA integrity_check.
func verifyIntegrity(dbPath string) error {
	db, err := sql.Open("sqlite", dbPath+"?_pragma=busy_timeout(5000)")
	if err != nil {
		return fmt.Errorf("opening backup for verification: %w", err)
	}
	defer db.Close()

	var result string
	row := db.QueryRow("PRAGMA integrity_check")
	if err := row.Scan(&result); err != nil {
		return fmt.Errorf("running integrity check: %w", err)
	}
	if result != "ok" {
		return fmt.Errorf("integrity check returned: %s", result)
	}
	return nil
}
