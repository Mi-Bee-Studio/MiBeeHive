package service

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	dbrepo "github.com/Mi-Bee-Studio/mibeehive/internal/db"
	"github.com/Mi-Bee-Studio/mibeehive/internal/diskutil"
	"github.com/Mi-Bee-Studio/mibeehive/internal/metrics"
	"github.com/Mi-Bee-Studio/mibeehive/internal/model"
)

// maxRetries is the number of download attempts before giving up.
const maxRetries = 3

// downloadTimeoutBounds controls per-attempt timeout scaling by file size.
// Min 5 minutes, allows 10 KB/s minimum download speed, capped at 2 hours.
const (
	downloadMinTimeout = 5 * time.Minute
	downloadMaxTimeout = 2 * time.Hour
	downloadMinSpeed   = 10 * 1024 // 10 KB/s
)

// retryBackoffs defines the delay between retries in seconds: 5s, 10s, 15s.
var retryBackoffs = []time.Duration{5 * time.Second, 10 * time.Second, 15 * time.Second}

// FileService handles downloading, streaming, and managing files on disk.
type FileService struct {
	db         *sql.DB
	fileRepo   *dbrepo.FileRepo
	resolver   *StorageResolver
	semaphore  chan struct{}
	httpClient *http.Client
	progress   sync.Map    // int64 (fileID) → *DownloadProgress
	metrics    *metrics.Metrics
	wg         sync.WaitGroup // tracks active downloads for graceful shutdown
}


// DownloadProgress tracks real-time progress of an active download.
type DownloadProgress struct {
	BytesRead  int64
	Total      int64
	Speed      int64     // bytes per second
	ETA        int64     // estimated seconds remaining (0 if unknown)
	LastBytes  int64     // bytes at last speed sample
	LastUpdate time.Time // timestamp of last speed sample
}

// GetActiveProgress returns progress for all currently downloading files.
func (s *FileService) GetActiveProgress() map[int64]*DownloadProgress {
	result := make(map[int64]*DownloadProgress)
	s.progress.Range(func(key, value any) bool {
		result[key.(int64)] = value.(*DownloadProgress)
		return true
	})
	return result
}

// NewFileService creates a new FileService with concurrency control.
func NewFileService(db *sql.DB, resolver *StorageResolver, maxConcurrent int, m *metrics.Metrics) *FileService {
	return &FileService{
		db:       db,
		fileRepo: dbrepo.NewFileRepo(db),
		resolver: resolver,
		semaphore: make(chan struct{}, maxConcurrent),
		httpClient: &http.Client{
			Transport: &http.Transport{
				ResponseHeaderTimeout: 30 * time.Second,
				IdleConnTimeout:       90 * time.Second,
			},
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 10 {
					return fmt.Errorf("too many redirects")
				}
				return nil
			},
		},
		metrics: m,
	}
}

// Shutdown cancels in-progress downloads and waits for them to finish.
// Call this during graceful shutdown after cancelling the global context.
func (s *FileService) Shutdown() {
	s.wg.Wait()
}

