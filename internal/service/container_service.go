// Package service provides business logic for container lifecycle management.
package service

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"github.com/docker/docker/api/types"
	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	networktypes "github.com/docker/docker/api/types/network"
	"github.com/docker/go-connections/nat"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/Mi-Bee-Studio/mibeehive/internal/model"
)

// dockerContainerClient defines the Docker SDK methods used by ContainerService.
// This interface allows testing with a stub instead of a real Docker daemon.
type dockerContainerClient interface {
	ContainerList(ctx context.Context, options containertypes.ListOptions) ([]types.Container, error)
	ContainerInspect(ctx context.Context, containerID string) (types.ContainerJSON, error)
	ContainerCreate(ctx context.Context, config *containertypes.Config, hostConfig *containertypes.HostConfig, networkingConfig *networktypes.NetworkingConfig, platform *ocispec.Platform, containerName string) (containertypes.CreateResponse, error)
	ContainerStart(ctx context.Context, containerID string, options containertypes.StartOptions) error
	ContainerStop(ctx context.Context, containerID string, options containertypes.StopOptions) error
	ContainerRestart(ctx context.Context, containerID string, options containertypes.StopOptions) error
	ContainerRemove(ctx context.Context, containerID string, options containertypes.RemoveOptions) error
}

// ContainerService manages Docker container lifecycle operations.
type ContainerService struct {
	cli    dockerContainerClient
	logger *slog.Logger
}

// NewContainerService creates a new ContainerService with the given Docker client.
func NewContainerService(cli dockerContainerClient, logger *slog.Logger) *ContainerService {
	return &ContainerService{
		cli:    cli,
		logger: logger.With("component", "container-service"),
	}
}

// List returns all containers from the Docker daemon (including stopped).
func (s *ContainerService) List(ctx context.Context) ([]model.Container, error) {
	containers, err := s.cli.ContainerList(ctx, containertypes.ListOptions{All: true})
	if err != nil {
		return nil, fmt.Errorf("container list: %w", err)
	}

	result := make([]model.Container, 0, len(containers))
	for _, c := range containers {
		result = append(result, dockerSummaryToModel(c))
	}
	return result, nil
}

// Get inspects a single container by ID and returns its details.
func (s *ContainerService) Get(ctx context.Context, id string) (*model.Container, error) {
	json, err := s.cli.ContainerInspect(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("container inspect %s: %w", id, err)
	}

	c := dockerInspectToModel(json)
	return &c, nil
}

// Create creates a new Docker container from the request parameters.
// The container is created but not started — call Start separately.
func (s *ContainerService) Create(ctx context.Context, req model.CreateContainerRequest) (*model.Container, error) {
	// Build container config.
	config := &containertypes.Config{
		Image: req.Image,
		Env:   envMapToSlice(req.Env),
	}
	if req.Command != "" {
		config.Cmd = []string{req.Command}
	}

	// Build exposed ports from request.
	exposedPorts, portBindings := buildPortConfig(req.Ports)
	if len(exposedPorts) > 0 {
		config.ExposedPorts = exposedPorts
	}

	// Build host config.
	hostConfig := &containertypes.HostConfig{
		RestartPolicy: containertypes.RestartPolicy{
			Name: containertypes.RestartPolicyMode(req.RestartPolicy),
		},
		PortBindings: portBindings,
		Binds:        buildBinds(req.Volumes),
	}

	// Set memory limit.
	if req.MemoryLimit != "" {
		if memBytes, err := parseMemoryLimit(req.MemoryLimit); err == nil {
			hostConfig.Memory = memBytes
		}
	}

	// Set CPU limit (NanoCPUs = CPU * 1e9).
	if req.CPULimit > 0 {
		hostConfig.NanoCPUs = int64(req.CPULimit * 1e9)
	}

	// Set volumes.
	if len(req.Volumes) > 0 {
		hostConfig.Mounts = buildMounts(req.Volumes)
	}

	resp, err := s.cli.ContainerCreate(ctx, config, hostConfig, nil, nil, req.Name)
	if err != nil {
		return nil, fmt.Errorf("container create %s: %w", req.Name, err)
	}

	s.logger.Info("container created", "id", resp.ID, "name", req.Name, "image", req.Image)

	return &model.Container{
		Name:        req.Name,
		Image:       req.Image,
		Command:     req.Command,
		Status:      "created",
		ContainerID: resp.ID,
		RestartPolicy: req.RestartPolicy,
		MemoryLimit: req.MemoryLimit,
		CPULimit:    req.CPULimit,
		Env:         req.Env,
		Ports:       req.Ports,
		Volumes:     req.Volumes,
	}, nil
}

// Start starts a container by ID.
func (s *ContainerService) Start(ctx context.Context, id string) error {
	if err := s.cli.ContainerStart(ctx, id, containertypes.StartOptions{}); err != nil {
		return fmt.Errorf("container start %s: %w", id, err)
	}
	s.logger.Info("container started", "id", id)
	return nil
}

// Stop stops a container by ID with the given timeout in seconds.
func (s *ContainerService) Stop(ctx context.Context, id string, timeout int) error {
	timeoutPtr := &[]int{timeout}[0]
	if err := s.cli.ContainerStop(ctx, id, containertypes.StopOptions{Timeout: timeoutPtr}); err != nil {
		return fmt.Errorf("container stop %s: %w", id, err)
	}
	s.logger.Info("container stopped", "id", id, "timeout", timeout)
	return nil
}

