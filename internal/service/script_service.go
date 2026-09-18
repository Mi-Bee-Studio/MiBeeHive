// Package service — scheduled script execution (#70).
//
// ScheduledScript rows drive a robfig/cron scheduler: each enabled script runs
// `sh <script>` on its 5-field cron schedule. Every execution (cron or manual)
// records a ScriptRun row with exit code and capped stdout/stderr. Scripts live
// in a dedicated directory (data-dir/scripts) that only the admin API can read
// or write — the WebDAV tree stays public-read and must not leak script content.
package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
)

// Task status values stored in scheduled_scripts.last_status.
const (
	ScriptStatusSuccess = "success"
	ScriptStatusFailed  = "failed"
)

// Run trigger values stored in script_runs.trigger.
const (
	ScriptTriggerCron   = "cron"
	ScriptTriggerManual = "manual"
)

const (
	// maxOutputBytes caps stored stdout/stderr per stream.
	maxOutputBytes = 64 * 1024
	// defaultScriptTimeout is used when timeout_seconds <= 0.
	defaultScriptTimeout = 300 * time.Second
	// killGrace waits after context cancellation for the process (and its
	// stdout/stderr writers) to settle before giving up.
	killGrace = 5 * time.Second
)

// scriptParser accepts exactly the classic 5-field cron syntax (minute hour
// dom month dow) — no seconds field, no @descriptors.
var scriptParser = cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

// ScheduledScript is one cron-driven script task.
type ScheduledScript struct {
	ID             int64      `json:"id"`
	Name           string     `json:"name"`
	Schedule       string     `json:"schedule"`
	ScriptPath     string     `json:"script_path"`
	Enabled        bool       `json:"enabled"`
	TimeoutSeconds int        `json:"timeout_seconds"`
	LastRunAt      *time.Time `json:"last_run_at"`
	LastStatus     *string    `json:"last_status"`
	// NextRunAt is computed (not stored): next fire time of Schedule when enabled.
	NextRunAt *time.Time `json:"next_run_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

// decorate fills NextRunAt for enabled scripts (best effort — invalid
// schedules can't reach the DB via validation, so errors are ignored).
func (sc *ScheduledScript) decorate() {
	if !sc.Enabled {
		return
	}
	if next, err := NextRun(sc.Schedule, time.Now().Local()); err == nil {
		sc.NextRunAt = &next
	}
}

// ScriptRun is one execution attempt. FinishedAt nil means still running.
type ScriptRun struct {
	ID         int64      `json:"id"`
	ScriptID   int64      `json:"script_id"`
	StartedAt  time.Time  `json:"started_at"`
	FinishedAt *time.Time `json:"finished_at"`
	ExitCode   *int       `json:"exit_code"`
	Trigger    string     `json:"trigger"`
	Stdout     string     `json:"stdout"`
	Stderr     string     `json:"stderr"`
	Error      string     `json:"error"`
}

// ScriptInput is the create/update payload.
type ScriptInput struct {
	Name           string `json:"name"`
	Schedule       string `json:"schedule"`
	ScriptPath     string `json:"script_path"`
	Enabled        *bool  `json:"enabled"`
	TimeoutSeconds int    `json:"timeout_seconds"`
}

// ErrScriptNotFound, ErrValidation mark the two error classes handlers map to
// HTTP 404 / 400.
var (
	ErrScriptNotFound = errors.New("script not found")
	ErrValidation     = errors.New("validation error")
)

func validationError(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrValidation, fmt.Sprintf(format, args...))
}

// ScriptService provides CRUD plus execution for scheduled scripts.
type ScriptService struct {
	db        *sql.DB
	scriptsDir string
	baseURL   string
	logger    *slog.Logger

	mu      sync.Mutex
	running map[int64]bool
}

// NewScriptService creates the service; scriptsDir is created if missing and
// stored as an absolute path so the admin UI can display it verbatim.
func NewScriptService(db *sql.DB, scriptsDir, baseURL string, logger *slog.Logger) (*ScriptService, error) {
	if abs, err := filepath.Abs(scriptsDir); err == nil {
		scriptsDir = abs
	}
	if err := os.MkdirAll(scriptsDir, 0o755); err != nil {
		return nil, fmt.Errorf("creating scripts dir: %w", err)
	}
	return &ScriptService{
		db:         db,
		scriptsDir: scriptsDir,
		baseURL:    baseURL,
		logger:     logger,
		running:    make(map[int64]bool),
	}, nil
}

// ScriptsDir returns the absolute directory scripts resolve against.
func (s *ScriptService) ScriptsDir() string { return s.scriptsDir }

// ValidateSchedule checks a 5-field cron expression.
func ValidateSchedule(spec string) error {
	if strings.TrimSpace(spec) == "" {
		return validationError("schedule is required")
	}
	if _, err := scriptParser.Parse(spec); err != nil {
		return validationError("invalid cron schedule %q: %v", spec, err)
	}
	return nil
}

// NextRun computes the next fire time of spec after t.
func NextRun(spec string, t time.Time) (time.Time, error) {
	sched, err := scriptParser.Parse(spec)
	if err != nil {
		return time.Time{}, err
	}
	return sched.Next(t), nil
}

func validateScriptPath(p string) error {
	if p == "" {
		return validationError("script_path is required")
	}
	// filepath.IsAbs alone is not enough: on Windows it misses Unix-style
	// absolute paths, which the Linux deployment would still honor.
	if filepath.IsAbs(p) || strings.HasPrefix(p, "/") || strings.HasPrefix(p, "\\") {
		return validationError("script_path must be relative to the scripts directory")
	}
	clean := filepath.Clean(p)
	if clean == "." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) || clean == ".." {
		return validationError("script_path must not escape the scripts directory")
	}
	if !strings.HasSuffix(p, ".sh") {
		return validationError("script_path must end with .sh")
	}
	return nil
}

func (in *ScriptInput) validate() error {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return validationError("name is required")
	}
	if len(name) > 64 {
		return validationError("name must be at most 64 characters")
	}
	if err := ValidateSchedule(in.Schedule); err != nil {
		return err
	}
	if err := validateScriptPath(in.ScriptPath); err != nil {
		return err
	}
	if in.TimeoutSeconds < 0 || in.TimeoutSeconds > 86400 {
		return validationError("timeout_seconds must be between 0 and 86400")
	}
	return nil
}

const scriptColumns = `id, name, schedule, script_path, enabled, timeout_seconds,
	last_run_at, last_status, created_at, updated_at`

func scanScript(row interface{ Scan(...any) error }) (*ScheduledScript, error) {
	var sc ScheduledScript
	var enabled int
	if err := row.Scan(&sc.ID, &sc.Name, &sc.Schedule, &sc.ScriptPath, &enabled, &sc.TimeoutSeconds,
		&sc.LastRunAt, &sc.LastStatus, &sc.CreatedAt, &sc.UpdatedAt); err != nil {
		return nil, err
	}
	sc.Enabled = enabled != 0
	return &sc, nil
}

// ListScripts returns all scripts ordered by name.
func (s *ScriptService) ListScripts(ctx context.Context) ([]*ScheduledScript, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+scriptColumns+` FROM scheduled_scripts ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("listing scripts: %w", err)
	}
	defer rows.Close()
	var out []*ScheduledScript
	for rows.Next() {
		sc, err := scanScript(rows)
		if err != nil {
			return nil, fmt.Errorf("scanning script: %w", err)
		}
		sc.decorate()
		out = append(out, sc)
	}
	return out, rows.Err()
}

