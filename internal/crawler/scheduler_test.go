package crawler

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

// TestScheduler_Running tests the Running() method returns correct state.
func TestScheduler_Running(t *testing.T) {
	logger := &mockLogger{}
	s := NewScheduler(logger)

	if s.Running("nonexistent") {
		t.Error("expected nonexistent project to not be running")
	}

	s.StartProject("proj1", 1*time.Hour, func(ctx context.Context) error { return nil })
	time.Sleep(50 * time.Millisecond)

	if !s.Running("proj1") {
		t.Error("expected proj1 to be running after StartProject")
	}

	s.StopProject("proj1")

	if s.Running("proj1") {
		t.Error("expected proj1 to not be running after StopProject")
	}
}

// TestScheduler_StopIdempotent verifies StopProject can be called multiple times without panic.
func TestScheduler_StopIdempotent(t *testing.T) {
	logger := &mockLogger{}
	s := NewScheduler(logger)

	// Stop a project that was never started.
	s.StopProject("never-started")

	// Start then stop twice.
	s.StartProject("proj", 1*time.Hour, func(ctx context.Context) error { return nil })
	time.Sleep(50 * time.Millisecond)

	s.StopProject("proj")
	s.StopProject("proj") // Second stop should not panic.
}

// TestScheduler_Restart verifies that StartProject can restart a stopped project.
func TestScheduler_Restart(t *testing.T) {
	logger := &mockLogger{}
	s := NewScheduler(logger)

	callCount := 0
	mu := sync.Mutex{}

	crawlFunc := func(ctx context.Context) error {
		mu.Lock()
		callCount++
		mu.Unlock()
		return nil
	}

	// Start, stop, then restart.
	s.StartProject("proj", 50*time.Millisecond, crawlFunc)
	time.Sleep(100 * time.Millisecond)
	s.StopProject("proj")
	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	countAfterFirst := callCount
	mu.Unlock()

	s.StartProject("proj", 50*time.Millisecond, crawlFunc)
	time.Sleep(100 * time.Millisecond)
	s.StopProject("proj")

	mu.Lock()
	countAfterRestart := callCount
	mu.Unlock()

	if countAfterFirst == 0 {
		t.Error("expected at least one crawl call during first run")
	}
	if countAfterRestart <= countAfterFirst {
		t.Errorf("expected more calls after restart; first=%d, total=%d", countAfterFirst, countAfterRestart)
	}
}

// TestScheduler_TickInterval verifies the scheduler triggers crawls on the configured interval.
func TestScheduler_TickInterval(t *testing.T) {
	logger := &mockLogger{}
	s := NewScheduler(logger)

	var mu sync.Mutex
	callCount := 0

	crawlFunc := func(ctx context.Context) error {
		mu.Lock()
		callCount++
		mu.Unlock()
		return nil
	}

	s.StartProject("ticker", 50*time.Millisecond, crawlFunc)
	defer s.StopProject("ticker")

	// Wait enough time for initial + at least 2 ticks.
	time.Sleep(250 * time.Millisecond)

	mu.Lock()
	count := callCount
	mu.Unlock()

	if count < 3 {
		t.Errorf("expected at least 3 calls (initial + 2 ticks), got %d", count)
	}
}

// TestScheduler_CrawlErrorLogged verifies that crawl errors are logged but don't stop the scheduler.
func TestScheduler_CrawlErrorLogged(t *testing.T) {
	logger := &mockLogger{}
	s := NewScheduler(logger)

	crawlFunc := func(ctx context.Context) error {
		return errTestCrawl
	}

	s.StartProject("errproj", 50*time.Millisecond, crawlFunc)
	time.Sleep(150 * time.Millisecond)
	s.StopProject("errproj")

	found := false
	for _, msg := range logger.msgs {
		if msg == "ERROR: initial crawl failed" || msg == "ERROR: scheduled crawl failed" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected error log for failed crawl")
	}
}

// TestScheduler_StopAllCleansState verifies StopAll cleans all internal maps.
func TestScheduler_StopAllCleansState(t *testing.T) {
	logger := &mockLogger{}
	s := NewScheduler(logger)

	s.StartProject("p1", 1*time.Hour, func(ctx context.Context) error { return nil })
	s.StartProject("p2", 1*time.Hour, func(ctx context.Context) error { return nil })
	time.Sleep(50 * time.Millisecond)

	s.StopAll()

	if s.Running("p1") {
		t.Error("expected p1 to not be running after StopAll")
	}
	if s.Running("p2") {
		t.Error("expected p2 to not be running after StopAll")
	}

	s.mu.Lock()
	intervals := len(s.intervals)
	doneChans := len(s.doneChans)
	s.mu.Unlock()

	if intervals != 0 {
		t.Errorf("expected 0 intervals after StopAll, got %d", intervals)
	}
	if doneChans != 0 {
		t.Errorf("expected 0 doneChans after StopAll, got %d", doneChans)
	}
}

// TestScheduler_MultipleProjects verifies multiple projects can run concurrently.
func TestScheduler_MultipleProjects(t *testing.T) {
	logger := &mockLogger{}
	s := NewScheduler(logger)

	var mu sync.Mutex
	p1Count, p2Count := 0, 0

	s.StartProject("proj1", 50*time.Millisecond, func(ctx context.Context) error {
		mu.Lock()
		p1Count++
		mu.Unlock()
		return nil
	})
	s.StartProject("proj2", 50*time.Millisecond, func(ctx context.Context) error {
		mu.Lock()
		p2Count++
		mu.Unlock()
		return nil
	})

	time.Sleep(150 * time.Millisecond)
	s.StopAll()

	mu.Lock()
	c1, c2 := p1Count, p2Count
	mu.Unlock()

	if c1 == 0 {
		t.Error("expected proj1 to have at least 1 crawl")
	}
	if c2 == 0 {
		t.Error("expected proj2 to have at least 1 crawl")
	}
}

// errTestCrawl is a sentinel error for crawl error tests.
var errTestCrawl = fmt.Errorf("test crawl error")
