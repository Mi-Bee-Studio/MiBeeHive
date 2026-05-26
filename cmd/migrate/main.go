// migrate is a standalone tool that scans existing files and imports them
// into the MiBeeHive SQLite database as "imported" status entries.
//
// Usage:
//
//	go run ./cmd/migrate/ -data-dir /var/lib/mibeehive/oss -db /var/lib/mibeehive/mibeehive.db
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/Mi-Bee-Studio/mibeehive/internal/config"
	"github.com/Mi-Bee-Studio/mibeehive/internal/db"
)

func main() {
	dataDir := flag.String("data-dir", "./data", "root directory to scan for files")
	configPath := flag.String("config", "./config.yaml", "config file for project metadata")
	dbPath := flag.String("db", "./mibeehive.db", "SQLite database path")
	flag.Parse()

	logger := log.Default()

	// Validate data-dir exists.
	if _, err := os.Stat(*dataDir); os.IsNotExist(err) {
		logger.Fatalf("error: data directory %q does not exist", *dataDir)
	}

	// Load config (optional — warn if missing).
	var cfg *config.Config
	if _, err := os.Stat(*configPath); err == nil {
		var loadErr error
		cfg, loadErr = config.Load(*configPath)
		if loadErr != nil {
			logger.Printf("warning: failed to load config %q: %v — will create projects with source_type=unknown", *configPath, loadErr)
			cfg = nil
		}
	} else {
		logger.Printf("warning: config file %q not found — will create projects with source_type=unknown", *configPath)
	}

	// Open database and run migrations.
	database, err := db.Open(*dbPath)
	if err != nil {
		logger.Fatalf("error: opening database: %v", err)
	}
	defer db.Close(database)

	// Run migrations — skip if tables already exist (idempotent).
	var tableCount int
	database.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='projects'").Scan(&tableCount)
	if tableCount == 0 {
		if err := db.Migrate(database); err != nil {
			logger.Fatalf("error: running migrations: %v", err)
		}
	}

	projectRepo := db.NewProjectRepo(database)
	fileRepo := db.NewFileRepo(database)
	ctx := context.Background()

	// Build project lookup from config.
	configProjects := make(map[string]config.ProjectConfig)
	if cfg != nil {
		for _, p := range cfg.Projects {
			configProjects[strings.ToLower(p.Name)] = p
		}
	}

	// Print header.
	fmt.Println("MiBeeHive Migration Tool")
	fmt.Println("========================")
	fmt.Printf("Data dir: %s\n", *dataDir)
	fmt.Printf("DB: %s\n", *dbPath)
	fmt.Println()

	// Read project directories.
	entries, err := os.ReadDir(*dataDir)
	if err != nil {
		logger.Fatalf("error: reading data directory: %v", err)
	}

	// Stats.
	var totalImported, totalSkipped, totalParseErrors int
	var totalSize int64
	var projectsFound, projectsCreated int

	fmt.Println("Scanning projects...")

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		dirName := entry.Name()
		projectsFound++

		// Try to match directory to a config project.
		projConfig, matched := matchProject(dirName, configProjects)

		// Get or create the project in DB.
		proj, err := projectRepo.GetByName(ctx, dirName)
		if err != nil {
			logger.Printf("  error: querying project %q: %v", dirName, err)
			continue
		}

		if proj == nil {
			// Create project.
			name := dirName
			displayName := dirName
			sourceType := "github" // fallback: DB CHECK constraint requires valid source_type
			sourceURL := ""
			if matched {
				name = projConfig.Name
				displayName = projConfig.DisplayName
				sourceType = string(projConfig.SourceType)
				sourceURL = projConfig.SourceURL
			}

			proj, err = projectRepo.Create(ctx, name, displayName, sourceType, sourceURL)
			if err != nil {
				logger.Printf("  error: creating project %q: %v", name, err)
				continue
			}
			projectsCreated++
		}

		// Scan files in the project directory.
		projectDir := filepath.Join(*dataDir, dirName)
		imported, skipped, parseErrors, size := scanProjectFiles(ctx, logger, fileRepo, proj.ID, projectDir)

		totalImported += imported
		totalSkipped += skipped
		totalParseErrors += parseErrors
		totalSize += size

		sourceLabel := "unknown"
		if proj.SourceType != "" {
			sourceLabel = proj.SourceType
		}
		fmt.Printf("  ✓ %s (%s) — %d imported, %d skipped", proj.Name, sourceLabel, imported, skipped)
		if parseErrors > 0 {
			fmt.Printf(", %d parse errors", parseErrors)
		}
		fmt.Println()
	}

	// Print summary.
	fmt.Println()
	fmt.Println("Summary:")
	fmt.Printf("  Projects: %d found, %d created\n", projectsFound, projectsCreated)
	fmt.Printf("  Files: %d imported, %d skipped, %d total\n", totalImported, totalSkipped, totalImported+totalSkipped)
	fmt.Printf("  Total size: %s\n", formatSize(totalSize))
	if totalParseErrors > 0 {
		fmt.Printf("  Parse errors: %d (files skipped)\n", totalParseErrors)
	}
}

