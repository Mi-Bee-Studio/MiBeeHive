package db

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/Mi-Bee-Studio/mibeehive/internal/config"
	"github.com/Mi-Bee-Studio/mibeehive/internal/model"
)

// SeedProjectsFromConfig imports projects from config into the database.
// It only seeds when the DB has no existing projects. Returns the count of seeded projects.
func SeedProjectsFromConfig(ctx context.Context, repo *ProjectRepo, projects []config.ProjectConfig) (int, error) {
	existing, err := repo.ListAll(ctx)
	if err != nil {
		return 0, fmt.Errorf("checking existing projects: %w", err)
	}
	if len(existing) > 0 {
		return 0, nil
	}

	count := 0
	for _, pc := range projects {
		settings := model.ProjectSettings{
			GitHubOwner:    pc.GitHubOwner,
			GitHubRepo:     pc.GitHubRepo,
			FilterPatterns: pc.FilterPatterns,
		}

		if pc.CrawlInterval != "" {
			d, err := time.ParseDuration(pc.CrawlInterval)
			if err != nil {
				slog.Warn("invalid crawl_interval, skipping", "project", pc.Name, "interval", pc.CrawlInterval, "error", err)
				continue
			}
			settings.CrawlInterval = int(d.Minutes())
		}

		_, err = repo.CreateWithSettings(ctx,
			pc.Name,
			pc.DisplayName,
			string(pc.SourceType),
			pc.SourceURL,
			settings,
		)
		if err != nil {
			slog.Warn("failed to seed project", "project", pc.Name, "error", err)
			continue
		}
		count++
	}

	slog.Info("seeded projects from config", "count", count)
	return count, nil
}

// SeedOSInstallConfigs seeds default OS installation configurations if they don't already exist.
// It checks by config_name to avoid re-creating configs the user has deliberately deleted.
func SeedOSInstallConfigs(ctx context.Context, repo *OsInstallConfigRepo) (int, error) {
	type seedConfig struct {
		Name       string
		ConfigName string
		OsType     string
		Config     string
	}

	defaults := []seedConfig{
		{
			Name:       "Ubuntu 22.04 LTS Server",
			ConfigName: "ubuntu-2204-default",
			OsType:     "ubuntu",
			Config:     `{"hostname":"ubuntu-2204-server","timezone":"Asia/Shanghai","language":"en_US","keyboard_layout":"us","disk":"/dev/sda","partition_scheme":"whole_disk"}`,
		},
		{
			Name:       "Ubuntu 24.04 LTS Server",
			ConfigName: "ubuntu-2404-default",
			OsType:     "ubuntu",
			Config:     `{"hostname":"ubuntu-2404-server","timezone":"Asia/Shanghai","language":"en_US","keyboard_layout":"us","disk":"/dev/sda","partition_scheme":"whole_disk"}`,
		},
		{
			Name:       "Debian 12 Server",
			ConfigName: "debian-12-default",
			OsType:     "debian",
			Config:     `{"hostname":"debian-server","timezone":"Asia/Shanghai","language":"en_US","keyboard_layout":"us","disk":"/dev/sda","partition_scheme":"whole_disk"}`,
		},
		{
			Name:       "CentOS Stream 9 Server",
			ConfigName: "centos-stream9-default",
			OsType:     "centos",
			Config:     `{"hostname":"centos-server","timezone":"Asia/Shanghai","language":"en_US","keyboard_layout":"us","disk":"/dev/sda","partition_scheme":"whole_disk"}`,
		},
	}

	existing, err := repo.List(ctx)
	if err != nil {
		return 0, fmt.Errorf("listing existing os install configs: %w", err)
	}

	existingNames := make(map[string]bool, len(existing))
	for _, c := range existing {
		existingNames[c.ConfigName] = true
	}

	count := 0
	for _, d := range defaults {
		if existingNames[d.ConfigName] {
			continue
		}
		_, err := repo.Create(ctx, d.Name, d.ConfigName, d.OsType, d.Config)
		if err != nil {
			slog.Warn("failed to seed os install config", "config", d.ConfigName, "error", err)
			continue
		}
		count++
	}

	if count > 0 {
		slog.Info("seeded default OS install configs", "count", count)
	}
	return count, nil
}
