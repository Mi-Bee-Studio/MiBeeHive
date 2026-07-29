package db

import (
	"testing"
)

func TestMigrationCountMatchesFiles(t *testing.T) {
	// Registered migrations must match the files on disk.
	// When adding a new migration, update both migrate.go and this list.
	expected := []struct{ name, path string }{
		{"001", "migrations/001_init.sql"},
		{"002", "migrations/002_system_stats.sql"},
		{"003", "migrations/003_file_retry.sql"},
		{"004", "migrations/004_project_web_management.sql"},
		{"005", "migrations/005_os_install_enhancements.sql"},
		{"006", "migrations/006_iso_catalog.sql"},
		{"007", "migrations/007_iso_catalog_seed.sql"},
		{"008", "migrations/008_fix_iso_regex_patterns.sql"},
		{"009", "migrations/009_iso_queue.sql"},
		{"010", "migrations/010_os_install_configs_seed.sql"},
		{"011", "migrations/011_containers.sql"},
		{"012", "migrations/012_app_templates_seed.sql"},
		{"013", "migrations/013_os_templates_seed.sql"},
		{"014", "migrations/014_iso_checksum.sql"},
		{"015", "migrations/015_registry.sql"},
		{"016", "migrations/016_add_indexes.sql"},
		{"017", "migrations/017_consolidated.sql"},
		{"018", "migrations/018_iso_catalog_v2.sql"},
		{"019", "migrations/019_storage_paths.sql"},
		{"020", "migrations/020_fix_iso_catalog.sql"},
		{"021", "migrations/021_source_type_any.sql"},
	}

	// Read all files in the embedded migrations directory.
	entries, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		t.Fatalf("ReadDir migrations: %v", err)
	}

	var sqlFiles int
	for _, e := range entries {
		if !e.IsDir() {
			sqlFiles++
		}
	}

	if len(expected) != sqlFiles {
		t.Fatalf("migration count mismatch: %d registered in migrate.go vs %d files on disk",
			len(expected), sqlFiles)
	}

	// Verify every registered migration file actually exists in the embed FS.
	for _, m := range expected {
		if _, err := migrationsFS.ReadFile(m.path); err != nil {
			t.Errorf("registered migration %q (%s) not found in embed FS: %v", m.name, m.path, err)
		}
	}
}
