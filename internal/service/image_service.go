package service

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"time"

	dockerclient "github.com/docker/docker/client"
	"github.com/docker/docker/api/types/image"
	"github.com/Mi-Bee-Studio/mibeehive/internal/model"
)

// ImageService manages Docker images through the Docker daemon.
type ImageService struct {
	cli    *dockerclient.Client
	logger *slog.Logger
}

// NewImageService creates a new image service with the given Docker client.
func NewImageService(dockerClient *dockerclient.Client, logger *slog.Logger) *ImageService {
	return &ImageService{
		cli:    dockerClient,
		logger: logger.With("component", "image-service"),
	}
}

// ImageList returns all images from the Docker daemon, converting Docker
// image summaries to model.Image values with sizes expressed in MB.
func (s *ImageService) ImageList(ctx context.Context) ([]model.Image, error) {
	summaries, err := s.cli.ImageList(ctx, image.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("image list: %w", err)
	}

	images := make([]model.Image, len(summaries))
	for i, sum := range summaries {
		images[i] = model.Image{
			ID:        sum.ID,
			RepoTags:  sum.RepoTags,
			SizeMB:    float64(sum.Size) / (1024 * 1024),
			CreatedAt: time.Unix(sum.Created, 0),
		}
	}
	return images, nil
}

// ImagePull pulls an image from a registry by name.
// It reads and closes the streaming response body from the Docker daemon.
func (s *ImageService) ImagePull(ctx context.Context, imageName string) error {
	reader, err := s.cli.ImagePull(ctx, imageName, image.PullOptions{})
	if err != nil {
		return fmt.Errorf("image pull %s: %w", imageName, err)
	}
	defer reader.Close()

	// Fully consume the response body — Docker sends streamed JSON status lines.
	// Discard the content but ensure we read to EOF so the pull completes.
	_, err = io.Copy(io.Discard, reader)
	if err != nil {
		return fmt.Errorf("image pull %s: reading response: %w", imageName, err)
	}

	s.logger.Info("image pulled", "image", imageName)
	return nil
}

// ImageDelete removes an image from the Docker daemon by ID.
func (s *ImageService) ImageDelete(ctx context.Context, imageID string) error {
	_, err := s.cli.ImageRemove(ctx, imageID, image.RemoveOptions{})
	if err != nil {
		return fmt.Errorf("image delete %s: %w", imageID, err)
	}

	s.logger.Info("image deleted", "image_id", imageID)
	return nil
}
