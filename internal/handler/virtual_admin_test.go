package handler

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Mi-Bee-Studio/mibeehive/internal/db"
	"github.com/Mi-Bee-Studio/mibeehive/internal/eventbus"
	"github.com/Mi-Bee-Studio/mibeehive/internal/model"
	"github.com/Mi-Bee-Studio/mibeehive/internal/service"
)

// setupVirtualAdminTest creates a test database and handler.
func setupVirtualAdminTest(t *testing.T) (*sql.DB, *VirtualAdminHandler) {
	t.Helper()

	testDB, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}

	if err := db.Migrate(testDB); err != nil {
		t.Fatalf("failed to migrate test db: %v", err)
	}

	virtualRepo := db.NewVirtualRepo(testDB)
	bus := eventbus.NewBus(100)
	virtualSvc := service.NewVirtualIndexService(virtualRepo, bus, nil)
	handler := NewVirtualAdminHandler(virtualSvc)

	return testDB, handler
}

func TestCreateChannelAndView(t *testing.T) {
	testDB, h := setupVirtualAdminTest(t)
	defer testDB.Close()

	// Create mux with route patterns needed for this test
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/admin/channels", h.CreateChannel)
	mux.HandleFunc("POST /api/v1/admin/channels/{channel_id}/views", h.CreateView)
	mux.HandleFunc("GET /api/v1/admin/channels/{channel_id}/views", h.ListViews)

	// Test creating a channel
	reqBody := `{"slug": "test-channel", "name": "Test Channel", "auth_mode": "public", "description": "Test description"}`
	req := httptest.NewRequest("POST", "/api/v1/admin/channels", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d", w.Code)
	}

	var channelResp model.ApiResponse[ChannelResponse]
	if err := json.NewDecoder(w.Body).Decode(&channelResp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !channelResp.Success {
		t.Errorf("expected success=true, got %v", channelResp.Success)
	}

	channel := channelResp.Data
	if channel.Name != "Test Channel" {
		t.Errorf("expected channel name 'Test Channel', got %s", channel.Name)
	}

	if channel.AuthMode != "public" {
		t.Errorf("expected auth_mode 'public', got %s", channel.AuthMode)
	}

	// Test creating a view in the channel
	viewReqBody := `{"slug": "test-view", "name": "Test View", "mode": "curated", "writable": true}`
	viewReq := httptest.NewRequest("POST", "/api/v1/admin/channels/1/views", strings.NewReader(viewReqBody))
	viewReq.Header.Set("Content-Type", "application/json")
	viewW := httptest.NewRecorder()
	mux.ServeHTTP(viewW, viewReq)

	if viewW.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d", viewW.Code)
	}

	var viewResp model.ApiResponse[ViewResponse]
	if err := json.NewDecoder(viewW.Body).Decode(&viewResp); err != nil {
		t.Fatalf("failed to decode view response: %v", err)
	}

	if !viewResp.Success {
		t.Errorf("expected success=true, got %v", viewResp.Success)
	}

	view := viewResp.Data
	if view.Name != "Test View" {
		t.Errorf("expected view name 'Test View', got %s", view.Name)
	}

	if !view.Writable {
		t.Errorf("expected writable=true, got %v", view.Writable)
	}

	if view.ChannelID != channel.ID {
		t.Errorf("expected channel_id %d, got %d", channel.ID, view.ChannelID)
	}

	// Verify we can list views for the channel
	listReq := httptest.NewRequest("GET", "/api/v1/admin/channels/1/views", nil)
	listW := httptest.NewRecorder()
	mux.ServeHTTP(listW, listReq)

	if listW.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", listW.Code)
	}

	var listResp model.ApiResponse[[]ViewResponse]
	if err := json.NewDecoder(listW.Body).Decode(&listResp); err != nil {
		t.Fatalf("failed to decode list response: %v", err)
	}

	if len(listResp.Data) != 1 {
		t.Errorf("expected 1 view, got %d", len(listResp.Data))
	}

	if listResp.Data[0].ID != view.ID {
		t.Errorf("expected view ID %d, got %d", view.ID, listResp.Data[0].ID)
	}
}

