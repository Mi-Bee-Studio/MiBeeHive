package docker

import (
	"fmt"
	"os"
	"runtime"
	"testing"
)

// TestMain skips the suite on Windows: every test here exercises unix-socket
// host URLs, which the Docker SDK client rejects on Windows ("protocol not
// available"). The production target is Linux, where these all run.
func TestMain(m *testing.M) {
	if runtime.GOOS == "windows" {
		fmt.Fprintln(os.Stderr, "skipping docker client tests: unix-socket hosts are Linux-only")
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func TestNewClient(t *testing.T) {
	t.Run("empty_socket_path_uses_default", func(t *testing.T) {
		t.Setenv("DOCKER_HOST", "")

		c, err := NewClient("")
		if err != nil {
			t.Fatalf("NewClient with empty path should not error: %v", err)
		}
		defer c.Close()

		host := c.DockerClient().DaemonHost()
		if host != "unix:///var/run/docker.sock" {
			t.Errorf("expected default host unix:///var/run/docker.sock, got %s", host)
		}
	})

	t.Run("custom_socket_path", func(t *testing.T) {
		t.Parallel()

		c, err := NewClient("/custom/path/docker.sock")
		if err != nil {
			t.Fatalf("NewClient with custom path should not error: %v", err)
		}
		defer c.Close()

		host := c.DockerClient().DaemonHost()
		if host != "unix:///custom/path/docker.sock" {
			t.Errorf("expected custom host, got %s", host)
		}
	})

	t.Run("docker_host_env_fallback", func(t *testing.T) {
		t.Setenv("DOCKER_HOST", "tcp://192.168.1.100:2376")

		c, err := NewClient("")
		if err != nil {
			t.Fatalf("NewClient should use DOCKER_HOST env: %v", err)
		}
		defer c.Close()

		host := c.DockerClient().DaemonHost()
		if host != "tcp://192.168.1.100:2376" {
			t.Errorf("expected DOCKER_HOST value, got %s", host)
		}
	})
}

func TestClient_IsAvailable_NoSocket(t *testing.T) {
	t.Parallel()

	// Point to a non-existent socket to simulate Docker unavailable
	c, err := NewClient("/nonexistent/docker.sock")
	if err != nil {
		t.Fatalf("NewClient should not error on creation: %v", err)
	}
	defer c.Close()

	// Ping should return false, not panic
	available := c.IsAvailable()
	if available {
		t.Error("expected IsAvailable=false when socket does not exist")
	}
}

func TestClient_Close(t *testing.T) {
	t.Parallel()

	t.Run("close_works", func(t *testing.T) {
		t.Parallel()

		c, err := NewClient("")
		if err != nil {
			t.Fatalf("NewClient should not error: %v", err)
		}

		if err := c.Close(); err != nil {
			t.Errorf("Close should not error: %v", err)
		}
	})

	t.Run("close_idempotent", func(t *testing.T) {
		t.Parallel()

		c, err := NewClient("")
		if err != nil {
			t.Fatalf("NewClient should not error: %v", err)
		}

		_ = c.Close()
		_ = c.Close() // Second close should not panic
	})
}

func TestClient_DockerClient(t *testing.T) {
	t.Parallel()

	c, err := NewClient("")
	if err != nil {
		t.Fatalf("NewClient should not error: %v", err)
	}
	defer c.Close()

	dc := c.DockerClient()
	if dc == nil {
		t.Error("DockerClient() should not return nil after successful creation")
	}
}

func TestResolveHostURL(t *testing.T) {
	t.Run("explicit_path", func(t *testing.T) {
		t.Parallel()
		got := resolveHostURL("/tmp/docker.sock")
		want := "unix:///tmp/docker.sock"
		if got != want {
			t.Errorf("resolveHostURL(%q) = %q, want %q", "/tmp/docker.sock", got, want)
		}
	})

	t.Run("empty_uses_env", func(t *testing.T) {
		t.Setenv("DOCKER_HOST", "tcp://localhost:2375")
		got := resolveHostURL("")
		if got != "tcp://localhost:2375" {
			t.Errorf("expected DOCKER_HOST value, got %q", got)
		}
	})

	t.Run("empty_no_env_uses_default", func(t *testing.T) {
		t.Setenv("DOCKER_HOST", "")
		got := resolveHostURL("")
		if got != "unix:///var/run/docker.sock" {
			t.Errorf("expected default socket, got %q", got)
		}
	})
}
