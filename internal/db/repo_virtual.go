package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

const channelColumns = `id, slug, name, auth_mode, description, created_at`
const viewColumns = `id, slug, name, channel_id, mode, writable, sort_order, created_at`
const nodeColumns = `id, view_id, parent_id, name, node_type, file_id, rule_config, sort_order, status, created_at`

// VirtualRepo provides CRUD operations for the virtual index tables
// (channels → virtual_views → virtual_nodes). All writes go through the
// writeDB connection (MaxOpenConns=1) so they are serialized.
type VirtualRepo struct {
	db *sql.DB
}

// NewVirtualRepo creates a new VirtualRepo.
func NewVirtualRepo(db *sql.DB) *VirtualRepo {
	return &VirtualRepo{db: db}
}

// --- Channels ---

// CreateChannel inserts a new channel and returns its generated ID.
func (r *VirtualRepo) CreateChannel(ctx context.Context, c *Channel) (int64, error) {
	res, err := r.db.ExecContext(ctx,
		`INSERT INTO channels (slug, name, auth_mode, description) VALUES (?, ?, ?, ?)`,
		c.Slug, c.Name, c.AuthMode, c.Description)
	if err != nil {
		return 0, fmt.Errorf("creating channel %q: %w", c.Name, err)
	}
	return res.LastInsertId()
}

// GetChannel retrieves a channel by ID. Returns nil, nil if not found.
func (r *VirtualRepo) GetChannel(ctx context.Context, id int64) (*Channel, error) {
	row := r.db.QueryRowContext(ctx, "SELECT "+channelColumns+" FROM channels WHERE id = ?", id)
	c, err := scanChannel(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("getting channel %d: %w", id, err)
	}
	return c, nil
}

// GetChannelBySlug retrieves a channel by its unique slug. Returns nil, nil if not found.
func (r *VirtualRepo) GetChannelBySlug(ctx context.Context, slug string) (*Channel, error) {
	row := r.db.QueryRowContext(ctx, "SELECT "+channelColumns+" FROM channels WHERE slug = ?", slug)
	c, err := scanChannel(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("getting channel by slug %q: %w", slug, err)
	}
	return c, nil
}

// ListChannels returns all channels ordered by name.
func (r *VirtualRepo) ListChannels(ctx context.Context) ([]*Channel, error) {
	rows, err := r.db.QueryContext(ctx, "SELECT "+channelColumns+" FROM channels ORDER BY name")
	if err != nil {
		return nil, fmt.Errorf("listing channels: %w", err)
	}
	defer rows.Close()

	var channels []*Channel
	for rows.Next() {
		c, err := scanChannel(rows)
		if err != nil {
			return nil, err
		}
		channels = append(channels, c)
	}
	return channels, rows.Err()
}

