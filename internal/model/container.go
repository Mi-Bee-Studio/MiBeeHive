package model

import "time"

// Container represents a managed Docker container application.
type Container struct {
	ID            int64             `json:"id"`
	Name          string            `json:"name"`
	Image         string            `json:"image"`
	Command       string            `json:"command"`
	Env           map[string]string `json:"env"`
	Ports         []PortMapping     `json:"ports"`
	Volumes       []VolumeMount     `json:"volumes"`
	RestartPolicy string            `json:"restart_policy"`
	MemoryLimit   string            `json:"memory_limit"`
	CPULimit      float64           `json:"cpu_limit"`
	Status        string            `json:"status"`
	ContainerID   string            `json:"container_id"`
	CreatedAt     time.Time         `json:"created_at"`
	UpdatedAt     time.Time         `json:"updated_at"`
}

// PortMapping represents a container-to-host port mapping.
type PortMapping struct {
	HostPort      int    `json:"host_port"`
	ContainerPort int    `json:"container_port"`
	Protocol      string `json:"protocol"`
}

// VolumeMount represents a container volume mount.
type VolumeMount struct {
	HostPath      string `json:"host_path"`
	ContainerPath string `json:"container_path"`
	Mode          string `json:"mode"`
}

// AppTemplate represents a pre-configured application template.
type AppTemplate struct {
	ID            int64             `json:"id"`
	Name          string            `json:"name"`
	Description   string            `json:"description"`
	Image         string            `json:"image"`
	Command       string            `json:"command"`
	Env           map[string]string `json:"env"`
	Ports         []PortMapping     `json:"ports"`
	Volumes       []VolumeMount     `json:"volumes"`
	RestartPolicy string            `json:"restart_policy"`
	Category      string            `json:"category"`
	Icon          string            `json:"icon"`
	Enabled       bool              `json:"enabled"`
	CreatedAt     time.Time         `json:"created_at"`
	UpdatedAt     time.Time         `json:"updated_at"`
}

// CreateContainerRequest is the request body for creating a new container.
type CreateContainerRequest struct {
	Name          string            `json:"name"`
	Image         string            `json:"image"`
	Command       string            `json:"command"`
	Env           map[string]string `json:"env"`
	Ports         []PortMapping     `json:"ports"`
	Volumes       []VolumeMount     `json:"volumes"`
	RestartPolicy string            `json:"restart_policy"`
	MemoryLimit   string            `json:"memory_limit"`
	CPULimit      float64           `json:"cpu_limit"`
}

// ContainerStats represents real-time resource usage stats for a container.
type ContainerStats struct {
	CPUUsagePercent float64 `json:"cpu_usage_percent"`
	MemoryUsageMB   float64 `json:"memory_usage_mb"`
	MemoryLimitMB   float64 `json:"memory_limit_mb"`
	NetworkRxBytes  int64   `json:"network_rx_bytes"`
	NetworkTxBytes  int64   `json:"network_tx_bytes"`
	BlockReadBytes  int64   `json:"block_read_bytes"`
	BlockWriteBytes int64   `json:"block_write_bytes"`
	Timestamp       int64   `json:"timestamp"`
}

// ContainerLogEntry represents a single log line from a container.
type ContainerLogEntry struct {
	Timestamp string `json:"timestamp"`
	Stream    string `json:"stream"` // "stdout" or "stderr"
	Content   string `json:"content"`
}

// Image represents a Docker image.
type Image struct {
	ID        string    `json:"id"`
	RepoTags  []string  `json:"repo_tags"`
	SizeMB    float64   `json:"size_mb"`
	CreatedAt time.Time `json:"created_at"`
}

// Task represents a background task in the task center.
type Task struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	Type       string  `json:"type"` // "crawl", "download", "backup", "iso_check"
	Status     string  `json:"status"`
	NextRunAt  string  `json:"next_run_at,omitempty"`
	LastRunAt  string  `json:"last_run_at,omitempty"`
	LastResult string  `json:"last_result,omitempty"`
	Schedule   string  `json:"schedule,omitempty"`
	Progress   float64 `json:"progress,omitempty"`
}

// LogEntry represents a unified log entry for the log viewer.
type LogEntry struct {
	ID        string `json:"id"`
	Type      string `json:"type"` // "crawl", "app", "download"
	Timestamp string `json:"timestamp"`
	Level     string `json:"level"`
	Message   string `json:"message"`
	Source    string `json:"source,omitempty"`
}

// SearchResult represents a global search result item.
type SearchResult struct {
	Type   string `json:"type"` // "project", "file", "config", "iso", "container"
	ID     int64  `json:"id"`
	Name   string `json:"name"`
	Detail string `json:"detail,omitempty"`
}

// SearchResponse represents the response from a global search query.
type SearchResponse struct {
	Projects   []SearchResult `json:"projects"`
	Files      []SearchResult `json:"files"`
	Configs    []SearchResult `json:"configs"`
	ISOs       []SearchResult `json:"isos"`
	Containers []SearchResult `json:"containers"`
	Total      int            `json:"total"`
}
