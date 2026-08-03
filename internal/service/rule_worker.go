package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/Mi-Bee-Studio/mibeehive/internal/db"
	"github.com/Mi-Bee-Studio/mibeehive/internal/eventbus"
)

// ruleNodeColumns mirrors the virtual_nodes columns needed to scan a node.
const ruleNodeColumns = `id, view_id, parent_id, name, node_type, file_id, rule_config, sort_order, status, created_at`

// RuleWorker maintains the materialized virtual_rule_entries table by
// subscribing to file lifecycle events and matching files against rule_folder
// rule_config JSON. All processing runs in async goroutines so the write path
// is never blocked.
//
// Backfill is idempotent (INSERT OR IGNORE) and crash-recoverable: re-running
// it after an interruption produces no duplicates. A checkpoint marker records
// each rule folder's backfill state ('indexing' → 'ready').
type RuleWorker struct {
	writeDB *sql.DB
	readDB  *sql.DB
	repo    *db.VirtualRepo
	bus     *eventbus.Bus
	logger  *slog.Logger
}

// NewRuleWorker creates a RuleWorker. readDB is used for reads (files, rule
// folders); writeDB for all writes to virtual_rule_entries and the backfill
// checkpoint table.
func NewRuleWorker(readDB, writeDB *sql.DB, repo *db.VirtualRepo, bus *eventbus.Bus, logger *slog.Logger) *RuleWorker {
	return &RuleWorker{
		writeDB: writeDB,
		readDB:  readDB,
		repo:    repo,
		bus:     bus,
		logger:  logger,
	}
}

// fileMeta is the subset of file metadata used for rule matching.
type fileMeta struct {
	ID         int64
	OS         string
	Arch       string
	SourceType string
	Category   string
}

// ruleMatch is a parsed rule_config JSON object mapping metadata field names
// to expected values.
type ruleMatch map[string]string

// parseRuleConfig unmarshals a rule_config JSON string into a ruleMatch.
func parseRuleConfig(raw string) (ruleMatch, error) {
	var m ruleMatch
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return nil, fmt.Errorf("parsing rule_config: %w", err)
	}
	return m, nil
}

// matches reports whether the file satisfies every constraint in the rule. An
// empty rule matches every file. Unknown keys are ignored.
func (m ruleMatch) matches(f fileMeta) bool {
	for k, v := range m {
		switch k {
		case "os":
			if f.OS != v {
				return false
			}
		case "arch":
			if f.Arch != v {
				return false
			}
		case "source_type":
			if f.SourceType != v {
				return false
			}
		case "category":
			if f.Category != v {
				return false
			}
		}
	}
	return true
}

// Start subscribes to the file and rule-folder event tags and launches a
// goroutine per subscription. It returns immediately; workers run until ctx is
// cancelled or the bus is closed.
func (w *RuleWorker) Start(ctx context.Context) {
	if w.bus == nil {
		return
	}
	w.startSubscriber(ctx, eventbus.TagFilePublished, func(e eventbus.Event) {
		if ev, ok := e.(eventbus.FilePublished); ok {
			w.handleFilePublished(ctx, ev.FileID)
		}
	})
	w.startSubscriber(ctx, eventbus.TagFileRemoved, func(e eventbus.Event) {
		if ev, ok := e.(eventbus.FileRemoved); ok {
			w.handleFileRemoved(ctx, ev.FileID)
		}
	})
	w.startSubscriber(ctx, eventbus.TagFileMetadataChanged, func(e eventbus.Event) {
		if ev, ok := e.(eventbus.FileMetadataChanged); ok {
			w.handleFileMetadataChanged(ctx, ev.FileID)
		}
	})
	w.startSubscriber(ctx, eventbus.TagRuleFolderCreated, func(e eventbus.Event) {
		if ev, ok := e.(eventbus.RuleFolderCreated); ok {
			w.handleRuleFolderCreated(ctx, ev.NodeID)
		}
	})
}