// DownloadFile downloads a file from its DownloadURL to LocalPath.
// It uses a temp file + atomic rename pattern, retries on network errors,
// and enforces concurrency limits via semaphore.
func (s *FileService) DownloadFile(ctx context.Context, file *model.File) error {
	s.wg.Add(1)
	defer s.wg.Done()

	// Clean up progress tracking when download finishes (success or error).
	defer s.progress.Delete(file.ID)

	// Acquire semaphore slot.
	select {
	case s.semaphore <- struct{}{}:
	case <-ctx.Done():
		return ctx.Err()
	}
	defer func() { <-s.semaphore }()

	// Ensure directory exists.
	if err := os.MkdirAll(filepath.Dir(file.LocalPath), 0755); err != nil {
		return fmt.Errorf("creating directory: %w", err)
	}

	// Check disk space before downloading.
	if file.SizeBytes > 0 {
		if err := s.CheckDiskSpace(file.SizeBytes); err != nil {
			return fmt.Errorf("disk space check failed: %w", err)
		}
	}

	// Update status to downloading.
	if err := s.fileRepo.UpdateStatus(ctx, file.ID, string(model.FileStatusDownloading), ""); err != nil {
		return fmt.Errorf("updating status to downloading: %w", err)
	}

	dir := filepath.Dir(file.LocalPath)
	filename := filepath.Base(file.LocalPath)
	tempPath := filepath.Join(dir, ".download-"+filename)

	// Track active download and duration via metrics.
	if s.metrics != nil {
		s.metrics.ActiveDownloads.Inc()
		defer s.metrics.ActiveDownloads.Dec()
	}
	start := time.Now()

	// Clean up temp file on context cancellation.
	defer func() {
		if ctx.Err() != nil {
			os.Remove(tempPath)
		}
	}()

	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			backoff := retryBackoffs[attempt-1]
			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				s.updateError(ctx, file.ID, "download cancelled during retry backoff")
				return ctx.Err()
			}
		}

		lastErr = s.downloadAttempt(ctx, file, tempPath)
		if lastErr == nil {
			break
		}

		// Do not retry on 4xx client errors.
		var httpErr *httpError
		if errors.As(lastErr, &httpErr) && httpErr.StatusCode >= 400 && httpErr.StatusCode < 500 {
			break
		}

		// Remove partial temp file before retry.
		os.Remove(tempPath)
	}

	if lastErr != nil {
		s.updateError(ctx, file.ID, lastErr.Error())
		if s.metrics != nil {
			s.metrics.DownloadTotal.WithLabelValues("error").Inc()
			s.metrics.DownloadDuration.Observe(time.Since(start).Seconds())
		}
		return fmt.Errorf("download failed after %d attempts: %w", maxRetries, lastErr)
	}

	// Verify downloaded file integrity.
	if err := s.VerifyIntegrity(tempPath); err != nil {
		s.updateError(ctx, file.ID, fmt.Sprintf("integrity check failed: %v", err))
		if s.metrics != nil {
			s.metrics.DownloadTotal.WithLabelValues("error").Inc()
			s.metrics.DownloadDuration.Observe(time.Since(start).Seconds())
		}
		return fmt.Errorf("integrity verification: %w", err)
	}

	// Atomic rename from temp to final path.
	if err := os.Rename(tempPath, file.LocalPath); err != nil {
		s.updateError(ctx, file.ID, fmt.Sprintf("rename failed: %v", err))
		if s.metrics != nil {
			s.metrics.DownloadTotal.WithLabelValues("error").Inc()
			s.metrics.DownloadDuration.Observe(time.Since(start).Seconds())
		}
		return fmt.Errorf("renaming temp file: %w", err)
	}

	// Update file size from actual downloaded size.
	if info, err := os.Stat(file.LocalPath); err == nil {
		file.SizeBytes = info.Size()
	}

	if err := s.fileRepo.UpdateStatus(ctx, file.ID, string(model.FileStatusComplete), ""); err != nil {
		return fmt.Errorf("updating status to complete: %w", err)
	}

	// Record successful download metrics.
	if s.metrics != nil {
		s.metrics.DownloadTotal.WithLabelValues("success").Inc()
		s.metrics.DownloadBytes.Add(float64(file.SizeBytes))
		s.metrics.DownloadDuration.Observe(time.Since(start).Seconds())
	}

	return nil
}

// downloadAttempt performs a single download attempt to tempPath.
// downloadAttemptTimeoutForSize calculates a per-attempt timeout based on file size.
func downloadAttemptTimeoutForSize(sizeBytes int64) time.Duration {
	if sizeBytes <= 0 {
		return downloadMinTimeout
	}
	timeout := time.Duration(sizeBytes/downloadMinSpeed) * time.Second
	if timeout < downloadMinTimeout {
		timeout = downloadMinTimeout
	}
	if timeout > downloadMaxTimeout {
		timeout = downloadMaxTimeout
	}
	return timeout
}

// progressWriter wraps an io.Writer and reports download progress to a sync.Map.
type progressWriter struct {
	w          io.Writer
	fileID     int64
	total      int64
	read       int64
	progress   *sync.Map
	lastBytes  int64
	lastUpdate time.Time
}

func (pw *progressWriter) Write(p []byte) (int, error) {
	n, err := pw.w.Write(p)
	if n > 0 {
		pw.read += int64(n)
		now := time.Now()
		var speed, eta int64
		if !pw.lastUpdate.IsZero() {
			elapsed := now.Sub(pw.lastUpdate).Seconds()
			if elapsed > 0 {
				speed = int64(float64(pw.read-pw.lastBytes) / elapsed)
				if speed > 0 && pw.total > 0 {
					eta = (pw.total - pw.read) / speed
				}
			}
		}
		pw.lastBytes = pw.read
		pw.lastUpdate = now
		pw.progress.Store(pw.fileID, &DownloadProgress{
			BytesRead:  pw.read,
			Total:      pw.total,
			Speed:      speed,
			ETA:        eta,
			LastBytes:  pw.lastBytes,
			LastUpdate: now,
		})
	}
	return n, err
}