func TestViewTree(t *testing.T) {
	testDB, h := setupVirtualAdminTest(t)
	defer testDB.Close()

	// Create channel and view
	channel := &db.Channel{
		Slug:     "test-channel",
		Name:     "Test Channel",
		AuthMode: "public",
	}
	channelID, _ := h.svc.CreateChannel(nil, channel)

	view := &db.View{
		Slug:      "test-view",
		Name:      "Test View",
		ChannelID: channelID.ID,
		Mode:      "curated",
		Writable:  true,
	}
	viewID, _ := h.svc.CreateView(nil, view)

	// Create nested folder structure
	folder1 := &db.Node{
		ViewID:   viewID.ID,
		Name:     "Folder1",
		NodeType: "folder",
		Status:   "visible",
	}
	f1ID, _ := h.svc.CreateNode(nil, folder1)

	subfolder := &db.Node{
		ViewID:   viewID.ID,
		ParentID: &f1ID.ID,
		Name:     "Subfolder",
		NodeType: "folder",
		Status:   "visible",
	}
	sfID, _ := h.svc.CreateNode(nil, subfolder)

	fileNode := &db.Node{
		ViewID:   viewID.ID,
		ParentID: &sfID.ID,
		Name:     "file.txt",
		NodeType: "file_ref",
		Status:   "visible",
	}
	_, _ = h.svc.CreateNode(nil, fileNode)

	// Create mux with route patterns needed for this test
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/admin/views/{view_id}/tree", h.GetViewTree)

	// Test getting tree with depth=1 (should only show Folder1)
	req := httptest.NewRequest("GET", "/api/v1/admin/views/1/tree?depth=1", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var treeResp model.ApiResponse[[]NodeResponse]
	if err := json.NewDecoder(w.Body).Decode(&treeResp); err != nil {
		t.Fatalf("failed to decode tree response: %v", err)
	}

	if !treeResp.Success {
		t.Errorf("expected success=true, got %v", treeResp.Success)
	}

	tree := treeResp.Data
	if len(tree) != 1 {
		t.Errorf("expected 1 root node, got %d", len(tree))
	}

	if tree[0].Name != "Folder1" {
		t.Errorf("expected root name 'Folder1', got %s", tree[0].Name)
	}

	// With depth=1, children should be nil
	if tree[0].Children != nil && len(tree[0].Children) > 0 {
		t.Errorf("expected no children with depth=1, got %d", len(tree[0].Children))
	}

	// Test getting tree with depth=3 (should show full structure)
	req3 := httptest.NewRequest("GET", "/api/v1/admin/views/1/tree?depth=3", nil)
	w3 := httptest.NewRecorder()
	mux.ServeHTTP(w3, req3)

	if w3.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w3.Code)
	}

	var treeResp3 model.ApiResponse[[]NodeResponse]
	if err := json.NewDecoder(w3.Body).Decode(&treeResp3); err != nil {
		t.Fatalf("failed to decode tree response: %v", err)
	}

	tree3 := treeResp3.Data
	if len(tree3) != 1 {
		t.Errorf("expected 1 root node, got %d", len(tree3))
	}

	// Check that nested structure is present
	if tree3[0].Children == nil || len(tree3[0].Children) != 1 {
		t.Errorf("expected 1 child under Folder1")
	}

	if tree3[0].Children[0].Name != "Subfolder" {
		t.Errorf("expected child name 'Subfolder', got %s", tree3[0].Children[0].Name)
	}

	if tree3[0].Children[0].Children == nil || len(tree3[0].Children[0].Children) != 1 {
		t.Errorf("expected 1 grandchild under Subfolder")
	}

	if tree3[0].Children[0].Children[0].Name != "file.txt" {
		t.Errorf("expected grandchild name 'file.txt', got %s", tree3[0].Children[0].Children[0].Name)
	}
}

