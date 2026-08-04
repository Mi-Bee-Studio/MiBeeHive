package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/Mi-Bee-Studio/mibeehive/internal/db"
	"github.com/Mi-Bee-Studio/mibeehive/internal/eventbus"
)

// ErrCircularRef is returned when reparenting a node would create a cycle in
// the virtual tree (a node cannot be moved under one of its own descendants).
var ErrCircularRef = errors.New("virtual index: circular reference detected")

// maxTreeDepth caps the ancestor-chain walk during cycle detection.
const maxTreeDepth = 50

// VirtualIndexService provides business logic for the virtual index
// (channels → views → nodes) used by the supply layer. It owns cycle
// detection, soft-delete cascading, and event emission; all persistence is
// delegated to the VirtualRepo.
type VirtualIndexService struct {
	repo   *db.VirtualRepo
	audit  *db.AuditRepo
	bus    *eventbus.Bus
	logger *slog.Logger
}

// NewVirtualIndexService creates a new VirtualIndexService.
func NewVirtualIndexService(repo *db.VirtualRepo, audit *db.AuditRepo, bus *eventbus.Bus, logger *slog.Logger) *VirtualIndexService {
	if logger == nil {
		logger = slog.Default()
	}
	return &VirtualIndexService{repo: repo, audit: audit, bus: bus, logger: logger}
}

// auditLog writes a structured log entry and persists to the audit table.
func (s *VirtualIndexService) auditLog(ctx context.Context, action, entityType string, id int64, name, diff string) {
	s.logger.Info("virtual index audit",
		"action", action, "entity_type", entityType,
		"entity_id", id, "entity_name", name)
	if s.audit != nil {
		_ = s.audit.Log(ctx, &db.AuditEntry{
			AdminUser:  "admin", // TODO: extract from context when auth metadata is threaded through
			Action:     action,
			EntityType: entityType,
			EntityID:   id,
			EntityName: name,
			DiffJSON:   diff,
		})
	}
}

// --- Channels ---

// CreateChannel creates a channel and emits a ChannelChanged event.
func (s *VirtualIndexService) CreateChannel(ctx context.Context, c *db.Channel) (*db.Channel, error) {
	id, err := s.repo.CreateChannel(ctx, c)
	if err != nil {
		return nil, err
	}
	c.ID = id
	s.auditLog(ctx, "create", "channel", id, c.Name, fmt.Sprintf("{\"slug\":\"%s\"}", c.Slug))
	s.emitChannelChanged(ctx, id)
	return c, nil
	s.emitChannelChanged(ctx, id)
	return c, nil
}

// UpdateChannel updates a channel and emits a ChannelChanged event.
func (s *VirtualIndexService) UpdateChannel(ctx context.Context, c *db.Channel) error {
	if err := s.repo.UpdateChannel(ctx, c); err != nil {
		return err
	}
	s.emitChannelChanged(ctx, c.ID)
	s.auditLog(ctx, "update", "channel", c.ID, c.Name, fmt.Sprintf("{\"slug\":\"%s\"}", c.Slug))
	return nil
	return nil
}

// DeleteChannel removes a channel.
func (s *VirtualIndexService) DeleteChannel(ctx context.Context, id int64) error {
	if err := s.repo.DeleteChannel(ctx, id); err != nil {
		return err
	}
	s.auditLog(ctx, "delete", "channel", id, "", "{}")
	return nil
}

// ListChannels returns all channels.
func (s *VirtualIndexService) ListChannels(ctx context.Context) ([]*db.Channel, error) {
	return s.repo.ListChannels(ctx)
}

// GetChannel returns a channel by ID, or nil if not found.
func (s *VirtualIndexService) GetChannel(ctx context.Context, id int64) (*db.Channel, error) {
	return s.repo.GetChannel(ctx, id)
}

// --- Views ---

// CreateView creates a view within a channel.
func (s *VirtualIndexService) CreateView(ctx context.Context, v *db.View) (*db.View, error) {
	id, err := s.repo.CreateView(ctx, v)
	if err != nil {
		return nil, err
	}
	v.ID = id
	s.auditLog(ctx, "create", "view", id, v.Name, fmt.Sprintf("{\"slug\":\"%s\",\"channel\":%d}", v.Slug, v.ChannelID))
	return v, nil
	return v, nil
}

// UpdateView updates a view.
func (s *VirtualIndexService) UpdateView(ctx context.Context, v *db.View) error {
	if err := s.repo.UpdateView(ctx, v); err != nil {
		return err
	}
	s.auditLog(ctx, "update", "view", v.ID, v.Name, "{}")
	return nil
}

// DeleteView removes a view.
func (s *VirtualIndexService) DeleteView(ctx context.Context, id int64) error {
	if err := s.repo.DeleteView(ctx, id); err != nil {
		return err
	}
	s.auditLog(ctx, "delete", "view", id, "", "{}")
	return nil
}

// ListViewsByChannel returns all views for a channel.
func (s *VirtualIndexService) ListViewsByChannel(ctx context.Context, channelID int64) ([]*db.View, error) {
	return s.repo.ListViewsByChannel(ctx, channelID)
}