// GetScript returns one script by id.
func (s *ScriptService) GetScript(ctx context.Context, id int64) (*ScheduledScript, error) {
	sc, err := scanScript(s.db.QueryRowContext(ctx,
		`SELECT `+scriptColumns+` FROM scheduled_scripts WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrScriptNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("getting script: %w", err)
	}
	return sc, nil
}

// CreateScript validates and inserts a new script task.
func (s *ScriptService) CreateScript(ctx context.Context, in ScriptInput) (*ScheduledScript, error) {
	in.Name = strings.TrimSpace(in.Name)
	if err := in.validate(); err != nil {
		return nil, err
	}
	enabled := 1
	if in.Enabled != nil && !*in.Enabled {
		enabled = 0
	}
	timeout := in.TimeoutSeconds
	if timeout <= 0 {
		timeout = int(defaultScriptTimeout.Seconds())
	}
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO scheduled_scripts (name, schedule, script_path, enabled, timeout_seconds)
		 VALUES (?, ?, ?, ?, ?)`,
		in.Name, in.Schedule, filepath.ToSlash(in.ScriptPath), enabled, timeout)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return nil, validationError("script name %q already exists", in.Name)
		}
		return nil, fmt.Errorf("inserting script: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("inserting script: %w", err)
	}
	return s.GetScript(ctx, id)
}