// downloadAttempt performs a single download attempt to tempPath.
func (s *FileService) downloadAttempt(ctx context.Context, file *model.File, tempPath string) error {
	dlCtx, cancel := context.WithTimeout(ctx, downloadAttemptTimeoutForSize(file.SizeBytes))
	defer cancel()

	req, err := http.NewRequestWithContext(dlCtx, http.MethodGet, file.DownloadURL, nil)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		io.Copy(io.Discard, resp.Body)
		return &httpError{StatusCode: resp.StatusCode, URL: file.DownloadURL}
	}

	f, err := os.Create(tempPath)
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	defer f.Close()

	// Wrap writer to track download progress in real time.
	pw := &progressWriter{w: f, fileID: file.ID, total: file.SizeBytes, progress: &s.progress}
	written, err := io.Copy(pw, resp.Body)
	if err != nil {
		return fmt.Errorf("writing to temp file: %w", err)
	}

	if written == 0 {
		return fmt.Errorf("downloaded file is empty (0 bytes)")
	}

	// Verify downloaded size matches expected size.
	if file.SizeBytes > 0 && written != file.SizeBytes {
		return fmt.Errorf("size mismatch: expected %d bytes, got %d bytes", file.SizeBytes, written)
	}

	return nil
}

// httpError represents an HTTP error response.
type httpError struct {
	StatusCode int
	URL        string
}

func (e *httpError) Error() string {
	return fmt.Sprintf("HTTP %d from %s", e.StatusCode, e.URL)
}

// updateError increments the retry count for a file and marks it as permanently failed
// when maxRetries is reached. Logs DB errors but does not return them.
func (s *FileService) updateError(ctx context.Context, fileID int64, msg string) {
	retryCount, err := s.fileRepo.IncrementRetryCount(ctx, fileID, msg)
	if err != nil {
		slog.Debug("failed to increment retry count", "file_id", fileID, "error", err)
		return
	}
	if retryCount >= maxRetries {
		if err := s.fileRepo.MarkFailedPermanent(ctx, fileID); err != nil {
			slog.Debug("failed to mark file as permanently failed", "file_id", fileID, "error", err)
		}
	}
}

// StreamFile streams a local file to an HTTP response writer.
// It uses io.Copy to avoid buffering the entire file in memory.
func (s *FileService) StreamFile(w http.ResponseWriter, file *model.File) error {
	f, err := os.Open(file.LocalPath)
	if err != nil {
		return fmt.Errorf("opening file: %w", err)
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return fmt.Errorf("stating file: %w", err)
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", info.Size()))
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, file.Filename))

	if _, err := io.Copy(w, f); err != nil {
		return fmt.Errorf("streaming file: %w", err)
	}
	return nil
}

// VerifyIntegrity checks that a file exists, has non-zero size, and is a valid archive.
func (s *FileService) VerifyIntegrity(localPath string) error {
	info, err := os.Stat(localPath)
	if err != nil {
		return fmt.Errorf("file not found: %w", err)
	}
	if info.Size() == 0 {
		return fmt.Errorf("file is empty (0 bytes)")
	}

	if err := validateArchive(localPath); err != nil {
		return fmt.Errorf("archive validation failed: %w", err)
	}

	return nil
}

// validateArchive attempts to read the file as tar.gz or zip to verify it's valid.
func validateArchive(localPath string) error {
	lower := strings.ToLower(localPath)

	switch {
	case strings.HasSuffix(lower, ".tar.gz"), strings.HasSuffix(lower, ".tgz"):
		return validateTarGz(localPath)
	case strings.HasSuffix(lower, ".zip"):
		return validateZip(localPath)
	default:
		// Unknown extension — skip archive validation, just check size (already done).
		return nil
	}
}

func validateTarGz(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("not a valid gzip file: %w", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	// Read at least one header to validate the tar archive.
	_, err = tr.Next()
	if err != nil {
		if errors.Is(err, io.EOF) {
			return fmt.Errorf("tar archive is empty")
		}
		return fmt.Errorf("not a valid tar archive: %w", err)
	}
	return nil
}

func validateZip(path string) error {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return fmt.Errorf("not a valid zip file: %w", err)
	}
	defer zr.Close()

	if len(zr.File) == 0 {
		return fmt.Errorf("zip archive is empty")
	}
	return nil
}

// GetDiskUsage returns total, used, and available disk space in bytes for the
// filesystem containing basePath.
func (s *FileService) GetDiskUsage(basePath string) (total, used, avail int64, err error) {
	t, free, a, err := diskutil.Usage(basePath)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("statfs failed: %w", err)
	}

	total = int64(t)
	avail = int64(a)
	// Count root-reserved blocks as used, matching the statfs semantics:
	// used = total - available - reserved.
	used = total - avail - int64(free-a)
	return total, used, avail, nil
}

// CheckDiskSpace verifies that the filesystem has at least 110% of the
// required bytes available (10% safety margin).
func (s *FileService) CheckDiskSpace(requiredBytes int64) error {
	_, _, avail, err := s.GetDiskUsage(s.resolver.ResolveOSS())
	if err != nil {
		return err
	}

	needed := int64(float64(requiredBytes) * 1.1)
	if avail < needed {
		return fmt.Errorf("insufficient disk space: need %d bytes, have %d bytes available", needed, avail)
	}
	return nil
}
