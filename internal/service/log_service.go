package service

import (
	"bufio"
	"context"
	"database/sql"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/Mi-Bee-Studio/mibeehive/internal/model"
)

// LogService aggregates logs from multiple sources: crawl logs, download logs, and app logs.
type LogService struct {
	db          *sql.DB
	logFilePath string
}

// NewLogService creates a new LogService.
// logFilePath is optional — if empty, app logs will return empty results.
func NewLogService(db *sql.DB, logFilePath string) *LogService {
	return &LogService{
		db:          db,
		logFilePath: logFilePath,
	}
}

// GetCrawlLogs returns recent crawl log entries.
func (s *LogService) GetCrawlLogs(ctx context.Context, limit, offset int) ([]model.LogEntry, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	if offset < 0 {
		offset = 0
	}

	rows, err := s.db.QueryContext(ctx,
		`SELECT cl.id, cl.started_at, cl.status,
		        cl.versions_found, cl.files_downloaded, cl.error_message,
		        COALESCE(p.name, '') as project_name
		 FROM crawl_logs cl
		 LEFT JOIN projects p ON cl.project_id = p.id
		 ORDER BY cl.started_at DESC LIMIT ? OFFSET ?`, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("querying crawl logs: %w", err)
	}
	defer rows.Close()

	var entries []model.LogEntry
	for rows.Next() {
		var id int64
		var startedAt, status string
		var versionsFound, filesDownloaded int
		var errorMsg, projectName string

		if err := rows.Scan(&id, &startedAt, &status,
			&versionsFound, &filesDownloaded, &errorMsg, &projectName); err != nil {
			return nil, fmt.Errorf("scanning crawl log: %w", err)
		}

		level := "info"
		msg := fmt.Sprintf("crawl %s: found %d versions, downloaded %d files",
			status, versionsFound, filesDownloaded)
		if status == "error" {
			level = "error"
			msg = fmt.Sprintf("crawl error: %s", errorMsg)
		}

		entries = append(entries, model.LogEntry{
			ID:        strconv.FormatInt(id, 10),
			Type:      "crawl",
			Timestamp: startedAt,
			Level:     level,
			Message:   msg,
			Source:    projectName,
		})
	}
	if entries == nil {
		entries = []model.LogEntry{}
	}
	return entries, rows.Err()
}

// GetCrawlLogsPaginated returns crawl log entries with total count.
func (s *LogService) GetCrawlLogsPaginated(ctx context.Context, limit, offset int) ([]model.LogEntry, int, error) {
	var total int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM crawl_logs`).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("counting crawl logs: %w", err)
	}
	entries, err := s.GetCrawlLogs(ctx, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	return entries, total, nil
}

// GetDownloadLogs returns recent download log entries from the files table.
func (s *LogService) GetDownloadLogs(ctx context.Context, limit, offset int) ([]model.LogEntry, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	if offset < 0 {
		offset = 0
	}

	rows, err := s.db.QueryContext(ctx,
		`SELECT f.id, f.filename, f.status, f.error_message, f.created_at,
		        COALESCE(p.name, '') as project_name
		 FROM files f
		 LEFT JOIN projects p ON f.project_id = p.id
		 ORDER BY f.created_at DESC LIMIT ? OFFSET ?`, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("querying download logs: %w", err)
	}
	defer rows.Close()

	var entries []model.LogEntry
	for rows.Next() {
		var id int64
		var fname, status, errorMsg, createdAt, projectName string

		if err := rows.Scan(&id, &fname, &status, &errorMsg, &createdAt, &projectName); err != nil {
			return nil, fmt.Errorf("scanning download log: %w", err)
		}

		level := "info"
		msg := fmt.Sprintf("download %s: %s", status, fname)
		if status == "error" || status == "failed_permanent" {
			level = "error"
			if errorMsg != "" {
				msg = fmt.Sprintf("download %s: %s - %s", status, fname, errorMsg)
			}
		}

		entries = append(entries, model.LogEntry{
			ID:        strconv.FormatInt(id, 10),
			Type:      "download",
			Timestamp: createdAt,
			Level:     level,
			Message:   msg,
			Source:    projectName,
		})
	}
	if entries == nil {
		entries = []model.LogEntry{}
	}
	return entries, rows.Err()
}

// GetDownloadLogsPaginated returns download log entries with total count.
func (s *LogService) GetDownloadLogsPaginated(ctx context.Context, limit, offset int) ([]model.LogEntry, int, error) {
	var total int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM files`).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("counting download logs: %w", err)
	}
	entries, err := s.GetDownloadLogs(ctx, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	return entries, total, nil
}

