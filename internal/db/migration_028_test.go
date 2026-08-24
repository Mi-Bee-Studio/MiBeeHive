package db

import (
	"context"
	"testing"
)

// TestMigration028FixHashiCorpOwner verifies the config repair for hashicorp
// projects seeded without github_owner: the project name (which IS the
// product name) is written into the config JSON (issue #60).
func TestMigration028FixHashiCorpOwner(t *testing.T) {
	db := testDB(t) // applies all migrations including 028 on a fresh in-memory DB
	ctx := context.Background()

	// Legacy broken shape: hashicorp project, no github_owner in config.
	if _, err := db.ExecContext(ctx,
		`INSERT INTO projects (name, display_name, source_type, source_url, config)
		 VALUES ('consul', 'Consul', 'hashicorp', 'https://releases.hashicorp.com/consul/', '{"crawl_interval":360}')`); err != nil {
		t.Fatalf("seed project: %v", err)
	}

	// Re-run the migration's repair statement (idempotent).
	if _, err := db.ExecContext(ctx,
		`UPDATE projects
		 SET config = json_set(config, '$.github_owner', name)
		 WHERE source_type = 'hashicorp'
		   AND json_valid(config)
		   AND COALESCE(json_extract(config, '$.github_owner'), '') = ''`); err != nil {
		t.Fatalf("repair update: %v", err)
	}

	var owner string
	if err := db.QueryRowContext(ctx,
		`SELECT json_extract(config, '$.github_owner') FROM projects WHERE name = 'consul'`).Scan(&owner); err != nil {
		t.Fatalf("query owner: %v", err)
	}
	if owner != "consul" {
		t.Errorf("config github_owner = %q, want %q", owner, "consul")
	}
}

// TestMigration028FixVmagentRepo verifies the vmagent project is repointed
// from the nonexistent VictoriaMetrics/vmagent repo to the main repo, scoped
// to the vmutils-* archives that actually contain vmagent (issue #60).
func TestMigration028FixVmagentRepo(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()

	// Legacy broken shape: vmagent pointing at a nonexistent repo.
	if _, err := db.ExecContext(ctx,
		`INSERT INTO projects (name, display_name, source_type, source_url, config)
		 VALUES ('vmagent', 'VMAgent', 'github', 'https://github.com/VictoriaMetrics/vmagent',
		         '{"github_owner":"VictoriaMetrics","github_repo":"vmagent","crawl_interval":360}')`); err != nil {
		t.Fatalf("seed project: %v", err)
	}

	// Re-run the migration's repair statement (idempotent).
	if _, err := db.ExecContext(ctx,
		`UPDATE projects
		 SET config = json_set(config,
		         '$.github_repo', 'VictoriaMetrics',
		         '$.filter_patterns', json('["vmutils-*"]'))
		 WHERE name = 'vmagent'
		   AND source_type = 'github'
		   AND json_valid(config)
		   AND COALESCE(json_extract(config, '$.github_repo'), '') = 'vmagent'`); err != nil {
		t.Fatalf("repair update: %v", err)
	}

	var repoName, filter string
	if err := db.QueryRowContext(ctx,
		`SELECT json_extract(config, '$.github_repo'), json_extract(config, '$.filter_patterns')
		 FROM projects WHERE name = 'vmagent'`).Scan(&repoName, &filter); err != nil {
		t.Fatalf("query config: %v", err)
	}
	if repoName != "VictoriaMetrics" {
		t.Errorf("github_repo = %q, want %q", repoName, "VictoriaMetrics")
	}
	if filter != `["vmutils-*"]` {
		t.Errorf("filter_patterns = %q, want %q", filter, `["vmutils-*"]`)
	}
}
