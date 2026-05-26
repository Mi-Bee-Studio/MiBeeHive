package handler

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"github.com/Mi-Bee-Studio/mibeehive/internal/middleware"
	"github.com/Mi-Bee-Studio/mibeehive/internal/model"
	"github.com/Mi-Bee-Studio/mibeehive/internal/service"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
)

// containerService defines the methods the handler needs from the container service.
// This interface allows testing without a real Docker daemon.
type containerService interface {
	List(ctx context.Context) ([]model.Container, error)
	Create(ctx context.Context, req model.CreateContainerRequest) (*model.Container, error)
	Start(ctx context.Context, id string) error
	Stop(ctx context.Context, id string, timeout int) error
	Restart(ctx context.Context, id string, timeout int) error
	Remove(ctx context.Context, id string, force bool) error
}

// imageService defines the methods the handler needs from the image service.
type imageService interface {
	ImageList(ctx context.Context) ([]model.Image, error)
	ImagePull(ctx context.Context, imageName string) error
	ImageDelete(ctx context.Context, imageID string) error
}

// containerLogReader defines the methods the handler needs for stats and logs.
type containerLogReader interface {
	ContainerStats(ctx context.Context, id string) (*model.ContainerStats, error)
	ContainerLogs(ctx context.Context, id string, tail string, since string) (io.ReadCloser, error)
}

// ContainerHandler handles container lifecycle, image, stats, and logs HTTP endpoints.
type ContainerHandler struct {
	svc             containerService
	imgSvc          imageService
	logSvc          containerLogReader
	logger          *slog.Logger
	dockerAvailable bool
	totalMemBytes   int64 // total system memory in bytes; 0 means unknown
}

// NewContainerHandler creates a new ContainerHandler.
func NewContainerHandler(svc *service.ContainerService, imgSvc *service.ImageService, logSvc *service.ContainerLogService, logger *slog.Logger) *ContainerHandler {
	memBytes, err := readSystemMemoryBytes()
	if err != nil {
		logger.Warn("cannot read system memory, skipping memory limit validation", "error", err)
	}
	return &ContainerHandler{
		svc:             svc,
		imgSvc:          imgSvc,
		logSvc:          logSvc,
		logger:          logger.With("component", "container-handler"),
		dockerAvailable: svc != nil,
		totalMemBytes:   memBytes,
	}
}

// checkDocker returns false and writes 503 if Docker is not available.
func (h *ContainerHandler) checkDocker(w http.ResponseWriter) bool {
	if !h.dockerAvailable {
		writeJSON(w, http.StatusServiceUnavailable, model.ApiResponse[any]{
			Success:   false,
			Message:   "Docker is not available on this server",
			ErrorCode: model.ERR_DOCKER_UNAVAILABLE,
		})
		return false
	}
	return true
}

// === Container Lifecycle Handlers ===

// HandleContainerList handles GET /api/v1/admin/containers.
func (h *ContainerHandler) HandleContainerList(w http.ResponseWriter, r *http.Request) {
	if !h.checkDocker(w) {
		return
	}

	limit := 50
	if v := r.URL.Query().Get("limit"); v != "" {
		if l, err := strconv.Atoi(v); err == nil && l > 0 {
			limit = l
		}
	}
	if limit > 200 {
		limit = 200
	}

	offset := 0
	if v := r.URL.Query().Get("offset"); v != "" {
		if o, err := strconv.Atoi(v); err == nil && o >= 0 {
			offset = o
		}
	}

	containers, err := h.svc.List(r.Context())
	if err != nil {
		middleware.WriteError(w, http.StatusServiceUnavailable, model.ERR_DOCKER_UNAVAILABLE, "容器服务不可用", err)
		return
	}

	if containers == nil {
		containers = []model.Container{}
	}

	total := len(containers)

	// Apply offset/limit slicing.
	if offset > total {
		offset = total
	}
	containers = containers[offset:]
	if limit < len(containers) {
		containers = containers[:limit]
	}

	w.Header().Set("X-Total-Count", strconv.Itoa(total))
	writeJSON(w, http.StatusOK, model.ApiResponse[[]model.Container]{
		Success: true,
		Data:    containers,
	})
}

