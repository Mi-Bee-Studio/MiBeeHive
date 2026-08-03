package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/Mi-Bee-Studio/mibeehive/internal/db"
	"github.com/Mi-Bee-Studio/mibeehive/internal/middleware"
	"github.com/Mi-Bee-Studio/mibeehive/internal/model"
	"github.com/Mi-Bee-Studio/mibeehive/internal/service"
)

// VirtualAdminHandler handles admin virtual index API endpoints.
type VirtualAdminHandler struct {
	svc    *service.VirtualIndexService
	logger *slog.Logger
}

// NewVirtualAdminHandler creates a new VirtualAdminHandler.
func NewVirtualAdminHandler(svc *service.VirtualIndexService) *VirtualAdminHandler {
	return &VirtualAdminHandler{
		svc:    svc,
		logger: slog.Default(),
	}
}

// --- Channel CRUD ---

// CreateChannelRequest is the request body for creating a channel.
type CreateChannelRequest struct {
	Slug        string `json:"slug"`
	Name        string `json:"name"`
	AuthMode    string `json:"auth_mode"`
	Description string `json:"description"`
}

// UpdateChannelRequest is the request body for updating a channel.
type UpdateChannelRequest struct {
	Slug        *string `json:"slug"`
	Name        *string `json:"name"`
	AuthMode    *string `json:"auth_mode"`
	Description *string `json:"description"`
}

// ChannelResponse is the response DTO for a channel.
type ChannelResponse struct {
	ID          int64     `json:"id"`
	Slug        string    `json:"slug"`
	Name        string    `json:"name"`
	AuthMode    string    `json:"auth_mode"`
	Description string    `json:"description"`
	CreatedAt   string    `json:"created_at"`
}

// toChannelResponse converts a db.Channel to ChannelResponse.
func toChannelResponse(c *db.Channel) ChannelResponse {
	return ChannelResponse{
		ID:          c.ID,
		Slug:        c.Slug,
		Name:        c.Name,
		AuthMode:    c.AuthMode,
		Description: c.Description,
		CreatedAt:   c.CreatedAt.Format(time.RFC3339),
	}
}

// CreateChannel handles POST /api/v1/admin/channels.
func (h *VirtualAdminHandler) CreateChannel(w http.ResponseWriter, r *http.Request) {
	var req CreateChannelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		middleware.WriteError(w, http.StatusBadRequest, model.ERR_VALIDATION, "invalid request body", err)
		return
	}

	if req.Name == "" || req.AuthMode == "" {
		middleware.WriteError(w, http.StatusBadRequest, model.ERR_VALIDATION, "name and auth_mode are required", nil)
		return
	}

	if req.Slug == "" {
		req.Slug = req.Name
	}

	channel := &db.Channel{
		Slug:        req.Slug,
		Name:        req.Name,
		AuthMode:    req.AuthMode,
		Description: req.Description,
	}

	created, err := h.svc.CreateChannel(r.Context(), channel)
	if err != nil {
		h.logger.Error("failed to create channel", "error", err)
		middleware.WriteError(w, http.StatusInternalServerError, model.ERR_INTERNAL, "failed to create channel", err)
		return
	}

	writeJSON(w, http.StatusCreated, model.ApiResponse[ChannelResponse]{
		Success: true,
		Data:    toChannelResponse(created),
	})
}

// ListChannels handles GET /api/v1/admin/channels.
func (h *VirtualAdminHandler) ListChannels(w http.ResponseWriter, r *http.Request) {
	channels, err := h.svc.ListChannels(r.Context())
	if err != nil {
		h.logger.Error("failed to list channels", "error", err)
		middleware.WriteError(w, http.StatusInternalServerError, model.ERR_INTERNAL, "failed to list channels", err)
		return
	}

	resp := make([]ChannelResponse, len(channels))
	for i, c := range channels {
		resp[i] = toChannelResponse(c)
	}

	writeJSON(w, http.StatusOK, model.ApiResponse[[]ChannelResponse]{
		Success: true,
		Data:    resp,
	})
}