// UpdateChannel updates a channel's mutable fields.
func (r *VirtualRepo) UpdateChannel(ctx context.Context, c *Channel) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE channels SET slug = ?, name = ?, auth_mode = ?, description = ? WHERE id = ?`,
		c.Slug, c.Name, c.AuthMode, c.Description, c.ID)
	if err != nil {
		return fmt.Errorf("updating channel %d: %w", c.ID, err)
	}
	return nil
}

// DeleteChannel removes a channel by ID.
func (r *VirtualRepo) DeleteChannel(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM channels WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("deleting channel %d: %w", id, err)
	}
	return nil
}

// --- Views ---

// CreateView inserts a new view and returns its generated ID.
func (r *VirtualRepo) CreateView(ctx context.Context, v *View) (int64, error) {
	writable := 0
	if v.Writable {
		writable = 1
	}
	res, err := r.db.ExecContext(ctx,
		`INSERT INTO virtual_views (slug, name, channel_id, mode, writable, sort_order) VALUES (?, ?, ?, ?, ?, ?)`,
		v.Slug, v.Name, v.ChannelID, v.Mode, writable, v.SortOrder)
	if err != nil {
		return 0, fmt.Errorf("creating view %q: %w", v.Name, err)
	}
	return res.LastInsertId()
}

// GetView retrieves a view by ID. Returns nil, nil if not found.
func (r *VirtualRepo) GetView(ctx context.Context, id int64) (*View, error) {
	row := r.db.QueryRowContext(ctx, "SELECT "+viewColumns+" FROM virtual_views WHERE id = ?", id)
	v, err := scanView(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("getting view %d: %w", id, err)
	}
	return v, nil
}

// ListViewsByChannel returns all views for a channel ordered by sort_order, name.
func (r *VirtualRepo) ListViewsByChannel(ctx context.Context, channelID int64) ([]*View, error) {
	rows, err := r.db.QueryContext(ctx,
		"SELECT "+viewColumns+" FROM virtual_views WHERE channel_id = ? ORDER BY sort_order, name", channelID)
	if err != nil {
		return nil, fmt.Errorf("listing views for channel %d: %w", channelID, err)
	}
	defer rows.Close()

	var views []*View
	for rows.Next() {
		v, err := scanView(rows)
		if err != nil {
			return nil, err
		}
		views = append(views, v)
	}
	return views, rows.Err()
}

// UpdateView updates a view's mutable fields.
func (r *VirtualRepo) UpdateView(ctx context.Context, v *View) error {
	writable := 0
	if v.Writable {
		writable = 1
	}
	_, err := r.db.ExecContext(ctx,
		`UPDATE virtual_views SET slug = ?, name = ?, channel_id = ?, mode = ?, writable = ?, sort_order = ? WHERE id = ?`,
		v.Slug, v.Name, v.ChannelID, v.Mode, writable, v.SortOrder, v.ID)
	if err != nil {
		return fmt.Errorf("updating view %d: %w", v.ID, err)
	}
	return nil
}

// DeleteView removes a view by ID.
func (r *VirtualRepo) DeleteView(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM virtual_views WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("deleting view %d: %w", id, err)
	}
	return nil
}

// --- Nodes ---

// CreateNode inserts a new node and returns its generated ID.
func (r *VirtualRepo) CreateNode(ctx context.Context, n *Node) (int64, error) {
	var parentID, fileID, ruleConfig any
	if n.ParentID != nil {
		parentID = *n.ParentID
	}
	if n.FileID != nil {
		fileID = *n.FileID
	}
	if n.RuleConfig != nil {
		ruleConfig = *n.RuleConfig
	}
	res, err := r.db.ExecContext(ctx,
		`INSERT INTO virtual_nodes (view_id, parent_id, name, node_type, file_id, rule_config, sort_order, status)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		n.ViewID, parentID, n.Name, n.NodeType, fileID, ruleConfig, n.SortOrder, n.Status)
	if err != nil {
		return 0, fmt.Errorf("creating node %q: %w", n.Name, err)
	}
	return res.LastInsertId()
}

// GetNode retrieves a node by ID. Returns nil, nil if not found.
func (r *VirtualRepo) GetNode(ctx context.Context, id int64) (*Node, error) {
	row := r.db.QueryRowContext(ctx, "SELECT "+nodeColumns+" FROM virtual_nodes WHERE id = ?", id)
	n, err := scanNode(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("getting node %d: %w", id, err)
	}
	return n, nil
}

// GetNodeParent returns the parent_id of a node, or nil if it is a root node.
// Returns an error if the node does not exist.
func (r *VirtualRepo) GetNodeParent(ctx context.Context, id int64) (*int64, error) {
	var parent sql.NullInt64
	err := r.db.QueryRowContext(ctx, "SELECT parent_id FROM virtual_nodes WHERE id = ?", id).Scan(&parent)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("node %d not found", id)
	}
	if err != nil {
		return nil, fmt.Errorf("getting parent of node %d: %w", id, err)
	}
	if !parent.Valid {
		return nil, nil
	}
	p := parent.Int64
	return &p, nil
}

// ListNodesByView returns all nodes in a view ordered by sort_order, name.
func (r *VirtualRepo) ListNodesByView(ctx context.Context, viewID int64) ([]*Node, error) {
	rows, err := r.db.QueryContext(ctx,
		"SELECT "+nodeColumns+" FROM virtual_nodes WHERE view_id = ? ORDER BY sort_order, name", viewID)
	if err != nil {
		return nil, fmt.Errorf("listing nodes for view %d: %w", viewID, err)
	}
	defer rows.Close()

	var nodes []*Node
	for rows.Next() {
		n, err := scanNode(rows)
		if err != nil {
			return nil, err
		}
		nodes = append(nodes, n)
	}
	return nodes, rows.Err()
}

// ListNodesByParent returns the direct children of a node (or the roots of a
// view when parentID is nil) ordered by sort_order, name.
func (r *VirtualRepo) ListNodesByParent(ctx context.Context, viewID int64, parentID *int64) ([]*Node, error) {
	var rows *sql.Rows
	var err error
	if parentID == nil {
		rows, err = r.db.QueryContext(ctx,
			"SELECT "+nodeColumns+" FROM virtual_nodes WHERE view_id = ? AND parent_id IS NULL ORDER BY sort_order, name", viewID)
	} else {
		rows, err = r.db.QueryContext(ctx,
			"SELECT "+nodeColumns+" FROM virtual_nodes WHERE view_id = ? AND parent_id = ? ORDER BY sort_order, name", viewID, *parentID)
	}
	if err != nil {
		return nil, fmt.Errorf("listing children of view %d: %w", viewID, err)
	}
	defer rows.Close()

	var nodes []*Node
	for rows.Next() {
		n, err := scanNode(rows)
		if err != nil {
			return nil, err
		}
		nodes = append(nodes, n)
	}
	return nodes, rows.Err()
}

