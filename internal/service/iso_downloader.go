package service

import (
 "context"
 "crypto/sha256"
 "encoding/hex"
 "errors"
 "fmt"
 "io"
 "log/slog"
 "net/http"
 "net/url"
 "os"
 "path/filepath"
 "strings"
 "sync"
 "syscall"
 "time"

	"github.com/Mi-Bee-Studio/mibeehive/internal/metrics"
)

// ISOInfo holds metadata about an ISO file on disk.
type ISOInfo struct {
	Name      string `json:"name"`
	SizeBytes int64  `json:"size_bytes"`
	ModTime   string `json:"mod_time"`
}

// isoDownloadTimeout is the maximum duration for a single ISO download.
const isoDownloadTimeout = 2 * time.Hour

// InsufficientStorageError indicates not enough disk space for the download.
type InsufficientStorageError struct {
	Available uint64
	Needed    uint64
}

func (e *InsufficientStorageError) Error() string {
	return fmt.Sprintf("insufficient disk space: %d bytes available, need %d bytes (with 10%% margin)", e.Available, e.Needed)
}

// StaleISOCheck is used to check for stale ISO download entries.
type StaleISOCheck struct {
	ID       int64
	Filename string
	Status   string
}

// ISOService handles downloading, listing, and deleting ISO files
// for OS installation provisioning.
type ISOService struct {
	resolver   *StorageResolver
	semaphore  chan struct{}
	httpClient *http.Client
	progress   sync.Map    // string (filename) → *DownloadProgress
	metrics    *metrics.Metrics
	wg         sync.WaitGroup // tracks active downloads for graceful shutdown
}


