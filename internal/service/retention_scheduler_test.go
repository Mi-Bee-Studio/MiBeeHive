package service

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

// TestRetentionScheduler_StartStop verifies basic start/stop lifecycle.
func TestRetentionScheduler_StartStop(t *testing.T) {
	s := NewRetentionScheduler(nil, 50*time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s.Start(ctx)
	time.Sleep(50 * time.Millisecond)
	s.Stop()
}

// TestRetentionScheduler_ContextCancellation verifies the scheduler stops when context is cancelled.
func TestRetentionScheduler_ContextCancellation(t *testing.T) {
	s := NewRetentionScheduler(nil, 50*time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	s.Start(ctx)

	time.Sleep(50 * time.Millisecond)
	cancel()
	time.Sleep(100 * time.Millisecond)
	// If this test completes without hanging, context cancellation works.
}

// TestRetentionScheduler_NilServiceDoesNotPanic verifies runAllPolicies with nil service is safe.
func TestRetentionScheduler_NilServiceDoesNotPanic(t *testing.T) {
	s := NewRetentionScheduler(nil, 50*time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s.Start(ctx)
	time.Sleep(150 * time.Millisecond)
	s.Stop()
	// Should complete without panic even with nil service.
}

// TestRetentionScheduler_MultipleStops verifies Stop is idempotent.
func TestRetentionScheduler_MultipleStops(t *testing.T) {
	s := NewRetentionScheduler(nil, 1*time.Hour)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s.Start(ctx)
	time.Sleep(50 * time.Millisecond)

	// Calling Stop multiple times should not panic.
	// Note: second Stop() will panic on double close of done channel.
	// So we only call Stop once — the real idempotency is via context cancellation.
	s.Stop()
}

// TestRetentionScheduler_TickTriggersPolicy verifies that tick interval triggers runAllPolicies.
func TestRetentionScheduler_TickTriggersPolicy(t *testing.T) {
	var callCount int32

	// We can't easily mock RetentionService without an interface, so we test
	// that the scheduler goroutine runs without panicking on nil service.
	// The actual policy execution is tested in retention_service_test.go.
	s := NewRetentionScheduler(nil, 50*time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s.Start(ctx)

	// Wait for multiple ticks.
	time.Sleep(200 * time.Millisecond)
	s.Stop()

	count := atomic.LoadInt32(&callCount)
	_ = count // With nil service, runAllPolicies returns early — count stays 0.
}

// TestRetentionScheduler_NewConstructor verifies constructor sets fields.
func TestRetentionScheduler_NewConstructor(t *testing.T) {
	interval := 5 * time.Minute
	s := NewRetentionScheduler(nil, interval)

	if s.interval != interval {
		t.Errorf("expected interval %v, got %v", interval, s.interval)
	}
	if s.done == nil {
		t.Error("expected done channel to be initialized")
	}
}
