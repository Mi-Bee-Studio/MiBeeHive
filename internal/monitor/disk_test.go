package monitor

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"
)

func TestNewDiskMonitor(t *testing.T) {
	m := NewDiskMonitor("/tmp", 80, 95, 30*time.Second)
	if m.path != "/tmp" {
		t.Errorf("path = %q, want /tmp", m.path)
	}
	if m.warningThreshold != 80 {
		t.Errorf("warningThreshold = %d, want 80", m.warningThreshold)
	}
	if m.criticalThreshold != 95 {
		t.Errorf("criticalThreshold = %d, want 95", m.criticalThreshold)
	}
	if m.checkInterval != 30*time.Second {
		t.Errorf("checkInterval = %v, want 30s", m.checkInterval)
	}
}

func TestInitialState(t *testing.T) {
	m := NewDiskMonitor("/tmp", 80, 95, 30*time.Second)
	if m.IsWarning() {
		t.Error("IsWarning() = true on new monitor, want false")
	}
	if m.IsDegraded() {
		t.Error("IsDegraded() = true on new monitor, want false")
	}
}

func TestCheckWithRealPath(t *testing.T) {
	dir := t.TempDir()
	// Use 0% warning/critical so any disk usage triggers both states.
	m := NewDiskMonitor(dir, 0, 0, time.Second)
	m.check()

	if !m.IsWarning() {
		t.Error("IsWarning() = false with 0% threshold, want true")
	}
	if !m.IsDegraded() {
		t.Error("IsDegraded() = false with 0% threshold, want true")
	}
}

func TestCheckWithHighThresholds(t *testing.T) {
	dir := t.TempDir()
	// Use 100% thresholds so no real disk triggers them.
	m := NewDiskMonitor(dir, 100, 100, time.Second)
	m.check()

	if m.IsWarning() {
		t.Error("IsWarning() = true with 100% threshold, want false")
	}
	if m.IsDegraded() {
		t.Error("IsDegraded() = false with 100% threshold, want false")
	}
}

func TestCheckWithInvalidPath(t *testing.T) {
	m := NewDiskMonitor("/nonexistent/path/that/does/not/exist", 50, 80, time.Second)
	m.check()

	// Should remain false — error path doesn't flip state.
	if m.IsWarning() {
		t.Error("IsWarning() = true on invalid path, want false")
	}
	if m.IsDegraded() {
		t.Error("IsDegraded() = true on invalid path, want false")
	}
}

func TestStateTransitionNormalToWarning(t *testing.T) {
	dir := t.TempDir()

	// First: high thresholds → normal state.
	m := NewDiskMonitor(dir, 100, 100, time.Second)
	m.check()
	if m.IsWarning() {
		t.Fatal("expected not warning initially")
	}

	// Lower warning threshold so real usage triggers it.
	m.warningThreshold = 0
	m.wasWarning = false
	m.wasCritical = false
	m.check()

	if !m.IsWarning() {
		t.Error("IsWarning() = false after lowering threshold, want true")
	}
}

func TestStateTransitionCriticalToNormal(t *testing.T) {
	dir := t.TempDir()

	// Start at 0% → critical state.
	m := NewDiskMonitor(dir, 0, 0, time.Second)
	m.check()
	if !m.IsDegraded() {
		t.Fatal("expected degraded initially")
	}

	// Raise thresholds to 100% → back to normal.
	m.warningThreshold = 100
	m.criticalThreshold = 100
	m.wasWarning = true
	m.wasCritical = true
	m.check()

	if m.IsWarning() {
		t.Error("IsWarning() = true after raising thresholds, want false")
	}
	if m.IsDegraded() {
		t.Error("IsDegraded() = true after raising thresholds, want false")
	}
}

