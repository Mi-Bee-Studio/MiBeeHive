package handler

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Mi-Bee-Studio/mibeehive/internal/model"
)

// BackupHandler handles backup listing and restore operations.
type BackupHandler struct {
	backupDir       string
	dbPath          string
	requestShutdown func()
}

// NewBackupHandler creates a new BackupHandler.
// The requestShutdown function is called after a successful restore to trigger
// graceful server shutdown (systemd will restart with the restored database).
func NewBackupHandler(backupDir, dbPath string, requestShutdown func()) *BackupHandler {
	return &BackupHandler{
		backupDir:       backupDir,
		dbPath:          dbPath,
		requestShutdown: requestShutdown,
	}
}

// backupEntry represents a single backup file in the listing response.
type backupEntry struct {
	Filename string `json:"filename"`
	Size     int64  `json:"size"`
	ModTime  string `json:"mod_time"`
}

// restoreRequest is the JSON body for the restore endpoint.
type restoreRequest struct {
	Filename string `json:"filename"`
}

// ListBackups handles GET /api/v1/admin/backups.
// It lists available backup files from the backup directory.
func (h *BackupHandler) ListBackups(w http.ResponseWriter, r *http.Request) {
	entries, err := os.ReadDir(h.backupDir)
	if err != nil {
		if os.IsNotExist(err) {
			writeJSON(w, http.StatusOK, model.ApiResponse[[]backupEntry]{
				Success: true,
				Data:    []backupEntry{},
			})
			return
		}
		writeJSON(w, http.StatusInternalServerError, model.ApiResponse[any]{
			Success: false,
			Message: fmt.Sprintf("failed to read backup directory: %v", err),
		})
		return
	}

	var backups []backupEntry
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		// Only list .db and .tar.gz backup files.
		if !strings.HasSuffix(name, ".db") && !strings.HasSuffix(name, ".tar.gz") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		backups = append(backups, backupEntry{
			Filename: name,
			Size:     info.Size(),
			ModTime:  info.ModTime().Format("2006-01-02 15:04:05"),
		})
	}

	// Sort by modification time, newest first.
	sort.Slice(backups, func(i, j int) bool {
		return backups[i].ModTime > backups[j].ModTime
	})

	if backups == nil {
		backups = []backupEntry{}
	}

	writeJSON(w, http.StatusOK, model.ApiResponse[[]backupEntry]{
		Success: true,
		Data:    backups,
	})
}

// RestoreBackup handles POST /api/v1/admin/backups/restore.
// It replaces the current database with the specified backup file, then requests
// a graceful shutdown so systemd restarts the service with the restored data.
func (h *BackupHandler) RestoreBackup(w http.ResponseWriter, r *http.Request) {
	var req restoreRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, model.ApiResponse[any]{
			Success: false,
			Message: "invalid request body",
		})
		return
	}

	if req.Filename == "" {
		writeJSON(w, http.StatusBadRequest, model.ApiResponse[any]{
			Success: false,
			Message: "filename is required",
		})
		return
	}

	// Sanitize filename — prevent path traversal.
	safeName := filepath.Base(req.Filename)
	if safeName != req.Filename || strings.Contains(safeName, "..") {
		writeJSON(w, http.StatusBadRequest, model.ApiResponse[any]{
			Success: false,
			Message: "invalid filename",
		})
		return
	}

	// Verify backup file exists.
	backupPath := filepath.Join(h.backupDir, safeName)
	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		writeJSON(w, http.StatusNotFound, model.ApiResponse[any]{
			Success: false,
			Message: fmt.Sprintf("backup file not found: %s", safeName),
		})
		return
	}

	// Create safety backup of current DB.
	safetyPath := h.dbPath + ".pre-restore.bak"
	if err := copyFileAtomic(h.dbPath, safetyPath); err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ApiResponse[any]{
			Success: false,
			Message: fmt.Sprintf("failed to create safety backup: %v", err),
		})
		return
	}

	slog.Info("backup restore: safety backup created", "path", safetyPath)

	// Restore: extract or copy the backup file to replace the current DB.
	var restoreErr error
	if strings.HasSuffix(safeName, ".tar.gz") {
		restoreErr = h.restoreFromTarGz(backupPath)
	} else {
		restoreErr = h.restoreFromDB(backupPath)
	}

	if restoreErr != nil {
		// Attempt to restore the safety backup.
		slog.Error("backup restore: failed, attempting to restore safety backup", "error", restoreErr)
		if recoverErr := copyFileAtomic(safetyPath, h.dbPath); recoverErr != nil {
			slog.Error("backup restore: safety recovery also failed", "error", recoverErr)
		} else {
			slog.Info("backup restore: safety backup restored")
		}
		writeJSON(w, http.StatusInternalServerError, model.ApiResponse[any]{
			Success: false,
			Message: fmt.Sprintf("restore failed: %v", restoreErr),
		})
		return
	}

	slog.Info("backup restore: completed successfully, exiting for restart", "backup", safeName)

	// Respond before exiting so the client gets the response.
	writeJSON(w, http.StatusOK, model.ApiResponse[any]{
		Success: true,
		Message: "restore completed, server restarting",
	})

	// Trigger graceful shutdown — systemd will restart with restored data.
	if h.requestShutdown != nil {
		go h.requestShutdown()
	}
}

// restoreFromDB copies a raw .db backup file to replace the current database.
func (h *BackupHandler) restoreFromDB(backupPath string) error {
	return copyFileAtomic(backupPath, h.dbPath)
}

// restoreFromTarGz extracts the .db file from a .tar.gz archive and replaces the current database.
func (h *BackupHandler) restoreFromTarGz(backupPath string) error {
	f, err := os.Open(backupPath)
	if err != nil {
		return fmt.Errorf("opening tar.gz: %w", err)
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("creating gzip reader: %w", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("reading tar: %w", err)
		}

		// Look for .db files in the archive.
		if hdr.Typeflag != tar.TypeReg {
			continue
		}
		name := filepath.Base(hdr.Name)
		if !strings.HasSuffix(name, ".db") {
			continue
		}

		// Write to temp file first, then rename atomically.
		tmpPath := h.dbPath + ".restoring"
		out, err := os.Create(tmpPath)
		if err != nil {
			return fmt.Errorf("creating temp restore file: %w", err)
		}

		if _, err := io.Copy(out, tr); err != nil {
			out.Close()
			os.Remove(tmpPath)
			return fmt.Errorf("extracting db from tar: %w", err)
		}
		out.Close()

		if err := os.Rename(tmpPath, h.dbPath); err != nil {
			os.Remove(tmpPath)
			return fmt.Errorf("replacing database file: %w", err)
		}

		return nil
	}

	return fmt.Errorf("no .db file found in archive")
}

// copyFileAtomic copies src to dst via temp file + rename for atomicity.
func copyFileAtomic(src, dst string) error {
	tmpPath := dst + ".tmp"
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("opening source %s: %w", src, err)
	}
	defer in.Close()

	out, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("creating temp file %s: %w", tmpPath, err)
	}

	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("copying %s to %s: %w", src, tmpPath, err)
	}

	if err := out.Sync(); err != nil {
		out.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("syncing temp file: %w", err)
	}
	out.Close()

	if err := os.Rename(tmpPath, dst); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("renaming temp to %s: %w", dst, err)
	}

	return nil
}