// projectResult holds the results of scanning a project directory.
type projectResult struct {
	imported    int
	skipped     int
	parseErrors int
	size        int64
}

// scanProjectFiles scans all files in a project directory and imports them into the DB.
func scanProjectFiles(ctx context.Context, logger *log.Logger, fileRepo *db.FileRepo, projectID int64, projectDir string) (imported, skipped, parseErrors int, totalSize int64) {
	entries, err := os.ReadDir(projectDir)
	if err != nil {
		logger.Printf("  warning: cannot read directory %q: %v", projectDir, err)
		return
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if entry.Type()&os.ModeSymlink != 0 {
			continue
		}
		if strings.HasPrefix(entry.Name(), ".") {
			continue
		}

		fullPath := filepath.Join(projectDir, entry.Name())
		info, err := os.Stat(fullPath)
		if err != nil {
			logger.Printf("  warning: cannot stat %q: %v", fullPath, err)
			parseErrors++
			continue
		}

		// Parse filename to extract version, os, arch, ext.
		parsed := parseFilename(entry.Name())

		// Check if file already exists (idempotent).
		existing, err := fileRepo.FindExisting(ctx, projectID, entry.Name())
		if err != nil {
			logger.Printf("  warning: DB error checking %q: %v", entry.Name(), err)
			continue
		}
		if existing != nil {
			skipped++
			totalSize += info.Size()
			continue
		}

		// Insert new file.
		file := &db.File{
			ProjectID: projectID,
			Version:   parsed.version,
			Filename:  entry.Name(),
			OS:        parsed.os,
			Arch:      parsed.arch,
			Ext:       parsed.ext,
			SizeBytes: info.Size(),
			LocalPath: fullPath,
			Status:    "imported",
		}

		if _, err := fileRepo.Create(ctx, file); err != nil {
			logger.Printf("  warning: DB error inserting %q: %v", entry.Name(), err)
			continue
		}

		imported++
		totalSize += info.Size()
	}

	return
}

// parsedFile holds the result of parsing a filename.
type parsedFile struct {
	version string
	os      string
	arch    string
	ext     string
}

// semverRegex matches version-like strings (e.g. 1.2.3, 3.11.3, v1.142.0).
var semverRegex = regexp.MustCompile(`v?\d+\.\d+(?:\.\d+)?`)

// knownOS is the set of recognized operating system identifiers.
var knownOS = map[string]bool{
	"linux":   true,
	"darwin":  true,
	"windows": true,
	"freebsd": true,
}

// knownArch is the set of recognized architecture identifiers.
var knownArch = map[string]bool{
	"amd64":   true,
	"arm64":   true,
	"armv6":   true,
	"armv7":   true,
	"386":     true,
	"s390x":   true,
	"ppc64le": true,
}

