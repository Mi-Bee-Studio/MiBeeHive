package service

import (
	"context"
	"database/sql"
	"log/slog"
	"testing"
	"time"

	"github.com/Mi-Bee-Studio/mibeehive/internal/db"
	"github.com/Mi-Bee-Studio/mibeehive/internal/eventbus"

	_ "modernc.org/sqlite"
)

// setupRuleWorkerTest creates an in-memory DB with the files, virtual_nodes,
// and virtual_rule_entries tables, a fresh event bus, and a RuleWorker wired
// to both.
func setupRuleWorkerTest(t *testing.T) (*sql.DB, *eventbus.Bus, *RuleWorker) {
	t.Helper()
	testDB, err := sql.Open("sqlite", ":memory:?_pragma=busy_timeout(5000)")
	if err != nil {
		t.Fatalf("opening test db: %v", err)
	}
	t.Cleanup(func() { testDB.Close() })
	testDB.SetMaxOpenConns(1)

	stmts := []string{
		`CREATE TABLE files (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			project_id INTEGER DEFAULT 0,
			version TEXT DEFAULT '',
			filename TEXT NOT NULL,
			os TEXT DEFAULT '',
			arch TEXT DEFAULT '',
			ext TEXT DEFAULT '',
			size_bytes INTEGER DEFAULT 0,
			download_url TEXT DEFAULT '',
			local_path TEXT DEFAULT '',
			checksum TEXT DEFAULT '',
			status TEXT DEFAULT 'pending',
			error_message TEXT DEFAULT '',
			retry_count INTEGER DEFAULT 0,
			last_attempt_at DATETIME,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			source_type TEXT DEFAULT NULL,
			category TEXT DEFAULT NULL,
			storage_subdir TEXT DEFAULT NULL,
			public_token TEXT DEFAULT NULL
		)`,
		`CREATE TABLE virtual_nodes (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			view_id INTEGER NOT NULL,
			parent_id INTEGER,
			name TEXT NOT NULL,
			node_type TEXT NOT NULL DEFAULT 'folder',
			file_id INTEGER,
			rule_config TEXT,
			sort_order INTEGER NOT NULL DEFAULT 0,
			status TEXT NOT NULL DEFAULT 'visible',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE virtual_rule_entries (
			rule_node_id INTEGER NOT NULL,
			file_id INTEGER NOT NULL,
			PRIMARY KEY (rule_node_id, file_id)
		)`,
	}
	for _, s := range stmts {
		if _, err := testDB.Exec(s); err != nil {
			t.Fatalf("creating table: %v", err)
		}
	}

	bus := eventbus.NewBus(10)
	t.Cleanup(bus.Close)
	repo := db.NewVirtualRepo(testDB)
	w := NewRuleWorker(testDB, testDB, repo, bus, slog.Default())
	return testDB, bus, w
}

func insertRuleFolder(t *testing.T, db *sql.DB, ruleConfig string) int64 {
	t.Helper()
	res, err := db.Exec(`INSERT INTO virtual_nodes (view_id, name, node_type, rule_config, status)
		VALUES (1, 'rules', 'rule_folder', ?, 'visible')`, ruleConfig)
	if err != nil {
		t.Fatalf("inserting rule folder: %v", err)
	}
	id, _ := res.LastInsertId()
	return id
}

func insertRuleTestFile(t *testing.T, db *sql.DB, os, arch, sourceType string) int64 {
	t.Helper()
	res, err := db.Exec(`INSERT INTO files (project_id, filename, os, arch, status, source_type)
		VALUES (0, 'tool.bin', ?, ?, 'complete', ?)`, os, arch, sourceType)
	if err != nil {
		t.Fatalf("inserting file: %v", err)
	}
	id, _ := res.LastInsertId()
	return id
}

func countRuleEntries(t *testing.T, db *sql.DB, ruleNodeID, fileID int64) int {
	t.Helper()
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM virtual_rule_entries WHERE rule_node_id = ? AND file_id = ?`,
		ruleNodeID, fileID).Scan(&n); err != nil {
		t.Fatalf("counting rule entries: %v", err)
	}
	return n
}

func countRuleEntriesForNode(t *testing.T, db *sql.DB, ruleNodeID int64) int {
	t.Helper()
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM virtual_rule_entries WHERE rule_node_id = ?`, ruleNodeID).Scan(&n); err != nil {
		t.Fatalf("counting rule entries for node: %v", err)
	}
	return n
}

func waitForRuleEntry(t *testing.T, db *sql.DB, ruleNodeID, fileID int64) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if countRuleEntries(t, db, ruleNodeID, fileID) > 0 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("rule entry (%d,%d) not materialized within deadline", ruleNodeID, fileID)
}