// UpdateNode updates a node's mutable fields (name, node_type, file_id,
// rule_config, sort_order).
func (r *VirtualRepo) UpdateNode(ctx context.Context, n *Node) error {
	var fileID, ruleConfig any
	if n.FileID != nil {
		fileID = *n.FileID
	}
	if n.RuleConfig != nil {
		ruleConfig = *n.RuleConfig
	}
	_, err := r.db.ExecContext(ctx,
		`UPDATE virtual_nodes SET name = ?, node_type = ?, file_id = ?, rule_config = ?, sort_order = ? WHERE id = ?`,
		n.Name, n.NodeType, fileID, ruleConfig, n.SortOrder, n.ID)
	if err != nil {
		return fmt.Errorf("updating node %d: %w", n.ID, err)
	}
	return nil
}

// SetNodeParent reparents a node. A nil parentID moves the node to the root.
func (r *VirtualRepo) SetNodeParent(ctx context.Context, id int64, parentID *int64) error {
	var p any
	if parentID != nil {
		p = *parentID
	}
	_, err := r.db.ExecContext(ctx, `UPDATE virtual_nodes SET parent_id = ? WHERE id = ?`, p, id)
	if err != nil {
		return fmt.Errorf("setting parent of node %d: %w", id, err)
	}
	return nil
}

// SoftDeleteNodeCascade marks a node and all of its descendants as 'hidden'.
// It collects the full subtree first (to avoid nested queries on the single
// write connection), then updates each row.
func (r *VirtualRepo) SoftDeleteNodeCascade(ctx context.Context, id int64) error {
	ids, err := r.collectSubtreeIDs(ctx, id)
	if err != nil {
		return err
	}
	for _, nid := range ids {
		if _, err := r.db.ExecContext(ctx,
			`UPDATE virtual_nodes SET status = 'hidden' WHERE id = ?`, nid); err != nil {
			return fmt.Errorf("soft-deleting node %d: %w", nid, err)
		}
	}
	return nil
}

// collectSubtreeIDs returns the IDs of a node and all of its descendants via
// a breadth-first walk. Each level is fully read before the next query, so it
// is safe on a single-connection pool.
func (r *VirtualRepo) collectSubtreeIDs(ctx context.Context, rootID int64) ([]int64, error) {
	var ids []int64
	queue := []int64{rootID}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		ids = append(ids, cur)

		rows, err := r.db.QueryContext(ctx, "SELECT id FROM virtual_nodes WHERE parent_id = ?", cur)
		if err != nil {
			return nil, fmt.Errorf("querying children of node %d: %w", cur, err)
		}
		var children []int64
		for rows.Next() {
			var c int64
			if err := rows.Scan(&c); err != nil {
				rows.Close()
				return nil, fmt.Errorf("scanning child of node %d: %w", cur, err)
			}
			children = append(children, c)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return nil, fmt.Errorf("iterating children of node %d: %w", cur, err)
		}
		queue = append(queue, children...)
	}
	return ids, nil
}

// --- Scanners ---

func scanChannel(scanner interface{ Scan(dest ...any) error }) (*Channel, error) {
	c := &Channel{}
	if err := scanner.Scan(&c.ID, &c.Slug, &c.Name, &c.AuthMode, &c.Description, &c.CreatedAt); err != nil {
		return nil, err
	}
	return c, nil
}

func scanView(scanner interface{ Scan(dest ...any) error }) (*View, error) {
	v := &View{}
	var writable int
	if err := scanner.Scan(&v.ID, &v.Slug, &v.Name, &v.ChannelID, &v.Mode, &writable, &v.SortOrder, &v.CreatedAt); err != nil {
		return nil, err
	}
	v.Writable = writable == 1
	return v, nil
}

func scanNode(scanner interface{ Scan(dest ...any) error }) (*Node, error) {
	n := &Node{}
	var parentID, fileID sql.NullInt64
	var ruleConfig sql.NullString
	if err := scanner.Scan(&n.ID, &n.ViewID, &parentID, &n.Name, &n.NodeType, &fileID, &ruleConfig, &n.SortOrder, &n.Status, &n.CreatedAt); err != nil {
		return nil, err
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
	return n, nil
}