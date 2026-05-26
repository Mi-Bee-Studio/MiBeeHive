package backup

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path"
	"strings"
)

// UploadBackup uploads a local backup file to a WebDAV server.
// It creates the remote directory structure if needed and verifies the upload.
func UploadBackup(ctx context.Context, localPath, remoteURL, username, password string) error {
	// 1. Open local file.
	f, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("opening local file: %w", err)
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		return fmt.Errorf("stat local file: %w", err)
	}

	// 2. Ensure remote directory exists (MKCOL).
	dirURL := parentDirURL(remoteURL)
	if dirURL != "" {
		req, err := http.NewRequestWithContext(ctx, "MKCOL", dirURL, nil)
		if err != nil {
			slog.Warn("backup: remote MKCOL request creation failed", "url", dirURL, "error", err)
		} else {
			req.SetBasicAuth(username, password)
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				slog.Warn("backup: remote MKCOL failed", "url", dirURL, "error", err)
				// Non-fatal — directory may already exist.
			} else {
				resp.Body.Close()
				slog.Debug("backup: MKCOL response", "url", dirURL, "status", resp.StatusCode)
			}
		}
	}

	// 3. Upload via HTTP PUT.
	putReq, err := http.NewRequestWithContext(ctx, "PUT", remoteURL, f)
	if err != nil {
		return fmt.Errorf("creating PUT request: %w", err)
	}
	putReq.SetBasicAuth(username, password)
	putReq.ContentLength = fi.Size()

	resp, err := http.DefaultClient.Do(putReq)
	if err != nil {
		return fmt.Errorf("PUT request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("PUT returned %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	// 4. Verify via HEAD request.
	headReq, err := http.NewRequestWithContext(ctx, "HEAD", remoteURL, nil)
	if err != nil {
		return fmt.Errorf("creating HEAD request: %w", err)
	}
	headReq.SetBasicAuth(username, password)

	headResp, err := http.DefaultClient.Do(headReq)
	if err != nil {
		return fmt.Errorf("HEAD verification: %w", err)
	}
	headResp.Body.Close()

	if headResp.ContentLength != fi.Size() {
		return fmt.Errorf("size mismatch: local=%d remote=%d", fi.Size(), headResp.ContentLength)
	}

	slog.Info("backup: remote upload verified", "url", remoteURL, "size", fi.Size())
	return nil
}

// parentDirURL extracts the parent directory URL from a full file URL.
// e.g. "https://host/path/to/file.db" → "https://host/path/to"
func parentDirURL(fileURL string) string {
	schemeEnd := strings.Index(fileURL, "://")
	if schemeEnd < 0 {
		dir := path.Dir(fileURL)
		if dir == "." || dir == "/" {
			return ""
		}
		return dir
	}
	prefix := fileURL[:schemeEnd+3]
	rest := fileURL[schemeEnd+3:]
	slashIdx := strings.Index(rest, "/")
	if slashIdx < 0 {
		return "" // No path component.
	}
	hostPart := rest[:slashIdx]
	pathPart := rest[slashIdx:]
	dirPath := path.Dir(pathPart)
	if dirPath == "." || dirPath == "/" {
		return ""
	}
	return prefix + hostPart + dirPath
}