func TestRuleMatch(t *testing.T) {
	cases := []struct {
		name   string
		config string
		file   fileMeta
		want   bool
	}{
		{"exact os arch", `{"os":"linux","arch":"arm64"}`, fileMeta{OS: "linux", Arch: "arm64"}, true},
		{"os only", `{"os":"linux"}`, fileMeta{OS: "linux", Arch: "x86_64"}, true},
		{"mismatch os", `{"os":"windows"}`, fileMeta{OS: "linux", Arch: "arm64"}, false},
		{"mismatch arch", `{"os":"linux","arch":"amd64"}`, fileMeta{OS: "linux", Arch: "arm64"}, false},
		{"source_type", `{"source_type":"github"}`, fileMeta{SourceType: "github"}, true},
		{"source_type mismatch", `{"source_type":"github"}`, fileMeta{SourceType: "npm"}, false},
		{"category", `{"category":"ops"}`, fileMeta{Category: "ops"}, true},
		{"empty rule matches all", `{}`, fileMeta{OS: "linux"}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m, err := parseRuleConfig(tc.config)
			if err != nil {
				t.Fatalf("parseRuleConfig: %v", err)
			}
			if got := m.matches(tc.file); got != tc.want {
				t.Errorf("matches(%+v) = %v, want %v", tc.file, got, tc.want)
			}
		})
	}
}

func TestMaterializeOnPublish(t *testing.T) {
	db, _, w := setupRuleWorkerTest(t)
	ctx := context.Background()
	ruleID := insertRuleFolder(t, db, `{"os":"linux","arch":"arm64"}`)
	fileID := insertRuleTestFile(t, db, "linux", "arm64", "github")

	w.handleFilePublished(ctx, fileID)
	if got := countRuleEntries(t, db, ruleID, fileID); got != 1 {
		t.Errorf("rule entries = %d, want 1", got)
	}
}

func TestMaterializeOnPublishNoMatch(t *testing.T) {
	db, _, w := setupRuleWorkerTest(t)
	ctx := context.Background()
	ruleID := insertRuleFolder(t, db, `{"os":"linux"}`)
	fileID := insertRuleTestFile(t, db, "windows", "amd64", "github")

	w.handleFilePublished(ctx, fileID)
	if got := countRuleEntries(t, db, ruleID, fileID); got != 0 {
		t.Errorf("rule entries = %d, want 0", got)
	}
}

func TestRemoveOnDelete(t *testing.T) {
	db, _, w := setupRuleWorkerTest(t)
	ctx := context.Background()
	ruleID := insertRuleFolder(t, db, `{"os":"linux"}`)
	fileID := insertRuleTestFile(t, db, "linux", "arm64", "github")

	w.handleFilePublished(ctx, fileID)
	if got := countRuleEntries(t, db, ruleID, fileID); got != 1 {
		t.Fatalf("initial entries = %d, want 1", got)
	}
	w.handleFileRemoved(ctx, fileID)
	if got := countRuleEntries(t, db, ruleID, fileID); got != 0 {
		t.Errorf("after remove entries = %d, want 0", got)
	}
}

func TestMetadataChangedRematch(t *testing.T) {
	db, _, w := setupRuleWorkerTest(t)
	ctx := context.Background()
	ruleID := insertRuleFolder(t, db, `{"os":"linux"}`)
	fileID := insertRuleTestFile(t, db, "windows", "amd64", "github")

	w.handleFilePublished(ctx, fileID)
	if got := countRuleEntries(t, db, ruleID, fileID); got != 0 {
		t.Fatalf("initial entries = %d, want 0", got)
	}
	if _, err := db.Exec(`UPDATE files SET os = 'linux' WHERE id = ?`, fileID); err != nil {
		t.Fatalf("updating file os: %v", err)
	}
	w.handleFileMetadataChanged(ctx, fileID)
	if got := countRuleEntries(t, db, ruleID, fileID); got != 1 {
		t.Errorf("after metadata change entries = %d, want 1", got)
	}
}

func TestBackfillCrashRecovery(t *testing.T) {
	db, _, w := setupRuleWorkerTest(t)
	ctx := context.Background()
	ruleID := insertRuleFolder(t, db, `{"os":"linux"}`)
	insertRuleTestFile(t, db, "linux", "arm64", "github")
	insertRuleTestFile(t, db, "linux", "amd64", "github")
	insertRuleTestFile(t, db, "windows", "amd64", "github") // no match

	if err := w.Backfill(ctx, ruleID); err != nil {
		t.Fatalf("first backfill: %v", err)
	}
	if got := countRuleEntriesForNode(t, db, ruleID); got != 2 {
		t.Fatalf("after first backfill entries = %d, want 2", got)
	}
	// Re-run (simulating crash recovery) — must not duplicate.
	if err := w.Backfill(ctx, ruleID); err != nil {
		t.Fatalf("second backfill: %v", err)
	}
	if got := countRuleEntriesForNode(t, db, ruleID); got != 2 {
		t.Errorf("after re-run entries = %d, want 2 (no duplicates)", got)
	}
}

func TestStartEventDriven(t *testing.T) {
	db, bus, w := setupRuleWorkerTest(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	w.Start(ctx)

	ruleID := insertRuleFolder(t, db, `{"os":"linux"}`)
	fileID := insertRuleTestFile(t, db, "linux", "arm64", "github")
	bus.Publish(ctx, eventbus.FilePublished{FileID: fileID})
	waitForRuleEntry(t, db, ruleID, fileID)
}
