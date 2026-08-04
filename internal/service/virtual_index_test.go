package service

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"testing"

	"github.com/Mi-Bee-Studio/mibeehive/internal/db"
	"github.com/Mi-Bee-Studio/mibeehive/internal/eventbus"

	_ "modernc.org/sqlite"
)

// setupVirtualIndexTest creates an in-memory DB with the virtual index tables
// (channels, virtual_views, virtual_nodes), a fresh event bus, and a service
// wired to both.
func setupVirtualIndexTest(t *testing.T) (*sql.DB, *eventbus.Bus, *VirtualIndexService) {
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
	}
	for _, s := range stmts {
		if _, err := testDB.Exec(s); err != nil {
			t.Fatalf("creating table: %v", err)
		}
	}

	bus := eventbus.NewBus(10)
	t.Cleanup(bus.Close)
	repo := db.NewVirtualRepo(testDB)
	svc := NewVirtualIndexService(repo, nil, bus, slog.Default())
	return testDB, bus, svc
}

func seedChannel(t *testing.T, svc *VirtualIndexService, slug string) *db.Channel {
	t.Helper()
	c, err := svc.CreateChannel(context.Background(), &db.Channel{Slug: slug, Name: slug, AuthMode: "anonymous"})
	if err != nil {
		t.Fatalf("creating channel %q: %v", slug, err)
	}
	return c
}

func seedView(t *testing.T, svc *VirtualIndexService, channelID int64, slug string) *db.View {
	t.Helper()
	v, err := svc.CreateView(context.Background(), &db.View{Slug: slug, Name: slug, ChannelID: channelID, Mode: "curated"})
	if err != nil {
		t.Fatalf("creating view %q: %v", slug, err)
	}
	return v
}

func seedNode(t *testing.T, svc *VirtualIndexService, viewID int64, parentID *int64, name, nodeType string) *db.Node {
	t.Helper()
	n, err := svc.CreateNode(context.Background(), &db.Node{
		ViewID: viewID, ParentID: parentID, Name: name, NodeType: nodeType, Status: "visible",
	})
	if err != nil {
		t.Fatalf("creating node %q: %v", name, err)
	}
	return n
}

func drainVirtualEvents(ch <-chan eventbus.Event) int {
	n := 0
	for {
		select {
		case <-ch:
			n++
		default:
			return n
		}
	}
}

func nodeStatus(t *testing.T, db *sql.DB, id int64) string {
	t.Helper()
	var status string
	if err := db.QueryRow("SELECT status FROM virtual_nodes WHERE id = ?", id).Scan(&status); err != nil {
		t.Fatalf("reading status of node %d: %v", id, err)
	}
	return status
}

func TestVirtualIndexCRUDLifecycle(t *testing.T) {
	_, _, svc := setupVirtualIndexTest(t)
	ctx := context.Background()

	// Channel lifecycle.
	ch := seedChannel(t, svc, "public")
	if ch.ID == 0 {
		t.Fatal("channel ID not assigned")
	}
	got, err := svc.GetChannel(ctx, ch.ID)
	if err != nil || got == nil {
		t.Fatalf("GetChannel: got=%v err=%v", got, err)
	}
	if got.Slug != "public" {
		t.Errorf("channel slug = %q, want public", got.Slug)
	}
	ch.Name = "Public Channel"
	if err := svc.UpdateChannel(ctx, ch); err != nil {
		t.Fatalf("UpdateChannel: %v", err)
	}
	got, _ = svc.GetChannel(ctx, ch.ID)
	if got.Name != "Public Channel" {
		t.Errorf("channel name = %q, want Public Channel", got.Name)
	}

	// View lifecycle.
	v := seedView(t, svc, ch.ID, "tools")
	if v.ID == 0 {
		t.Fatal("view id not assigned")
	}
	views, err := svc.ListViewsByChannel(ctx, ch.ID)
	if err != nil || len(views) != 1 {
		t.Fatalf("ListViewsByChannel: len=%d err=%v", len(views), err)
	}

	// Node lifecycle: root folder + child + file_ref.
	root := seedNode(t, svc, v.ID, nil, "root", "folder")
	child := seedNode(t, svc, v.ID, &root.ID, "child", "folder")
	fileRef := seedNode(t, svc, v.ID, &child.ID, "tool.bin", "file_ref")

	nodes, err := svc.ListNodesByView(ctx, v.ID)
	if err != nil || len(nodes) != 3 {
		t.Fatalf("ListNodesByView: len=%d err=%v", len(nodes), err)
	}

	// Rename via UpdateNode.
	fileRef.Name = "renamed.bin"
	if err := svc.UpdateNode(ctx, fileRef); err != nil {
		t.Fatalf("UpdateNode: %v", err)
	}
	gotNode, _ := svc.GetNode(ctx, fileRef.ID)
	if gotNode.Name != "renamed.bin" {
		t.Errorf("node name = %q, want renamed.bin", gotNode.Name)
	}

	// Move child to root level (valid, no cycle).
	if err := svc.MoveNode(ctx, child.ID, nil); err != nil {
		t.Fatalf("MoveNode to root: %v", err)
	}
	roots, err := svc.ListNodesByParent(ctx, v.ID, nil)
	if err != nil || len(roots) != 2 {
		t.Fatalf("ListNodesByParent(root): len=%d err=%v", len(roots), err)
	}

	// Delete channel.
	if err := svc.DeleteChannel(ctx, ch.ID); err != nil {
		t.Fatalf("DeleteChannel: %v", err)
	}
	got, _ = svc.GetChannel(ctx, ch.ID)
	if got != nil {
		t.Errorf("channel still present after delete")
	}
}

