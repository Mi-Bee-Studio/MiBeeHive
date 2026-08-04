package crawler

import (
	"context"
	"sync"
	"time"
)

// Scheduler manages per-project crawl timers using stdlib time.Ticker.
type Scheduler struct {
	mu          sync.Mutex
	timers      map[string]*time.Timer   // projectName -> timer
	intervals   map[string]time.Duration // projectName -> interval
	cancelFuncs map[string]context.CancelFunc
	doneChans   map[string]chan struct{} // projectName -> done signal (closed when goroutine exits)
	logger      Logger
}

// Logger abstracts slog.Logger for testability.
type Logger interface {
	Info(msg string, args ...any)
	Error(msg string, args ...any)
	Warn(msg string, args ...any)
	Debug(msg string, args ...any)
}

// NewScheduler creates a new Scheduler.
func NewScheduler(logger Logger) *Scheduler {
	return &Scheduler{
		timers:      make(map[string]*time.Timer),
		intervals:   make(map[string]time.Duration),
		cancelFuncs: make(map[string]context.CancelFunc),
		doneChans:   make(map[string]chan struct{}),
		logger:      logger,
	}
}

// StartProject begins periodic crawling for a project at the given interval.
// It creates a goroutine with a ticker that calls crawlFunc on each tick.
// If a crawl is already running for this project, StartProject waits for it
// to fully exit before starting a replacement, preventing duplicate goroutines.
func (s *Scheduler) StartProject(name string, interval time.Duration, crawlFunc func(ctx context.Context) error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Stop existing goroutine if running and wait for it to fully exit.
	// We hold the mutex during the wait — safe because the exiting goroutine
	// only closes the done channel and never acquires the scheduler mutex.
	// Holding the lock prevents concurrent StartProject calls from starting
	// duplicate goroutines while the old one is still winding down.
	if cancel, ok := s.cancelFuncs[name]; ok {
		cancel()
		if done, ok := s.doneChans[name]; ok {
			<-done // Wait for old goroutine to finish (holds mutex).
		}
		delete(s.cancelFuncs, name)
		delete(s.doneChans, name)
	}
	if timer, ok := s.timers[name]; ok {
		timer.Stop()
		delete(s.timers, name)
	}

	ctx, cancel := context.WithCancel(context.Background())
	s.cancelFuncs[name] = cancel
	s.intervals[name] = interval
	done := make(chan struct{})
	s.doneChans[name] = done

	// Run immediately on start, then on interval.
	go func() {
		defer close(done) // Signal goroutine has exited.

		// Initial crawl.
		if err := crawlFunc(ctx); err != nil && ctx.Err() == nil {
			s.logger.Error("initial crawl failed", "project", name, "error", err)
		}

		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := crawlFunc(ctx); err != nil && ctx.Err() == nil {
					s.logger.Error("scheduled crawl failed", "project", name, "error", err)
				}
			}
		}
	}()

	s.timers[name] = time.NewTimer(interval) // placeholder to track lifecycle
	s.logger.Info("started project crawler", "project", name, "interval", interval)
}

// StopProject stops the crawler for a single project.
func (s *Scheduler) StopProject(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if cancel, ok := s.cancelFuncs[name]; ok {
		cancel()
		delete(s.cancelFuncs, name)
	}
	if timer, ok := s.timers[name]; ok {
		timer.Stop()
		delete(s.timers, name)
	}
	delete(s.intervals, name)
	delete(s.doneChans, name)
	s.logger.Info("stopped project crawler", "project", name)
}

// StopAll stops all project crawlers.
func (s *Scheduler) StopAll() {
	s.mu.Lock()
	defer s.mu.Unlock()

	for name, cancel := range s.cancelFuncs {
		cancel()
		delete(s.cancelFuncs, name)
	}
	for name, timer := range s.timers {
		timer.Stop()
		delete(s.timers, name)
	}
	s.intervals = make(map[string]time.Duration)
	s.doneChans = make(map[string]chan struct{})
	s.logger.Info("stopped all project crawlers")
}

// Running returns true if a project has an active crawler.
func (s *Scheduler) Running(name string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.cancelFuncs[name]
	return ok
}
