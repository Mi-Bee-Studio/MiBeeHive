package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/system"
)

// mockDockerClient implements dockerStatsClient for testing.
type mockDockerClient struct {
	statsResponse container.StatsResponseReader
	statsErr      error
	logsReader    io.ReadCloser
	logsErr       error
	infoResponse  system.Info
	infoErr       error
}

func (m *mockDockerClient) ContainerStatsOneShot(_ context.Context, _ string) (container.StatsResponseReader, error) {
	return m.statsResponse, m.statsErr
}

func (m *mockDockerClient) ContainerLogs(_ context.Context, _ string, _ container.LogsOptions) (io.ReadCloser, error) {
	return m.logsReader, m.logsErr
}

func (m *mockDockerClient) Info(_ context.Context) (system.Info, error) {
	return m.infoResponse, m.infoErr
}

// mockReadCloser wraps a string reader with a Close method.
type mockReadCloser struct {
	*strings.Reader
	closed bool
}

func (m *mockReadCloser) Close() error {
	m.closed = true
	return nil
}

func newMockReadCloser(s string) *mockReadCloser {
	return &mockReadCloser{Reader: strings.NewReader(s)}
}

// buildStatsJSON creates a Docker stats JSON response for testing.
func buildStatsJSON(cpuDelta, systemDelta, onlineCPUs uint64,
	memUsage, memLimit uint64,
	networks map[string]container.NetworkStats,
	blkioEntries []container.BlkioStatEntry,
) string {
	stats := container.StatsResponse{
		Stats: container.Stats{
			PreCPUStats: container.CPUStats{
				CPUUsage: container.CPUUsage{
					TotalUsage: 1000,
				},
				SystemUsage: 5000,
			},
			CPUStats: container.CPUStats{
				CPUUsage: container.CPUUsage{
					TotalUsage: 1000 + cpuDelta,
				},
				SystemUsage: 5000 + systemDelta,
				OnlineCPUs:  uint32(onlineCPUs),
			},
			MemoryStats: container.MemoryStats{
				Usage: memUsage,
				Limit: memLimit,
			},
			BlkioStats: container.BlkioStats{
				IoServiceBytesRecursive: blkioEntries,
			},
		},
		Networks: networks,
	}
	b, _ := json.Marshal(stats)
	return string(b)
}

func TestContainerStats_Success(t *testing.T) {
	networks := map[string]container.NetworkStats{
		"eth0": {RxBytes: 1024, TxBytes: 2048},
	}
	blkio := []container.BlkioStatEntry{
		{Op: "Read", Value: 4096},
		{Op: "Write", Value: 8192},
	}
	statsJSON := buildStatsJSON(500, 10000, 2, 104857600, 209715200, networks, blkio)
	rc := newMockReadCloser(statsJSON)

	svc := &ContainerLogService{
		client: &mockDockerClient{
			statsResponse: container.StatsResponseReader{Body: rc},
		},
	}

	stats, err := svc.ContainerStats(context.Background(), "test-container-id")
	if err != nil {
		t.Fatalf("ContainerStats() error = %v", err)
	}

	// CPU% = (500 / 10000) * 2 * 100 = 10.0
	if want := 10.0; stats.CPUUsagePercent != want {
		t.Errorf("CPUUsagePercent = %v, want %v", stats.CPUUsagePercent, want)
	}

	// Memory usage: 104857600 / (1024*1024) = 100 MB
	if want := 100.0; stats.MemoryUsageMB != want {
		t.Errorf("MemoryUsageMB = %v, want %v", stats.MemoryUsageMB, want)
	}

	// Memory limit: 209715200 / (1024*1024) = 200 MB
	if want := 200.0; stats.MemoryLimitMB != want {
		t.Errorf("MemoryLimitMB = %v, want %v", stats.MemoryLimitMB, want)
	}

	// Network
	if want := int64(1024); stats.NetworkRxBytes != want {
		t.Errorf("NetworkRxBytes = %v, want %v", stats.NetworkRxBytes, want)
	}
	if want := int64(2048); stats.NetworkTxBytes != want {
		t.Errorf("NetworkTxBytes = %v, want %v", stats.NetworkTxBytes, want)
	}

	// Block I/O
	if want := int64(4096); stats.BlockReadBytes != want {
		t.Errorf("BlockReadBytes = %v, want %v", stats.BlockReadBytes, want)
	}
	if want := int64(8192); stats.BlockWriteBytes != want {
		t.Errorf("BlockWriteBytes = %v, want %v", stats.BlockWriteBytes, want)
	}

	// Body should be closed
	if !rc.closed {
		t.Error("response body was not closed")
	}
}