// GetChannel handles GET /api/v1/admin/channels/{id}.
func (h *VirtualAdminHandler) GetChannel(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		middleware.WriteError(w, http.StatusBadRequest, model.ERR_VALIDATION, "invalid channel id", err)
		return
	}

	channel, err := h.svc.GetChannel(r.Context(), id)
	if err != nil {
		h.logger.Error("failed to get channel", "id", id, "error", err)
		middleware.WriteError(w, http.StatusInternalServerError, model.ERR_INTERNAL, "failed to get channel", err)
		return
	}

	if channel == nil {
		middleware.WriteError(w, http.StatusNotFound, model.ERR_NOT_FOUND, "channel not found", nil)
		return
	}

	writeJSON(w, http.StatusOK, model.ApiResponse[ChannelResponse]{
		Success: true,
		Data:    toChannelResponse(channel),
	})
}

// UpdateChannel handles PUT /api/v1/admin/channels/{id}.
func (h *VirtualAdminHandler) UpdateChannel(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		middleware.WriteError(w, http.StatusBadRequest, model.ERR_VALIDATION, "invalid channel id", err)
		return
	}

	var req UpdateChannelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		middleware.WriteError(w, http.StatusBadRequest, model.ERR_VALIDATION, "invalid request body", err)
		return
	}

	channel, err := h.svc.GetChannel(r.Context(), id)
	if err != nil {
		h.logger.Error("failed to get channel", "id", id, "error", err)
		middleware.WriteError(w, http.StatusInternalServerError, model.ERR_INTERNAL, "failed to get channel", err)
		return
	}

	if channel == nil {
		middleware.WriteError(w, http.StatusNotFound, model.ERR_NOT_FOUND, "channel not found", nil)
		return
	}

	if req.Slug != nil {
		channel.Slug = *req.Slug
	}
	if req.Name != nil {
		channel.Name = *req.Name
	}
	if req.AuthMode != nil {
		channel.AuthMode = *req.AuthMode
	}
	if req.Description != nil {
		channel.Description = *req.Description
	}

	if err := h.svc.UpdateChannel(r.Context(), channel); err != nil {
		h.logger.Error("failed to update channel", "id", id, "error", err)
		middleware.WriteError(w, http.StatusInternalServerError, model.ERR_INTERNAL, "failed to update channel", err)
		return
	}

	writeJSON(w, http.StatusOK, model.ApiResponse[ChannelResponse]{
		Success: true,
		Data:    toChannelResponse(channel),
	})
}

// DeleteChannel handles DELETE /api/v1/admin/channels/{id}.
func (h *VirtualAdminHandler) DeleteChannel(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		middleware.WriteError(w, http.StatusBadRequest, model.ERR_VALIDATION, "invalid channel id", err)
		return
	}

	if err := h.svc.DeleteChannel(r.Context(), id); err != nil {
		h.logger.Error("failed to delete channel", "id", id, "error", err)
		middleware.WriteError(w, http.StatusInternalServerError, model.ERR_INTERNAL, "failed to delete channel", err)
		return
	}

	writeJSON(w, http.StatusOK, model.ApiResponse[any]{
		Success: true,
		Message: "channel deleted",
	})
}

// --- View CRUD ---

// CreateViewRequest is the request body for creating a view.
type CreateViewRequest struct {
	Slug      string `json:"slug"`
	Name      string `json:"name"`
	IsDefault bool   `json:"is_default"`
	Writable  bool   `json:"writable"`
	Mode      string `json:"mode"`
}

// UpdateViewRequest is the request body for updating a view.
type UpdateViewRequest struct {
	Slug      *string `json:"slug"`
	Name      *string `json:"name"`
	Writable  *bool   `json:"writable"`
	Mode      *string `json:"mode"`
	SortOrder *int    `json:"sort_order"`
}