// HandleContainerCreate handles POST /api/v1/admin/containers.
func (h *ContainerHandler) HandleContainerCreate(w http.ResponseWriter, r *http.Request) {
	if !h.checkDocker(w) {
		return
	}
	var req model.CreateContainerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, model.ApiResponse[any]{
			Success: false,
			Message: "invalid request body",
		})
		return
	}

	if req.Name == "" {
		writeJSON(w, http.StatusBadRequest, model.ApiResponse[any]{
			Success: false,
			Message: "name is required",
		})
		return
	}

	if req.Image == "" {
		writeJSON(w, http.StatusBadRequest, model.ApiResponse[any]{
			Success: false,
			Message: "image is required",
		})
		return
	}

	// Validate resource limits.
	if req.MemoryLimit != "" {
		memBytes, err := parseMemLimit(req.MemoryLimit)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, model.ApiResponse[any]{
				Success: false,
				Message: fmt.Sprintf("invalid memory limit: %s", req.MemoryLimit),
			})
			return
		}
		if h.totalMemBytes > 0 && memBytes > h.totalMemBytes {
			writeJSON(w, http.StatusBadRequest, model.ApiResponse[any]{
				Success: false,
				Message: fmt.Sprintf("memory limit exceeds device capacity (%d MB)", h.totalMemBytes/(1024*1024)),
			})
			return
		}
	}
	if req.CPULimit != 0 && (req.CPULimit < 0.1 || req.CPULimit > 4.0) {
		writeJSON(w, http.StatusBadRequest, model.ApiResponse[any]{
			Success: false,
			Message: "CPU limit must be between 0.1 and 4.0",
		})
		return
	}

	container, err := h.svc.Create(r.Context(), req)
	if err != nil {
		middleware.WriteError(w, http.StatusServiceUnavailable, model.ERR_INTERNAL, "创建容器失败", err)
		return
	}

	writeJSON(w, http.StatusCreated, model.ApiResponse[model.Container]{
		Success: true,
		Data:    *container,
	})
}

// HandleContainerStart handles POST /api/v1/admin/containers/{id}/start.
func (h *ContainerHandler) HandleContainerStart(w http.ResponseWriter, r *http.Request) {
	if !h.checkDocker(w) {
		return
	}
	id := r.PathValue("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, model.ApiResponse[any]{
			Success: false,
			Message: "container id is required",
		})
		return
	}

	if err := h.svc.Start(r.Context(), id); err != nil {
		middleware.WriteError(w, http.StatusServiceUnavailable, model.ERR_INTERNAL, "启动容器失败", err)
		return
	}

	writeJSON(w, http.StatusOK, model.ApiResponse[any]{
		Success: true,
		Message: fmt.Sprintf("container %s started", id),
	})
}

// HandleContainerStop handles POST /api/v1/admin/containers/{id}/stop.
func (h *ContainerHandler) HandleContainerStop(w http.ResponseWriter, r *http.Request) {
	if !h.checkDocker(w) {
		return
	}
	id := r.PathValue("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, model.ApiResponse[any]{
			Success: false,
			Message: "container id is required",
		})
		return
	}

	if err := h.svc.Stop(r.Context(), id, 10); err != nil {
		middleware.WriteError(w, http.StatusServiceUnavailable, model.ERR_INTERNAL, "停止容器失败", err)
		return
	}

	writeJSON(w, http.StatusOK, model.ApiResponse[any]{
		Success: true,
		Message: fmt.Sprintf("container %s stopped", id),
	})
}

// HandleContainerRestart handles POST /api/v1/admin/containers/{id}/restart.
func (h *ContainerHandler) HandleContainerRestart(w http.ResponseWriter, r *http.Request) {
	if !h.checkDocker(w) {
		return
	}
	id := r.PathValue("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, model.ApiResponse[any]{
			Success: false,
			Message: "container id is required",
		})
		return
	}

	if err := h.svc.Restart(r.Context(), id, 10); err != nil {
		middleware.WriteError(w, http.StatusServiceUnavailable, model.ERR_INTERNAL, "重启容器失败", err)
		return
	}

	writeJSON(w, http.StatusOK, model.ApiResponse[any]{
		Success: true,
		Message: fmt.Sprintf("container %s restarted", id),
	})
}