// GetActiveProgress returns download progress for all active ISO downloads.
func (s *ISOService) GetActiveProgress() map[string]*DownloadProgress {
	result := make(map[string]*DownloadProgress)
	s.progress.Range(func(key, value any) bool {
		result[key.(string)] = value.(*DownloadProgress)
		return true
	})
	return result
}
// NewISOService creates a new ISOService with the given storage resolver.
// maxConcurrent limits the number of simultaneous ISO downloads.
func NewISOService(resolver *StorageResolver, maxConcurrent int, m *metrics.Metrics) *ISOService {
	return &ISOService{
		resolver:  resolver,
		semaphore: make(chan struct{}, maxConcurrent),
		httpClient: &http.Client{
			Timeout: 0, // No timeout for large ISO downloads; controlled by context.
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

// Shutdown waits for all active ISO downloads to finish.
func (s *ISOService) Shutdown() {
	s.wg.Wait()
}

// isoDir returns the full path to the os-install directory.
func (s *ISOService) isoDir() string {
	return s.resolver.ResolveISO()
}

// ResetStaleDownloads checks downloading entries and determines which are stale.
// For each stale entry, it calls the resetFn callback to update the DB status.
// Returns the number of entries that need reset.
func (s *ISOService) ResetStaleDownloads(ctx context.Context, entries []StaleISOCheck, resetFn func(id int64, status string) error) (int, error) {
	dir := s.isoDir()
	var reset int
	for _, e := range entries {
		if e.Status != "downloading" {
			continue
		}
		finalPath := filepath.Join(dir, e.Filename)
		tempPath := finalPath + ".tmp"
		// If final file exists, download completed but status wasn't updated.
		if info, err := os.Stat(finalPath); err == nil && info.Size() > 0 {
			os.Remove(tempPath)
			if err := resetFn(e.ID, "downloaded"); err != nil {
				return reset, fmt.Errorf("marking stale entry %d as downloaded: %w", e.ID, err)
			}
			reset++
			continue
		}
		// If temp file doesn't exist, the download was interrupted — reset to pending.
		if _, err := os.Stat(tempPath); os.IsNotExist(err) {
			if err := resetFn(e.ID, "pending"); err != nil {
				return reset, fmt.Errorf("resetting stale entry %d to pending: %w", e.ID, err)
			}
			reset++
		}
	}
	return reset, nil
}

// validateFilename rejects path traversal attempts.
func validateFilename(filename string) error {
	if filename == "" {
		return fmt.Errorf("filename is empty")
	}
	if strings.Contains(filename, "..") || strings.ContainsAny(filename, "/\\") {
		return fmt.Errorf("invalid filename %q: path separators not allowed", filename)
	}
	return nil
}

func (s *ISOService) DownloadISO(ctx context.Context, filename string, rawURL string, expectedSHA256 string) error {
	s.wg.Add(1)
	defer s.wg.Done()

	if err := validateFilename(filename); err != nil {
		return err
	}
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return fmt.Errorf("unsupported URL scheme %q", parsedURL.Scheme)
	}

	// Wrap context with 2-hour download timeout.
	dlCtx, cancel := context.WithTimeout(ctx, isoDownloadTimeout)
	defer cancel()


	s.semaphore <- struct{}{}
	defer func() { <-s.semaphore }()

	dir := s.isoDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating os-install directory: %w", err)
	}

	finalPath := filepath.Join(dir, filename)
	tempPath := finalPath + ".tmp"

	// Check for existing temp file to support resume.
	var existingSize int64
	if info, err := os.Stat(tempPath); err == nil {
		existingSize = info.Size()
		slog.Info("found existing temp file for resume", "filename", filename, "existing_bytes", existingSize)
	}

	// Build request, optionally with Range header for resume.
	req, err := http.NewRequestWithContext(dlCtx, http.MethodGet, rawURL, nil)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	if existingSize > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", existingSize))
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("download request failed: %w", err)
	}
	defer resp.Body.Close()

	// Determine total size and whether we're resuming.
	var totalBytes int64
	var resumeOffset int64
	resuming := false

	switch resp.StatusCode {
	case http.StatusPartialContent:
		// Server supports Range — resume download.
		resuming = true
		resumeOffset = existingSize
		if resp.ContentLength > 0 {
			totalBytes = resumeOffset + resp.ContentLength
		} else {
			totalBytes = -1
		}
		slog.Info("resuming ISO download", "filename", filename, "offset", resumeOffset, "remaining", resp.ContentLength)

	case http.StatusOK:
		// Server doesn't support Range or full response — start fresh.
		totalBytes = resp.ContentLength
		if existingSize > 0 {
			slog.Info("server does not support Range, starting fresh", "filename", filename)
			if err := os.Remove(tempPath); err != nil && !os.IsNotExist(err) {
				slog.Warn("failed to remove temp file", "path", tempPath, "error", err)
			}
			existingSize = 0
		}

	default:
		io.Copy(io.Discard, resp.Body)
		return fmt.Errorf("HTTP %d from %s", resp.StatusCode, rawURL)
	}

	// Disk space check using Content-Length.
	neededBytes := calcNeededBytes(totalBytes, resumeOffset)
	if err := s.checkDiskSpace(neededBytes); err != nil {
		return err
	}

	// Open temp file for writing.
	var f *os.File
	if resuming {
		f, err = os.OpenFile(tempPath, os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return fmt.Errorf("opening temp file for append: %w", err)
		}
	} else {
		f, err = os.Create(tempPath)
		if err != nil {
			return fmt.Errorf("creating temp file: %w", err)
		}
	}
	defer f.Close()

	// Validate temp file size before resuming.
	if resuming {
		if actualSize, err := f.Seek(0, io.SeekEnd); err != nil {
			os.Remove(tempPath)
			return fmt.Errorf("seeking temp file: %w", err)
		} else if actualSize != existingSize {
			f.Close()
			os.Remove(tempPath)
			return fmt.Errorf("temp file size mismatch: expected %d, got %d", existingSize, actualSize)
		}
	}

	// Wrap writer to track download progress in real time.
	pw := &isoProgressWriter{w: f, filename: filename, total: totalBytes, read: resumeOffset, progress: &s.progress}
	written, err := io.Copy(pw, resp.Body)
	if err != nil {
		// Only remove temp file on corruption, not transient errors.
		if isCorruptionError(err) {
			os.Remove(tempPath)
		}
		return fmt.Errorf("writing ISO to disk: %w", err)
	}

	if written == 0 && resumeOffset == 0 {
		os.Remove(tempPath)
		return fmt.Errorf("downloaded file is empty (0 bytes)")
	}

	if err := os.Rename(tempPath, finalPath); err != nil {
		os.Remove(tempPath)
		return fmt.Errorf("renaming temp file: %w", err)
	}

	// Verify SHA256 checksum if provided.
	if expectedSHA256 != "" {
		if err := verifyChecksum(finalPath, expectedSHA256); err != nil {
			os.Remove(finalPath)
			return fmt.Errorf("checksum verification failed: %w", err)
			}
		slog.Info("ISO checksum verified", "filename", filename, "sha256", expectedSHA256)
	}

	finalSize := resumeOffset + written
	slog.Info("ISO download complete", "filename", filename, "size_bytes", finalSize, "resumed", resuming)
	if s.metrics != nil {
		s.metrics.ISODownloadsTotal.WithLabelValues("unknown", "success").Inc()
	}
	return nil
}

