package service

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"

	"github.com/Mi-Bee-Studio/mibeehive/internal/eventbus"
)

// RegisterFile registers a new file in the files table with a generated public_token
// and emits a FilePublished event. It is idempotent: if local_path already exists,
// it returns the existing file ID without inserting.
func RegisterFile(
	ctx context.Context,
	db *sql.DB,
	bus *eventbus.Bus,
	sourceType, category, storageSubdir, localPath string,
	projectID int64,
	filename, version string,
	sizeBytes int64,
	checksum string,
) (int64, error) {
	// Validate required inputs
	if localPath == "" {
		return 0, fmt.Errorf("local_path cannot be empty")
	}
	if sourceType == "" {
		return 0, fmt.Errorf("source_type cannot be empty")
	}
	if category == "" {
		return 0, fmt.Errorf("category cannot be empty")
	}

	// Idempotency check: skip if local_path already exists
	var existingID int64
	err := db.QueryRowContext(ctx,
		"SELECT id FROM files WHERE local_path = ?", localPath).Scan(&existingID)
	if err == nil {
		slog.Info("file registry: file already registered", "local_path", localPath, "file_id", existingID)
		return existingID, nil
	}
	if err != sql.ErrNoRows {
		return 0, fmt.Errorf("checking existing file: %w", err)
	}

	// Generate public_token
	checker := &sqlTokenChecker{db: db}
	publicToken, err := GeneratePublicToken(checker)
	if err != nil {
		return 0, fmt.Errorf("generating public_token: %w", err)
	}

	// Insert file with all required columns
	res, err := db.ExecContext(ctx, `
		INSERT INTO files (project_id, version, filename, os, arch, ext,
			size_bytes, download_url, local_path, checksum, status,
			source_type, category, storage_subdir, public_token)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		projectID, version, filename, "", "", "",
		sizeBytes, "", localPath, checksum, "complete",
		sourceType, category, storageSubdir, publicToken,
	)
	if err != nil {
		return 0, fmt.Errorf("inserting file: %w", err)
	}

	newID, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("getting last insert id: %w", err)
	}

	// Emit FilePublished event
	if bus != nil {
		bus.Publish(ctx, eventbus.FilePublished{FileID: newID})
	}

	slog.Info("file registry: file registered",
		"file_id", newID,
		"local_path", localPath,
		"source_type", sourceType,
		"category", category,
		"public_token", publicToken,
	)

	return newID, nil
}

// SoftDelete marks a file as deleted (status='deleted') and emits a FileRemoved event.
func SoftDelete(ctx context.Context, db *sql.DB, bus *eventbus.Bus, fileID int64) error {
	// Update status to 'deleted'
	res, err := db.ExecContext(ctx,
		"UPDATE files SET status = 'deleted' WHERE id = ?", fileID)
	if err != nil {
		return fmt.Errorf("updating file status: %w", err)
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("getting rows affected: %w", err)
	}

	if rowsAffected == 0 {
		slog.Warn("file registry: soft delete no rows affected", "file_id", fileID)
		return fmt.Errorf("file %d not found", fileID)
	}

	// Emit FileRemoved event
	if bus != nil {
		bus.Publish(ctx, eventbus.FileRemoved{FileID: fileID})
	}

	slog.Info("file registry: file soft deleted", "file_id", fileID)

	return nil
}