func TestVirtualIndexCycleDetection(t *testing.T) {
	_, _, svc := setupVirtualIndexTest(t)
	ctx := context.Background()

	ch := seedChannel(t, svc, "public")
	v := seedView(t, svc, ch.ID, "tools")
	a := seedNode(t, svc, v.ID, nil, "A", "folder")
	b := seedNode(t, svc, v.ID, &a.ID, "B", "folder")

	// Moving A under B would create A→B→A.
	err := svc.MoveNode(ctx, a.ID, &b.ID)
	if !errors.Is(err, ErrCircularRef) {
		t.Fatalf("MoveNode(A under B) err = %v, want ErrCircularRef", err)
	}

	// The tree must be unchanged.
	parent, err := svc.repo.GetNodeParent(ctx, a.ID)
	if err != nil {
		t.Fatalf("GetNodeParent(A): %v", err)
	}
	if parent != nil {
		t.Errorf("A parent = %v, want nil (unchanged)", *parent)
	}
}

func TestVirtualIndexSoftDeleteCascade(t *testing.T) {
	db, _, svc := setupVirtualIndexTest(t)
	ctx := context.Background()

	ch := seedChannel(t, svc, "public")
	v := seedView(t, svc, ch.ID, "tools")
	root := seedNode(t, svc, v.ID, nil, "root", "folder")
	child := seedNode(t, svc, v.ID, &root.ID, "child", "folder")
	grandchild := seedNode(t, svc, v.ID, &child.ID, "grandchild", "folder")

	if err := svc.SoftDeleteNode(ctx, root.ID); err != nil {
		t.Fatalf("SoftDeleteNode: %v", err)
	}

	for _, id := range []int64{root.ID, child.ID, grandchild.ID} {
		if got := nodeStatus(t, db, id); got != "hidden" {
			t.Errorf("node %d status = %q, want hidden", id, got)
		}
	}
}

func TestVirtualIndexSoftDeleteIsolated(t *testing.T) {
	db, _, svc := setupVirtualIndexTest(t)
	ctx := context.Background()

	ch := seedChannel(t, svc, "public")
	v := seedView(t, svc, ch.ID, "tools")
	root := seedNode(t, svc, v.ID, nil, "root", "folder")
	sibling := seedNode(t, svc, v.ID, nil, "sibling", "folder")

	if err := svc.SoftDeleteNode(ctx, root.ID); err != nil {
		t.Fatalf("SoftDeleteNode: %v", err)
	}
	if got := nodeStatus(t, db, sibling.ID); got != "visible" {
		t.Errorf("sibling status = %q, want visible (not cascaded)", got)
	}
}

func TestVirtualIndexEvents(t *testing.T) {
	_, bus, svc := setupVirtualIndexTest(t)
	ctx := context.Background()

	chEvents := bus.Subscribe(eventbus.TagChannelChanged)
	treeEvents := bus.Subscribe(eventbus.TagNodeTreeChanged)
	ruleEvents := bus.Subscribe(eventbus.TagRuleFolderCreated)

	// Channel create → ChannelChanged.
	ch := seedChannel(t, svc, "public")
	if got := drainVirtualEvents(chEvents); got != 1 {
		t.Errorf("ChannelChanged events = %d, want 1", got)
	}

	// Channel update → ChannelChanged.
	ch.Name = "Renamed"
	if err := svc.UpdateChannel(ctx, ch); err != nil {
		t.Fatalf("UpdateChannel: %v", err)
	}
	if got := drainVirtualEvents(chEvents); got != 1 {
		t.Errorf("ChannelChanged events after update = %d, want 1", got)
	}

	v := seedView(t, svc, ch.ID, "tools")

	// Folder create → NodeTreeChanged only.
	root := seedNode(t, svc, v.ID, nil, "root", "folder")
	if got := drainVirtualEvents(treeEvents); got != 1 {
		t.Errorf("NodeTreeChanged after folder create = %d, want 1", got)
	}
	if got := drainVirtualEvents(ruleEvents); got != 0 {
		t.Errorf("RuleFolderCreated after folder create = %d, want 0", got)
	}

	// Rule folder create → NodeTreeChanged + RuleFolderCreated.
	rule := seedNode(t, svc, v.ID, &root.ID, "rules", "rule_folder")
	if got := drainVirtualEvents(treeEvents); got != 1 {
		t.Errorf("NodeTreeChanged after rule create = %d, want 1", got)
	}
	if got := drainVirtualEvents(ruleEvents); got != 1 {
		t.Errorf("RuleFolderCreated after rule create = %d, want 1", got)
	}

	// Move → NodeTreeChanged.
	if err := svc.MoveNode(ctx, rule.ID, nil); err != nil {
		t.Fatalf("MoveNode: %v", err)
	}
	if got := drainVirtualEvents(treeEvents); got != 1 {
		t.Errorf("NodeTreeChanged after move = %d, want 1", got)
	}

	// Soft-delete → NodeTreeChanged.
	if err := svc.SoftDeleteNode(ctx, root.ID); err != nil {
		t.Fatalf("SoftDeleteNode: %v", err)
	}
	if got := drainVirtualEvents(treeEvents); got != 1 {
		t.Errorf("NodeTreeChanged after soft-delete = %d, want 1", got)
	}
}