func TestNodeCRUD(t *testing.T) {
	testDB, h := setupVirtualAdminTest(t)
	defer testDB.Close()

	// Create channel and view
	channel := &db.Channel{
		Slug:     "test-channel",
		Name:     "Test Channel",
		AuthMode: "public",
	}
	channelID, _ := h.svc.CreateChannel(nil, channel)

	view := &db.View{
		Slug:      "test-view",
		Name:      "Test View",
		ChannelID: channelID.ID,
		Mode:      "curated",
		Writable:  true,
	}
	_, _ = h.svc.CreateView(nil, view)

	// Create mux with route patterns needed for this test
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/admin/views/{view_id}/nodes", h.CreateNode)
	mux.HandleFunc("PUT /api/v1/admin/nodes/{id}", h.UpdateNode)
	mux.HandleFunc("DELETE /api/v1/admin/nodes/{id}", h.DeleteNode)

	// Create a folder node
	createBody := `{"name": "TestFolder", "node_type": "folder"}`
	createReq := httptest.NewRequest("POST", "/api/v1/admin/views/1/nodes", strings.NewReader(createBody))
	createReq.Header.Set("Content-Type", "application/json")
	createW := httptest.NewRecorder()
	mux.ServeHTTP(createW, createReq)

	if createW.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d: %s", createW.Code, createW.Body.String())
	}

	var createResp model.ApiResponse[NodeResponse]
	if err := json.NewDecoder(createW.Body).Decode(&createResp); err != nil {
		t.Fatalf("failed to decode create response: %v", err)
	}

	node := createResp.Data
	if node.Name != "TestFolder" {
		t.Errorf("expected node name 'TestFolder', got %s", node.Name)
	}

	// Test updating (renaming) the node
	updateBody := `{"name": "RenamedFolder"}`
	updateReq := httptest.NewRequest("PUT", "/api/v1/admin/nodes/1", strings.NewReader(updateBody))
	updateReq.Header.Set("Content-Type", "application/json")
	updateW := httptest.NewRecorder()
	mux.ServeHTTP(updateW, updateReq)

	if updateW.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", updateW.Code, updateW.Body.String())
	}

	var updateResp model.ApiResponse[NodeResponse]
	if err := json.NewDecoder(updateW.Body).Decode(&updateResp); err != nil {
		t.Fatalf("failed to decode update response: %v", err)
	}

	if updateResp.Data.Name != "RenamedFolder" {
		t.Errorf("expected updated name 'RenamedFolder', got %s", updateResp.Data.Name)
	}

	// Test deleting the node
	deleteReq := httptest.NewRequest("DELETE", "/api/v1/admin/nodes/1", nil)
	deleteW := httptest.NewRecorder()
	mux.ServeHTTP(deleteW, deleteReq)

	if deleteW.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", deleteW.Code, deleteW.Body.String())
	}

	// Verify the node is soft-deleted (status=hidden)
	deletedNode, err := h.svc.GetNode(nil, 1)
	if err != nil {
		t.Fatalf("failed to get deleted node: %v", err)
	}

	if deletedNode == nil {
		t.Error("expected node to still exist (soft deleted)")
	}

	if deletedNode.Status != "hidden" {
		t.Errorf("expected status 'hidden', got %s", deletedNode.Status)
	}
}

func TestTreeDepthLimit(t *testing.T) {
	testDB, h := setupVirtualAdminTest(t)
	defer testDB.Close()

	// Create channel and view
	channel := &db.Channel{
		Slug:     "test-channel",
		Name:     "Test Channel",
		AuthMode: "public",
	}
	channelID, _ := h.svc.CreateChannel(nil, channel)

	view := &db.View{
		Slug:      "test-view",
		Name:      "Test View",
		ChannelID: channelID.ID,
		Mode:      "curated",
		Writable:  true,
	}
	_, _ = h.svc.CreateView(nil, view)

	// Create a deep chain: folder1 -> folder2 -> folder3 -> ... -> folder15
	var parentID *int64
	for i := 1; i <= 15; i++ {
		_ = int64(i)
		folder := &db.Node{
			ViewID:   1,
			ParentID: parentID,
			Name:     "Folder" + string(rune('A'+i-1)),
			NodeType: "folder",
			Status:   "visible",
		}
		createdNode, _ := h.svc.CreateNode(nil, folder)
		parentID = &createdNode.ID
	}

	// Create mux with route pattern needed for this test
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/admin/views/{view_id}/tree", h.GetViewTree)

	// Request with depth=999 should be capped at 10
	req := httptest.NewRequest("GET", "/api/v1/admin/views/1/tree?depth=999", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var treeResp model.ApiResponse[[]NodeResponse]
	if err := json.NewDecoder(w.Body).Decode(&treeResp); err != nil {
		t.Fatalf("failed to decode tree response: %v", err)
	}

	tree := treeResp.Data

	// Count depth by traversing the first chain
	depth := 0
	current := &tree[0]
	for current != nil {
		depth++
		if len(current.Children) > 0 {
			current = &current.Children[0]
		} else {
			current = nil
		}
	}

	// Depth should be capped at 10
	if depth > 10 {
		t.Errorf("expected max depth 10, got %d", depth)
	}
}