// UpdateScript applies a full or partial update; nil fields keep old values.
func (s *ScriptService) UpdateScript(ctx context.Context, id int64, in ScriptInput) (*ScheduledScript, error) {
	cur, err := s.GetScript(ctx, id)
	if err != nil {
		return nil, err
	}
	merged := ScriptInput{
		Name:           cur.Name,
		Schedule:       cur.Schedule,
		ScriptPath:     cur.ScriptPath,
		Enabled:        &cur.Enabled,
		TimeoutSeconds: cur.TimeoutSeconds,
	}
	if in.Name != "" {
		merged.Name = in.Name
	}
	if in.Schedule != "" {
		merged.Schedule = in.Schedule
	}
	if in.ScriptPath != "" {
		merged.ScriptPath = in.ScriptPath
	}
	if in.Enabled != nil {
		merged.Enabled = in.Enabled
	}
	if in.TimeoutSeconds != 0 {
		merged.TimeoutSeconds = in.TimeoutSeconds
	}
	merged.Name = strings.TrimSpace(merged.Name)
	if err := merged.validate(); err != nil {
		return nil, err
	}
	enabled := 0
	if merged.Enabled != nil && *merged.Enabled {
		enabled = 1
	}
	if _, err := s.db.ExecContext(ctx,
		`UPDATE scheduled_scripts
		 SET name = ?, schedule = ?, script_path = ?, enabled = ?, timeout_seconds = ?,
		     updated_at = CURRENT_TIMESTAMP
		 WHERE id = ?`,
		merged.Name, merged.Schedule, filepath.ToSlash(merged.ScriptPath), enabled,
		merged.TimeoutSeconds, id); err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return nil, validationError("script name %q already exists", merged.Name)
		}
		return nil, fmt.Errorf("updating script: %w", err)
	}
	return s.GetScript(ctx, id)
}

// DeleteScript removes the script and (via cascade) its run history.
func (s *ScriptService) DeleteScript(ctx context.Context, id int64) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM scheduled_scripts WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("deleting script: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrScriptNotFound
	}
	return nil
}

// resolvePath maps a stored relative script_path to an absolute path inside
// the scripts directory.
func (s *ScriptService) resolvePath(rel string) string {
	return filepath.Join(s.scriptsDir, filepath.FromSlash(rel))
}

// cappedWriter keeps the first limit bytes of a stream and remembers overflow.
type cappedWriter struct {
	buf     strings.Builder
	limit   int
	trunc   bool
}

func (w *cappedWriter) Write(p []byte) (int, error) {
	if w.buf.Len() < w.limit {
		room := w.limit - w.buf.Len()
		if len(p) > room {
			w.buf.Write(p[:room])
			w.trunc = true
		} else {
			w.buf.Write(p)
		}
	} else {
		w.trunc = true
	}
	return len(p), nil // always claim full length so the pipe never blocks
}

func (w *cappedWriter) String() string {
	if w.trunc {
		return w.buf.String() + "\n...[output truncated]"
	}
	return w.buf.String()
}

func (s *ScriptService) isRunning(id int64) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running[id]
}

func (s *ScriptService) setRunning(id int64, on bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if on {
		s.running[id] = true
	} else {
		delete(s.running, id)
	}
}

// IsRunning reports whether a script execution is currently in flight.
func (s *ScriptService) IsRunning(id int64) bool { return s.isRunning(id) }

