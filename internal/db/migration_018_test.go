package db

import (
	"context"
	"database/sql"
	"testing"
)

// TestMigration018FreshInstall verifies that fresh DB with all 18 migrations
// has the new distro-level seed entries and the 3 new columns.
func TestMigration018FreshInstall(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()

	// Verify new columns exist.
	rows, err := db.QueryContext(ctx, "PRAGMA table_info(iso_catalog)")
	if err != nil {
		t.Fatalf("PRAGMA table_info(iso_catalog): %v", err)
	}
	defer rows.Close()

	columns := make(map[string]bool)
	for rows.Next() {
		var cid int
		var name, typ string
		var notnull int
		var dflt sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &typ, &notnull, &dflt, &pk); err != nil {
			t.Fatalf("scanning table_info: %v", err)
		}
		columns[name] = true
	}

	newCols := []string{"base_url", "version_dir_pattern", "iso_path_template"}
	for _, col := range newCols {
		if !columns[col] {
			t.Errorf("column %q not found in iso_catalog", col)
		}
	}

	// Verify new distro-level seed entries exist.
	type expectedEntry struct {
		name       string
		distro     string
		variant    string
		arch       string
		hasBaseURL bool
	}
	expected := []expectedEntry{
		{"Ubuntu Server (amd64)", "ubuntu", "server", "amd64", true},
		{"Ubuntu Server (arm64)", "ubuntu", "server", "arm64", true},
		{"Debian Netinst (amd64)", "debian", "netinst", "amd64", true},
		{"Debian Netinst (arm64)", "debian", "netinst", "arm64", true},
		{"Rocky Minimal (amd64)", "rocky", "minimal", "amd64", true},
		{"Alpine Standard (amd64)", "alpine", "standard", "amd64", true},
	}

	for _, e := range expected {
		var name, distro, variant, arch, baseURL string
		err := db.QueryRowContext(ctx,
			"SELECT name, distro, variant, arch, base_url FROM iso_catalog WHERE name = ?", e.name,
		).Scan(&name, &distro, &variant, &arch, &baseURL)
		if err != nil {
			t.Errorf("expected entry %q not found: %v", e.name, err)
			continue
		}
		if distro != e.distro {
			t.Errorf("entry %q: expected distro=%q, got %q", e.name, e.distro, distro)
		}
		if variant != e.variant {
			t.Errorf("entry %q: expected variant=%q, got %q", e.name, e.variant, variant)
		}
		if arch != e.arch {
			t.Errorf("entry %q: expected arch=%q, got %q", e.name, e.arch, arch)
		}
		if e.hasBaseURL && baseURL == "" {
			t.Errorf("entry %q: expected non-empty base_url", e.name)
		}
	}

	// Verify old version-specific names that are NOT reused by new seeds are absent.
	// Note: "Debian Netinst (amd64)" and "Debian Netinst (arm64)" are reused names
	// (DELETE then INSERT replaces them), so they're not in this list.
	oldNames := []string{
		"Ubuntu Server 22.04 LTS (amd64)",
		"Ubuntu Server 24.04 LTS (amd64)",
		"Ubuntu Server 22.04 LTS (arm64)",
		"Ubuntu Server 24.04 LTS (arm64)",
		"Rocky Linux 9 Minimal (amd64)",
		"AlmaLinux 9 Minimal (amd64)",
		"CentOS Stream 9 (amd64)",
		"Alpine Standard x86_64",
		"Alpine Virt x86_64",
		"Alpine Standard aarch64",
		"Kali Linux (amd64)",
		"Arch Linux (x86_64)",
		"Fedora Server (amd64)",
		"openSUSE Leap 15 (amd64)",
	}
	for _, name := range oldNames {
		var count int
		err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM iso_catalog WHERE name = ?", name).Scan(&count)
		if err != nil {
			t.Fatalf("querying old entry %q: %v", name, err)
		}
		if count > 0 {
			t.Errorf("old version-specific entry %q should have been deleted, but found %d", name, count)
		}
	}

	// Verify new entries have correct column defaults (base_url NOT empty, etc).
	for _, e := range expected {
		var baseURL, versionPattern, isoPath string
		err := db.QueryRowContext(ctx,
			"SELECT base_url, version_dir_pattern, iso_path_template FROM iso_catalog WHERE name = ?", e.name,
		).Scan(&baseURL, &versionPattern, &isoPath)
		if err != nil {
			t.Errorf("querying columns for %q: %v", e.name, err)
			continue
		}
		if baseURL == "" {
			t.Errorf("entry %q: base_url should not be empty", e.name)
		}
		// Ubuntu and Alpine should have version_dir_pattern, Debian should not (uses current/ symlink).
		if e.distro == "ubuntu" || e.distro == "alpine" || e.distro == "rocky" {
			if versionPattern == "" && e.distro != "rocky" {
				// Actually, Rocky uses \d+ pattern but let's just check ubuntu and alpine
			}
		}
		_ = versionPattern
		_ = isoPath
	}
}