// HandleContainerDelete handles DELETE /api/v1/admin/containers/{id}.
func (h *ContainerHandler) HandleContainerDelete(w http.ResponseWriter, r *http.Request) {
	if !h.checkDocker(w) {
		return
	}
	id := r.PathValue("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, model.ApiResponse[any]{
			Success: false,
			Message: "container id is required",
		})
		return
	}

	if err := h.svc.Remove(r.Context(), id, false); err != nil {
		middleware.WriteError(w, http.StatusServiceUnavailable, model.ERR_INTERNAL, "删除容器失败", err)
		return
	}

	writeJSON(w, http.StatusOK, model.ApiResponse[any]{
		Success: true,
		Message: fmt.Sprintf("container %s deleted", id),
	})
}

// === Image Handlers ===

// HandleImageList handles GET /api/v1/admin/images.
func (h *ContainerHandler) HandleImageList(w http.ResponseWriter, r *http.Request) {
	if !h.checkDocker(w) {
		return
	}
	images, err := h.imgSvc.ImageList(r.Context())
	if err != nil {
		middleware.WriteError(w, http.StatusServiceUnavailable, model.ERR_INTERNAL, "镜像服务不可用", err)
		return
	}

	if images == nil {
		images = []model.Image{}
	}

	writeJSON(w, http.StatusOK, model.ApiResponse[[]model.Image]{
		Success: true,
		Data:    images,
	})
}

