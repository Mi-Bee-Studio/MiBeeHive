package service

import (
	"context"
	"fmt"
	"log/slog"
	"testing"

	"github.com/docker/docker/api/types"
	containertypes "github.com/docker/docker/api/types/container"
	networktypes "github.com/docker/docker/api/types/network"
	"github.com/docker/go-connections/nat"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/Mi-Bee-Studio/mibeehive/internal/model"
)

// ---------------------------------------------------------------------------
// Docker client stub — satisfies the dockerContainerClient interface used by
// ContainerService. Only the lifecycle methods are implemented.
// ---------------------------------------------------------------------------

type stubDockerClient struct {
	// container data store: id -> stubContainer
	containers map[string]*stubContainer
	// nextID is auto-incremented on Create.
	nextID int
	// err can be set to force every call to return this error.
	err error
}

type stubContainer struct {
	id      string
	name    string
	image   string
	command string
	env     []string
	status  string // "running", "created", "exited", "paused"
	restart string
	memory  int64
	nanoCPU int64
	binds   []string
}

func newStubClient() *stubDockerClient {
	return &stubDockerClient{
		containers: make(map[string]*stubContainer),
		nextID:     1,
	}
}

func (s *stubDockerClient) addContainer(c *stubContainer) {
	s.containers[c.id] = c
}

// --- dockerContainerClient implementation ---

func (s *stubDockerClient) ContainerList(ctx context.Context, options containertypes.ListOptions) ([]types.Container, error) {
	if s.err != nil {
		return nil, s.err
	}
	var result []types.Container
	for _, c := range s.containers {
		if !options.All && c.status != "running" {
			continue
		}
		result = append(result, types.Container{
			ID:      c.id,
			Names:   []string{"/" + c.name},
			Image:   c.image,
			Command: c.command,
			State:   c.status,
			Status:  c.status,
		})
	}
	return result, nil
}

func (s *stubDockerClient) ContainerInspect(ctx context.Context, id string) (types.ContainerJSON, error) {
	if s.err != nil {
		return types.ContainerJSON{}, s.err
	}
	c, ok := s.containers[id]
	if !ok {
		return types.ContainerJSON{}, fmt.Errorf("no such container %s", id)
	}
	state := &types.ContainerState{
		Status:  c.status,
		Running: c.status == "running",
		Paused:  c.status == "paused",
	}
	return types.ContainerJSON{
		ContainerJSONBase: &types.ContainerJSONBase{
			ID:         c.id,
			Name:       "/" + c.name,
			State:      state,
			HostConfig: &containertypes.HostConfig{
				RestartPolicy: containertypes.RestartPolicy{Name: containertypes.RestartPolicyMode(c.restart)},
				Binds:         c.binds,
				Resources:     containertypes.Resources{Memory: c.memory, NanoCPUs: c.nanoCPU},
			},
		},
		Config: &containertypes.Config{
			Image: c.image,
			Cmd:   []string{c.command},
			Env:   c.env,
		},
	}, nil
}

func (s *stubDockerClient) ContainerCreate(ctx context.Context, config *containertypes.Config, hostConfig *containertypes.HostConfig, networkingConfig *networktypes.NetworkingConfig, platform *ocispec.Platform, name string) (containertypes.CreateResponse, error) {
	if s.err != nil {
		return containertypes.CreateResponse{}, s.err
	}
	id := fmt.Sprintf("container_%d", s.nextID)
	s.nextID++

	cmd := ""
	if len(config.Cmd) > 0 {
		cmd = config.Cmd[0]
	}

	s.containers[id] = &stubContainer{
		id:      id,
		name:    name,
		image:   config.Image,
		command: cmd,
		env:     config.Env,
		status:  "created",
		restart: string(hostConfig.RestartPolicy.Name),
		memory:  hostConfig.Memory,
		nanoCPU: hostConfig.NanoCPUs,
		binds:   hostConfig.Binds,
	}

	return containertypes.CreateResponse{ID: id}, nil
}

