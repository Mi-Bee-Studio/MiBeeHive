package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/system"
	"github.com/Mi-Bee-Studio/mibeehive/internal/model"
)

// dockerStatsClient abstracts the Docker client operations needed for
// container stats, logs, and engine info.
type dockerStatsClient interface {
	ContainerStatsOneShot(ctx context.Context, containerID string) (container.StatsResponseReader, error)
	ContainerLogs(ctx context.Context, container string, options container.LogsOptions) (io.ReadCloser, error)
	Info(ctx context.Context) (system.Info, error)
}

// ContainerLogService provides container stats, log streaming, and Docker
// engine info through the Docker SDK client.
type ContainerLogService struct {
	client dockerStatsClient
	logger *slog.Logger
}

// NewContainerLogService creates a new ContainerLogService with the given
// Docker client wrapper.
func NewContainerLogService(client dockerStatsClient) *ContainerLogService {
	return &ContainerLogService{
		client: client,
		logger: slog.Default().With("component", "container_log_service"),
	}
}

// ContainerStats retrieves a single snapshot of resource usage stats for the
// given container. It decodes the JSON response from Docker, calculates CPU
// percentage, and returns populated model.ContainerStats.
//
// CPU calculation: CPU% = (cpu_delta / system_delta) * nr_cpus * 100
// If system_delta is 0 the result is 0 (avoids division by zero).
// Falls back to len(PercpuUsage) when OnlineCPUs is not set.
func (s *ContainerLogService) ContainerStats(ctx context.Context, id string) (*model.ContainerStats, error) {
	resp, err := s.client.ContainerStatsOneShot(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("container stats: %w", err)
	}
	defer resp.Body.Close()

	var stats container.StatsResponse
	if err := json.NewDecoder(resp.Body).Decode(&stats); err != nil {
		return nil, fmt.Errorf("decode stats: %w", err)
	}

	// CPU percentage calculation.
	cpuDelta := float64(stats.CPUStats.CPUUsage.TotalUsage - stats.PreCPUStats.CPUUsage.TotalUsage)
	systemDelta := float64(stats.CPUStats.SystemUsage - stats.PreCPUStats.SystemUsage)

	var cpuPercent float64
	if systemDelta > 0 {
		nrCPUs := uint32(stats.CPUStats.OnlineCPUs)
		if nrCPUs == 0 && len(stats.CPUStats.CPUUsage.PercpuUsage) > 0 {
			nrCPUs = uint32(len(stats.CPUStats.CPUUsage.PercpuUsage))
		}
		if nrCPUs == 0 {
			nrCPUs = 1
		}
		cpuPercent = (cpuDelta / systemDelta) * float64(nrCPUs) * 100.0
	}

	// Memory in MB.
	const bytesPerMB = 1024 * 1024
	memUsageMB := float64(stats.MemoryStats.Usage) / float64(bytesPerMB)
	memLimitMB := float64(stats.MemoryStats.Limit) / float64(bytesPerMB)

	// Network totals across all interfaces.
	var rxTotal, txTotal int64
	for _, ns := range stats.Networks {
		rxTotal += int64(ns.RxBytes)
		txTotal += int64(ns.TxBytes)
	}

	// Block I/O totals.
	var blockRead, blockWrite int64
	for _, entry := range stats.BlkioStats.IoServiceBytesRecursive {
		switch entry.Op {
		case "Read":
			blockRead += int64(entry.Value)
		case "Write":
			blockWrite += int64(entry.Value)
		}
	}

	return &model.ContainerStats{
		CPUUsagePercent: cpuPercent,
		MemoryUsageMB:   memUsageMB,
		MemoryLimitMB:   memLimitMB,
		NetworkRxBytes:  rxTotal,
		NetworkTxBytes:  txTotal,
		BlockReadBytes:  blockRead,
		BlockWriteBytes: blockWrite,
	}, nil
}

// ContainerLogs returns an io.ReadCloser for streaming container logs.
// The tail parameter controls how many lines from the end are returned
// (defaults to "100" if empty). The since parameter filters logs after
// a given timestamp (RFC 3339 or Unix timestamp).
//
// The caller is responsible for closing the returned reader.
func (s *ContainerLogService) ContainerLogs(ctx context.Context, id string, tail string, since string) (io.ReadCloser, error) {
	if tail == "" {
		tail = "100"
	}

	opts := container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Tail:       tail,
		Since:      since,
	}

	reader, err := s.client.ContainerLogs(ctx, id, opts)
	if err != nil {
		return nil, fmt.Errorf("container logs: %w", err)
	}

	return reader, nil
}

// DockerInfo returns the Docker engine information as a map.
// This includes system-level details like OS, CPU count, memory,
// container counts, and Docker version.
func (s *ContainerLogService) DockerInfo(ctx context.Context) (map[string]any, error) {
	info, err := s.client.Info(ctx)
	if err != nil {
		return nil, fmt.Errorf("docker info: %w", err)
	}

	// Use JSON marshal/unmarshal to convert the struct to a generic map.
	// This is the simplest approach to handle the complex system.Info struct
	// with its many fields and nested types.
	b, err := json.Marshal(info)
	if err != nil {
		return nil, fmt.Errorf("marshal docker info: %w", err)
	}

	var result map[string]any
	if err := json.Unmarshal(b, &result); err != nil {
		return nil, fmt.Errorf("unmarshal docker info: %w", err)
	}

	return result, nil
}