func TestContainerStats_ZeroSystemDelta(t *testing.T) {
	// When system delta is 0, CPU% should be 0 (avoid division by zero)
	statsJSON := buildStatsJSON(0, 0, 2, 52428800, 104857600, nil, nil)
	rc := newMockReadCloser(statsJSON)

	svc := &ContainerLogService{
		client: &mockDockerClient{
			statsResponse: container.StatsResponseReader{Body: rc},
		},
	}

	stats, err := svc.ContainerStats(context.Background(), "test-id")
	if err != nil {
		t.Fatalf("ContainerStats() error = %v", err)
	}

	if stats.CPUUsagePercent != 0 {
		t.Errorf("CPUUsagePercent = %v, want 0 when system delta is 0", stats.CPUUsagePercent)
	}
}

func TestContainerStats_FallbackToPercpuUsage(t *testing.T) {
	// When OnlineCPUs is 0, fall back to len(PercpuUsage)
	stats := container.StatsResponse{
		Stats: container.Stats{
			PreCPUStats: container.CPUStats{
				CPUUsage:    container.CPUUsage{TotalUsage: 100},
				SystemUsage: 500,
			},
			CPUStats: container.CPUStats{
				CPUUsage: container.CPUUsage{
					TotalUsage:  200,
					PercpuUsage: []uint64{1, 1, 1, 1}, // 4 CPUs
				},
				SystemUsage: 1000,
				OnlineCPUs:  0, // not set
			},
			MemoryStats: container.MemoryStats{Usage: 52428800, Limit: 104857600},
		},
	}
	b, _ := json.Marshal(stats)
	rc := newMockReadCloser(string(b))

	svc := &ContainerLogService{
		client: &mockDockerClient{
			statsResponse: container.StatsResponseReader{Body: rc},
		},
	}

	result, err := svc.ContainerStats(context.Background(), "test-id")
	if err != nil {
		t.Fatalf("ContainerStats() error = %v", err)
	}

	// CPU% = (100 / 500) * 4 * 100 = 80.0
	if want := 80.0; result.CPUUsagePercent != want {
		t.Errorf("CPUUsagePercent = %v, want %v", result.CPUUsagePercent, want)
	}
}

