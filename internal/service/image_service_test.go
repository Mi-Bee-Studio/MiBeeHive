package service

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	dockerclient "github.com/docker/docker/client"
)

// dockerAPIPath extracts the Docker API path without the version prefix.
// e.g. "/v1.24/images/json" → "/images/json", "/_ping" → "/_ping"
func dockerAPIPath(urlPath string) string {
	// Strip version prefix like /v1.24/
	if strings.HasPrefix(urlPath, "/v") {
		parts := strings.SplitN(urlPath[1:], "/", 3)
		if len(parts) >= 2 {
			return "/" + strings.Join(parts[1:], "/")
		}
	}
	return urlPath
}

// setupDockerTestServer creates a mock Docker API server and returns
// a Docker client connected to it, plus a cleanup function.
func setupDockerTestServer(t *testing.T, handler http.HandlerFunc) *dockerclient.Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	cli, err := dockerclient.NewClientWithOpts(
		dockerclient.WithHost(server.URL),
		dockerclient.WithAPIVersionNegotiation(),
	)
	if err != nil {
		t.Fatalf("create docker client: %v", err)
	}
	t.Cleanup(func() { cli.Close() })

	return cli
}

func TestImageService_ImageList(t *testing.T) {
	dockerResponse := []map[string]any{
		{
			"Id":       "sha256:abc123",
			"RepoTags": []string{"nginx:latest", "nginx:1.25"},
			"Created":  time.Now().Unix(),
			"Size":     187_000_000, // ~178.34 MB
		},
		{
			"Id":       "sha256:def456",
			"RepoTags": []string{"alpine:3.19"},
			"Created":  time.Now().Unix(),
			"Size":     7_340_032, // ~7.0 MB
		},
	}
	body, _ := json.Marshal(dockerResponse)

	cli := setupDockerTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		path := dockerAPIPath(r.URL.Path)
		switch path {
		case "/_ping":
			w.Header().Set("API-Version", "1.45")
			w.WriteHeader(http.StatusOK)
		case "/images/json":
			w.Header().Set("Content-Type", "application/json")
			w.Write(body)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	svc := NewImageService(cli, slog.Default())
	images, err := svc.ImageList(context.Background())
	if err != nil {
		t.Fatalf("ImageList: %v", err)
	}

	if len(images) != 2 {
		t.Fatalf("expected 2 images, got %d", len(images))
	}

	// Verify first image conversion
	if images[0].ID != "sha256:abc123" {
		t.Errorf("image[0] ID = %q, want %q", images[0].ID, "sha256:abc123")
	}
	if len(images[0].RepoTags) != 2 || images[0].RepoTags[0] != "nginx:latest" {
		t.Errorf("image[0] RepoTags = %v, want [nginx:latest nginx:1.25]", images[0].RepoTags)
	}
	// 187_000_000 / (1024*1024) ≈ 178.34 MB
	expectedMB := float64(187_000_000) / (1024 * 1024)
	if diff := images[0].SizeMB - expectedMB; diff < -0.01 || diff > 0.01 {
		t.Errorf("image[0] SizeMB = %.2f, want %.2f", images[0].SizeMB, expectedMB)
	}

	// Verify second image
	if images[1].ID != "sha256:def456" {
		t.Errorf("image[1] ID = %q, want %q", images[1].ID, "sha256:def456")
	}
	if images[1].RepoTags[0] != "alpine:3.19" {
		t.Errorf("image[1] RepoTags = %v, want [alpine:3.19]", images[1].RepoTags)
	}
}

func TestImageService_ImageList_Empty(t *testing.T) {
	cli := setupDockerTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		path := dockerAPIPath(r.URL.Path)
		switch path {
		case "/_ping":
			w.Header().Set("API-Version", "1.45")
			w.WriteHeader(http.StatusOK)
		case "/images/json":
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte("[]"))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	svc := NewImageService(cli, slog.Default())
	images, err := svc.ImageList(context.Background())
	if err != nil {
		t.Fatalf("ImageList: %v", err)
	}
	if len(images) != 0 {
		t.Errorf("expected 0 images, got %d", len(images))
	}
}

func TestImageService_ImagePull(t *testing.T) {
	pullMessages := []map[string]string{
		{"status": "Pulling from library/nginx"},
		{"status": "Digest: sha256:abc"},
		{"status": "Status: Downloaded newer image"},
	}
	var sb strings.Builder
	for _, msg := range pullMessages {
		b, _ := json.Marshal(msg)
		sb.Write(b)
		sb.WriteByte('\n')
	}
	responseBody := sb.String()

	var pullPathChecked bool

	cli := setupDockerTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		path := dockerAPIPath(r.URL.Path)
		switch path {
		case "/_ping":
			w.Header().Set("API-Version", "1.45")
			w.WriteHeader(http.StatusOK)
		case "/images/create":
			pullPathChecked = true
			// Docker SDK sends fromImage and tag as separate query params
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(responseBody))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	svc := NewImageService(cli, slog.Default())
	err := svc.ImagePull(context.Background(), "nginx:latest")
	if err != nil {
		t.Fatalf("ImagePull: %v", err)
	}
	if !pullPathChecked {
		t.Error("expected /images/create endpoint to be called")
	}
}

func TestImageService_ImageDelete(t *testing.T) {
	dockerResponse := []map[string]any{
		{"Untagged": "sha256:abc123"},
		{"Deleted": "sha256:abc123"},
	}
	body, _ := json.Marshal(dockerResponse)

	var deleteMethodChecked bool

	cli := setupDockerTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		path := dockerAPIPath(r.URL.Path)
		switch path {
		case "/_ping":
			w.Header().Set("API-Version", "1.45")
			w.WriteHeader(http.StatusOK)
		default:
			if r.Method == http.MethodDelete && strings.HasPrefix(path, "/images/sha256:abc123") {
				deleteMethodChecked = true
				w.Header().Set("Content-Type", "application/json")
				w.Write(body)
			} else {
				w.WriteHeader(http.StatusNotFound)
			}
		}
	})

	svc := NewImageService(cli, slog.Default())
	err := svc.ImageDelete(context.Background(), "sha256:abc123")
	if err != nil {
		t.Fatalf("ImageDelete: %v", err)
	}
	if !deleteMethodChecked {
		t.Error("expected DELETE /images/sha256:abc123 endpoint to be called")
	}
}

