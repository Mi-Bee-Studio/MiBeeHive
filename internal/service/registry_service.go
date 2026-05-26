package service

import (
	"context"
	"fmt"

	"github.com/Mi-Bee-Studio/mibeehive/internal/db"
	"github.com/Mi-Bee-Studio/mibeehive/internal/model"
	"github.com/Mi-Bee-Studio/mibeehive/internal/registry"
)

// RegistryService manages container registry connections and operations.
type RegistryService struct {
	repo           *db.RegistryRepo
}

// NewRegistryService creates a new RegistryService.
func NewRegistryService(repo *db.RegistryRepo) *RegistryService {
	return &RegistryService{repo: repo}
}

// ListRegistries returns all registered container registries.
func (s *RegistryService) ListRegistries(ctx context.Context) ([]model.Registry, error) {
	return s.repo.List(ctx)
}

// GetRegistry retrieves a registry by its ID.
func (s *RegistryService) GetRegistry(ctx context.Context, id int64) (*model.Registry, error) {
	return s.repo.GetByID(ctx, id)
}

// CreateRegistry creates a new container registry from a request.
func (s *RegistryService) CreateRegistry(ctx context.Context, req model.CreateRegistryRequest) (*model.Registry, error) {
	reg := &model.Registry{
		Name:             req.Name,
		URL:              req.URL,
		Type:             req.Type,
		Username:         req.Username,
		EncryptedPassword: req.Password,
		Enabled:          true,
	}
	id, err := s.repo.Create(ctx, reg)
	if err != nil {
		return nil, fmt.Errorf("creating registry: %w", err)
	}
	reg.ID = id
	return reg, nil
}

// UpdateRegistry modifies an existing registry connection.
func (s *RegistryService) UpdateRegistry(ctx context.Context, id int64, req model.UpdateRegistryRequest) (*model.Registry, error) {
	existing, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("getting registry %d: %w", id, err)
	}
	if existing == nil {
		return nil, fmt.Errorf("registry %d not found", id)
	}
	existing.Name = req.Name
	existing.URL = req.URL
	existing.Type = req.Type
	existing.Username = req.Username
	if req.Password != "" {
		existing.EncryptedPassword = req.Password
	}
	if err := s.repo.Update(ctx, existing); err != nil {
		return nil, fmt.Errorf("updating registry %d: %w", id, err)
	}
	return existing, nil
}

// DeleteRegistry removes a registry by ID.
func (s *RegistryService) DeleteRegistry(ctx context.Context, id int64) error {
	return s.repo.Delete(ctx, id)
}

// TestConnection verifies connectivity to a registry by creating a client and pinging it.
func (s *RegistryService) TestConnection(ctx context.Context, id int64) (*model.TestConnectionResponse, error) {
	reg, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("getting registry %d: %w", id, err)
	}
	if reg == nil {
		return nil, fmt.Errorf("registry %d not found", id)
	}
	password, err := s.repo.DecryptPassword(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("decrypting password: %w", err)
	}
	var creds *registry.Credentials
	if reg.Username != "" {
		creds = &registry.Credentials{Username: reg.Username, Password: password}
	}
	client, err := registry.NewClient(reg.URL, creds)
	if err != nil {
		return &model.TestConnectionResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("creating registry client: %v", err),
		}, nil
	}
	if err := client.Ping(ctx); err != nil {
		return &model.TestConnectionResponse{
			Success:      false,
			ErrorMessage: err.Error(),
		}, nil
	}
	return &model.TestConnectionResponse{
		Success: true,
		Version: "unknown",
	}, nil
}

// BrowseCatalog lists repositories in a registry with optional pagination.
func (s *RegistryService) BrowseCatalog(ctx context.Context, id int64, n int, last string) ([]string, error) {
	client, err := s.getClient(ctx, id)
	if err != nil {
		return nil, err
	}
	return client.Catalog(ctx, n, last)
}

// BrowseTags lists tags for a repository with optional pagination.
func (s *RegistryService) BrowseTags(ctx context.Context, id int64, repo string, n int, last string) ([]string, error) {
	client, err := s.getClient(ctx, id)
	if err != nil {
		return nil, err
	}
	return client.Tags(ctx, repo, n, last)
}

// GetTagDetail returns both tag metadata and manifest detail for a specific tag.
func (s *RegistryService) GetTagDetail(ctx context.Context, id int64, repo, tag string) (*model.RegistryTag, *model.ManifestDetail, error) {
	client, err := s.getClient(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	return client.TagDetail(ctx, repo, tag)
}

// DeleteTag deletes a specific tag from a repository.
func (s *RegistryService) DeleteTag(ctx context.Context, id int64, repo, tag string) error {
	client, err := s.getClient(ctx, id)
	if err != nil {
		return err
	}
	// Resolve tag to digest first.
	tagDetail, _, err := client.TagDetail(ctx, repo, tag)
	if err != nil {
		return fmt.Errorf("resolving tag %q: %w", tag, err)
	}
	return client.DeleteManifest(ctx, repo, tagDetail.Digest)
}

// getClient creates a registry client from a registry record.
func (s *RegistryService) getClient(ctx context.Context, id int64) (*registry.RegistryClient, error) {
	reg, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("getting registry %d: %w", id, err)
	}
	if reg == nil {
		return nil, fmt.Errorf("registry %d not found", id)
	}
	password, err := s.repo.DecryptPassword(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("decrypting password: %w", err)
	}
	var creds *registry.Credentials
	if reg.Username != "" {
		creds = &registry.Credentials{Username: reg.Username, Password: password}
	}
	return registry.NewClient(reg.URL, creds)
}