// ViewResponse is the response DTO for a view.
type ViewResponse struct {
	ID        int64  `json:"id"`
	Slug      string `json:"slug"`
	Name      string `json:"name"`
	ChannelID int64  `json:"channel_id"`
	Mode      string `json:"mode"`
	Writable  bool   `json:"writable"`
	SortOrder int    `json:"sort_order"`
	CreatedAt string `json:"created_at"`
}

// toViewResponse converts a db.View to ViewResponse.
func toViewResponse(v *db.View) ViewResponse {
	return ViewResponse{
		ID:        v.ID,
		Slug:      v.Slug,
		Name:      v.Name,
		ChannelID: v.ChannelID,
		Mode:      v.Mode,
		Writable:  v.Writable,
		SortOrder: v.SortOrder,
		CreatedAt: v.CreatedAt.Format(time.RFC3339),
	}
}

// CreateView handles POST /api/v1/admin/channels/{channel_id}/views.
func (h *VirtualAdminHandler) CreateView(w http.ResponseWriter, r *http.Request) {
	channelID, err := strconv.ParseInt(r.PathValue("channel_id"), 10, 64)
	if err != nil {
		middleware.WriteError(w, http.StatusBadRequest, model.ERR_VALIDATION, "invalid channel id", err)
		return
	}

	var req CreateViewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		middleware.WriteError(w, http.StatusBadRequest, model.ERR_VALIDATION, "invalid request body", err)
		return
	}

	if req.Name == "" {
		middleware.WriteError(w, http.StatusBadRequest, model.ERR_VALIDATION, "name is required", nil)
		return
	}

	if req.Slug == "" {
		req.Slug = req.Name
	}

	view := &db.View{
		Slug:      req.Slug,
		Name:      req.Name,
		ChannelID: channelID,
		Mode:      req.Mode,
		Writable:  req.Writable,
	}

	created, err := h.svc.CreateView(r.Context(), view)
	if err != nil {
		h.logger.Error("failed to create view", "channel_id", channelID, "error", err)
		middleware.WriteError(w, http.StatusInternalServerError, model.ERR_INTERNAL, "failed to create view", err)
		return
	}

	writeJSON(w, http.StatusCreated, model.ApiResponse[ViewResponse]{
		Success: true,
		Data:    toViewResponse(created),
	})
}

// ListViews handles GET /api/v1/admin/channels/{channel_id}/views.
func (h *VirtualAdminHandler) ListViews(w http.ResponseWriter, r *http.Request) {
	channelID, err := strconv.ParseInt(r.PathValue("channel_id"), 10, 64)
	if err != nil {
		middleware.WriteError(w, http.StatusBadRequest, model.ERR_VALIDATION, "invalid channel id", err)
		return
	}

	views, err := h.svc.ListViewsByChannel(r.Context(), channelID)
	if err != nil {
		h.logger.Error("failed to list views", "channel_id", channelID, "error", err)
		middleware.WriteError(w, http.StatusInternalServerError, model.ERR_INTERNAL, "failed to list views", err)
		return
	}

	resp := make([]ViewResponse, len(views))
	for i, v := range views {
		resp[i] = toViewResponse(v)
	}

	writeJSON(w, http.StatusOK, model.ApiResponse[[]ViewResponse]{
		Success: true,
		Data:    resp,
	})
}

// GetView handles GET /api/v1/admin/views/{id}.
func (h *VirtualAdminHandler) GetView(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		middleware.WriteError(w, http.StatusBadRequest, model.ERR_VALIDATION, "invalid view id", err)
		return
	}

	view, err := h.svc.GetView(r.Context(), id)
	if err != nil {
		h.logger.Error("failed to get view", "id", id, "error", err)
		middleware.WriteError(w, http.StatusInternalServerError, model.ERR_INTERNAL, "failed to get view", err)
		return
	}

	if view == nil {
		middleware.WriteError(w, http.StatusNotFound, model.ERR_NOT_FOUND, "view not found", nil)
		return
	}

	writeJSON(w, http.StatusOK, model.ApiResponse[ViewResponse]{
		Success: true,
		Data:    toViewResponse(view),
	})
}