// HandleImagePull handles POST /api/v1/admin/images/pull.
func (h *ContainerHandler) HandleImagePull(w http.ResponseWriter, r *http.Request) {
	if !h.checkDocker(w) {
		return
	}
	var req struct {
		Image string `json:"image"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, model.ApiResponse[any]{
			Success: false,
			Message: "invalid request body",
		})
		return
	}

	if req.Image == "" {
		writeJSON(w, http.StatusBadRequest, model.ApiResponse[any]{
			Success: false,
			Message: "image is required",
		})
		return
	}

	if err := h.imgSvc.ImagePull(r.Context(), req.Image); err != nil {
		middleware.WriteError(w, http.StatusServiceUnavailable, model.ERR_NETWORK, "拉取镜像失败", err)
		return
	}

	writeJSON(w, http.StatusAccepted, model.ApiResponse[any]{
		Success: true,
		Message: fmt.Sprintf("image %s pulled", req.Image),
	})
}

// HandleImageDelete handles DELETE /api/v1/admin/images/{id}.
func (h *ContainerHandler) HandleImageDelete(w http.ResponseWriter, r *http.Request) {
	if !h.checkDocker(w) {
		return
	}
	id := r.PathValue("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, model.ApiResponse[any]{
			Success: false,
			Message: "image id is required",
		})
		return
	}

	if err := h.imgSvc.ImageDelete(r.Context(), id); err != nil {
		middleware.WriteError(w, http.StatusServiceUnavailable, model.ERR_INTERNAL, "删除镜像失败", err)
		return
	}

	writeJSON(w, http.StatusOK, model.ApiResponse[any]{
		Success: true,
		Message: fmt.Sprintf("image %s deleted", id),
	})
}

// === Container Stats & Logs Handlers ===

// HandleContainerStats handles GET /api/v1/admin/containers/{id}/stats.
func (h *ContainerHandler) HandleContainerStats(w http.ResponseWriter, r *http.Request) {
	if !h.checkDocker(w) {
		return
	}
	id := r.PathValue("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, model.ApiResponse[any]{
			Success: false,
			Message: "container id is required",
		})
		return
	}

	stats, err := h.logSvc.ContainerStats(r.Context(), id)
	if err != nil {
		middleware.WriteError(w, http.StatusServiceUnavailable, model.ERR_INTERNAL, "获取容器状态失败", err)
		return
	}

	writeJSON(w, http.StatusOK, model.ApiResponse[model.ContainerStats]{
		Success: true,
		Data:    *stats,
	})
}

// HandleContainerLogs handles GET /api/v1/admin/containers/{id}/logs.
// Query params: tail (default "100"), since (timestamp string).
// Returns a JSON array of log entries.
func (h *ContainerHandler) HandleContainerLogs(w http.ResponseWriter, r *http.Request) {
	if !h.checkDocker(w) {
		return
	}
	id := r.PathValue("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, model.ApiResponse[any]{
			Success: false,
			Message: "container id is required",
		})
		return
	}

	tail := r.URL.Query().Get("tail")
	if tail == "" {
		tail = "100"
	}
	since := r.URL.Query().Get("since")

	reader, err := h.logSvc.ContainerLogs(r.Context(), id, tail, since)
	if err != nil {
		middleware.WriteError(w, http.StatusServiceUnavailable, model.ERR_INTERNAL, "获取容器日志失败", err)
		return
	}
	defer reader.Close()

	entries, err := parseDockerLogStream(reader)
	if err != nil {
		middleware.WriteError(w, http.StatusInternalServerError, model.ERR_INTERNAL, "解析容器日志失败", err)
		return
	}

	writeJSON(w, http.StatusOK, model.ApiResponse[[]model.ContainerLogEntry]{
		Success: true,
		Data:    entries,
	})
}

// parseDockerLogStream reads a Docker multiplexed log stream and returns
// parsed log entries. Docker's stream format uses 8-byte frames:
//   - byte 0: stream type (1=stdout, 2=stderr)
//   - bytes 1-3: padding
//   - bytes 4-7: payload size (big-endian uint32)
//   - then N bytes of payload
func parseDockerLogStream(r io.Reader) ([]model.ContainerLogEntry, error) {
	var entries []model.ContainerLogEntry
	header := make([]byte, 8)

	for {
		_, err := io.ReadFull(r, header)
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			break
		}
		if err != nil {
			break
		}

		streamType := header[0]
		size := binary.BigEndian.Uint32(header[4:8])
		if size == 0 {
			continue
		}

		payload := make([]byte, size)
		if _, err := io.ReadFull(r, payload); err != nil {
			break
		}

		stream := "stdout"
		if streamType == 2 {
			stream = "stderr"
		}

		content := strings.TrimRight(string(payload), "\n\r")
		if content == "" {
			continue
		}

		for _, line := range strings.Split(content, "\n") {
			line = strings.TrimRight(line, "\r")
			if line != "" {
				entries = append(entries, model.ContainerLogEntry{
					Stream:  stream,
					Content: line,
				})
			}
		}
	}

	if entries == nil {
		entries = []model.ContainerLogEntry{}
	}
	return entries, nil
}

// readSystemMemoryBytes reads total system memory from /proc/meminfo.
// Returns 0 if the information cannot be read.
func readSystemMemoryBytes() (int64, error) {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0, fmt.Errorf("read /proc/meminfo: %w", err)
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "MemTotal:") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				kb, err := strconv.ParseInt(parts[1], 10, 64)
				if err != nil {
					return 0, fmt.Errorf("parse MemTotal: %w", err)
				}
				return kb * 1024, nil // MemTotal is in kB
			}
		}
	}
	return 0, fmt.Errorf("MemTotal not found in /proc/meminfo")
}

// parseMemLimit parses a human-readable memory limit string to bytes.
// Supports: "512m", "1g", "1024k", "2147483648" (plain bytes).
func parseMemLimit(s string) (int64, error) {
	if s == "" {
		return 0, fmt.Errorf("empty memory limit")
	}
	s = strings.ToLower(strings.TrimSpace(s))
	// Try plain number first (bytes).
	if v, err := strconv.ParseInt(s, 10, 64); err == nil {
		return v, nil
	}
	// Parse suffix.
	multipliers := map[string]int64{
		"k":  1024,
		"kb": 1024,
		"m":  1024 * 1024,
		"mb": 1024 * 1024,
		"g":  1024 * 1024 * 1024,
		"gb": 1024 * 1024 * 1024,
	}
	for suffix, mult := range multipliers {
		if strings.HasSuffix(s, suffix) {
			numStr := strings.TrimSuffix(s, suffix)
			v, err := strconv.ParseFloat(numStr, 64)
			if err != nil {
				return 0, fmt.Errorf("parse memory limit %q: %w", s, err)
			}
			return int64(v * float64(mult)), nil
		}
	}
	return 0, fmt.Errorf("unrecognized memory limit format: %q", s)
}
