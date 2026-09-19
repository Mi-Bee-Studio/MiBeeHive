package service

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Mi-Bee-Studio/mibeehive/internal/db"
)

func newScriptTestService(t *testing.T) (*ScriptService, string) {
	t.Helper()
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	if err := db.Migrate(database); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	dir := t.TempDir()
	svc, err := NewScriptService(database, dir, "http://127.0.0.1:9090", slog.Default())
	if err != nil {
		t.Fatalf("new service: %v", err)
	}
	return svc, dir
}

func writeScript(t *testing.T, dir, rel, body string) {
	t.Helper()
	abs := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(abs, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
}

func TestValidateSchedule(t *testing.T) {
	for _, ok := range []string{"* * * * *", "*/5 * * * *", "0 3 * * 1-5", "30 4 1,15 * *"} {
		if err := ValidateSchedule(ok); err != nil {
			t.Errorf("ValidateSchedule(%q) = %v, want nil", ok, err)
		}
	}
	for _, bad := range []string{"", "* * * *", "60 * * * *", "@every 1h", "*/5 * * * * *"} {
		if err := ValidateSchedule(bad); err == nil {
			t.Errorf("ValidateSchedule(%q) = nil, want error", bad)
		}
	}
}

func TestValidateScriptPath(t *testing.T) {
	for _, ok := range []string{"hello.sh", "sub/dir/job.sh"} {
		if err := validateScriptPath(ok); err != nil {
			t.Errorf("validateScriptPath(%q) = %v, want nil", ok, err)
		}
	}
	for _, bad := range []string{"", "/abs/path.sh", "../escape.sh", "..", "sub/../../x.sh", "noext"} {
		if err := validateScriptPath(bad); err == nil {
			t.Errorf("validateScriptPath(%q) = nil, want error", bad)
		}
	}
}

func TestScriptCRUD(t *testing.T) {
	svc, dir := newScriptTestService(t)
	ctx := context.Background()

	if err := writeScriptFile(t, dir, "hello.sh"); err != nil {
		t.Fatal(err)
	}
	sc, err := svc.CreateScript(ctx, ScriptInput{
		Name: "hello", Schedule: "*/5 * * * *", ScriptPath: "hello.sh",
		Enabled: boolPtr(true), TimeoutSeconds: 30,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if sc.ID == 0 || !sc.Enabled || sc.TimeoutSeconds != 30 {
		t.Fatalf("created = %+v", sc)
	}

	// Duplicate name → validation error.
	if _, err := svc.CreateScript(ctx, ScriptInput{Name: "hello", Schedule: "* * * * *", ScriptPath: "a.sh"}); !errors.Is(err, ErrValidation) {
		t.Errorf("duplicate name err = %v, want ErrValidation", err)
	}
	// Bad schedule rejected.
	if _, err := svc.CreateScript(ctx, ScriptInput{Name: "x", Schedule: "nope", ScriptPath: "a.sh"}); !errors.Is(err, ErrValidation) {
		t.Errorf("bad schedule err = %v, want ErrValidation", err)
	}

	// Partial update: disable only.
	upd, err := svc.UpdateScript(ctx, sc.ID, ScriptInput{Enabled: boolPtr(false)})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if upd.Enabled || upd.Schedule != "*/5 * * * *" || upd.Name != "hello" {
		t.Errorf("updated = %+v", upd)
	}

	list, err := svc.ListScripts(ctx)
	if err != nil || len(list) != 1 {
		t.Fatalf("list = %v, %v", list, err)
	}

	if err := svc.DeleteScript(ctx, sc.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := svc.GetScript(ctx, sc.ID); !errors.Is(err, ErrScriptNotFound) {
		t.Errorf("get after delete = %v, want ErrScriptNotFound", err)
	}
}

func writeScriptFile(t *testing.T, dir, rel string) error {
	t.Helper()
	return os.WriteFile(filepath.Join(dir, rel), []byte("echo hi\n"), 0o755)
}

// waitForRun polls until the run row reaches a terminal state.
func waitForRun(t *testing.T, svc *ScriptService, runID int64, timeout time.Duration) *ScriptRun {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		run, err := svc.GetRun(context.Background(), runID)
		if err != nil {
			t.Fatalf("get run: %v", err)
		}
		if run.FinishedAt != nil {
			return run
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("run %d did not finish within %s", runID, timeout)
	return nil
}

func TestScriptRunSuccessAndFailure(t *testing.T) {
	svc, dir := newScriptTestService(t)
	ctx := context.Background()

	writeScript(t, dir, "ok.sh", "echo hello-script\n")
	writeScript(t, dir, "fail.sh", "echo to-stderr >&2\nexit 7\n")

	ok, err := svc.CreateScript(ctx, ScriptInput{Name: "ok", Schedule: "0 3 * * *", ScriptPath: "ok.sh"})
	if err != nil {
		t.Fatal(err)
	}
	fail, err := svc.CreateScript(ctx, ScriptInput{Name: "fail", Schedule: "0 3 * * *", ScriptPath: "fail.sh"})
	if err != nil {
		t.Fatal(err)
	}

	runOK, err := svc.StartRun(ctx, ok.ID, ScriptTriggerManual)
	if err != nil {
		t.Fatalf("start ok: %v", err)
	}
	got := waitForRun(t, svc, runOK.ID, 10*time.Second)
	if got.ExitCode == nil || *got.ExitCode != 0 {
		t.Errorf("ok run exit = %v err=%q, want 0", got.ExitCode, got.Error)
	}
	if !strings.Contains(got.Stdout, "hello-script") {
		t.Errorf("ok stdout = %q, want it to contain hello-script", got.Stdout)
	}

	runFail, err := svc.StartRun(ctx, fail.ID, ScriptTriggerManual)
	if err != nil {
		t.Fatalf("start fail: %v", err)
	}
	got = waitForRun(t, svc, runFail.ID, 10*time.Second)
	if got.ExitCode == nil || *got.ExitCode != 7 {
		t.Errorf("fail run exit = %v, want 7", got.ExitCode)
	}
	if !strings.Contains(got.Stderr, "to-stderr") {
		t.Errorf("fail stderr = %q, want it to contain to-stderr", got.Stderr)
	}

	// last_status updated per script.
	sc, _ := svc.GetScript(ctx, ok.ID)
	if sc.LastStatus == nil || *sc.LastStatus != ScriptStatusSuccess {
		t.Errorf("ok last_status = %v, want success", sc.LastStatus)
	}
	sc, _ = svc.GetScript(ctx, fail.ID)
	if sc.LastStatus == nil || *sc.LastStatus != ScriptStatusFailed {
		t.Errorf("fail last_status = %v, want failed", sc.LastStatus)
	}
}

func TestScriptRunMissingFile(t *testing.T) {
	svc, _ := newScriptTestService(t)
	ctx := context.Background()

	sc, err := svc.CreateScript(ctx, ScriptInput{Name: "ghost", Schedule: "0 3 * * *", ScriptPath: "ghost.sh"})
	if err != nil {
		t.Fatal(err)
	}
	run, err := svc.StartRun(ctx, sc.ID, ScriptTriggerManual)
	if err != nil {
		t.Fatal(err)
	}
	got := waitForRun(t, svc, run.ID, 10*time.Second)
	if got.ExitCode == nil || *got.ExitCode == 0 {
		t.Errorf("ghost exit = %v, want non-zero", got.ExitCode)
	}
	if !strings.Contains(got.Error, "not found") {
		t.Errorf("ghost error = %q, want file-not-found hint", got.Error)
	}
}

func TestScriptRunTimeout(t *testing.T) {
	svc, dir := newScriptTestService(t)
	ctx := context.Background()

	writeScript(t, dir, "slow.sh", "sleep 30\n")
	sc, err := svc.CreateScript(ctx, ScriptInput{Name: "slow", Schedule: "0 3 * * *", ScriptPath: "slow.sh", TimeoutSeconds: 1})
	if err != nil {
		t.Fatal(err)
	}
	run, err := svc.StartRun(ctx, sc.ID, ScriptTriggerManual)
	if err != nil {
		t.Fatal(err)
	}
	got := waitForRun(t, svc, run.ID, 15*time.Second)
	if got.ExitCode == nil || *got.ExitCode != -1 {
		t.Errorf("timeout exit = %v, want -1", got.ExitCode)
	}
	if !strings.Contains(got.Error, "timed out") {
		t.Errorf("timeout error = %q, want timed-out message", got.Error)
	}
}

func TestScriptContentRoundTrip(t *testing.T) {
	svc, _ := newScriptTestService(t)
	ctx := context.Background()

	sc, err := svc.CreateScript(ctx, ScriptInput{Name: "edit", Schedule: "0 3 * * *", ScriptPath: "edit.sh"})
	if err != nil {
		t.Fatal(err)
	}
	// Not uploaded yet → not found (distinct from task-not-found: 404 body differs in handler).
	if _, err := svc.GetScriptContent(ctx, sc.ID); !errors.Is(err, ErrScriptNotFound) {
		t.Errorf("content before upload = %v, want ErrScriptNotFound", err)
	}

	// CRLF normalized to LF so `sh` never sees \r.
	if err := svc.PutScriptContent(ctx, sc.ID, strings.NewReader("#!/bin/sh\r\necho edited\r\n")); err != nil {
		t.Fatalf("put content: %v", err)
	}
	body, err := svc.GetScriptContent(ctx, sc.ID)
	if err != nil {
		t.Fatalf("get content: %v", err)
	}
	if body != "#!/bin/sh\necho edited\n" {
		t.Errorf("content = %q, want CRLF normalized to LF", body)
	}

	// Oversized content rejected.
	big := strings.Repeat("a", maxContentBytes+1)
	if err := svc.PutScriptContent(ctx, sc.ID, strings.NewReader(big)); !errors.Is(err, ErrValidation) {
		t.Errorf("oversized content err = %v, want ErrValidation", err)
	}
}

func TestScriptSchedulerLifecycle(t *testing.T) {
	svc, dir := newScriptTestService(t)
	ctx := context.Background()

	writeScript(t, dir, "tick.sh", "echo tick\n")
	enabled, err := svc.CreateScript(ctx, ScriptInput{Name: "tick", Schedule: "* * * * *", ScriptPath: "tick.sh", Enabled: boolPtr(true)})
	if err != nil {
		t.Fatal(err)
	}
	disabled, err := svc.CreateScript(ctx, ScriptInput{Name: "off", Schedule: "* * * * *", ScriptPath: "tick.sh", Enabled: boolPtr(false)})
	if err != nil {
		t.Fatal(err)
	}

	s := NewScriptScheduler(svc)
	if err := s.Start(ctx); err != nil {
		t.Fatalf("start: %v", err)
	}
	// Double start is a no-op.
	if err := s.Start(ctx); err != nil {
		t.Fatalf("second start: %v", err)
	}
	if len(s.c.Entries()) != 1 {
		t.Errorf("cron entries = %d, want 1 (enabled only)", len(s.c.Entries()))
	}

	// Enable the disabled script → rescheduled in.
	disabled, err = svc.UpdateScript(ctx, disabled.ID, ScriptInput{Enabled: boolPtr(true)})
	if err != nil {
		t.Fatal(err)
	}
	s.Reschedule(disabled)
	if len(s.c.Entries()) != 2 {
		t.Errorf("cron entries after enable = %d, want 2", len(s.c.Entries()))
	}

	// Rename/update keeps one entry per script.
	enabled, err = svc.UpdateScript(ctx, enabled.ID, ScriptInput{Schedule: "*/10 * * * *"})
	if err != nil {
		t.Fatal(err)
	}
	s.Reschedule(enabled)
	if len(s.c.Entries()) != 2 {
		t.Errorf("cron entries after update = %d, want 2", len(s.c.Entries()))
	}

	s.Remove(disabled.ID)
	if len(s.c.Entries()) != 1 {
		t.Errorf("cron entries after remove = %d, want 1", len(s.c.Entries()))
	}
	s.Stop() // must return promptly; entries stay listed but stop firing.
}

func TestNextRun(t *testing.T) {
	base := time.Date(2026, 9, 18, 12, 0, 0, 0, time.UTC)
	next, err := NextRun("30 4 * * *", base)
	if err != nil {
		t.Fatal(err)
	}
	if next.Hour() != 4 || next.Minute() != 30 || next.Day() != 19 {
		t.Errorf("NextRun = %v, want 2026-09-19 04:30", next)
	}
}

func TestTaskServiceIncludesScripts(t *testing.T) {
	svc, dir := newScriptTestService(t)
	ts := NewTaskService(svc.db)
	ctx := context.Background()
	_ = dir

	sc, err := svc.CreateScript(ctx, ScriptInput{Name: "tasked", Schedule: "15 2 * * *", ScriptPath: "x.sh", Enabled: boolPtr(true)})
	if err != nil {
		t.Fatal(err)
	}
	tasks, err := ts.GetAllTasks(ctx)
	if err != nil {
		t.Fatalf("GetAllTasks: %v", err)
	}
	found := false
	for _, task := range tasks {
		if task.ID == "script-"+strconv.FormatInt(sc.ID, 10) {
			found = true
			if task.Type != "script" || task.Schedule != "15 2 * * *" || task.Status != "scheduled" {
				t.Errorf("script task = %+v", task)
			}
			if task.NextRunAt == "" {
				t.Errorf("script task missing next_run_at: %+v", task)
			}
		}
	}
	if !found {
		t.Errorf("script task missing from GetAllTasks: %+v", tasks)
	}
}