// StartRun launches one execution asynchronously: it inserts a "running"
// ScriptRun row and returns it immediately; completion happens in the
// background. trigger is ScriptTriggerCron or ScriptTriggerManual.
func (s *ScriptService) StartRun(ctx context.Context, id int64, trigger string) (*ScriptRun, error) {
	if _, err := s.GetScript(ctx, id); err != nil {
		return nil, err
	}
	if s.isRunning(id) {
		return nil, validationError("script is already running")
	}
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO script_runs (script_id, trigger_type) VALUES (?, ?)`, id, trigger)
	if err != nil {
		return nil, fmt.Errorf("inserting run: %w", err)
	}
	runID, err := res.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("inserting run: %w", err)
	}
	run, err := s.GetRun(ctx, runID)
	if err != nil {
		return nil, err
	}

	s.setRunning(id, true)
	go func() {
		defer s.setRunning(id, false)
		// Detached from the request context: a run outlives its trigger.
		s.execute(run)
	}()
	return run, nil
}

func (s *ScriptService) execute(run *ScriptRun) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sc, err := s.GetScript(ctx, run.ScriptID)
	if err != nil {
		s.finishRun(run.ID, nil, "", "", fmt.Sprintf("script vanished: %v", err), nil)
		return
	}
	timeout := time.Duration(sc.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = defaultScriptTimeout
	}
	runCtx, timeoutCancel := context.WithTimeout(ctx, timeout)
	defer timeoutCancel()

	abs := s.resolvePath(sc.ScriptPath)
	if info, err := os.Stat(abs); err != nil {
		s.finishRunNotFound(run.ID, sc,
			fmt.Sprintf("script file not found: %s (upload it via the script content endpoint)", sc.ScriptPath))
		return
	} else if info.IsDir() {
		s.finishRunNotFound(run.ID, sc, "script path is a directory")
		return
	}

	cmd := exec.CommandContext(runCtx, "sh", abs)
	cmd.Dir = s.scriptsDir
	cmd.Env = append(os.Environ(), "MIBEEHIVE_URL="+s.baseURL)
	cmd.WaitDelay = killGrace
	stdout, stderr := &cappedWriter{limit: maxOutputBytes}, &cappedWriter{limit: maxOutputBytes}
	cmd.Stdout, cmd.Stderr = stdout, stderr

	start := time.Now()
	runErr := cmd.Run()
	duration := time.Since(start)

	var exitErr *exec.ExitError
	exitCode := 0
	errMsg := ""
	switch {
	case runCtx.Err() != nil:
		// The context kill (timeout/shutdown) caused this failure — the
		// process-level ExitError it surfaces is platform-dependent noise.
		exitCode = -1
		if errors.Is(runCtx.Err(), context.DeadlineExceeded) {
			errMsg = fmt.Sprintf("timed out after %s", timeout)
		} else {
			errMsg = "canceled"
		}
	case runErr == nil:
		// success
	case errors.As(runErr, &exitErr):
		exitCode = exitErr.ExitCode()
	default:
		exitCode = -1
		errMsg = runErr.Error()
	}
	s.finishRun(run.ID, &exitCode, stdout.String(), stderr.String(), errMsg, sc)
	s.logger.Info("script run finished",
		"script", sc.Name, "trigger", run.Trigger,
		"exit_code", exitCode, "duration", duration.String())
}

// finishRunNotFound records a pre-execution failure (missing/invalid file)
// with exit code -1.
func (s *ScriptService) finishRunNotFound(runID int64, sc *ScheduledScript, errMsg string) {
	code := -1
	s.finishRun(runID, &code, "", "", errMsg, sc)
}

// finishRun stores the outcome of a run and updates the script's last_run fields.
func (s *ScriptService) finishRun(runID int64, exitCode *int, stdout, stderr, errMsg string, sc *ScheduledScript) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if _, err := s.db.ExecContext(ctx,
		`UPDATE script_runs
		 SET finished_at = CURRENT_TIMESTAMP, exit_code = ?, stdout = ?, stderr = ?, error = ?
		 WHERE id = ?`,
		exitCode, stdout, stderr, errMsg, runID); err != nil {
		s.logger.Error("failed to record script run", "run_id", runID, "error", err)
	}
	if sc != nil {
		status := ScriptStatusSuccess
		if exitCode == nil || *exitCode != 0 {
			status = ScriptStatusFailed
		}
		if _, err := s.db.ExecContext(ctx,
			`UPDATE scheduled_scripts SET last_run_at = CURRENT_TIMESTAMP, last_status = ? WHERE id = ?`,
			status, sc.ID); err != nil {
			s.logger.Error("failed to update script status", "script_id", sc.ID, "error", err)
		}
	}
}

// GetRun returns one run row.
func (s *ScriptService) GetRun(ctx context.Context, runID int64) (*ScriptRun, error) {
	return scanScriptRun(s.db.QueryRowContext(ctx, `SELECT `+runColumns+` FROM script_runs WHERE id = ?`, runID))
}

// ListRuns returns run history for a script, newest first.
func (s *ScriptService) ListRuns(ctx context.Context, scriptID int64, limit, offset int) ([]*ScriptRun, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+runColumns+` FROM script_runs WHERE script_id = ? ORDER BY id DESC LIMIT ? OFFSET ?`,
		scriptID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("listing runs: %w", err)
	}
	defer rows.Close()
	var out []*ScriptRun
	for rows.Next() {
		r, err := scanScriptRun(rows)
		if err != nil {
			return nil, fmt.Errorf("scanning run: %w", err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

const runColumns = `id, script_id, started_at, finished_at, exit_code, trigger_type, stdout, stderr, error`

func scanScriptRun(row interface{ Scan(...any) error }) (*ScriptRun, error) {
	var r ScriptRun
	var stdout, stderr, errMsg sql.NullString
	if err := row.Scan(&r.ID, &r.ScriptID, &r.StartedAt, &r.FinishedAt, &r.ExitCode, &r.Trigger,
		&stdout, &stderr, &errMsg); err != nil {
		return nil, err
	}
	r.Stdout, r.Stderr, r.Error = stdout.String, stderr.String, errMsg.String
	return &r, nil
}

// GetScriptContent reads the script file body (max 256KB).
func (s *ScriptService) GetScriptContent(ctx context.Context, id int64) (string, error) {
	sc, err := s.GetScript(ctx, id)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(s.resolvePath(sc.ScriptPath))
	if errors.Is(err, os.ErrNotExist) {
		return "", ErrScriptNotFound
	}
	if err != nil {
		return "", fmt.Errorf("reading script: %w", err)
	}
	return string(data), nil
}

// maxContentBytes caps script uploads.
const maxContentBytes = 256 * 1024

// PutScriptContent writes the script file body, enforcing LF line endings so
// shell scripts never carry CRLF into `sh`.
func (s *ScriptService) PutScriptContent(ctx context.Context, id int64, body io.Reader) error {
	sc, err := s.GetScript(ctx, id)
	if err != nil {
		return err
	}
	data, err := io.ReadAll(io.LimitReader(body, maxContentBytes+1))
	if err != nil {
		return fmt.Errorf("reading body: %w", err)
	}
	if len(data) > maxContentBytes {
		return validationError("script content exceeds %d bytes", maxContentBytes)
	}
	content := strings.ReplaceAll(string(data), "\r\n", "\n")
	abs := s.resolvePath(sc.ScriptPath)
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return fmt.Errorf("creating script dir: %w", err)
	}
	if err := os.WriteFile(abs, []byte(content), 0o755); err != nil {
		return fmt.Errorf("writing script: %w", err)
	}
	return nil
}

// ScriptScheduler wires enabled scripts into a single cron instance and keeps
// entries in sync across CRUD changes.
type ScriptScheduler struct {
	svc *ScriptService
	c   *cron.Cron

	mu      sync.Mutex
	entries map[int64]cron.EntryID
	started bool
}

// NewScriptScheduler creates a scheduler bound to svc.
func NewScriptScheduler(svc *ScriptService) *ScriptScheduler {
	return &ScriptScheduler{
		svc:     svc,
		c:       cron.New(cron.WithParser(scriptParser)),
		entries: make(map[int64]cron.EntryID),
	}
}

// Start loads all enabled scripts into the cron instance and starts ticking.
func (s *ScriptScheduler) Start(ctx context.Context) error {
	scripts, err := s.svc.ListScripts(ctx)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.started {
		return nil
	}
	for _, sc := range scripts {
		if !sc.Enabled {
			continue
		}
		if err := s.addLocked(sc); err != nil {
			s.svc.logger.Warn("skipping invalid script schedule", "script", sc.Name, "error", err)
		}
	}
	s.c.Start()
	s.started = true
	return nil
}

// addLocked registers an entry; caller holds s.mu.
func (s *ScriptScheduler) addLocked(sc *ScheduledScript) error {
	id := sc.ID
	schedule, err := scriptParser.Parse(sc.Schedule)
	if err != nil {
		return err
	}
	s.entries[id] = s.c.Schedule(schedule, cron.FuncJob(func() {
		// Skip if previous run still going — overlap guard lives in StartRun
		// (it rejects with "already running" and we drop the tick silently).
		if _, err := s.svc.StartRun(context.Background(), id, ScriptTriggerCron); err != nil {
			s.svc.logger.Debug("cron tick skipped", "script_id", id, "error", err)
		}
	}))
	return nil
}

// Reschedule syncs one script's cron entry after create/update/delete.
func (s *ScriptScheduler) Reschedule(sc *ScheduledScript) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if old, ok := s.entries[sc.ID]; ok {
		s.c.Remove(old)
		delete(s.entries, sc.ID)
	}
	if !s.started || !sc.Enabled {
		return
	}
	if err := s.addLocked(sc); err != nil {
		s.svc.logger.Warn("failed to reschedule script", "script", sc.Name, "error", err)
	}
}

// Remove drops a script's entry (delete path).
func (s *ScriptScheduler) Remove(id int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if old, ok := s.entries[id]; ok {
		s.c.Remove(old)
		delete(s.entries, id)
	}
}

// Stop halts the cron instance and waits briefly for in-flight jobs.
func (s *ScriptScheduler) Stop() {
	s.mu.Lock()
	started := s.started
	s.started = false
	s.mu.Unlock()
	if !started {
		return
	}
	ctx := s.c.Stop() // returns context that completes when running jobs finish
	select {
	case <-ctx.Done():
	case <-time.After(10 * time.Second):
	}
}