// Restart restarts a container by ID with the given timeout in seconds.
func (s *ContainerService) Restart(ctx context.Context, id string, timeout int) error {
	timeoutPtr := &[]int{timeout}[0]
	if err := s.cli.ContainerRestart(ctx, id, containertypes.StopOptions{Timeout: timeoutPtr}); err != nil {
		return fmt.Errorf("container restart %s: %w", id, err)
	}
	s.logger.Info("container restarted", "id", id)
	return nil
}

// Remove removes a container by ID. If force is true, it removes a running container.
func (s *ContainerService) Remove(ctx context.Context, id string, force bool) error {
	if err := s.cli.ContainerRemove(ctx, id, containertypes.RemoveOptions{Force: force}); err != nil {
		return fmt.Errorf("container remove %s: %w", id, err)
	}
	s.logger.Info("container removed", "id", id, "force", force)
	return nil
}

// ---------------------------------------------------------------------------
// Conversion helpers
// ---------------------------------------------------------------------------

// dockerSummaryToModel converts a Docker API Container summary to our model.Container.
func dockerSummaryToModel(c types.Container) model.Container {
	name := ""
	if len(c.Names) > 0 {
		name = strings.TrimPrefix(c.Names[0], "/")
	}

	return model.Container{
		Name:        name,
		Image:       c.Image,
		Command:     c.Command,
		Status:      c.State,
		ContainerID: c.ID,
	}
}

// dockerInspectToModel converts a Docker API ContainerJSON to our model.Container.
func dockerInspectToModel(cjson types.ContainerJSON) model.Container {
	var name, image, command, status, restartPolicy string
	var envMap map[string]string

	if cjson.ContainerJSONBase != nil {
		name = strings.TrimPrefix(cjson.ContainerJSONBase.Name, "/")
		if cjson.ContainerJSONBase.State != nil {
			status = cjson.ContainerJSONBase.State.Status
		}
	}

	if cjson.Config != nil {
		image = cjson.Config.Image
		if len(cjson.Config.Cmd) > 0 {
			command = strings.Join(cjson.Config.Cmd, " ")
		}
		envMap = envSliceToMap(cjson.Config.Env)
	}

	var memoryLimit string
	if cjson.HostConfig != nil {
		restartPolicy = string(cjson.HostConfig.RestartPolicy.Name)
		if cjson.HostConfig.Memory > 0 {
			memoryLimit = fmt.Sprintf("%d", cjson.HostConfig.Memory)
		}
	}

	containerID := ""
	if cjson.ContainerJSONBase != nil {
		containerID = cjson.ContainerJSONBase.ID
	}

	return model.Container{
		Name:          name,
		Image:         image,
		Command:       command,
		Env:           envMap,
		Status:        status,
		ContainerID:   containerID,
		RestartPolicy: restartPolicy,
		MemoryLimit:   memoryLimit,
	}
}

// envMapToSlice converts a map of env vars to KEY=VALUE slice.
func envMapToSlice(env map[string]string) []string {
	if len(env) == 0 {
		return nil
	}
	result := make([]string, 0, len(env))
	for k, v := range env {
		result = append(result, k+"="+v)
	}
	return result
}

// envSliceToMap converts a KEY=VALUE slice to a map.
func envSliceToMap(env []string) map[string]string {
	if len(env) == 0 {
		return nil
	}
	m := make(map[string]string, len(env))
	for _, e := range env {
		parts := strings.SplitN(e, "=", 2)
		if len(parts) == 2 {
			m[parts[0]] = parts[1]
		}
	}
	return m
}

// buildPortConfig builds exposed ports and port bindings from model port mappings.
func buildPortConfig(ports []model.PortMapping) (nat.PortSet, nat.PortMap) {
	if len(ports) == 0 {
		return nil, nil
	}

	exposed := make(nat.PortSet)
	bindings := make(nat.PortMap)

	for _, p := range ports {
		proto := p.Protocol
		if proto == "" {
			proto = "tcp"
		}
		port := nat.Port(fmt.Sprintf("%d/%s", p.ContainerPort, proto))
		exposed[port] = struct{}{}
		bindings[port] = []nat.PortBinding{
			{HostIP: "0.0.0.0", HostPort: fmt.Sprintf("%d", p.HostPort)},
		}
	}
	return exposed, bindings
}

// buildBinds converts volume mounts to Docker bind strings (host:container:mode).
func buildBinds(volumes []model.VolumeMount) []string {
	if len(volumes) == 0 {
		return nil
	}
	binds := make([]string, 0, len(volumes))
	for _, v := range volumes {
		mode := v.Mode
		if mode == "" {
			mode = "rw"
		}
		binds = append(binds, fmt.Sprintf("%s:%s:%s", v.HostPath, v.ContainerPath, mode))
	}
	return binds
}

// buildMounts converts volume mounts to Docker mount structs.
func buildMounts(volumes []model.VolumeMount) []mount.Mount {
	if len(volumes) == 0 {
		return nil
	}
	mounts := make([]mount.Mount, 0, len(volumes))
	for _, v := range volumes {
		mounts = append(mounts, mount.Mount{
			Type:   mount.TypeBind,
			Source: v.HostPath,
			Target: v.ContainerPath,
		})
	}
	return mounts
}

// parseMemoryLimit parses a human-readable memory limit string to bytes.
// Supports: "512m", "1g", "1024k", "2147483648" (plain bytes).
func parseMemoryLimit(s string) (int64, error) {
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
