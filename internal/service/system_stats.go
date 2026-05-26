package service

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Mi-Bee-Studio/mibeehive/internal/model"
)

// FetchSystemStats scrapes a node_exporter metrics endpoint for live CPU/memory stats.
func FetchSystemStats(ctx context.Context, nodeExporterURL string) (*model.SystemStatsResponse, error) {
	httpCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(httpCtx, http.MethodGet, nodeExporterURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch metrics: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("node_exporter returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read metrics body: %w", err)
	}

	// Parse Prometheus text format
	lines := strings.Split(string(body), "\n")

	type cpuData struct {
		idle  float64
		total float64
	}
	cpuMap := make(map[int]cpuData)

	var memTotal, memAvailable float64
	var netRxTotal, netTxTotal float64

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		if strings.HasPrefix(line, "node_cpu_seconds_total") {
			val := parseMetricValue(line)
			if val == 0 {
				continue
			}
			cpuIdx := extractLabelValue(line, "cpu")
			if cpuIdx == "" {
				continue
			}
			cpuNum, err := strconv.Atoi(cpuIdx)
			if err != nil {
				continue
			}
			d := cpuMap[cpuNum]
			d.total += val
			if strings.Contains(line, `mode="idle"`) {
				d.idle += val
			}
			cpuMap[cpuNum] = d
			continue
		}

		if strings.HasPrefix(line, "node_memory_MemTotal_bytes ") {
			memTotal = parseMetricValue(line)
			continue
		}

		if strings.HasPrefix(line, "node_memory_MemAvailable_bytes ") {
			memAvailable = parseMetricValue(line)
			continue
		}

		if strings.HasPrefix(line, "node_network_receive_bytes_total") && !strings.Contains(line, "device=\"lo\"") {
			netRxTotal += parseMetricValue(line)
			continue
		}

		if strings.HasPrefix(line, "node_network_transmit_bytes_total") && !strings.Contains(line, "device=\"lo\"") {
			netTxTotal += parseMetricValue(line)
			continue
		}
	}

	// Calculate CPU usage: average across all CPUs
	var cpuUsage float64
	if len(cpuMap) > 0 {
		var totalUsage float64
		for _, d := range cpuMap {
			if d.total > 0 {
				totalUsage += (1 - d.idle/d.total) * 100
			}
		}
		cpuUsage = totalUsage / float64(len(cpuMap))
	}

	// Calculate memory
	memUsed := memTotal - memAvailable
	var memUsagePercent float64
	if memTotal > 0 {
		memUsagePercent = (memUsed / memTotal) * 100
	}

	return &model.SystemStatsResponse{
		CpuUsagePercent:    cpuUsage,
		MemoryTotalBytes:   uint64(memTotal),
		MemoryUsedBytes:    uint64(memUsed),
		MemoryUsagePercent: memUsagePercent,
		NetworkRxBytes:     uint64(netRxTotal),
		NetworkTxBytes:     uint64(netTxTotal),
	}, nil
}

// parseMetricValue extracts the numeric value from a Prometheus metric line.
func parseMetricValue(line string) float64 {
	parts := strings.Fields(line)
	for i := len(parts) - 1; i >= 0; i-- {
		val, err := strconv.ParseFloat(parts[i], 64)
		if err == nil {
			return val
		}
	}
	return 0
}

// extractLabelValue extracts a label value from a Prometheus metric line.
func extractLabelValue(line, label string) string {
	start := strings.Index(line, "{")
	if start == -1 {
		return ""
	}
	end := strings.Index(line, "}")
	if end == -1 {
		return ""
	}
	labels := line[start+1 : end]

	prefix := label + `="`
	idx := strings.Index(labels, prefix)
	if idx == -1 {
		return ""
	}
	rest := labels[idx+len(prefix):]
	closeIdx := strings.Index(rest, `"`)
	if closeIdx == -1 {
		return ""
	}
	return rest[:closeIdx]
}
