package service

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"strings"
	"testing"

	"github.com/Mi-Bee-Studio/mibeehive/internal/cache"
	"github.com/Mi-Bee-Studio/mibeehive/internal/db"
	"github.com/Mi-Bee-Studio/mibeehive/internal/eventbus"

	_ "modernc.org/sqlite"
)

// setupVPathTest creates an in-memory DB with the virtual index tables plus
// the materialized virtual_node_paths table, and returns the raw DB, the
// virtual index service (for mutating the tree), and the vpath service.
func setupVPathTest(t *testing.T) (*sql.DB, *VirtualIndexService, *VPathIndexService) {
	t.Helper()
	testDB, err := sql.Open("sqlite", ":memory:?_pragma=busy_timeout(5000)")
	if err != nil {
		t.Fatalf("opening test db: %v", err)
	}
	t.Cleanup(func() { testDB.Close() })
	testDB.SetMaxOpenConns(1)

	stmts := []string{
		`CREATE TABLE channels (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			slug TEXT NOT NULL UNIQUE,
			name TEXT NOT NULL,
			auth_mode TEXT NOT NULL DEFAULT 'anonymous',
			description TEXT DEFAULT '',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE virtual_views (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			slug TEXT NOT NULL UNIQUE,
			name TEXT NOT NULL,
			channel_id INTEGER NOT NULL,
			mode TEXT NOT NULL DEFAULT 'curated',
			writable BOOLEAN NOT NULL DEFAULT 0,
			sort_order INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
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
		`CREATE TABLE virtual_node_paths (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			view_id INTEGER NOT NULL,
			node_id INTEGER NOT NULL,
			full_path TEXT NOT NULL UNIQUE,
			UNIQUE(view_id, node_id)
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
	vsvc := NewVirtualIndexService(repo, nil, bus, slog.Default())
	vpath := NewVPathIndexService(testDB, testDB, slog.Default())

	cache.PathCache.Purge()
	return testDB, vsvc, vpath
}

// pathSegments returns the number of node-ID segments in a full_path (the
// segment after the leading view_id). The materialized table has no depth
// column, so depth is derived from the path.
func pathSegments(fullPath string) int {
	trimmed := strings.TrimPrefix(fullPath, "/")
	parts := strings.Split(trimmed, "/")
	// parts[0] is the view_id; the rest are node IDs.
	return len(parts) - 1
}

func TestResolvePath(t *testing.T) {
	_, vsvc, vpath := setupVPathTest(t)
	ctx := context.Background()

	ch := seedChannel(t, vsvc, "public")
	v := seedView(t, vsvc, ch.ID, "tools")
	root := seedNode(t, vsvc, v.ID, nil, "root", "folder")
	child := seedNode(t, vsvc, v.ID, &root.ID, "child", "folder")
	grand := seedNode(t, vsvc, v.ID, &child.ID, "grandchild", "folder")

	if err := vpath.UpdatePath(ctx, v.ID); err != nil {
		t.Fatalf("UpdatePath: %v", err)
	}

	path := fmt.Sprintf("/%d/%d/%d/%d", v.ID, root.ID, child.ID, grand.ID)
	got, err := vpath.ResolvePath(ctx, path)
	if err != nil {
		t.Fatalf("ResolvePath: %v", err)
	}
	if got != grand.ID {
		t.Errorf("ResolvePath(%q) = %d, want %d", path, got, grand.ID)
	}

	// Second call must be served from the cache.
	if _, ok := cache.PathCache.Get(path); !ok {
		t.Errorf("path %q not present in PathCache after first resolve", path)
	}
	got2, err := vpath.ResolvePath(ctx, path)
	if err != nil || got2 != grand.ID {
		t.Errorf("ResolvePath cached = %d err=%v, want %d", got2, err, grand.ID)
	}

	// Unknown path resolves to (0, nil).
	missing, err := vpath.ResolvePath(ctx, "/999/888")
	if err != nil || missing != 0 {
		t.Errorf("ResolvePath(missing) = %d err=%v, want 0 nil", missing, err)
	}
}

func TestRenameStable(t *testing.T) {
	_, vsvc, vpath := setupVPathTest(t)
	ctx := context.Background()

	ch := seedChannel(t, vsvc, "public")
	v := seedView(t, vsvc, ch.ID, "tools")
	root := seedNode(t, vsvc, v.ID, nil, "root", "folder")
	child := seedNode(t, vsvc, v.ID, &root.ID, "child", "folder")

	if err := vpath.UpdatePath(ctx, v.ID); err != nil {
		t.Fatalf("UpdatePath: %v", err)
	}
	path := fmt.Sprintf("/%d/%d/%d", v.ID, root.ID, child.ID)

	// Rename the child. Because paths use node IDs, the full_path is unchanged.
	child.Name = "renamed"
	if err := vsvc.UpdateNode(ctx, child); err != nil {
		t.Fatalf("UpdateNode: %v", err)
	}
	if err := vpath.UpdatePath(ctx, v.ID); err != nil {
		t.Fatalf("UpdatePath after rename: %v", err)
	}

	got, err := vpath.ResolvePath(ctx, path)
	if err != nil || got != child.ID {
		t.Errorf("ResolvePath after rename = %d err=%v, want %d (path stable by ID)", got, err, child.ID)
	}
}

func TestIncrementalUpdate(t *testing.T) {
	db, vsvc, vpath := setupVPathTest(t)
	ctx := context.Background()

	ch := seedChannel(t, vsvc, "public")
	viewA := seedView(t, vsvc, ch.ID, "view-a")
	viewB := seedView(t, vsvc, ch.ID, "view-b")

	// View A: root -> child.
	rootA := seedNode(t, vsvc, viewA.ID, nil, "rootA", "folder")
	childA := seedNode(t, vsvc, viewA.ID, &rootA.ID, "childA", "folder")
	// View B: an unrelated node.
	rootB := seedNode(t, vsvc, viewB.ID, nil, "rootB", "folder")

	if err := vpath.UpdatePath(ctx, viewA.ID); err != nil {
		t.Fatalf("UpdatePath(A): %v", err)
	}
	if err := vpath.UpdatePath(ctx, viewB.ID); err != nil {
		t.Fatalf("UpdatePath(B): %v", err)
	}

	// Snapshot view B's rows before the move.
	beforeB := countPathRows(t, db, viewB.ID)

	// Move childA to the root of view A.
	if err := vsvc.MoveNode(ctx, childA.ID, nil); err != nil {
		t.Fatalf("MoveNode: %v", err)
	}
	if err := vpath.UpdatePath(ctx, viewA.ID); err != nil {
		t.Fatalf("UpdatePath(A) after move: %v", err)
	}

	// childA now resolves at the root path of view A.
	newPath := fmt.Sprintf("/%d/%d", viewA.ID, childA.ID)
	got, err := vpath.ResolvePath(ctx, newPath)
	if err != nil || got != childA.ID {
		t.Errorf("ResolvePath(moved) = %d err=%v, want %d", got, err, childA.ID)
	}

	// The old nested path must no longer resolve.
	oldPath := fmt.Sprintf("/%d/%d/%d", viewA.ID, rootA.ID, childA.ID)
	if old, _ := vpath.ResolvePath(ctx, oldPath); old != 0 {
		t.Errorf("old path %q still resolves to %d, want 0", oldPath, old)
	}

	// View B must be untouched (incremental: only the affected view rebuilt).
	afterB := countPathRows(t, db, viewB.ID)
	if beforeB != afterB {
		t.Errorf("view B path rows changed: before=%d after=%d (should be untouched)", beforeB, afterB)
	}
	bPath := fmt.Sprintf("/%d/%d", viewB.ID, rootB.ID)
	if id, _ := vpath.ResolvePath(ctx, bPath); id != rootB.ID {
		t.Errorf("view B path %q = %d, want %d", bPath, id, rootB.ID)
	}
}

func TestDepthBoundary(t *testing.T) {
	_, vsvc, vpath := setupVPathTest(t)
	ctx := context.Background()

	ch := seedChannel(t, vsvc, "public")
	v := seedView(t, vsvc, ch.ID, "tools")

	// Build a 5-level deep tree.
	ids := make([]int64, 0, 5)
	var parent *int64
	for i := 0; i < 5; i++ {
		n := seedNode(t, vsvc, v.ID, parent, fmt.Sprintf("level-%d", i), "folder")
		ids = append(ids, n.ID)
		parent = &n.ID
	}

	if err := vpath.UpdatePath(ctx, v.ID); err != nil {
		t.Fatalf("UpdatePath: %v", err)
	}

	// Deepest node's path must have 5 node-ID segments.
	deepPath := fmt.Sprintf("/%d", v.ID)
	for _, id := range ids {
		deepPath += fmt.Sprintf("/%d", id)
	}
	got, err := vpath.ResolvePath(ctx, deepPath)
	if err != nil || got != ids[4] {
		t.Errorf("ResolvePath(deep) = %d err=%v, want %d", got, err, ids[4])
	}
	if depth := pathSegments(deepPath); depth != 5 {
		t.Errorf("deep path depth = %d, want 5", depth)
	}

	// A mid-level node resolves to its own path with the correct depth.
	midPath := fmt.Sprintf("/%d/%d/%d", v.ID, ids[0], ids[1])
	if id, err := vpath.ResolvePath(ctx, midPath); err != nil || id != ids[1] {
		t.Errorf("ResolvePath(mid) = %d err=%v, want %d", id, err, ids[1])
	}
	if depth := pathSegments(midPath); depth != 2 {
		t.Errorf("mid path depth = %d, want 2", depth)
	}
}

func countPathRows(t *testing.T, db *sql.DB, viewID int64) int {
	t.Helper()
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM virtual_node_paths WHERE view_id = ?`, viewID).Scan(&n); err != nil {
		t.Fatalf("counting path rows for view %d: %v", viewID, err)
	}
	return n
}