// parseFilename extracts version, os, arch, and ext from a release asset filename.
//
// Supported patterns:
//
//	prometheus-3.11.3.linux-arm64.tar.gz       → version=3.11.3, os=linux, arch=arm64, ext=tar.gz
//	go1.26.2.linux-arm64.tar.gz               → version=1.26.2, os=linux, arch=arm64, ext=tar.gz
//	consul_1.22.5_linux_arm64.zip              → version=1.22.5, os=linux, arch=arm64, ext=zip
//	victoria-metrics-darwin-amd64-v1.142.0.tar.gz → version=1.142.0, os=darwin, arch=amd64, ext=tar.gz
//	node_exporter-1.9.0.linux-amd64.tar.gz      → version=1.9.0, os=linux, arch=amd64, ext=tar.gz
func parseFilename(name string) parsedFile {
	// Extract extension first (handles compound extensions like .tar.gz).
	ext := fileExt(name)
	base := name
	if ext != "" {
		base = strings.TrimSuffix(name, "."+ext)
	}

	// Normalize: replace underscores and dots with hyphens for consistent splitting.
	normalized := strings.ReplaceAll(base, "_", "-")
	normalized = strings.ReplaceAll(normalized, ".", "-")
	parts := strings.Split(normalized, "-")

	var osVal, archVal, version string

	for _, p := range parts {
		p = strings.ToLower(p)

		if knownOS[p] && osVal == "" {
			osVal = p
			continue
		}
		if knownArch[p] && archVal == "" {
			archVal = p
			continue
		}
		// Check for version-like pattern (e.g. "v1-142-0" after dot→dash normalization).
		if version == "" && semverRegex.MatchString(p) {
			// Reconstruct version from this segment (dots were replaced with dashes).
			version = strings.ReplaceAll(p, "-", ".")
			// Strip leading 'v' if present.
			version = strings.TrimPrefix(version, "v")
		}
	}

	return parsedFile{
		version: version,
		os:      osVal,
		arch:    archVal,
		ext:     ext,
	}
}

// fileExt returns the file extension, handling compound extensions like .tar.gz.
func fileExt(name string) string {
	for _, ext := range []string{".tar.gz", ".tar.bz2", ".tar.xz"} {
		if strings.HasSuffix(name, ext) {
			return ext[1:] // strip leading dot
		}
	}
	if idx := strings.LastIndex(name, "."); idx >= 0 {
		return name[idx+1:]
	}
	return ""
}

// matchProject tries to match a directory name to a config project.
// Returns the matched config and whether a match was found.
func matchProject(dirName string, configProjects map[string]config.ProjectConfig) (config.ProjectConfig, bool) {
	dirLower := strings.ToLower(dirName)

	// 1. Exact match (case-insensitive).
	if p, ok := configProjects[dirLower]; ok {
		return p, true
	}

	// 2. Strip common suffixes from directory name.
	stripped := stripSuffix(dirLower)
	if stripped != dirLower {
		if p, ok := configProjects[stripped]; ok {
			return p, true
		}
	}

	// 3. Directory name contains project name or vice versa.
	for name, p := range configProjects {
		if strings.Contains(dirLower, name) || strings.Contains(name, dirLower) {
			return p, true
		}
		// Also try stripping dashes for comparison.
		nameNoDash := strings.ReplaceAll(name, "-", "")
		dirNoDash := strings.ReplaceAll(dirLower, "-", "")
		if nameNoDash == dirNoDash {
			return p, true
		}
	}

	return config.ProjectConfig{}, false
}

// stripSuffix removes common suffixes from directory names.
func stripSuffix(name string) string {
	suffixes := []string{"-releases", "_releases", "-download", "_download", "-dl", "_dl"}
	for _, s := range suffixes {
		if strings.HasSuffix(name, s) {
			return strings.TrimSuffix(name, s)
		}
	}
	return name
}

// formatSize formats a byte size into a human-readable string.
func formatSize(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
		TB = GB * 1024
	)

	switch {
	case bytes >= TB:
		return fmt.Sprintf("%.1f TB", float64(bytes)/float64(TB))
	case bytes >= GB:
		return fmt.Sprintf("%.1f GB", float64(bytes)/float64(GB))
	case bytes >= MB:
		return fmt.Sprintf("%.1f MB", float64(bytes)/float64(MB))
	case bytes >= KB:
		return fmt.Sprintf("%.1f KB", float64(bytes)/float64(KB))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}
