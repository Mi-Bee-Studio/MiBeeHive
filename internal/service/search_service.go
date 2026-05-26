package service

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"

	"github.com/Mi-Bee-Studio/mibeehive/internal/model"
)

const searchLimit = 10

// SearchService provides global search across all entity types.
type SearchService struct {
	db *sql.DB
}

// NewSearchService creates a new SearchService.
func NewSearchService(db *sql.DB) *SearchService {
	return &SearchService{db: db}
}

// Search performs a global search across the specified type or all types.
// searchType must be one of: "all", "project", "file", "config", "iso", "container".
func (s *SearchService) Search(ctx context.Context, query string, searchType string) (*model.SearchResponse, error) {
	if query == "" {
		return nil, fmt.Errorf("search query must not be empty")
	}

	resp := &model.SearchResponse{
		Projects:   []model.SearchResult{},
		Files:      []model.SearchResult{},
		Configs:    []model.SearchResult{},
		ISOs:       []model.SearchResult{},
		Containers: []model.SearchResult{},
	}

	pattern := "%" + query + "%"

	switch searchType {
	case "project":
		results, err := s.searchProjects(ctx, pattern)
		if err != nil {
			return nil, fmt.Errorf("searching projects: %w", err)
		}
		resp.Projects = results
	case "file":
		results, err := s.searchFiles(ctx, pattern)
		if err != nil {
			return nil, fmt.Errorf("searching files: %w", err)
		}
		resp.Files = results
	case "config":
		results, err := s.searchConfigs(ctx, pattern)
		if err != nil {
			return nil, fmt.Errorf("searching configs: %w", err)
		}
		resp.Configs = results
	case "iso":
		results, err := s.searchISOs(ctx, pattern)
		if err != nil {
			return nil, fmt.Errorf("searching ISOs: %w", err)
		}
		resp.ISOs = results
	case "container":
		results, err := s.searchContainers(ctx, pattern)
		if err != nil {
			return nil, fmt.Errorf("searching containers: %w", err)
		}
		resp.Containers = results
	default: // "all"
		var err error
		if resp.Projects, err = s.searchProjects(ctx, pattern); err != nil {
			return nil, fmt.Errorf("searching projects: %w", err)
		}
		if resp.Files, err = s.searchFiles(ctx, pattern); err != nil {
			return nil, fmt.Errorf("searching files: %w", err)
		}
		if resp.Configs, err = s.searchConfigs(ctx, pattern); err != nil {
			return nil, fmt.Errorf("searching configs: %w", err)
		}
		if resp.ISOs, err = s.searchISOs(ctx, pattern); err != nil {
			return nil, fmt.Errorf("searching ISOs: %w", err)
		}
		if resp.Containers, err = s.searchContainers(ctx, pattern); err != nil {
			return nil, fmt.Errorf("searching containers: %w", err)
		}
	}

	resp.Total = len(resp.Projects) + len(resp.Files) + len(resp.Configs) + len(resp.ISOs) + len(resp.Containers)

	slog.Debug("global search completed", "query", query, "type", searchType, "total", resp.Total)
	return resp, nil
}

// SearchPaginated performs a global search with per-category limit/offset support.
func (s *SearchService) SearchPaginated(ctx context.Context, query string, searchType string, limit, offset int) (*model.SearchResponse, error) {
	if query == "" {
		return nil, fmt.Errorf("search query must not be empty")
	}

	resp := &model.SearchResponse{
		Projects:   []model.SearchResult{},
		Files:      []model.SearchResult{},
		Configs:    []model.SearchResult{},
		ISOs:       []model.SearchResult{},
		Containers: []model.SearchResult{},
	}

	pattern := "%" + query + "%"

	switch searchType {
	case "project":
		results, total, err := s.searchProjectsPaginated(ctx, pattern, limit, offset)
		if err != nil {
			return nil, fmt.Errorf("searching projects: %w", err)
		}
		resp.Projects = results
		resp.Total = total
	case "file":
		results, total, err := s.searchFilesPaginated(ctx, pattern, limit, offset)
		if err != nil {
			return nil, fmt.Errorf("searching files: %w", err)
		}
		resp.Files = results
		resp.Total = total
	case "config":
		results, total, err := s.searchConfigsPaginated(ctx, pattern, limit, offset)
		if err != nil {
			return nil, fmt.Errorf("searching configs: %w", err)
		}
		resp.Configs = results
		resp.Total = total
	case "iso":
		results, total, err := s.searchISOsPaginated(ctx, pattern, limit, offset)
		if err != nil {
			return nil, fmt.Errorf("searching ISOs: %w", err)
		}
		resp.ISOs = results
		resp.Total = total
	case "container":
		results, total, err := s.searchContainersPaginated(ctx, pattern, limit, offset)
		if err != nil {
			return nil, fmt.Errorf("searching containers: %w", err)
		}
		resp.Containers = results
		resp.Total = total
	default: // "all"
		var err error
		if resp.Projects, _, err = s.searchProjectsPaginated(ctx, pattern, limit, offset); err != nil {
			return nil, fmt.Errorf("searching projects: %w", err)
		}
		if resp.Files, _, err = s.searchFilesPaginated(ctx, pattern, limit, offset); err != nil {
			return nil, fmt.Errorf("searching files: %w", err)
		}
		if resp.Configs, _, err = s.searchConfigsPaginated(ctx, pattern, limit, offset); err != nil {
			return nil, fmt.Errorf("searching configs: %w", err)
		}
		if resp.ISOs, _, err = s.searchISOsPaginated(ctx, pattern, limit, offset); err != nil {
			return nil, fmt.Errorf("searching ISOs: %w", err)
		}
		if resp.Containers, _, err = s.searchContainersPaginated(ctx, pattern, limit, offset); err != nil {
			return nil, fmt.Errorf("searching containers: %w", err)
		}
		resp.Total = len(resp.Projects) + len(resp.Files) + len(resp.Configs) + len(resp.ISOs) + len(resp.Containers)
	}

	slog.Debug("global search completed", "query", query, "type", searchType, "total", resp.Total)
	return resp, nil
}