func (s *stubDockerClient) ContainerStart(ctx context.Context, id string, options containertypes.StartOptions) error {
	if s.err != nil {
		return s.err
	}
	c, ok := s.containers[id]
	if !ok {
		return fmt.Errorf("no such container %s", id)
	}
	c.status = "running"
	return nil
}

func (s *stubDockerClient) ContainerStop(ctx context.Context, id string, options containertypes.StopOptions) error {
	if s.err != nil {
		return s.err
	}
	c, ok := s.containers[id]
	if !ok {
		return fmt.Errorf("no such container %s", id)
	}
	c.status = "exited"
	return nil
}

func (s *stubDockerClient) ContainerRestart(ctx context.Context, id string, options containertypes.StopOptions) error {
	if s.err != nil {
		return s.err
	}
	c, ok := s.containers[id]
	if !ok {
		return fmt.Errorf("no such container %s", id)
	}
	c.status = "running"
	return nil
}

func (s *stubDockerClient) ContainerRemove(ctx context.Context, id string, options containertypes.RemoveOptions) error {
	if s.err != nil {
		return s.err
	}
	if _, ok := s.containers[id]; !ok {
		return fmt.Errorf("no such container %s", id)
	}
	delete(s.containers, id)
	return nil
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func newTestContainerService(stub *stubDockerClient) *ContainerService {
	return &ContainerService{
		cli:    stub,
		logger: slog.Default().With("component", "container-service-test"),
	}
}

func TestContainerService_List(t *testing.T) {
	stub := newStubClient()
	stub.addContainer(&stubContainer{id: "abc123", name: "web", image: "nginx:latest", command: "nginx", status: "running"})
	stub.addContainer(&stubContainer{id: "def456", name: "db", image: "postgres:15", command: "postgres", status: "exited"})

	svc := newTestContainerService(stub)

	containers, err := svc.List(context.Background())
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(containers) != 2 {
		t.Fatalf("expected 2 containers, got %d", len(containers))
	}

	var webFound bool
	for _, c := range containers {
		if c.Name == "web" {
			webFound = true
			if c.Image != "nginx:latest" {
				t.Errorf("expected image nginx:latest, got %s", c.Image)
			}
			if c.Status != "running" {
				t.Errorf("expected status running, got %s", c.Status)
			}
			if c.ContainerID != "abc123" {
				t.Errorf("expected container_id abc123, got %s", c.ContainerID)
			}
		}
	}
	if !webFound {
		t.Error("web container not found in list")
	}
}

func TestContainerService_ListEmpty(t *testing.T) {
	stub := newStubClient()
	svc := newTestContainerService(stub)

	containers, err := svc.List(context.Background())
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(containers) != 0 {
		t.Fatalf("expected 0 containers, got %d", len(containers))
	}
}

func TestContainerService_Get(t *testing.T) {
	stub := newStubClient()
	stub.addContainer(&stubContainer{
		id: "abc123", name: "web", image: "nginx:latest",
		command: "nginx", status: "running", restart: "always",
		memory: 512 * 1024 * 1024, // 512MB
	})

	svc := newTestContainerService(stub)

	container, err := svc.Get(context.Background(), "abc123")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if container.Name != "web" {
		t.Errorf("expected name web, got %s", container.Name)
	}
	if container.Image != "nginx:latest" {
		t.Errorf("expected image nginx:latest, got %s", container.Image)
	}
	if container.Status != "running" {
		t.Errorf("expected status running, got %s", container.Status)
	}
	if container.ContainerID != "abc123" {
		t.Errorf("expected container_id abc123, got %s", container.ContainerID)
	}
	if container.RestartPolicy != "always" {
		t.Errorf("expected restart_policy always, got %s", container.RestartPolicy)
	}
}

func TestContainerService_Get_NotFound(t *testing.T) {
	stub := newStubClient()
	svc := newTestContainerService(stub)

	_, err := svc.Get(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent container, got nil")
	}
}

func TestContainerService_Create(t *testing.T) {
	stub := newStubClient()
	svc := newTestContainerService(stub)

	req := model.CreateContainerRequest{
		Name:    "redis",
		Image:   "redis:7",
		Command: "redis-server",
		Env:     map[string]string{"REDIS_PASSWORD": "secret"},
		Ports: []model.PortMapping{
			{HostPort: 6379, ContainerPort: 6379, Protocol: "tcp"},
		},
		Volumes: []model.VolumeMount{
			{HostPath: "/data/redis", ContainerPath: "/data", Mode: "rw"},
		},
		RestartPolicy: "always",
		MemoryLimit:   "512m",
		CPULimit:      1.5,
	}

	container, err := svc.Create(context.Background(), req)
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if container.Name != "redis" {
		t.Errorf("expected name redis, got %s", container.Name)
	}
	if container.Image != "redis:7" {
		t.Errorf("expected image redis:7, got %s", container.Image)
	}
	if container.ContainerID == "" {
		t.Error("expected non-empty container_id")
	}
	if container.Status != "created" {
		t.Errorf("expected status created, got %s", container.Status)
	}
	if container.RestartPolicy != "always" {
		t.Errorf("expected restart_policy always, got %s", container.RestartPolicy)
	}
}

func TestContainerService_Create_Minimal(t *testing.T) {
	stub := newStubClient()
	svc := newTestContainerService(stub)

	req := model.CreateContainerRequest{
		Name:  "alpine",
		Image: "alpine:latest",
	}

	container, err := svc.Create(context.Background(), req)
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if container.Name != "alpine" {
		t.Errorf("expected name alpine, got %s", container.Name)
	}
	if container.Image != "alpine:latest" {
		t.Errorf("expected image alpine:latest, got %s", container.Image)
	}
}

func TestContainerService_Start(t *testing.T) {
	stub := newStubClient()
	stub.addContainer(&stubContainer{id: "abc123", name: "web", image: "nginx:latest", status: "created"})
	svc := newTestContainerService(stub)

	err := svc.Start(context.Background(), "abc123")
	if err != nil {
		t.Fatalf("Start returned error: %v", err)
	}

	if stub.containers["abc123"].status != "running" {
		t.Errorf("expected status running, got %s", stub.containers["abc123"].status)
	}
}

func TestContainerService_Start_NotFound(t *testing.T) {
	stub := newStubClient()
	svc := newTestContainerService(stub)

	err := svc.Start(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent container, got nil")
	}
}

func TestContainerService_Stop(t *testing.T) {
	stub := newStubClient()
	stub.addContainer(&stubContainer{id: "abc123", name: "web", image: "nginx:latest", status: "running"})
	svc := newTestContainerService(stub)

	err := svc.Stop(context.Background(), "abc123", 10)
	if err != nil {
		t.Fatalf("Stop returned error: %v", err)
	}

	if stub.containers["abc123"].status != "exited" {
		t.Errorf("expected status exited, got %s", stub.containers["abc123"].status)
	}
}

func TestContainerService_Stop_NotFound(t *testing.T) {
	stub := newStubClient()
	svc := newTestContainerService(stub)

	err := svc.Stop(context.Background(), "nonexistent", 10)
	if err == nil {
		t.Fatal("expected error for nonexistent container, got nil")
	}
}

func TestContainerService_Restart(t *testing.T) {
	stub := newStubClient()
	stub.addContainer(&stubContainer{id: "abc123", name: "web", image: "nginx:latest", status: "running"})
	svc := newTestContainerService(stub)

	err := svc.Restart(context.Background(), "abc123", 10)
	if err != nil {
		t.Fatalf("Restart returned error: %v", err)
	}

	if stub.containers["abc123"].status != "running" {
		t.Errorf("expected status running, got %s", stub.containers["abc123"].status)
	}
}

func TestContainerService_Restart_NotFound(t *testing.T) {
	stub := newStubClient()
	svc := newTestContainerService(stub)

	err := svc.Restart(context.Background(), "nonexistent", 10)
	if err == nil {
		t.Fatal("expected error for nonexistent container, got nil")
	}
}

func TestContainerService_Remove(t *testing.T) {
	stub := newStubClient()
	stub.addContainer(&stubContainer{id: "abc123", name: "web", image: "nginx:latest", status: "exited"})
	svc := newTestContainerService(stub)

	err := svc.Remove(context.Background(), "abc123", false)
	if err != nil {
		t.Fatalf("Remove returned error: %v", err)
	}

	if _, ok := stub.containers["abc123"]; ok {
		t.Error("container should have been removed")
	}
}

func TestContainerService_Remove_Force(t *testing.T) {
	stub := newStubClient()
	stub.addContainer(&stubContainer{id: "abc123", name: "web", image: "nginx:latest", status: "running"})
	svc := newTestContainerService(stub)

	err := svc.Remove(context.Background(), "abc123", true)
	if err != nil {
		t.Fatalf("Remove (force) returned error: %v", err)
	}

	if _, ok := stub.containers["abc123"]; ok {
		t.Error("container should have been removed")
	}
}

func TestContainerService_Remove_NotFound(t *testing.T) {
	stub := newStubClient()
	svc := newTestContainerService(stub)

	err := svc.Remove(context.Background(), "nonexistent", false)
	if err == nil {
		t.Fatal("expected error for nonexistent container, got nil")
	}
}

func TestContainerService_DockerUnavailable(t *testing.T) {
	stub := newStubClient()
	stub.err = fmt.Errorf("docker daemon not running")
	svc := newTestContainerService(stub)

	_, err := svc.List(context.Background())
	if err == nil {
		t.Fatal("expected error when Docker unavailable, got nil")
	}

	err = svc.Start(context.Background(), "abc123")
	if err == nil {
		t.Fatal("expected error when Docker unavailable, got nil")
	}

	err = svc.Stop(context.Background(), "abc123", 10)
	if err == nil {
		t.Fatal("expected error when Docker unavailable, got nil")
	}
}

func TestContainerService_Create_WithEnvAndPorts(t *testing.T) {
	stub := newStubClient()
	svc := newTestContainerService(stub)

	req := model.CreateContainerRequest{
		Name:  "postgres",
		Image: "postgres:15",
		Env:   map[string]string{"POSTGRES_PASSWORD": "secret", "POSTGRES_DB": "mydb"},
		Ports: []model.PortMapping{
			{HostPort: 5432, ContainerPort: 5432, Protocol: "tcp"},
			{HostPort: 9100, ContainerPort: 9100, Protocol: "tcp"},
		},
		Volumes: []model.VolumeMount{
			{HostPath: "/data/pg", ContainerPath: "/var/lib/postgresql/data", Mode: "rw"},
		},
		RestartPolicy: "unless-stopped",
	}

	container, err := svc.Create(context.Background(), req)
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if container.ContainerID == "" {
		t.Error("expected non-empty container_id")
	}

	// Verify the stub stored the container correctly
	stored := stub.containers[container.ContainerID]
	if stored == nil {
		t.Fatal("container not found in stub")
	}
	if stored.name != "postgres" {
		t.Errorf("expected name postgres, got %s", stored.name)
	}
	if stored.restart != "unless-stopped" {
		t.Errorf("expected restart unless-stopped, got %s", stored.restart)
	}
}

func TestContainerService_Stop_WithZeroTimeout(t *testing.T) {
	stub := newStubClient()
	stub.addContainer(&stubContainer{id: "abc123", name: "web", image: "nginx:latest", status: "running"})
	svc := newTestContainerService(stub)

	err := svc.Stop(context.Background(), "abc123", 0)
	if err != nil {
		t.Fatalf("Stop(0) returned error: %v", err)
	}
}

func TestParseMemoryLimit(t *testing.T) {
	tests := []struct {
		input    string
		expected int64
		wantErr  bool
	}{
		{"512m", 512 * 1024 * 1024, false},
		{"1g", 1024 * 1024 * 1024, false},
		{"256M", 256 * 1024 * 1024, false},
		{"2G", 2 * 1024 * 1024 * 1024, false},
		{"1024k", 1024 * 1024, false},
		{"2147483648", 2147483648, false},
		{"", 0, true},
		{"abc", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := parseMemoryLimit(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("parseMemoryLimit(%q) expected error, got nil", tt.input)
				}
			} else {
				if err != nil {
					t.Errorf("parseMemoryLimit(%q) unexpected error: %v", tt.input, err)
				}
				if got != tt.expected {
					t.Errorf("parseMemoryLimit(%q) = %d, want %d", tt.input, got, tt.expected)
				}
			}
		})
	}
}