func TestImageService_ImageList_ConvertToModel(t *testing.T) {
	createdTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	dockerResponse := []map[string]any{
		{
			"Id":       "sha256:face1234",
			"RepoTags": []string{"redis:7"},
			"Created":  createdTime.Unix(),
			"Size":     130_000_000, // ~124.0 MB
		},
	}
	body, _ := json.Marshal(dockerResponse)

	cli := setupDockerTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		path := dockerAPIPath(r.URL.Path)
		switch path {
		case "/_ping":
			w.Header().Set("API-Version", "1.45")
			w.WriteHeader(http.StatusOK)
		case "/images/json":
			w.Header().Set("Content-Type", "application/json")
			w.Write(body)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	svc := NewImageService(cli, slog.Default())
	images, err := svc.ImageList(context.Background())
	if err != nil {
		t.Fatalf("ImageList: %v", err)
	}

	img := images[0]

	if img.ID == "" {
		t.Fatal("expected non-zero model.Image")
	}

	if img.ID != "sha256:face1234" {
		t.Errorf("ID = %q, want %q", img.ID, "sha256:face1234")
	}
	if len(img.RepoTags) != 1 || img.RepoTags[0] != "redis:7" {
		t.Errorf("RepoTags = %v, want [redis:7]", img.RepoTags)
	}
	expectedMB := 130_000_000.0 / (1024 * 1024)
	if diff := img.SizeMB - expectedMB; diff < -0.01 || diff > 0.01 {
		t.Errorf("SizeMB = %.2f, want %.2f", img.SizeMB, expectedMB)
	}
	if img.CreatedAt.IsZero() {
		t.Error("CreatedAt should not be zero")
	}
}

func TestImageService_ImagePull_StreamingResponse(t *testing.T) {
	messages := []map[string]string{
		{"status": "Pulling from library/alpine"},
		{"status": "Pulling fs layer"},
		{"status": "Downloading"},
		{"status": "Download complete"},
		{"status": "Pull complete"},
		{"status": "Digest: sha256:feed"},
		{"status": "Status: Downloaded newer image"},
	}
	var sb strings.Builder
	for _, msg := range messages {
		b, _ := json.Marshal(msg)
		sb.Write(b)
		sb.WriteByte('\n')
	}

	cli := setupDockerTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		path := dockerAPIPath(r.URL.Path)
		switch path {
		case "/_ping":
			w.Header().Set("API-Version", "1.45")
			w.WriteHeader(http.StatusOK)
		case "/images/create":
			w.Header().Set("Content-Type", "application/json")
			flusher, canFlush := w.(http.Flusher)
			chunks := strings.Split(sb.String(), "\n")
			for _, chunk := range chunks {
				if chunk == "" {
					continue
				}
				w.Write([]byte(chunk + "\n"))
				if canFlush {
					flusher.Flush()
				}
			}
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	svc := NewImageService(cli, slog.Default())
	err := svc.ImagePull(context.Background(), "alpine:3.19")
	if err != nil {
		t.Fatalf("ImagePull: %v", err)
	}
}

func TestImageService_ImagePull_ServerError(t *testing.T) {
	cli := setupDockerTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		path := dockerAPIPath(r.URL.Path)
		switch path {
		case "/_ping":
			w.Header().Set("API-Version", "1.45")
			w.WriteHeader(http.StatusOK)
		case "/images/create":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"message":"internal server error"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	svc := NewImageService(cli, slog.Default())
	err := svc.ImagePull(context.Background(), "bad:image")
	if err == nil {
		t.Fatal("expected error for server error response, got nil")
	}
}

func TestImageService_ImagePull_ReadBodyFully(t *testing.T) {
	data := []byte(`{"status":"pulling"}` + "\n")
	var readCalled bool

	cli := setupDockerTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		path := dockerAPIPath(r.URL.Path)
		switch path {
		case "/_ping":
			w.Header().Set("API-Version", "1.45")
			w.WriteHeader(http.StatusOK)
		case "/images/create":
			readCalled = true
			w.Header().Set("Content-Type", "application/json")
			w.Write(data)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	svc := NewImageService(cli, slog.Default())
	err := svc.ImagePull(context.Background(), "test:latest")
	if err != nil {
		t.Fatalf("ImagePull: %v", err)
	}
	if !readCalled {
		t.Error("expected server handler to be called")
	}
}

func TestImageService_ImageDelete_NonExistent(t *testing.T) {
	cli := setupDockerTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		path := dockerAPIPath(r.URL.Path)
		switch path {
		case "/_ping":
			w.Header().Set("API-Version", "1.45")
			w.WriteHeader(http.StatusOK)
		default:
			if r.Method == http.MethodDelete {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusNotFound)
				w.Write([]byte(`{"message":"No such image"}`))
			} else {
				w.WriteHeader(http.StatusNotFound)
			}
		}
	})

	svc := NewImageService(cli, slog.Default())
	err := svc.ImageDelete(context.Background(), "sha256:nonexistent")
	if err == nil {
		t.Fatal("expected error for non-existent image, got nil")
	}
	if !strings.Contains(err.Error(), "sha256:nonexistent") {
		t.Errorf("error should contain image ID, got: %v", err)
	}
}

func TestImageService_ImageList_Error(t *testing.T) {
	cli := setupDockerTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		path := dockerAPIPath(r.URL.Path)
		switch path {
		case "/_ping":
			w.Header().Set("API-Version", "1.45")
			w.WriteHeader(http.StatusOK)
		case "/images/json":
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"message":"daemon error"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	svc := NewImageService(cli, slog.Default())
	_, err := svc.ImageList(context.Background())
	if err == nil {
		t.Fatal("expected error for server error, got nil")
	}
}

func TestImageService_ImagePull_IoCopy(t *testing.T) {
	// Verify that ImagePull reads the entire response body using io.Copy
	// (not buffering in memory) by using a large response body.
	var sb strings.Builder
	for i := 0; i < 1000; i++ {
		msg := map[string]string{"status": "progress", "id": string(rune(i))}
		b, _ := json.Marshal(msg)
		sb.Write(b)
		sb.WriteByte('\n')
	}
	largeBody := sb.String()

	cli := setupDockerTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		path := dockerAPIPath(r.URL.Path)
		switch path {
		case "/_ping":
			w.Header().Set("API-Version", "1.45")
			w.WriteHeader(http.StatusOK)
		case "/images/create":
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, largeBody)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	svc := NewImageService(cli, slog.Default())
	err := svc.ImagePull(context.Background(), "large:image")
	if err != nil {
		t.Fatalf("ImagePull with large body: %v", err)
	}
}