func (s *SearchService) searchProjects(ctx context.Context, pattern string) ([]model.SearchResult, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, display_name FROM projects
		 WHERE name LIKE ? OR display_name LIKE ?
		 ORDER BY name LIMIT ?`, pattern, pattern, searchLimit)
	if err != nil {
		return nil, fmt.Errorf("querying projects: %w", err)
	}
	defer rows.Close()

	var results []model.SearchResult
	for rows.Next() {
		var r model.SearchResult
		if err := rows.Scan(&r.ID, &r.Name, &r.Detail); err != nil {
			return nil, fmt.Errorf("scanning project result: %w", err)
		}
		r.Type = "project"
		results = append(results, r)
	}
	if results == nil {
		results = []model.SearchResult{}
	}
	return results, rows.Err()
}

func (s *SearchService) searchFiles(ctx context.Context, pattern string) ([]model.SearchResult, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, filename, version FROM files
		 WHERE filename LIKE ? OR version LIKE ?
		 ORDER BY filename LIMIT ?`, pattern, pattern, searchLimit)
	if err != nil {
		return nil, fmt.Errorf("querying files: %w", err)
	}
	defer rows.Close()

	var results []model.SearchResult
	for rows.Next() {
		var r model.SearchResult
		if err := rows.Scan(&r.ID, &r.Name, &r.Detail); err != nil {
			return nil, fmt.Errorf("scanning file result: %w", err)
		}
		r.Type = "file"
		results = append(results, r)
	}
	if results == nil {
		results = []model.SearchResult{}
	}
	return results, rows.Err()
}

func (s *SearchService) searchConfigs(ctx context.Context, pattern string) ([]model.SearchResult, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, config_name FROM os_install_configs
		 WHERE name LIKE ? OR config_name LIKE ?
		 ORDER BY name LIMIT ?`, pattern, pattern, searchLimit)
	if err != nil {
		return nil, fmt.Errorf("querying configs: %w", err)
	}
	defer rows.Close()

	var results []model.SearchResult
	for rows.Next() {
		var r model.SearchResult
		if err := rows.Scan(&r.ID, &r.Name, &r.Detail); err != nil {
			return nil, fmt.Errorf("scanning config result: %w", err)
		}
		r.Type = "config"
		results = append(results, r)
	}
	if results == nil {
		results = []model.SearchResult{}
	}
	return results, rows.Err()
}

func (s *SearchService) searchISOs(ctx context.Context, pattern string) ([]model.SearchResult, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, distro FROM iso_catalog
		 WHERE name LIKE ?
		 ORDER BY name LIMIT ?`, pattern, searchLimit)
	if err != nil {
		return nil, fmt.Errorf("querying ISOs: %w", err)
	}
	defer rows.Close()

	var results []model.SearchResult
	for rows.Next() {
		var r model.SearchResult
		if err := rows.Scan(&r.ID, &r.Name, &r.Detail); err != nil {
			return nil, fmt.Errorf("scanning ISO result: %w", err)
		}
		r.Type = "iso"
		results = append(results, r)
	}
	if results == nil {
		results = []model.SearchResult{}
	}
	return results, rows.Err()
}