func TestBuildPortConfig(t *testing.T) {
	ports := []model.PortMapping{
		{HostPort: 8080, ContainerPort: 80, Protocol: "tcp"},
		{HostPort: 8443, ContainerPort: 443, Protocol: "tcp"},
	}

	exposed, bindings := buildPortConfig(ports)
	if len(exposed) != 2 {
		t.Fatalf("expected 2 exposed ports, got %d", len(exposed))
	}
	if len(bindings) != 2 {
		t.Fatalf("expected 2 port bindings, got %d", len(bindings))
	}

	// Check that port 80/tcp is in exposed
	port80 := nat.Port("80/tcp")
	if _, ok := exposed[port80]; !ok {
		t.Error("port 80/tcp not in exposed ports")
	}

	// Check binding for port 80
	if bb, ok := bindings[port80]; ok {
		if len(bb) != 1 || bb[0].HostPort != "8080" {
			t.Errorf("unexpected binding for port 80: %+v", bb)
		}
	} else {
		t.Error("no binding for port 80/tcp")
	}
}

func TestBuildBinds(t *testing.T) {
	volumes := []model.VolumeMount{
		{HostPath: "/data/app", ContainerPath: "/app", Mode: "rw"},
		{HostPath: "/data/config", ContainerPath: "/config", Mode: "ro"},
	}

	binds := buildBinds(volumes)
	if len(binds) != 2 {
		t.Fatalf("expected 2 binds, got %d", len(binds))
	}
	if binds[0] != "/data/app:/app:rw" {
		t.Errorf("unexpected bind[0]: %s", binds[0])
	}
	if binds[1] != "/data/config:/config:ro" {
		t.Errorf("unexpected bind[1]: %s", binds[1])
	}
}

func TestEnvMapToSlice(t *testing.T) {
	env := map[string]string{"FOO": "bar", "BAZ": "qux"}
	slice := envMapToSlice(env)
	if len(slice) != 2 {
		t.Fatalf("expected 2 env vars, got %d", len(slice))
	}
	// Check all values are present (order not guaranteed)
	found := map[string]bool{}
	for _, e := range slice {
		found[e] = true
	}
	if !found["FOO=bar"] || !found["BAZ=qux"] {
		t.Errorf("expected FOO=bar and BAZ=qux, got %v", slice)
	}
}

func TestEnvSliceToMap(t *testing.T) {
	slice := []string{"FOO=bar", "BAZ=qux=with=equals"}
	m := envSliceToMap(slice)
	if len(m) != 2 {
		t.Fatalf("expected 2 env vars, got %d", len(m))
	}
	if m["FOO"] != "bar" {
		t.Errorf("expected FOO=bar, got FOO=%s", m["FOO"])
	}
	if m["BAZ"] != "qux=with=equals" {
		t.Errorf("expected BAZ=qux=with=equals, got BAZ=%s", m["BAZ"])
	}
}