// GetView returns a view by ID, or nil if not found.
func (s *VirtualIndexService) GetView(ctx context.Context, id int64) (*db.View, error) {
	return s.repo.GetView(ctx, id)
}


// --- Nodes ---

// CreateNode creates a node. Emits NodeTreeChanged, and additionally
// RuleFolderCreated when the node is a rule folder.
func (s *VirtualIndexService) CreateNode(ctx context.Context, n *db.Node) (*db.Node, error) {
	id, err := s.repo.CreateNode(ctx, n)
	if err != nil {
		return nil, err
	}
	n.ID = id
	s.auditLog(ctx, "create", "node", id, n.Name, fmt.Sprintf("{\"type\":\"%s\",\"view\":%d}", n.NodeType, n.ViewID))
	s.emitNodeTreeChanged(ctx, n.ViewID)
	if n.NodeType == "rule_folder" {
		s.emitRuleFolderCreated(ctx, id)
	}
	return n, nil
}

// UpdateNode updates a node (rename, retype, reorder) and emits a
// NodeTreeChanged event.
func (s *VirtualIndexService) UpdateNode(ctx context.Context, n *db.Node) error {
	if err := s.repo.UpdateNode(ctx, n); err != nil {
		return err
	}
	s.emitNodeTreeChanged(ctx, n.ViewID)
	s.auditLog(ctx, "update", "node", n.ID, n.Name, "{}")
	return nil
	return nil
}

// MoveNode reparents a node. It rejects moves that would create a cycle by
// walking the new parent's ancestor chain (bounded by maxTreeDepth). Emits a
// NodeTreeChanged event on success.
func (s *VirtualIndexService) MoveNode(ctx context.Context, nodeID int64, newParentID *int64) error {
	if newParentID != nil {
		if err := s.checkCycle(ctx, nodeID, *newParentID); err != nil {
			return err
		}
	}
	node, err := s.repo.GetNode(ctx, nodeID)
	if err != nil {
		return err
	}
	if node == nil {
		return fmt.Errorf("virtual index: node %d not found", nodeID)
	}
	if err := s.repo.SetNodeParent(ctx, nodeID, newParentID); err != nil {
		return err
	}
	s.emitNodeTreeChanged(ctx, node.ViewID)
	return nil
}

// SoftDeleteNode marks a node and all of its descendants as 'hidden' and
// emits a NodeTreeChanged event.
func (s *VirtualIndexService) SoftDeleteNode(ctx context.Context, nodeID int64) error {
	node, err := s.repo.GetNode(ctx, nodeID)
	if err != nil {
		return err
	}
	if node == nil {
		return fmt.Errorf("virtual index: node %d not found", nodeID)
	}
	if err := s.repo.SoftDeleteNodeCascade(ctx, nodeID); err != nil {
		return err
	}
	s.emitNodeTreeChanged(ctx, node.ViewID)
	s.auditLog(ctx, "delete", "node", nodeID, node.Name, "{}")
	return nil
	return nil
}

// ListNodesByView returns all nodes in a view.
func (s *VirtualIndexService) ListNodesByView(ctx context.Context, viewID int64) ([]*db.Node, error) {
	return s.repo.ListNodesByView(ctx, viewID)
}

// ListNodesByParent returns the direct children of a node (or the roots of a
// view when parentID is nil).
func (s *VirtualIndexService) ListNodesByParent(ctx context.Context, viewID int64, parentID *int64) ([]*db.Node, error) {
	return s.repo.ListNodesByParent(ctx, viewID, parentID)
}

// GetNode returns a node by ID, or nil if not found.
func (s *VirtualIndexService) GetNode(ctx context.Context, id int64) (*db.Node, error) {
	return s.repo.GetNode(ctx, id)
}

// checkCycle walks the ancestor chain of newParentID (bounded by
// maxTreeDepth) and returns ErrCircularRef if nodeID is encountered, which
// would make the node its own ancestor.
func (s *VirtualIndexService) checkCycle(ctx context.Context, nodeID, newParentID int64) error {
	cur := newParentID
	for depth := 0; depth < maxTreeDepth; depth++ {
		if cur == nodeID {
			return ErrCircularRef
		}
		parent, err := s.repo.GetNodeParent(ctx, cur)
		if err != nil {
			return err
		}
		if parent == nil {
			// Reached the root without finding nodeID — no cycle.
			return nil
		}
		cur = *parent
	}
	return fmt.Errorf("virtual index: ancestor chain exceeds max depth %d", maxTreeDepth)
}

// --- Event emission ---

func (s *VirtualIndexService) emitNodeTreeChanged(ctx context.Context, viewID int64) {
	if s.bus != nil {
		s.bus.Publish(ctx, eventbus.NodeTreeChanged{ViewID: viewID})
	}
}

func (s *VirtualIndexService) emitChannelChanged(ctx context.Context, channelID int64) {
	if s.bus != nil {
		s.bus.Publish(ctx, eventbus.ChannelChanged{ChannelID: channelID})
	}
}

func (s *VirtualIndexService) emitRuleFolderCreated(ctx context.Context, nodeID int64) {
	if s.bus != nil {
		s.bus.Publish(ctx, eventbus.RuleFolderCreated{NodeID: nodeID})
	}
}