func (s *SearchService) searchContainers(ctx context.Context, pattern string) ([]model.SearchResult, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, image FROM container_apps
		 WHERE name LIKE ?
		 ORDER BY name LIMIT ?`, pattern, searchLimit)
	if err != nil {
		return nil, fmt.Errorf("querying containers: %w", err)
	}
	defer rows.Close()

	var results []model.SearchResult
	for rows.Next() {
		var r model.SearchResult
		if err := rows.Scan(&r.ID, &r.Name, &r.Detail); err != nil {
			return nil, fmt.Errorf("scanning container result: %w", err)
		}
		r.Type = "container"
		results = append(results, r)
	}
	if results == nil {
		results = []model.SearchResult{}
	}
	return results, rows.Err()
}

// --- Paginated search helpers ---

func (s *SearchService) searchProjectsPaginated(ctx context.Context, pattern string, limit, offset int) ([]model.SearchResult, int, error) {
	var total int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM projects WHERE name LIKE ? OR display_name LIKE ?`, pattern, pattern).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("counting projects: %w", err)
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, display_name FROM projects
		 WHERE name LIKE ? OR display_name LIKE ?
		 ORDER BY name LIMIT ? OFFSET ?`, pattern, pattern, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("querying projects: %w", err)
	}
	defer rows.Close()
	var results []model.SearchResult
	for rows.Next() {
		var r model.SearchResult
		if err := rows.Scan(&r.ID, &r.Name, &r.Detail); err != nil {
			return nil, 0, fmt.Errorf("scanning project result: %w", err)
		}
		r.Type = "project"
		results = append(results, r)
	}
	if results == nil {
		results = []model.SearchResult{}
	}
	return results, total, rows.Err()
}

func (s *SearchService) searchFilesPaginated(ctx context.Context, pattern string, limit, offset int) ([]model.SearchResult, int, error) {
	var total int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM files WHERE filename LIKE ? OR version LIKE ?`, pattern, pattern).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("counting files: %w", err)
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, filename, version FROM files
		 WHERE filename LIKE ? OR version LIKE ?
		 ORDER BY filename LIMIT ? OFFSET ?`, pattern, pattern, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("querying files: %w", err)
	}
	defer rows.Close()
	var results []model.SearchResult
	for rows.Next() {
		var r model.SearchResult
		if err := rows.Scan(&r.ID, &r.Name, &r.Detail); err != nil {
			return nil, 0, fmt.Errorf("scanning file result: %w", err)
		}
		r.Type = "file"
		results = append(results, r)
	}
	if results == nil {
		results = []model.SearchResult{}
	}
	return results, total, rows.Err()
}

func (s *SearchService) searchConfigsPaginated(ctx context.Context, pattern string, limit, offset int) ([]model.SearchResult, int, error) {
	var total int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM os_install_configs WHERE name LIKE ? OR config_name LIKE ?`, pattern, pattern).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("counting configs: %w", err)
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, config_name FROM os_install_configs
		 WHERE name LIKE ? OR config_name LIKE ?
		 ORDER BY name LIMIT ? OFFSET ?`, pattern, pattern, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("querying configs: %w", err)
	}
	defer rows.Close()
	var results []model.SearchResult
	for rows.Next() {
		var r model.SearchResult
		if err := rows.Scan(&r.ID, &r.Name, &r.Detail); err != nil {
			return nil, 0, fmt.Errorf("scanning config result: %w", err)
		}
		r.Type = "config"
		results = append(results, r)
	}
	if results == nil {
		results = []model.SearchResult{}
	}
	return results, total, rows.Err()
}

func (s *SearchService) searchISOsPaginated(ctx context.Context, pattern string, limit, offset int) ([]model.SearchResult, int, error) {
	var total int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM iso_catalog WHERE name LIKE ?`, pattern).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("counting ISOs: %w", err)
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, distro FROM iso_catalog
		 WHERE name LIKE ?
		 ORDER BY name LIMIT ? OFFSET ?`, pattern, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("querying ISOs: %w", err)
	}
	defer rows.Close()
	var results []model.SearchResult
	for rows.Next() {
		var r model.SearchResult
		if err := rows.Scan(&r.ID, &r.Name, &r.Detail); err != nil {
			return nil, 0, fmt.Errorf("scanning ISO result: %w", err)
		}
		r.Type = "iso"
		results = append(results, r)
	}
	if results == nil {
		results = []model.SearchResult{}
	}
	return results, total, rows.Err()
}

func (s *SearchService) searchContainersPaginated(ctx context.Context, pattern string, limit, offset int) ([]model.SearchResult, int, error) {
	var total int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM container_apps WHERE name LIKE ?`, pattern).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("counting containers: %w", err)
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, image FROM container_apps
		 WHERE name LIKE ?
		 ORDER BY name LIMIT ? OFFSET ?`, pattern, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("querying containers: %w", err)
	}
	defer rows.Close()
	var results []model.SearchResult
	for rows.Next() {
		var r model.SearchResult
		if err := rows.Scan(&r.ID, &r.Name, &r.Detail); err != nil {
			return nil, 0, fmt.Errorf("scanning container result: %w", err)
		}
		r.Type = "container"
		results = append(results, r)
	}
	if results == nil {
		results = []model.SearchResult{}
	}
	return results, total, rows.Err()
}