// UpdateView handles PUT /api/v1/admin/views/{id}.
func (h *VirtualAdminHandler) UpdateView(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		middleware.WriteError(w, http.StatusBadRequest, model.ERR_VALIDATION, "invalid view id", err)
		return
	}

	var req UpdateViewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		middleware.WriteError(w, http.StatusBadRequest, model.ERR_VALIDATION, "invalid request body", err)
		return
	}

	view, err := h.svc.GetView(r.Context(), id)
	if err != nil {
		h.logger.Error("failed to get view", "id", id, "error", err)
		middleware.WriteError(w, http.StatusInternalServerError, model.ERR_INTERNAL, "failed to get view", err)
		return
	}

	if view == nil {
		middleware.WriteError(w, http.StatusNotFound, model.ERR_NOT_FOUND, "view not found", nil)
		return
	}

	if req.Slug != nil {
		view.Slug = *req.Slug
	}
	if req.Name != nil {
		view.Name = *req.Name
	}
	if req.Writable != nil {
		view.Writable = *req.Writable
	}
	if req.Mode != nil {
		view.Mode = *req.Mode
	}
	if req.SortOrder != nil {
		view.SortOrder = *req.SortOrder
	}

	if err := h.svc.UpdateView(r.Context(), view); err != nil {
		h.logger.Error("failed to update view", "id", id, "error", err)
		middleware.WriteError(w, http.StatusInternalServerError, model.ERR_INTERNAL, "failed to update view", err)
		return
	}

	writeJSON(w, http.StatusOK, model.ApiResponse[ViewResponse]{
		Success: true,
		Data:    toViewResponse(view),
	})
}

// DeleteView handles DELETE /api/v1/admin/views/{id}.
func (h *VirtualAdminHandler) DeleteView(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		middleware.WriteError(w, http.StatusBadRequest, model.ERR_VALIDATION, "invalid view id", err)
		return
	}

	if err := h.svc.DeleteView(r.Context(), id); err != nil {
		h.logger.Error("failed to delete view", "id", id, "error", err)
		middleware.WriteError(w, http.StatusInternalServerError, model.ERR_INTERNAL, "failed to delete view", err)
		return
	}

	writeJSON(w, http.StatusOK, model.ApiResponse[any]{
		Success: true,
		Message: "view deleted",
	})
}

// --- Node CRUD ---

// CreateNodeRequest is the request body for creating a node.
type CreateNodeRequest struct {
	Name     string  `json:"name"`
	ParentID *int64  `json:"parent_id,omitempty"`
	NodeType string  `json:"node_type"` // folder, file_ref, rule_folder
	FileID   *int64  `json:"file_id,omitempty"`   // Required for file_ref nodes
}

// UpdateNodeRequest is the request body for updating a node.
type UpdateNodeRequest struct {
	Name      *string `json:"name,omitempty"`
	NodeType  *string `json:"node_type,omitempty"`
	ParentID  *int64  `json:"parent_id,omitempty"`
	FileID    *int64  `json:"file_id,omitempty"`
	SortOrder *int    `json:"sort_order,omitempty"`
}

// NodeResponse is the response DTO for a node.
type NodeResponse struct {
	ID         int64  `json:"id"`
	ViewID     int64  `json:"view_id"`
	ParentID   *int64 `json:"parent_id,omitempty"`
	Name       string `json:"name"`
	NodeType   string `json:"node_type"`
	FileID     *int64 `json:"file_id,omitempty"`
	RuleConfig *string `json:"rule_config,omitempty"`
	SortOrder  int    `json:"sort_order"`
	Status     string `json:"status"`
	CreatedAt  string `json:"created_at"`
	Children   []NodeResponse `json:"children,omitempty"` // Only in tree responses
}