// isoProgressWriter wraps an io.Writer and reports ISO download progress.
type isoProgressWriter struct {
	w          io.Writer
	filename   string
	total      int64
	read       int64
	progress   *sync.Map
	lastBytes  int64
	lastUpdate time.Time
}

func (pw *isoProgressWriter) Write(p []byte) (int, error) {
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
		pw.progress.Store(pw.filename, &DownloadProgress{
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

// ListISOs returns metadata for all files in the os-install directory.
func (s *ISOService) ListISOs() ([]ISOInfo, error) {
	dir := s.isoDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []ISOInfo{}, nil
		}
		return nil, fmt.Errorf("reading os-install directory: %w", err)
	}

	var isos []ISOInfo
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		// Skip temp files from in-progress downloads.
		if strings.HasSuffix(entry.Name(), ".tmp") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		isos = append(isos, ISOInfo{
			Name:      entry.Name(),
			SizeBytes: info.Size(),
			ModTime:   info.ModTime().Format(time.RFC3339),
		})
	}
	if isos == nil {
		isos = []ISOInfo{}
	}
	return isos, nil
}

// DeleteISO removes an ISO file from the os-install directory.
func (s *ISOService) DeleteISO(filename string) error {
	if err := validateFilename(filename); err != nil {
		return err
	}
	fullPath := filepath.Join(s.isoDir(), filename)
	if err := os.Remove(fullPath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("ISO file %q not found", filename)
		}
		return fmt.Errorf("deleting ISO: %w", err)
	}
	slog.Info("ISO deleted", "filename", filename)
	return nil
}

// DiskAvailable returns available disk space in bytes on the partition
// containing the os-install directory.
func (s *ISOService) DiskAvailable() (uint64, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(s.resolver.ResolveISO(), &stat); err != nil {
		return 0, fmt.Errorf("statfs failed: %w", err)
	}
	return uint64(stat.Bavail) * uint64(stat.Bsize), nil
}

// checkDiskSpace verifies that the filesystem has at least 110% of the
// needed bytes available (10% safety margin).
func (s *ISOService) checkDiskSpace(neededBytes uint64) error {
	avail, err := s.DiskAvailable()
	if err != nil {
		return fmt.Errorf("checking disk space: %w", err)
	}
	needed := uint64(float64(neededBytes) * 1.1)
	if avail < needed {
		return &InsufficientStorageError{Available: avail, Needed: needed}
	}
	return nil
}

// calcNeededBytes returns the number of bytes still needed for download.
// If totalBytes is unknown (-1), falls back to 2GB minimum.
// Subtracts resumeOffset if resuming.
func calcNeededBytes(totalBytes, resumeOffset int64) uint64 {
	const fallbackBytes uint64 = 2 * 1024 * 1024 * 1024 // 2GB fallback
	if totalBytes <= 0 {
		return fallbackBytes
	}
	remaining := totalBytes - resumeOffset
	if remaining < 0 {
		return fallbackBytes
	}
	return uint64(remaining)
}

// isCorruptionError returns true if the error indicates file corruption
// or a non-transient problem that justifies deleting the temp file.
// Context cancellation, deadline exceeded, and timeout are transient —
// the temp file is kept for resume.
func isCorruptionError(err error) bool {
	if err == nil {
		return false
	}
	// Transient errors — keep temp file for potential resume.
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	return true
}
// verifyChecksum computes the SHA256 hash of the file at filePath and compares
// it with the expected hex-encoded hash. Returns an error on mismatch.
func verifyChecksum(filePath, expectedSHA256 string) error {
	f, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("opening file for checksum: %w", err)
	}
	defer f.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, f); err != nil {
		return fmt.Errorf("hashing file: %w", err)
	}

	actual := hex.EncodeToString(hasher.Sum(nil))
	if actual != expectedSHA256 {
		return fmt.Errorf("checksum mismatch: expected %s, got %s", expectedSHA256, actual)
	}
	return nil
}

// StreamISO streams an ISO file to the HTTP response writer.
// It opens the file from the os-install directory, sets appropriate
// headers (Content-Type, Content-Length, Content-Disposition), and
// copies the file contents using io.Copy.
func (s *ISOService) StreamISO(w http.ResponseWriter, filename string) error {
	if err := validateFilename(filename); err != nil {
		return err
	}

	f, err := os.Open(filepath.Join(s.isoDir(), filename))
	if err != nil {
		return fmt.Errorf("opening ISO file: %w", err)
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return fmt.Errorf("stating ISO file: %w", err)
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", info.Size()))
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))

	if _, err := io.Copy(w, f); err != nil {
		return fmt.Errorf("streaming ISO file: %w", err)
	}
	return nil
}