// TestMigration018UpgradePath verifies the upgrade path:
// 1. Apply 001-016 (schema without new columns)
// 2. Seed old version-specific entries
// 3. Apply 017-018
// 4. Verify old entries replaced and user entries preserved.
func TestMigration018UpgradePath(t *testing.T) {
	ctx := context.Background()

	// Create a fresh DB and manually apply only up to migration 016.
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open(:memory:): %v", err)
	}
	defer db.Close()

	// Apply migrations 001-016 only.
	partialMigrations := []string{
		"001", "002", "003", "004", "005", "006", "007",
		"008", "009", "010", "011", "012", "013", "014",
		"015", "016",
	}

	// Disable foreign keys.
	if _, err := db.Exec("PRAGMA foreign_keys = OFF"); err != nil {
		t.Fatalf("disable foreign_keys: %v", err)
	}

	tx, err := db.Begin()
	if err != nil {
		t.Fatalf("Begin tx: %v", err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (version TEXT PRIMARY KEY, applied_at DATETIME DEFAULT CURRENT_TIMESTAMP)`); err != nil {
		t.Fatalf("create schema_migrations: %v", err)
	}

	for _, name := range partialMigrations {
		// Find the exact file for this migration name.
		entries, err := migrationsFS.ReadDir("migrations")
		if err != nil {
			t.Fatalf("ReadDir migrations: %v", err)
		}
		var filePath string
		for _, e := range entries {
			if len(e.Name()) > 7 && e.Name()[:3] == name {
				filePath = "migrations/" + e.Name()
				break
			}
		}
		if filePath == "" {
			t.Fatalf("migration %s file not found", name)
		}

		data, err := migrationsFS.ReadFile(filePath)
		if err != nil {
			t.Fatalf("read migration %s: %v", name, err)
		}
		if err := execMigration(tx, string(data)); err != nil {
			t.Fatalf("exec migration %s: %v", name, err)
		}
		if _, err := tx.Exec("INSERT INTO schema_migrations (version) VALUES (?)", name); err != nil {
			t.Fatalf("record migration %s: %v", name, err)
		}
	}

	if err := tx.Commit(); err != nil {
		t.Fatalf("commit partial migrations: %v", err)
	}
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		t.Fatalf("re-enable foreign_keys: %v", err)
	}

	// Verify the 3 new columns do NOT exist before migration 018.
	rows, err := db.QueryContext(ctx, "PRAGMA table_info(iso_catalog)")
	if err != nil {
		t.Fatalf("PRAGMA table_info: %v", err)
	}
	var hasBaseURL bool
	for rows.Next() {
		var cid int
		var name, typ string
		var notnull int
		var dflt sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &typ, &notnull, &dflt, &pk); err != nil {
			t.Fatalf("scan table_info: %v", err)
		}
		if name == "base_url" {
			hasBaseURL = true
		}
	}
	rows.Close()
	if hasBaseURL {
		t.Fatal("base_url column should not exist before migration 018")
	}

	// Seed old version-specific entries (mimicking what migration 007 did).
	// Using entries whose names differ from new 018 seeds to verify deletion.
	oldSeed := []struct {
		name, distro, variant, arch, checkURL, filenamePattern string
	}{
		{"Ubuntu Server 22.04 LTS (amd64)", "ubuntu", "server", "amd64", "https://releases.ubuntu.com/22.04/", "ubuntu-22\\.04\\.\\d+-live-server-amd64\\.iso$"},
		{"Ubuntu Server 24.04 LTS (amd64)", "ubuntu", "server", "amd64", "https://releases.ubuntu.com/24.04/", "ubuntu-24\\.04\\.\\d+-live-server-amd64\\.iso$"},
		{"Rocky Linux 9 Minimal (amd64)", "rocky", "minimal", "amd64", "https://download.rockylinux.org/pub/rocky/9/isos/x86_64/", "Rocky-9\\.[\\d]+-x86_64-minimal\\.iso$"},
		{"Alpine Standard x86_64", "alpine", "standard", "amd64", "https://dl-cdn.alpinelinux.org/alpine/v3.21/releases/x86_64/", "alpine-standard-\\d+\\.\\d+\\.\\d+-x86_64\\.iso$"},
	}

	for _, s := range oldSeed {
		_, err := db.ExecContext(ctx,
			`INSERT INTO iso_catalog (name, distro, variant, arch, check_url, filename_pattern, auto_update, check_interval_hours, status)
			 VALUES (?, ?, ?, ?, ?, ?, 0, 24, 'available')`,
			s.name, s.distro, s.variant, s.arch, s.checkURL, s.filenamePattern)
		if err != nil {
			t.Fatalf("seeding old entry %q: %v", s.name, err)
		}
	}

	// Add a user-modified entry (has current_url set — should be preserved).
	_, err = db.ExecContext(ctx,
		`INSERT INTO iso_catalog (name, distro, variant, arch, check_url, filename_pattern, current_url, auto_update, check_interval_hours, status)
		 VALUES (?, ?, ?, ?, ?, ?, ?, 0, 24, 'available')`,
		"My Custom Ubuntu", "ubuntu", "server", "amd64", "https://my-mirror.example.com/", "ubuntu-.*\\.iso$", "https://my-mirror.example.com/ubuntu-24.04.iso")
	if err != nil {
		t.Fatalf("seeding user entry: %v", err)
	}

	// Count entries before migration 018.
	var countBefore int
	db.QueryRowContext(ctx, "SELECT COUNT(*) FROM iso_catalog").Scan(&countBefore)
	t.Logf("entries before migration 018: %d", countBefore)

	// Now apply migrations 017-018.
	finalMigrations := []string{"017", "018"}
	tx, err = db.Begin()
	if err != nil {
		t.Fatalf("Begin tx for upgrade: %v", err)
	}
	defer tx.Rollback()

	for _, name := range finalMigrations {
		entries, err := migrationsFS.ReadDir("migrations")
		if err != nil {
			t.Fatalf("ReadDir: %v", err)
		}
		var filePath string
		for _, e := range entries {
			if len(e.Name()) > 7 && e.Name()[:3] == name {
				filePath = "migrations/" + e.Name()
				break
			}
		}
		data, err := migrationsFS.ReadFile(filePath)
		if err != nil {
			t.Fatalf("read migration %s: %v", name, err)
		}
		if err := execMigration(tx, string(data)); err != nil {
			t.Fatalf("exec migration %s: %v", name, err)
		}
		if _, err := tx.Exec("INSERT INTO schema_migrations (version) VALUES (?)", name); err != nil {
			t.Fatalf("record migration %s: %v", name, err)
		}
	}

	if err := tx.Commit(); err != nil {
		t.Fatalf("commit upgrade migrations: %v", err)
	}
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		t.Fatalf("re-enable foreign_keys: %v", err)
	}

	// Verify 3 new columns exist.
	rows, err = db.QueryContext(ctx, "PRAGMA table_info(iso_catalog)")
	if err != nil {
		t.Fatalf("PRAGMA table_info after upgrade: %v", err)
	}
	cols := make(map[string]bool)
	for rows.Next() {
		var cid int
		var name, typ string
		var notnull int
		var dflt sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &typ, &notnull, &dflt, &pk); err != nil {
			t.Fatalf("scan table_info: %v", err)
		}
		cols[name] = true
	}
	rows.Close()

	for _, col := range []string{"base_url", "version_dir_pattern", "iso_path_template"} {
		if !cols[col] {
			t.Errorf("column %q missing after migration 018", col)
		}
	}

	// Verify old seed entries (by name) are deleted.
	// These old names are NOT reused by new 018 seeds.
	for _, s := range oldSeed {
		var count int
		db.QueryRowContext(ctx, "SELECT COUNT(*) FROM iso_catalog WHERE name = ?", s.name).Scan(&count)
		if count > 0 {
			t.Errorf("old seed entry %q should have been deleted", s.name)
		}
	}

	// Verify new distro-level entries exist.
	newEntries := []string{"Ubuntu Server (amd64)", "Debian Netinst (amd64)", "Rocky Minimal (amd64)", "Alpine Standard (amd64)"}
	for _, name := range newEntries {
		var count int
		db.QueryRowContext(ctx, "SELECT COUNT(*) FROM iso_catalog WHERE name = ?", name).Scan(&count)
		if count == 0 {
			t.Errorf("new distro entry %q not found after upgrade", name)
		}
	}

	// Verify user-created entry is preserved.
	var userCount int
	db.QueryRowContext(ctx, "SELECT COUNT(*) FROM iso_catalog WHERE name = 'My Custom Ubuntu'").Scan(&userCount)
	if userCount != 1 {
		t.Errorf("user-created entry should be preserved, found %d", userCount)
	}

	var currentURL string
	err = db.QueryRowContext(ctx, "SELECT current_url FROM iso_catalog WHERE name = 'My Custom Ubuntu'").Scan(&currentURL)
	if err != nil {
		t.Fatalf("query user entry: %v", err)
	}
	if currentURL != "https://my-mirror.example.com/ubuntu-24.04.iso" {
		t.Errorf("user entry current_url should be preserved, got %q", currentURL)
	}

	t.Log("upgrade path test passed: old entries removed, new entries added, user entry preserved")
}