// startSubscriber runs a single event-tag subscription in a goroutine.
func (w *RuleWorker) startSubscriber(ctx context.Context, tag string, handler func(eventbus.Event)) {
	ch := w.bus.Subscribe(tag)
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case e, ok := <-ch:
				if !ok {
					return
				}
				handler(e)
			}
		}
	}()
}

// handleFilePublished matches a newly published file against every visible rule
// folder and inserts the matching entries.
func (w *RuleWorker) handleFilePublished(ctx context.Context, fileID int64) {
	meta, err := w.getFileMeta(ctx, fileID)
	if err != nil {
		w.logger.Error("rule worker: reading file metadata failed", "file_id", fileID, "error", err)
		return
	}
	if meta == nil {
		return
	}
	folders, err := w.listRuleFolders(ctx)
	if err != nil {
		w.logger.Error("rule worker: listing rule folders failed", "error", err)
		return
	}
	for _, f := range folders {
		if f.RuleConfig == nil {
			continue
		}
		m, err := parseRuleConfig(*f.RuleConfig)
		if err != nil {
			w.logger.Warn("rule worker: skipping invalid rule_config", "node_id", f.ID, "error", err)
			continue
		}
		if m.matches(*meta) {
			if err := w.insertEntry(ctx, f.ID, fileID); err != nil {
				w.logger.Error("rule worker: inserting rule entry failed", "node_id", f.ID, "file_id", fileID, "error", err)
			}
		}
	}
}

// handleFileRemoved deletes all rule entries referencing a removed file.
func (w *RuleWorker) handleFileRemoved(ctx context.Context, fileID int64) {
	if _, err := w.writeDB.ExecContext(ctx,
		`DELETE FROM virtual_rule_entries WHERE file_id = ?`, fileID); err != nil {
		w.logger.Error("rule worker: deleting rule entries failed", "file_id", fileID, "error", err)
	}
}

// handleFileMetadataChanged re-evaluates a file against all rule folders after
// its metadata changed: existing entries are dropped, then the file is
// re-matched.
func (w *RuleWorker) handleFileMetadataChanged(ctx context.Context, fileID int64) {
	w.handleFileRemoved(ctx, fileID)
	w.handleFilePublished(ctx, fileID)
}

// handleRuleFolderCreated triggers a backfill for a newly created rule folder.
func (w *RuleWorker) handleRuleFolderCreated(ctx context.Context, nodeID int64) {
	if err := w.Backfill(ctx, nodeID); err != nil {
		w.logger.Error("rule worker: backfill failed", "node_id", nodeID, "error", err)
	}
}

// Backfill materializes all entries for a single rule folder. It is idempotent
// (INSERT OR IGNORE) and crash-recoverable: re-running after an interruption
// leaves no duplicates. A checkpoint marker records the folder's state.
func (w *RuleWorker) Backfill(ctx context.Context, nodeID int64) error {
	node, err := w.repo.GetNode(ctx, nodeID)
	if err != nil {
		return err
	}
	if node == nil {
		return fmt.Errorf("rule worker: node %d not found", nodeID)
	}
	if node.NodeType != "rule_folder" {
		return fmt.Errorf("rule worker: node %d is not a rule_folder", nodeID)
	}
	if node.RuleConfig == nil {
		return nil
	}
	m, err := parseRuleConfig(*node.RuleConfig)
	if err != nil {
		return fmt.Errorf("rule worker: node %d: %w", nodeID, err)
	}
	if err := w.setCheckpoint(ctx, nodeID, "indexing"); err != nil {
		return err
	}
	files, err := w.listActiveFiles(ctx)
	if err != nil {
		return err
	}
	for _, f := range files {
		if m.matches(*f) {
			if err := w.insertEntry(ctx, nodeID, f.ID); err != nil {
				return err
			}
		}
	}
	return w.setCheckpoint(ctx, nodeID, "ready")
}

// getFileMeta reads the metadata subset of a single file. Returns nil, nil if
// the file does not exist.
func (w *RuleWorker) getFileMeta(ctx context.Context, fileID int64) (*fileMeta, error) {
	var m fileMeta
	err := w.readDB.QueryRowContext(ctx,
		`SELECT id, os, arch, COALESCE(source_type, ''), COALESCE(category, '') FROM files WHERE id = ?`, fileID).
		Scan(&m.ID, &m.OS, &m.Arch, &m.SourceType, &m.Category)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading file %d metadata: %w", fileID, err)
	}
	return &m, nil
}

