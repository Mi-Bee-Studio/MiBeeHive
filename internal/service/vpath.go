package service

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"strconv"

	"github.com/Mi-Bee-Studio/mibeehive/internal/cache"
	"github.com/Mi-Bee-Studio/mibeehive/internal/db"
	"github.com/Mi-Bee-Studio/mibeehive/internal/eventbus"
)

// VPathIndexService maintains the materialized full_path index
// (virtual_node_paths) for the virtual directory trees. full_path values are
// composed from node IDs (never display names), so a rename does not change a
// node's path and lookups stay stable.
//
// Reads (ResolvePath and the tree traversal during an update) go through the
// readDB connection; writes to the materialized index go through writeDB.
type VPathIndexService struct {
	readRepo *db.VirtualRepo // backed by readDB for tree reads
	readDB   *sql.DB         // reads: ResolvePath + tree traversal
	writeDB  *sql.DB         // writes: INSERT/DELETE on virtual_node_paths
	logger   *slog.Logger
}

// NewVPathIndexService creates a VPathIndexService. readDB is used for all
// reads, writeDB for all writes to the materialized path index.
func NewVPathIndexService(readDB *sql.DB, writeDB *sql.DB, logger *slog.Logger) *VPathIndexService {
	return &VPathIndexService{
		readRepo: db.NewVirtualRepo(readDB),
		readDB:   readDB,
		writeDB:  writeDB,
		logger:   logger,
	}
}

// ResolvePath maps a full_path string to its node_id. It checks the PathCache
// first; on a miss it queries virtual_node_paths via readDB and populates the
// cache. Returns (0, nil) when the path is not present.
func (s *VPathIndexService) ResolvePath(ctx context.Context, fullPath string) (int64, error) {
	if nodeID, ok := cache.PathCache.Get(fullPath); ok {
		return nodeID, nil
	}

	var nodeID int64
	err := s.readDB.QueryRowContext(ctx,
		`SELECT node_id FROM virtual_node_paths WHERE full_path = ?`, fullPath).Scan(&nodeID)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("resolving path %q: %w", fullPath, err)
	}

	cache.PathCache.Put(fullPath, nodeID)
	return nodeID, nil
}

// UpdatePath incrementally refreshes the materialized paths for a single view.
// It deletes that view's rows and recomputes them by walking the tree from the
// roots, composing each full_path from ancestor node IDs. Only the affected
// view is touched; other views are left untouched.
func (s *VPathIndexService) UpdatePath(ctx context.Context, viewID int64) error {
	if _, err := s.writeDB.ExecContext(ctx,
		`DELETE FROM virtual_node_paths WHERE view_id = ?`, viewID); err != nil {
		return fmt.Errorf("clearing paths for view %d: %w", viewID, err)
	}

	roots, err := s.readRepo.ListNodesByParent(ctx, viewID, nil)
	if err != nil {
		return fmt.Errorf("listing roots of view %d: %w", viewID, err)
	}

	viewPrefix := "/" + strconv.FormatInt(viewID, 10)
	for _, root := range roots {
		if err := s.updateSubtree(ctx, viewID, root.ID, viewPrefix); err != nil {
			return err
		}
	}
	return nil
}

// updateSubtree writes the full_path for nodeID (composed by appending its ID
// to parentPath) and recursively updates all of its descendants.
func (s *VPathIndexService) updateSubtree(ctx context.Context, viewID, nodeID int64, parentPath string) error {
	fullPath := parentPath + "/" + strconv.FormatInt(nodeID, 10)
	if _, err := s.writeDB.ExecContext(ctx,
		`INSERT INTO virtual_node_paths (view_id, node_id, full_path) VALUES (?, ?, ?)`,
		viewID, nodeID, fullPath); err != nil {
		return fmt.Errorf("writing path for node %d: %w", nodeID, err)
	}

	children, err := s.readRepo.ListNodesByParent(ctx, viewID, &nodeID)
	if err != nil {
		return fmt.Errorf("listing children of node %d: %w", nodeID, err)
	}
	for _, child := range children {
		if err := s.updateSubtree(ctx, viewID, child.ID, fullPath); err != nil {
			return err
		}
	}
	return nil
}

// Run subscribes to NodeTreeChanged events and rebuilds the affected view's
// paths in the background. The cache layer (EventInvalidator) already purges
// PathCache on the same event, so this only needs to refresh the materialized
// index. It returns immediately; the worker runs until ctx is cancelled or the
// bus is closed.
func (s *VPathIndexService) Run(ctx context.Context, bus *eventbus.Bus) {
	if bus == nil {
		return
	}
	ch := bus.Subscribe(eventbus.TagNodeTreeChanged)
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case e, ok := <-ch:
				if !ok {
					return
				}
				ev, ok := e.(eventbus.NodeTreeChanged)
				if !ok {
					continue
				}
				if err := s.UpdatePath(ctx, ev.ViewID); err != nil {
					s.logger.Error("vpath: rebuilding paths failed",
						"view_id", ev.ViewID, "error", err)
					continue
				}
				s.logger.Info("vpath: rebuilt paths", "view_id", ev.ViewID)
			}
		}
	}()
}