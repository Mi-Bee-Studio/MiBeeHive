package monitor

import (
	"context"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/Mi-Bee-Studio/mibeehive/internal/diskutil"
)

// DiskMonitor periodically checks disk usage and tracks warning/critical states.
// State is exposed via atomic booleans for lock-free reads from any goroutine.
// Logging only fires on state transitions — not on every check.
type DiskMonitor struct {
	path              string
	warningThreshold  int // percentage
	criticalThreshold int // percentage
	checkInterval     time.Duration

	isWarning  atomic.Bool
	isCritical atomic.Bool

	// Previous state for change detection (only written in check goroutine).
	wasWarning  bool
	wasCritical bool
}

// NewDiskMonitor creates a disk monitor for the given path.
// warningPct and criticalPct are usage percentages (0-100) that trigger states.
func NewDiskMonitor(path string, warningPct, criticalPct int, checkInterval time.Duration) *DiskMonitor {
	return &DiskMonitor{
		path:              path,
		warningThreshold:  warningPct,
		criticalThreshold: criticalPct,
		checkInterval:     checkInterval,
	}
}

// IsWarning returns true if disk usage is at or above the warning threshold.
func (d *DiskMonitor) IsWarning() bool { return d.isWarning.Load() }

// IsDegraded returns true if disk usage is at or above the critical threshold.
func (d *DiskMonitor) IsDegraded() bool { return d.isCritical.Load() }

// Start begins the background monitoring loop.
// Performs an initial check immediately, then checks at the configured interval.
// Blocks the calling goroutine until ctx is cancelled.
func (d *DiskMonitor) Start(ctx context.Context) {
	d.check()

	ticker := time.NewTicker(d.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			d.check()
		case <-ctx.Done():
			slog.Info("disk monitor stopped")
			return
		}
	}
}

func (d *DiskMonitor) check() {
	usagePct, available, err := d.usagePercent()
	if err != nil {
		slog.Error("disk monitor: statfs failed", "path", d.path, "error", err)
		return
	}

	nowWarning := usagePct >= d.warningThreshold
	nowCritical := usagePct >= d.criticalThreshold

	d.isWarning.Store(nowWarning)
	d.isCritical.Store(nowCritical)

	// Log only on state changes.
	if nowCritical != d.wasCritical {
		if nowCritical {
			slog.Error("disk critically full — entering degraded mode",
				"path", d.path,
				"used_percent", usagePct,
				"critical_threshold", d.criticalThreshold,
				"available_bytes", available,
			)
		} else {
			slog.Info("disk recovered from critical state",
				"path", d.path,
				"used_percent", usagePct,
				"available_bytes", available,
			)
		}
	} else if nowWarning != d.wasWarning {
		if nowWarning {
			slog.Warn("disk usage above warning threshold",
				"path", d.path,
				"used_percent", usagePct,
				"warning_threshold", d.warningThreshold,
				"available_bytes", available,
			)
		} else {
			slog.Info("disk usage below warning threshold",
				"path", d.path,
				"used_percent", usagePct,
			)
		}
	}

	d.wasWarning = nowWarning
	d.wasCritical = nowCritical
}

// usagePercent returns current disk usage as a percentage and available bytes.
func (d *DiskMonitor) usagePercent() (usedPct int, available uint64, err error) {
	total, _, available, err := diskutil.Usage(d.path)
	if err != nil {
		return 0, 0, err
	}

	used := total - available

	if total > 0 {
		usedPct = int((used * 100) / total)
	}
	return usedPct, available, nil
}

// GetUsage returns current disk usage stats (total, used, available bytes).
func (d *DiskMonitor) GetUsage() (total, used, available uint64, err error) {
	total, _, available, err = diskutil.Usage(d.path)
	if err != nil {
		return 0, 0, 0, err
	}
	used = total - available
	return total, used, available, nil
}
