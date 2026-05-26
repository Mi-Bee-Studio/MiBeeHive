package db

import (
	"database/sql"
	"embed"
	"strings"
)

//go:embed all:migrations
var migrationsFS embed.FS

// Migrate reads and executes all migration SQL files in order,
// skipping any that have already been applied.
func Migrate(db *sql.DB) error {
	// Disable foreign keys before transaction — PRAGMA cannot be changed inside a transaction.
	if _, err := db.Exec("PRAGMA foreign_keys = OFF"); err != nil {
		return err
	}

	migrations := []struct{ name, path string }{
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
	}
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (version TEXT PRIMARY KEY, applied_at DATETIME DEFAULT CURRENT_TIMESTAMP)`); err != nil {
		return err
	}

	for _, m := range migrations {
		var exists bool
		if err := tx.QueryRow("SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version = ?)", m.name).Scan(&exists); err != nil {
			return err
		}
		if exists {
			continue
		}

		data, err := migrationsFS.ReadFile(m.path)
		if err != nil {
			return err
		}
		if err := execMigration(tx, string(data)); err != nil {
			return err
		}

		if _, err := tx.Exec("INSERT INTO schema_migrations (version) VALUES (?)", m.name); err != nil {
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	// Re-enable foreign keys after transaction.
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		return err
	}
	return nil
}

// execMigration splits a migration file into individual statements and executes each.
// ALTER TABLE ADD COLUMN statements that fail with "duplicate column" are silently skipped,
// making migrations idempotent for column additions.
func execMigration(tx *sql.Tx, sql string) error {
	for _, stmt := range splitStatements(sql) {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}
		if _, err := tx.Exec(stmt); err != nil {
			// Ignore duplicate column errors on ALTER TABLE ADD COLUMN
			if strings.Contains(err.Error(), "duplicate column name") {
				continue
			}
			return err
		}
	}
	return nil
}

// splitStatements splits SQL text into individual statements by semicolons,
// respecting single-line comments.
func splitStatements(sql string) []string {
	var stmts []string
	var current strings.Builder
	for _, line := range strings.Split(sql, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "--") {
			continue
		}
		current.WriteString(line)
		current.WriteString("\n")
		if strings.HasSuffix(trimmed, ";") {
			stmts = append(stmts, current.String())
			current.Reset()
		}
	}
	if current.Len() > 0 {
		stmts = append(stmts, current.String())
	}
	return stmts
}
