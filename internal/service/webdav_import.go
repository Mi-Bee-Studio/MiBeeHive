package service

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/Mi-Bee-Studio/mibeehive/internal/eventbus"
)

// ImportWebDAVFiles scans the webdav directory recursively, registering each file
// found in the files table with source_type='manual_upload'. Existing entries
// (by local_path) are skipped (idempotent).
// On completion, publishes a FilePublished event for each new file via the event bus.
func ImportWebDAVFiles(ctx context.Context, db *sql.DB, bus *eventbus.Bus, webdavDir string) (int, error) {
	checker := &sqlTokenChecker{db: db}
	imported := 0

	err := filepath.WalkDir(webdavDir, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			slog.Warn("webdav import: walkdir error", "path", path, "error", walkErr)
			return nil
		}
		// Only regular files are imported; directories and symlinks are skipped.
		if d.IsDir() {
			return nil
		}
		info, infoErr := d.Info()
		if infoErr != nil {
			slog.Warn("webdav import: stat error", "path", path, "error", infoErr)
			return nil
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return nil
		}

		absPath, absErr := filepath.Abs(path)
		if absErr != nil {
			slog.Warn("webdav import: abs error", "path", path, "error", absErr)
			return nil
		}

		// Idempotency: skip files already registered by local_path.
		var existingID int64
		existsErr := db.QueryRowContext(ctx,
			"SELECT id FROM files WHERE local_path = ?", absPath).Scan(&existingID)
		if existsErr == nil {
			// Already registered — refresh size so the registry stays in sync.
			if _, updErr := db.ExecContext(ctx,
				"UPDATE files SET size_bytes = ? WHERE id = ?", info.Size(), existingID); updErr != nil {
				slog.Warn("webdav import: updating size failed", "path", absPath, "error", updErr)
			}
			return nil
		}
		if existsErr != sql.ErrNoRows {
			slog.Warn("webdav import: lookup failed", "path", absPath, "error", existsErr)
			return nil
		}

		token, tokenErr := GeneratePublicToken(checker)
		if tokenErr != nil {
			return fmt.Errorf("webdav import: generating public_token for %q: %w", absPath, tokenErr)
		}

		res, insertErr := db.ExecContext(ctx, `
			INSERT INTO files (project_id, version, filename, os, arch, ext,
				size_bytes, download_url, local_path, checksum, status,
				source_type, category, storage_subdir, public_token)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			0, "", d.Name(), "", "", filepath.Ext(d.Name()),
			info.Size(), "", absPath, "", "complete",
			"manual_upload", "manual", "webdav", token,
		)
		if insertErr != nil {
			return fmt.Errorf("webdav import: inserting file %q: %w", absPath, insertErr)
		}
		newID, idErr := res.LastInsertId()
		if idErr != nil {
			return fmt.Errorf("webdav import: getting last insert id for %q: %w", absPath, idErr)
		}

		if bus != nil {
			bus.Publish(ctx, eventbus.FilePublished{FileID: newID})
		}
		imported++
		slog.Info("webdav import: file imported", "path", absPath, "file_id", newID)
		return nil
	})
	if err != nil {
		return imported, fmt.Errorf("webdav import: walking %q: %w", webdavDir, err)
	}
	return imported, nil
}