func TestContainerStats_DockerError(t *testing.T) {
	svc := &ContainerLogService{
		client: &mockDockerClient{
			statsErr: fmt.Errorf("container not found"),
		},
	}

	_, err := svc.ContainerStats(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestContainerStats_DecodeError(t *testing.T) {
	rc := newMockReadCloser("not valid json")
	svc := &ContainerLogService{
		client: &mockDockerClient{
			statsResponse: container.StatsResponseReader{Body: rc},
		},
	}

	_, err := svc.ContainerStats(context.Background(), "test-id")
	if err == nil {
		t.Fatal("expected decode error, got nil")
	}

	if !rc.closed {
		t.Error("response body was not closed after decode error")
	}
}

func TestContainerStats_MultipleNetworks(t *testing.T) {
	networks := map[string]container.NetworkStats{
		"eth0": {RxBytes: 1000, TxBytes: 2000},
		"eth1": {RxBytes: 3000, TxBytes: 4000},
	}
	statsJSON := buildStatsJSON(100, 1000, 1, 52428800, 104857600, networks, nil)
	rc := newMockReadCloser(statsJSON)

	svc := &ContainerLogService{
		client: &mockDockerClient{
			statsResponse: container.StatsResponseReader{Body: rc},
		},
	}

	stats, err := svc.ContainerStats(context.Background(), "test-id")
	if err != nil {
		t.Fatalf("ContainerStats() error = %v", err)
	}

	// Total: rx=4000, tx=6000
	if want := int64(4000); stats.NetworkRxBytes != want {
		t.Errorf("NetworkRxBytes = %v, want %v", stats.NetworkRxBytes, want)
	}
	if want := int64(6000); stats.NetworkTxBytes != want {
		t.Errorf("NetworkTxBytes = %v, want %v", stats.NetworkTxBytes, want)
	}
}

func TestContainerLogs_Success(t *testing.T) {
	logData := "line1\nline2\nline3\n"
	rc := newMockReadCloser(logData)

	svc := &ContainerLogService{
		client: &mockDockerClient{
			logsReader: rc,
		},
	}

	reader, err := svc.ContainerLogs(context.Background(), "test-id", "100", "2024-01-01")
	if err != nil {
		t.Fatalf("ContainerLogs() error = %v", err)
	}
	if reader == nil {
		t.Fatal("ContainerLogs() returned nil reader")
	}

	// Verify we can read from the returned reader
	buf, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}
	if string(buf) != logData {
		t.Errorf("log content = %q, want %q", string(buf), logData)
	}
}

func TestContainerLogs_DefaultTail(t *testing.T) {
	rc := newMockReadCloser("")
	var capturedOpts container.LogsOptions

	svc := &ContainerLogService{
		client: &mockClientWithLogsOpts{
			reader:   rc,
			captured: &capturedOpts,
		},
	}

	_, err := svc.ContainerLogs(context.Background(), "test-id", "", "")
	if err != nil {
		t.Fatalf("ContainerLogs() error = %v", err)
	}

	if capturedOpts.Tail != "100" {
		t.Errorf("Tail = %q, want %q", capturedOpts.Tail, "100")
	}
	if !capturedOpts.ShowStdout {
		t.Error("ShowStdout should be true")
	}
	if !capturedOpts.ShowStderr {
		t.Error("ShowStderr should be true")
	}
}

// mockClientWithLogsOpts captures LogsOptions for verification.
type mockClientWithLogsOpts struct {
	reader   io.ReadCloser
	captured *container.LogsOptions
}

func (m *mockClientWithLogsOpts) ContainerStatsOneShot(_ context.Context, _ string) (container.StatsResponseReader, error) {
	return container.StatsResponseReader{}, nil
}

func (m *mockClientWithLogsOpts) ContainerLogs(_ context.Context, _ string, opts container.LogsOptions) (io.ReadCloser, error) {
	*m.captured = opts
	return m.reader, nil
}

func (m *mockClientWithLogsOpts) Info(_ context.Context) (system.Info, error) {
	return system.Info{}, nil
}

func TestContainerLogs_DockerError(t *testing.T) {
	svc := &ContainerLogService{
		client: &mockDockerClient{
			logsErr: fmt.Errorf("container not running"),
		},
	}

	_, err := svc.ContainerLogs(context.Background(), "test-id", "100", "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestDockerInfo_Success(t *testing.T) {
	info := system.Info{
		ID:                "ABCD1234",
		Containers:        10,
		ContainersRunning: 5,
		Images:            20,
		OperatingSystem:   "Debian GNU/Linux 12",
		NCPU:              4,
		MemTotal:          469 * 1024 * 1024 * 1024, // 469 GB in bytes
		ServerVersion:     "27.5.1",
	}

	svc := &ContainerLogService{
		client: &mockDockerClient{
			infoResponse: info,
		},
	}

	result, err := svc.DockerInfo(context.Background())
	if err != nil {
		t.Fatalf("DockerInfo() error = %v", err)
	}

	if result["ID"] != "ABCD1234" {
		t.Errorf("ID = %v, want ABCD1234", result["ID"])
	}
	if result["Containers"] != float64(10) {
		t.Errorf("Containers = %v, want 10", result["Containers"])
	}
	if result["ContainersRunning"] != float64(5) {
		t.Errorf("ContainersRunning = %v, want 5", result["ContainersRunning"])
	}
	if result["Images"] != float64(20) {
		t.Errorf("Images = %v, want 20", result["Images"])
	}
	if result["OperatingSystem"] != "Debian GNU/Linux 12" {
		t.Errorf("OperatingSystem = %v, want Debian GNU/Linux 12", result["OperatingSystem"])
	}
}

func TestDockerInfo_DockerError(t *testing.T) {
	svc := &ContainerLogService{
		client: &mockDockerClient{
			infoErr: fmt.Errorf("daemon not running"),
		},
	}

	_, err := svc.DockerInfo(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