// listActiveFiles returns the metadata of all non-deleted files.
func (w *RuleWorker) listActiveFiles(ctx context.Context) ([]*fileMeta, error) {
	rows, err := w.readDB.QueryContext(ctx,
		`SELECT id, os, arch, COALESCE(source_type, ''), COALESCE(category, '') FROM files WHERE status != 'deleted'`)
	if err != nil {
		return nil, fmt.Errorf("listing active files: %w", err)
	}
	defer rows.Close()
	var files []*fileMeta
	for rows.Next() {
		var m fileMeta
		if err := rows.Scan(&m.ID, &m.OS, &m.Arch, &m.SourceType, &m.Category); err != nil {
			return nil, fmt.Errorf("scanning active file: %w", err)
		}
		files = append(files, &m)
	}
	return files, rows.Err()
}

// listRuleFolders returns all visible rule_folder nodes.
func (w *RuleWorker) listRuleFolders(ctx context.Context) ([]*db.Node, error) {
	rows, err := w.readDB.QueryContext(ctx,
		`SELECT `+ruleNodeColumns+` FROM virtual_nodes WHERE node_type = 'rule_folder' AND status = 'visible'`)
	if err != nil {
		return nil, fmt.Errorf("listing rule folders: %w", err)
	}
	defer rows.Close()
	var nodes []*db.Node
	for rows.Next() {
		n := &db.Node{}
		var parentID, fileID sql.NullInt64
		var ruleConfig sql.NullString
		if err := rows.Scan(&n.ID, &n.ViewID, &parentID, &n.Name, &n.NodeType, &fileID, &ruleConfig, &n.SortOrder, &n.Status, &n.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning rule folder: %w", err)
		}
		if parentID.Valid {
			p := parentID.Int64
			n.ParentID = &p
		}
		if fileID.Valid {
			f := fileID.Int64
			n.FileID = &f
		}
		if ruleConfig.Valid {
			rc := ruleConfig.String
			n.RuleConfig = &rc
		}
		nodes = append(nodes, n)
	}
	return nodes, rows.Err()
}

// insertEntry writes a rule entry, ignoring duplicates (idempotent).
func (w *RuleWorker) insertEntry(ctx context.Context, ruleNodeID, fileID int64) error {
	if _, err := w.writeDB.ExecContext(ctx,
		`INSERT OR IGNORE INTO virtual_rule_entries (rule_node_id, file_id) VALUES (?, ?)`,
		ruleNodeID, fileID); err != nil {
		return fmt.Errorf("inserting rule entry (%d,%d): %w", ruleNodeID, fileID, err)
	}
	return nil
}

// ensureCheckpointTable lazily creates the backfill checkpoint table.
func (w *RuleWorker) ensureCheckpointTable(ctx context.Context) error {
	if _, err := w.writeDB.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS rule_backfill_checkpoints (
		rule_node_id INTEGER PRIMARY KEY,
		state TEXT NOT NULL,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`); err != nil {
		return fmt.Errorf("creating rule_backfill_checkpoints: %w", err)
	}
	return nil
}

// setCheckpoint records the backfill state of a rule folder.
func (w *RuleWorker) setCheckpoint(ctx context.Context, nodeID int64, state string) error {
	if err := w.ensureCheckpointTable(ctx); err != nil {
		return err
	}
	if _, err := w.writeDB.ExecContext(ctx,
		`INSERT INTO rule_backfill_checkpoints (rule_node_id, state) VALUES (?, ?)
		 ON CONFLICT(rule_node_id) DO UPDATE SET state = excluded.state, updated_at = CURRENT_TIMESTAMP`,
		nodeID, state); err != nil {
		return fmt.Errorf("setting checkpoint for node %d: %w", nodeID, err)
	}
	return nil
}