// toNodeResponse converts a db.Node to NodeResponse (without local_path leak).
func toNodeResponse(n *db.Node) NodeResponse {
	return NodeResponse{
		ID:         n.ID,
		ViewID:     n.ViewID,
		ParentID:   n.ParentID,
		Name:       n.Name,
		NodeType:   n.NodeType,
		FileID:     n.FileID,
		RuleConfig: n.RuleConfig,
		SortOrder:  n.SortOrder,
		Status:     n.Status,
		CreatedAt:  n.CreatedAt.Format(time.RFC3339),
	}
}

// CreateNode handles POST /api/v1/admin/views/{view_id}/nodes.
func (h *VirtualAdminHandler) CreateNode(w http.ResponseWriter, r *http.Request) {
	viewID, err := strconv.ParseInt(r.PathValue("view_id"), 10, 64)
	if err != nil {
		middleware.WriteError(w, http.StatusBadRequest, model.ERR_VALIDATION, "invalid view id", err)
		return
	}

	var req CreateNodeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		middleware.WriteError(w, http.StatusBadRequest, model.ERR_VALIDATION, "invalid request body", err)
		return
	}

	if req.Name == "" || req.NodeType == "" {
		middleware.WriteError(w, http.StatusBadRequest, model.ERR_VALIDATION, "name and node_type are required", nil)
		return
	}

	// Validate node_type
	if req.NodeType != "folder" && req.NodeType != "file_ref" && req.NodeType != "rule_folder" {
		middleware.WriteError(w, http.StatusBadRequest, model.ERR_VALIDATION, "node_type must be folder, file_ref, or rule_folder", nil)
		return
	}

	// file_ref nodes must have file_id
	if req.NodeType == "file_ref" && req.FileID == nil {
		middleware.WriteError(w, http.StatusBadRequest, model.ERR_VALIDATION, "file_id is required for file_ref nodes", nil)
		return
	}

	node := &db.Node{
		ViewID:   viewID,
		ParentID: req.ParentID,
		Name:     req.Name,
		NodeType: req.NodeType,
		FileID:   req.FileID,
		Status:   "visible",
	}

	created, err := h.svc.CreateNode(r.Context(), node)
	if err != nil {
		h.logger.Error("failed to create node", "view_id", viewID, "error", err)
		middleware.WriteError(w, http.StatusInternalServerError, model.ERR_INTERNAL, "failed to create node", err)
		return
	}

	writeJSON(w, http.StatusCreated, model.ApiResponse[NodeResponse]{
		Success: true,
		Data:    toNodeResponse(created),
	})
}

// UpdateNode handles PUT /api/v1/admin/nodes/{id}.
func (h *VirtualAdminHandler) UpdateNode(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		middleware.WriteError(w, http.StatusBadRequest, model.ERR_VALIDATION, "invalid node id", err)
		return
	}

	var req UpdateNodeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		middleware.WriteError(w, http.StatusBadRequest, model.ERR_VALIDATION, "invalid request body", err)
		return
	}

	node, err := h.svc.GetNode(r.Context(), id)
	if err != nil {
		h.logger.Error("failed to get node", "id", id, "error", err)
		middleware.WriteError(w, http.StatusInternalServerError, model.ERR_INTERNAL, "failed to get node", err)
		return
	}

	if node == nil {
		middleware.WriteError(w, http.StatusNotFound, model.ERR_NOT_FOUND, "node not found", nil)
		return
	}

	// If parent_id changed, use MoveNode for cycle detection
	if req.ParentID != nil && (node.ParentID == nil || *req.ParentID != *node.ParentID) {
		if err := h.svc.MoveNode(r.Context(), id, req.ParentID); err != nil {
			h.logger.Error("failed to move node", "id", id, "new_parent_id", req.ParentID, "error", err)
			if err == service.ErrCircularRef {
				middleware.WriteError(w, http.StatusBadRequest, model.ERR_VALIDATION, "cannot move node: circular reference detected", err)
				return
			}
			middleware.WriteError(w, http.StatusInternalServerError, model.ERR_INTERNAL, "failed to move node", err)
			return
		}
		node.ParentID = req.ParentID
	}

	if req.Name != nil {
		node.Name = *req.Name
	}
	if req.NodeType != nil {
		node.NodeType = *req.NodeType
	}
	if req.FileID != nil {
		node.FileID = req.FileID
	}
	if req.SortOrder != nil {
		node.SortOrder = *req.SortOrder
	}

	if err := h.svc.UpdateNode(r.Context(), node); err != nil {
		h.logger.Error("failed to update node", "id", id, "error", err)
		middleware.WriteError(w, http.StatusInternalServerError, model.ERR_INTERNAL, "failed to update node", err)
		return
	}

	writeJSON(w, http.StatusOK, model.ApiResponse[NodeResponse]{
		Success: true,
		Data:    toNodeResponse(node),
	})
}

