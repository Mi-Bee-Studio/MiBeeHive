package service

import (
	"context"
	"log/slog"
	"time"
)

// RetentionScheduler runs enabled retention policies on a configurable interval.
type RetentionScheduler struct {
	service  *RetentionService
	interval time.Duration
	done     chan struct{}
}

// NewRetentionScheduler creates a new RetentionScheduler.
func NewRetentionScheduler(service *RetentionService, interval time.Duration) *RetentionScheduler {
	return &RetentionScheduler{
		service:  service,
		interval: interval,
		done:     make(chan struct{}),
	}
}

// Start begins the scheduler loop. It runs all enabled policies at each tick.
// Policies are executed sequentially (single-writer DB constraint).
func (s *RetentionScheduler) Start(ctx context.Context) {
	ticker := time.NewTicker(s.interval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				slog.Info("retention scheduler stopped via context")
				return
			case <-s.done:
				slog.Info("retention scheduler stopped")
				return
			case <-ticker.C:
				s.runAllPolicies(ctx)
			}
		}
	}()
	slog.Info("retention scheduler started", "interval", s.interval)
}

// Stop signals the scheduler to stop. It closes the done channel.
func (s *RetentionScheduler) Stop() {
	close(s.done)
	slog.Info("retention scheduler stop requested")
}

// runAllPolicies fetches enabled policies and executes them sequentially.
func (s *RetentionScheduler) runAllPolicies(ctx context.Context) {
	if s.service == nil || s.service.policyRepo == nil {
		return
	}


	policies, err := s.service.policyRepo.ListEnabled(ctx)
	if err != nil {
		slog.Error("retention scheduler: listing enabled policies", "error", err)
		return
	}

	if len(policies) == 0 {
		return
	}

	slog.Info("retention scheduler: executing policies", "count", len(policies))

	for _, policy := range policies {
		if ctx.Err() != nil {
			slog.Info("retention scheduler: context cancelled, stopping")
			return
		}

		deleted, err := s.service.ExecutePolicy(ctx, policy.ID)
		if err != nil {
			slog.Error("retention scheduler: policy execution failed",
				"policy_id", policy.ID, "error", err)
			continue
		}
		slog.Info("retention scheduler: policy executed",
			"policy_id", policy.ID, "tags_deleted", deleted)
	}
}