// GetAppLogs reads application logs from the configured log file.
// Returns empty slice if the file is not available.
func (s *LogService) GetAppLogs(ctx context.Context, limit int) ([]model.LogEntry, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}

	if s.logFilePath == "" {
		return []model.LogEntry{}, nil
	}

	f, err := os.Open(s.logFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			return []model.LogEntry{}, nil
		}
		return nil, fmt.Errorf("opening log file %s: %w", s.logFilePath, err)
	}
	defer f.Close()

	// Read the last N lines using a ring buffer approach.
	// This avoids buffering the entire file in memory.
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 64*1024)

	lines := make([]string, 0, limit+1)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
		if len(lines) > limit {
			lines = lines[1:]
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading log file: %w", err)
	}

	entries := make([]model.LogEntry, 0, len(lines))
	for i, line := range lines {
		entries = append(entries, parseAppLogLine(line, i))
	}
	return entries, nil
}

// GetRecentLogs returns log entries for the given type.
func (s *LogService) GetRecentLogs(ctx context.Context, logType string, limit, offset int) ([]model.LogEntry, error) {
	switch logType {
	case "crawl":
		return s.GetCrawlLogs(ctx, limit, offset)
	case "download":
		return s.GetDownloadLogs(ctx, limit, offset)
	case "app":
		// App logs don't support offset — just use limit.
		return s.GetAppLogs(ctx, limit)
	default:
		return nil, fmt.Errorf("unknown log type: %q", logType)
	}
}

// GetRecentLogsPaginated returns log entries with total count for pagination.
func (s *LogService) GetRecentLogsPaginated(ctx context.Context, logType string, limit, offset int) ([]model.LogEntry, int, error) {
	switch logType {
	case "crawl":
		return s.GetCrawlLogsPaginated(ctx, limit, offset)
	case "download":
		return s.GetDownloadLogsPaginated(ctx, limit, offset)
	case "app":
		entries, err := s.GetAppLogs(ctx, limit)
		return entries, len(entries), err
	default:
		return nil, 0, fmt.Errorf("unknown log type: %q", logType)
	}
}

// parseAppLogLine parses a log line into a LogEntry.
func parseAppLogLine(line string, index int) model.LogEntry {
	entry := model.LogEntry{
		ID:        strconv.Itoa(index),
		Type:      "app",
		Timestamp: "",
		Level:     "info",
		Message:   line,
	}

	if line == "" {
		return entry
	}

	// Try to extract timestamp and level from slog text format.
	// Format: "time=... level=INFO msg=..."
	parts := strings.Fields(line)
	for _, part := range parts {
		k, v, ok := strings.Cut(part, "=")
		if !ok {
			continue
		}
		switch k {
		case "time":
			entry.Timestamp = v
		case "level":
			entry.Level = strings.ToLower(v)
		case "msg":
			entry.Message = v
		}
	}

	// Normalize level.
	switch strings.ToUpper(entry.Level) {
	case "WARN", "WARNING":
		entry.Level = "warn"
	case "ERR", "ERROR":
		entry.Level = "error"
	case "DBG", "DEBUG":
		entry.Level = "debug"
	case "INF", "INFO":
		entry.Level = "info"
	}

	return entry
}