// DeleteNode handles DELETE /api/v1/admin/nodes/{id}.
func (h *VirtualAdminHandler) DeleteNode(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		middleware.WriteError(w, http.StatusBadRequest, model.ERR_VALIDATION, "invalid node id", err)
		return
	}

	// Use SoftDeleteNode to cascade delete descendants
	if err := h.svc.SoftDeleteNode(r.Context(), id); err != nil {
		h.logger.Error("failed to delete node", "id", id, "error", err)
		middleware.WriteError(w, http.StatusInternalServerError, model.ERR_INTERNAL, "failed to delete node", err)
		return
	}

	writeJSON(w, http.StatusOK, model.ApiResponse[any]{
		Success: true,
		Message: "node deleted",
	})
}

// GetViewTree handles GET /api/v1/admin/views/{view_id}/tree?depth=1.
func (h *VirtualAdminHandler) GetViewTree(w http.ResponseWriter, r *http.Request) {
	viewID, err := strconv.ParseInt(r.PathValue("view_id"), 10, 64)
	if err != nil {
		middleware.WriteError(w, http.StatusBadRequest, model.ERR_VALIDATION, "invalid view id", err)
		return
	}

	// Parse and cap depth parameter
	depth := 1 // Default depth
	if depthStr := r.URL.Query().Get("depth"); depthStr != "" {
		if d, err := strconv.Atoi(depthStr); err == nil && d > 0 {
			depth = d
		}
	}

	const maxTreeDepth = 10
	if depth > maxTreeDepth {
		depth = maxTreeDepth
	}

	// Build tree: nodeID=0 means root (parent_id IS NULL)
	tree, err := h.buildTree(r.Context(), viewID, nil, depth)
	if err != nil {
		h.logger.Error("failed to get view tree", "view_id", viewID, "depth", depth, "error", err)
		middleware.WriteError(w, http.StatusInternalServerError, model.ERR_INTERNAL, "failed to get view tree", err)
		return
	}

	writeJSON(w, http.StatusOK, model.ApiResponse[[]NodeResponse]{
		Success: true,
		Data:    tree,
	})
}

// buildTree recursively builds a node tree up to the specified depth.
func (h *VirtualAdminHandler) buildTree(ctx context.Context, viewID int64, parentID *int64, depth int) ([]NodeResponse, error) {
	nodes, err := h.svc.ListNodesByParent(ctx, viewID, parentID)
	if err != nil {
		return nil, fmt.Errorf("failed to list nodes: %w", err)
	}

	result := make([]NodeResponse, 0, len(nodes))
	for _, n := range nodes {
		// Skip hidden nodes
		if n.Status == "hidden" {
			continue
		}

		nodeResp := toNodeResponse(n)

		// Recursively fetch children if depth > 1
		if depth > 1 {
			children, err := h.buildTree(ctx, viewID, &n.ID, depth-1)
			if err != nil {
				return nil, err
			}
			nodeResp.Children = children
		}

		result = append(result, nodeResp)
	}

	return result, nil
}