package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// SystemStat represents a sampled system metric point.
type SystemStat struct {
	ID                 int64
	SampledAt          time.Time
	CpuUsagePercent    float64
	MemoryTotalBytes   uint64
	MemoryUsedBytes    uint64
	MemoryUsagePercent float64
	NetworkRxBytes     uint64
	NetworkTxBytes     uint64
}

// SystemStatsRepo provides operations for system_stats table.
type SystemStatsRepo struct {
	db *sql.DB
}

// NewSystemStatsRepo creates a new SystemStatsRepo.
func NewSystemStatsRepo(db *sql.DB) *SystemStatsRepo {
	return &SystemStatsRepo{db: db}
}

// Insert adds a new system stat sample.
func (r *SystemStatsRepo) Insert(ctx context.Context, s *SystemStat) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO system_stats (sampled_at, cpu_usage_percent, memory_total_bytes,
		 memory_used_bytes, memory_usage_percent, network_rx_bytes, network_tx_bytes)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		s.SampledAt, s.CpuUsagePercent, s.MemoryTotalBytes,
		s.MemoryUsedBytes, s.MemoryUsagePercent, s.NetworkRxBytes, s.NetworkTxBytes)
	if err != nil {
		return fmt.Errorf("inserting system stat: %w", err)
	}
	return nil
}

// QueryHistory returns all samples since the given time, ordered by sampled_at ASC.
func (r *SystemStatsRepo) QueryHistory(ctx context.Context, since time.Time) ([]*SystemStat, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, sampled_at, cpu_usage_percent, memory_total_bytes, memory_used_bytes,
		 memory_usage_percent, network_rx_bytes, network_tx_bytes
		 FROM system_stats WHERE sampled_at >= ? ORDER BY sampled_at ASC`, since)
	if err != nil {
		return nil, fmt.Errorf("querying system stats history: %w", err)
	}
	defer rows.Close()

	var stats []*SystemStat
	for rows.Next() {
		s := &SystemStat{}
		if err := rows.Scan(&s.ID, &s.SampledAt, &s.CpuUsagePercent, &s.MemoryTotalBytes,
			&s.MemoryUsedBytes, &s.MemoryUsagePercent, &s.NetworkRxBytes, &s.NetworkTxBytes); err != nil {
			return nil, fmt.Errorf("scanning system stat: %w", err)
		}
		stats = append(stats, s)
	}
	return stats, rows.Err()
}

// PurgeOlderThan deletes samples older than the given time.
func (r *SystemStatsRepo) PurgeOlderThan(ctx context.Context, before time.Time) (int64, error) {
	result, err := r.db.ExecContext(ctx, `DELETE FROM system_stats WHERE sampled_at < ?`, before)
	if err != nil {
		return 0, fmt.Errorf("purging system stats: %w", err)
	}
	return result.RowsAffected()
}
