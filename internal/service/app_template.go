package service

import (
	"context"
	"log/slog"

	"github.com/Mi-Bee-Studio/mibeehive/internal/db"
	"github.com/Mi-Bee-Studio/mibeehive/internal/model"
)

// AppTemplateService provides business logic for application templates.
type AppTemplateService struct {
	repo   *db.AppTemplateRepo
	logger *slog.Logger
}

// NewAppTemplateService creates a new template service.
func NewAppTemplateService(repo *db.AppTemplateRepo, logger *slog.Logger) *AppTemplateService {
	return &AppTemplateService{repo: repo, logger: logger}
}

// ListTemplates returns all enabled application templates.
func (s *AppTemplateService) ListTemplates(ctx context.Context) ([]model.AppTemplate, error) {
	templates, err := s.repo.List(ctx)
	if err != nil {
		s.logger.Error("failed to list templates", "error", err)
		return nil, err
	}
	return templates, nil
}

// GetTemplate returns a single template by ID.
func (s *AppTemplateService) GetTemplate(ctx context.Context, id int64) (*model.AppTemplate, error) {
	t, err := s.repo.GetByID(ctx, id)
	if err != nil {
		s.logger.Error("failed to get template", "id", id, "error", err)
		return nil, err
	}
	return t, nil
}

// CreateFromTemplate creates a new container config from a template with optional overrides.
func (s *AppTemplateService) CreateFromTemplate(ctx context.Context, templateID int64, overrides *model.CreateContainerRequest) (*model.CreateContainerRequest, error) {
	tmpl, err := s.repo.GetByID(ctx, templateID)
	if err != nil {
		return nil, err
	}
	if tmpl == nil {
		return nil, nil
	}

	req := &model.CreateContainerRequest{
		Image:         tmpl.Image,
		Command:       tmpl.Command,
		Env:           tmpl.Env,
		Ports:         tmpl.Ports,
		Volumes:       tmpl.Volumes,
		RestartPolicy: tmpl.RestartPolicy,
	}

	// Apply overrides
	if overrides != nil {
		if overrides.Name != "" {
			req.Name = overrides.Name
		}
		if overrides.Image != "" {
			req.Image = overrides.Image
		}
		if overrides.Command != "" {
			req.Command = overrides.Command
		}
		if len(overrides.Env) > 0 {
			if req.Env == nil {
				req.Env = make(map[string]string)
			}
			for k, v := range overrides.Env {
				req.Env[k] = v
			}
		}
		if len(overrides.Ports) > 0 {
			req.Ports = overrides.Ports
		}
		if len(overrides.Volumes) > 0 {
			req.Volumes = overrides.Volumes
		}
		if overrides.RestartPolicy != "" {
			req.RestartPolicy = overrides.RestartPolicy
		}
		if overrides.MemoryLimit != "" {
			req.MemoryLimit = overrides.MemoryLimit
		}
		if overrides.CPULimit > 0 {
			req.CPULimit = overrides.CPULimit
		}
	}

	s.logger.Info("created container config from template", "template", tmpl.Name, "image", req.Image)
	return req, nil
}