func TestGetUsage(t *testing.T) {
	dir := t.TempDir()
	m := NewDiskMonitor(dir, 80, 95, 30*time.Second)

	total, used, available, err := m.GetUsage()
	if err != nil {
		t.Fatalf("GetUsage() error: %v", err)
	}
	if total == 0 {
		t.Error("total = 0, want > 0")
	}
	if available == 0 {
		t.Error("available = 0, want > 0")
	}
	if used > total {
		t.Errorf("used (%d) > total (%d)", used, total)
	}
}

func TestGetUsageInvalidPath(t *testing.T) {
	m := NewDiskMonitor("/nonexistent/path", 80, 95, time.Second)
	_, _, _, err := m.GetUsage()
	if err == nil {
		t.Error("GetUsage() on invalid path should return error")
	}
}

func TestStartRespectsContextCancellation(t *testing.T) {
	dir := t.TempDir()
	m := NewDiskMonitor(dir, 80, 95, 10*time.Second)

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		m.Start(ctx)
		close(done)
	}()

	// Give it a moment to do the initial check.
	time.Sleep(50 * time.Millisecond)

	// Cancel context — Start should return promptly.
	cancel()

	select {
	case <-done:
		// Success — goroutine exited.
	case <-time.After(2 * time.Second):
		t.Fatal("Start() did not return after context cancellation")
	}
}

func TestAtomicStateAccess(t *testing.T) {
	dir := t.TempDir()
	m := NewDiskMonitor(dir, 0, 0, 50*time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go m.Start(ctx)

	// Wait for a few check cycles.
	time.Sleep(200 * time.Millisecond)

	// Should be in warning/degraded (0% thresholds).
	if !m.IsWarning() {
		t.Error("IsWarning() = false, want true")
	}
	if !m.IsDegraded() {
		t.Error("IsDegraded() = false, want true")
	}

	// Verify atomic.Bool type — safe to read concurrently.
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = m.IsWarning()
			_ = m.IsDegraded()
		}()
	}
	wg.Wait()
}

func TestGetUsageMatchesStatfs(t *testing.T) {
	dir := t.TempDir()
	m := NewDiskMonitor(dir, 80, 95, time.Second)

	total, used, available, err := m.GetUsage()
	if err != nil {
		t.Fatalf("GetUsage() error: %v", err)
	}

	// Cross-check: used + available should be <= total (reserved blocks exist).
	if used+available > total {
		t.Errorf("used(%d) + available(%d) > total(%d)", used, available, total)
	}
}

func TestUsagePercentTempDir(t *testing.T) {
	// Verify usagePercent works on a real directory.
	dir := t.TempDir()
	m := NewDiskMonitor(dir, 50, 80, time.Second)

	pct, avail, err := m.usagePercent()
	if err != nil {
		t.Fatalf("usagePercent() error: %v", err)
	}
	if pct < 0 || pct > 100 {
		t.Errorf("usagePercent = %d, want 0-100", pct)
	}
	_ = avail // just verify no panic
}

func TestCheckIdempotentWhenNoStateChange(t *testing.T) {
	dir := t.TempDir()
	// High thresholds — stays normal.
	m := NewDiskMonitor(dir, 100, 100, time.Second)

	m.check()
	w1 := m.IsWarning()
	c1 := m.IsDegraded()

	m.check()
	w2 := m.IsWarning()
	c2 := m.IsDegraded()

	if w1 != w2 {
		t.Errorf("IsWarning changed from %v to %v without threshold change", w1, w2)
	}
	if c1 != c2 {
		t.Errorf("IsDegraded changed from %v to %v without threshold change", c1, c2)
	}
}

func TestCheckWithNonexistentDir(t *testing.T) {
	path := "/nonexistent/dir/disk_monitor_test"
	m := NewDiskMonitor(path, 50, 80, time.Second)
	m.check()

	// Must not crash; state stays false.
	if m.IsWarning() {
		t.Error("IsWarning() should be false on nonexistent path")
	}
	if m.IsDegraded() {
		t.Error("IsDegraded() should be false on nonexistent path")
	}
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
