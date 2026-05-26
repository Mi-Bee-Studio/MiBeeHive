// Package docker provides a thin wrapper around the Docker SDK client
// for container management operations within MiBeeHive.
package docker

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	dockerclient "github.com/docker/docker/client"
)

// Client wraps the Docker SDK client with convenience methods
// and graceful error handling when Docker is unavailable.
type Client struct {
	client *dockerclient.Client
	logger *slog.Logger
}

// NewClient creates a new Docker client wrapper.
// If socketPath is empty, it falls back to the DOCKER_HOST environment
// variable, and finally to the default "/var/run/docker.sock".
// Returns an error if the client cannot be created (e.g. socket missing),
// but never panics.
func NewClient(socketPath string) (*Client, error) {
	hostURL := resolveHostURL(socketPath)

	c, err := dockerclient.NewClientWithOpts(
		dockerclient.WithHost(hostURL),
		dockerclient.WithAPIVersionNegotiation(),
	)
	if err != nil {
		return nil, fmt.Errorf("docker client creation: %w", err)
	}

	logger := slog.Default().With("component", "docker")

	return &Client{
		client: c,
		logger: logger,
	}, nil
}

// IsAvailable pings the Docker daemon to check connectivity.
// Returns true if the daemon is reachable, false otherwise.
// Never panics — errors are logged and swallowed.
func (c *Client) IsAvailable() bool {
	if c.client == nil {
		return false
	}

	_, err := c.client.Ping(context.Background())
	if err != nil {
		c.logger.Debug("docker daemon unavailable", "error", err)
		return false
	}
	return true
}

// Close closes the underlying Docker client connection.
func (c *Client) Close() error {
	if c.client == nil {
		return nil
	}
	return c.client.Close()
}

// DockerClient returns the underlying Docker SDK client for use
// by the service layer when direct API access is needed.
func (c *Client) DockerClient() *dockerclient.Client {
	return c.client
}

// resolveHostURL determines the Docker host URL from the given socket path,
// the DOCKER_HOST environment variable, or the default unix socket.
func resolveHostURL(socketPath string) string {
	if socketPath != "" {
		return "unix://" + socketPath
	}

	if envHost := os.Getenv("DOCKER_HOST"); envHost != "" {
		return envHost
	}

	return "unix:///var/run/docker.sock"
}
