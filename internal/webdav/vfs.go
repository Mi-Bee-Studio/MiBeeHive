package webdav

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/Mi-Bee-Studio/mibeehive/internal/db"
	"github.com/Mi-Bee-Studio/mibeehive/internal/eventbus"
	"github.com/Mi-Bee-Studio/mibeehive/internal/service"
	"golang.org/x/net/webdav"
)

// manualUploadsSubdir is the physical directory (relative to storagePath) that
// writable views delegate their write operations to. Files written here are
// registered in the files table with source_type='manual_upload'.
const manualUploadsSubdir = "webdav/manual_uploads"

// VirtualFS implements golang.org/x/net/webdav.FileSystem over the virtual
// index (channels → views → nodes). Read operations resolve the requested
// path through the virtual tree and stream the underlying physical file;
// write operations are only permitted on writable views and are delegated to
// a real webdav.Dir rooted at {storagePath}/webdav/manual_uploads.
//
// The WebDAV path layout is /{channel_slug}/{view_slug}/{node_name}/... where
// node names are resolved by walking the virtual tree (ListNodesByParent).
type VirtualFS struct {
	readDB       *sql.DB
	writeDB      *sql.DB
	storagePath  string
	vpathService *service.VPathIndexService
	virtualRepo  *db.VirtualRepo
	bus          *eventbus.Bus
	logger       *slog.Logger
}

// NewVirtualFS creates a VirtualFS. readDB is used for all virtual-index and
// file reads; writeDB for file registration and soft-delete on write paths.
func NewVirtualFS(
	readDB, writeDB *sql.DB,
	storagePath string,
	vpathService *service.VPathIndexService,
	virtualRepo *db.VirtualRepo,
	bus *eventbus.Bus,
	logger *slog.Logger,
) *VirtualFS {
	if logger == nil {
		logger = slog.Default()
	}
	return &VirtualFS{
		readDB:       readDB,
		writeDB:      writeDB,
		storagePath:  storagePath,
		vpathService: vpathService,
		virtualRepo:  virtualRepo,
		bus:          bus,
		logger:       logger,
	}
}


// getOrCreateManualProject returns the ID of the "Manual Uploads" project,
// creating it if necessary. Used as the project_id for WebDAV uploads.
func (fs *VirtualFS) getOrCreateManualProject() (int64, error) {
	var id int64
	err := fs.writeDB.QueryRow(
		`SELECT id FROM projects WHERE source_type = 'manual_upload' LIMIT 1`).Scan(&id)
	if err == nil {
		return id, nil
	}
	if err != sql.ErrNoRows {
		return 0, fmt.Errorf("querying manual project: %w", err)
	}
	res, err := fs.writeDB.Exec(`
		INSERT INTO projects (name, display_name, source_type, source_url, config, enabled)
		VALUES ('manual_uploads', 'Manual Uploads', 'manual_upload', '', '{}', 1)`)
	if err != nil {
		return 0, fmt.Errorf("creating manual project: %w", err)
	}
	return res.LastInsertId()
}

// --- Path resolution -------------------------------------------------------

// resolveView parses /{channel_slug}/{view_slug}/{rest...} and returns the
// view plus the remaining path (relative to the view root). It does not
// require the rest of the path to exist, so it is safe to call for write
// targets that do not exist yet.
func (fs *VirtualFS) resolveView(ctx context.Context, name string) (*db.View, string, error) {
	clean := path.Clean("/" + name)
	parts := strings.Split(strings.TrimPrefix(clean, "/"), "/")
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return nil, "", os.ErrNotExist
	}
	channel, err := fs.virtualRepo.GetChannelBySlug(ctx, parts[0])
	if err != nil {
		return nil, "", fmt.Errorf("vfs: resolving channel %q: %w", parts[0], err)
	}
	if channel == nil {
		return nil, "", os.ErrNotExist
	}
	view, err := fs.findViewBySlug(ctx, channel.ID, parts[1])
	if err != nil {
		return nil, "", fmt.Errorf("vfs: resolving view %q: %w", parts[1], err)
	}
	if view == nil {
		return nil, "", os.ErrNotExist
	}
	rest := strings.Join(parts[2:], "/")
	return view, rest, nil
}

// findViewBySlug returns the view with the given slug within a channel, or nil.
func (fs *VirtualFS) findViewBySlug(ctx context.Context, channelID int64, slug string) (*db.View, error) {
	views, err := fs.virtualRepo.ListViewsByChannel(ctx, channelID)
	if err != nil {
		return nil, err
	}
	for _, v := range views {
		if v.Slug == slug {
			return v, nil
		}
	}
	return nil, nil
}

// resolveNode walks the virtual tree by node name. It returns the view and the
// resolved node. node is nil when the path names the view root itself.
func (fs *VirtualFS) resolveNode(ctx context.Context, name string) (*db.View, *db.Node, error) {
	view, rest, err := fs.resolveView(ctx, name)
	if err != nil {
		return nil, nil, err
	}
	if rest == "" {
		return view, nil, nil
	}
	var node *db.Node
	var parentID *int64
	for _, seg := range strings.Split(rest, "/") {
		if seg == "" {
			continue
		}
		children, err := fs.virtualRepo.ListNodesByParent(ctx, view.ID, parentID)
		if err != nil {
			return nil, nil, fmt.Errorf("vfs: listing children of %q: %w", seg, err)
		}
		var found *db.Node
		for _, c := range children {
			if c.Status == "visible" && c.Name == seg {
				found = c
				break
			}
		}
		if found == nil {
			return nil, nil, os.ErrNotExist
		}
		node = found
		parentID = &found.ID
	}
	return view, node, nil
}

// checkWritable reports whether the view named by the path is writable.
func (fs *VirtualFS) checkWritable(ctx context.Context, name string) bool {
	view, _, err := fs.resolveView(ctx, name)
	if err != nil {
		return false
	}
	return view != nil && view.Writable
}

// --- webdav.FileSystem ------------------------------------------------------

// Stat implements webdav.FileSystem.Stat.
func (fs *VirtualFS) Stat(ctx context.Context, name string) (os.FileInfo, error) {
	view, node, err := fs.resolveNode(ctx, name)
	if err != nil {
		return nil, err
	}
	if node == nil {
		return &virtualFileInfo{name: view.Slug, mode: os.ModeDir | 0o755, modTime: view.CreatedAt}, nil
	}
	switch node.NodeType {
	case "folder", "rule_folder":
		return &virtualFileInfo{name: node.Name, mode: os.ModeDir | 0o755, modTime: node.CreatedAt}, nil
	case "file_ref":
		if node.FileID == nil {
			return nil, os.ErrNotExist
		}
		file, err := fs.getFile(ctx, *node.FileID)
		if err != nil {
			return nil, err
		}
		if file == nil {
			return nil, os.ErrNotExist
		}
		return &virtualFileInfo{name: node.Name, size: file.SizeBytes, mode: 0o644, modTime: file.CreatedAt}, nil
	}
	return nil, os.ErrNotExist
}

// OpenFile implements webdav.FileSystem.OpenFile. Read-only opens resolve the
// virtual tree; write opens are only allowed on writable views and delegate to
// the real manual-upload directory.
func (fs *VirtualFS) OpenFile(ctx context.Context, name string, flag int, perm os.FileMode) (webdav.File, error) {
	if flag&(os.O_WRONLY|os.O_RDWR) != 0 {
		return fs.openWrite(ctx, name, flag, perm)
	}

	view, node, err := fs.resolveNode(ctx, name)
	if err != nil {
		return nil, err
	}
	if node == nil {
		return fs.openDir(ctx, view, nil), nil
	}
	switch node.NodeType {
	case "folder", "rule_folder":
		return fs.openDir(ctx, view, node), nil
	case "file_ref":
		if node.FileID == nil {
			return nil, os.ErrNotExist
		}
		file, err := fs.getFile(ctx, *node.FileID)
		if err != nil {
			return nil, err
		}
		if file == nil {
			return nil, os.ErrNotExist
		}
		fullPath := filepath.Join(fs.storagePath, file.LocalPath)
		if !fs.withinStorage(fullPath) {
			return nil, os.ErrNotExist
		}
		f, err := os.Open(fullPath)
		if err != nil {
			return nil, err
		}
		return &virtualFile{file: f, name: node.Name}, nil
	}
	return nil, os.ErrNotExist
}

// Mkdir implements webdav.FileSystem.Mkdir. Only writable views may create
// directories; the physical directory is created under manual_uploads.
func (fs *VirtualFS) Mkdir(ctx context.Context, name string, perm os.FileMode) error {
	view, rest, err := fs.resolveView(ctx, name)
	if err != nil {
		return err
	}
	if !view.Writable {
		return os.ErrPermission
	}
	if rest == "" {
		return os.ErrInvalid
	}
	dir := webdav.Dir(filepath.Join(fs.storagePath, manualUploadsSubdir))
	if err := dir.Mkdir(ctx, rest, perm); err != nil {
		return err
	}
	// Reflect the new directory in the virtual tree so it is visible on read.
	if cerr := fs.createFolderNode(ctx, view, rest); cerr != nil {
		fs.logger.Warn("vfs: creating folder node failed", "path", rest, "error", cerr)
	}
	return nil
}

// RemoveAll implements webdav.FileSystem.RemoveAll. Only writable views may be
// modified; the physical removal is delegated to the manual-upload directory.
func (fs *VirtualFS) RemoveAll(ctx context.Context, name string) error {
	view, rest, err := fs.resolveView(ctx, name)
	if err != nil {
		return err
	}
	if !view.Writable {
		return os.ErrPermission
	}
	if rest == "" {
		return os.ErrInvalid
	}
	dir := webdav.Dir(filepath.Join(fs.storagePath, manualUploadsSubdir))
	if err := dir.RemoveAll(ctx, rest); err != nil {
		return err
	}
	// Soft-delete the underlying file if the removed node is a file reference.
	if _, node, rerr := fs.resolveNode(ctx, name); rerr == nil && node != nil && node.NodeType == "file_ref" && node.FileID != nil {
		if serr := service.SoftDelete(ctx, fs.writeDB, fs.bus, *node.FileID); serr != nil {
			fs.logger.Warn("vfs: soft-deleting file failed", "file_id", *node.FileID, "error", serr)
		}
	}
	return nil
}

// Rename implements webdav.FileSystem.Rename. Only writable views may be
// modified; the physical rename is delegated to the manual-upload directory.
func (fs *VirtualFS) Rename(ctx context.Context, oldName, newName string) error {
	view, oldRest, err := fs.resolveView(ctx, oldName)
	if err != nil {
		return err
	}
	if !view.Writable {
		return os.ErrPermission
	}
	_, newRest, err := fs.resolveView(ctx, newName)
	if err != nil {
		return err
	}
	if oldRest == "" || newRest == "" {
		return os.ErrInvalid
	}
	dir := webdav.Dir(filepath.Join(fs.storagePath, manualUploadsSubdir))
	if err := dir.Rename(ctx, oldRest, newRest); err != nil {
		return err
	}
	// Update the node name to match the new path.
	if _, node, rerr := fs.resolveNode(ctx, oldName); rerr == nil && node != nil {
		node.Name = path.Base(newRest)
		if uerr := fs.virtualRepo.UpdateNode(ctx, node); uerr != nil {
			fs.logger.Warn("vfs: updating node name after rename failed", "node_id", node.ID, "error", uerr)
		}
	}
	return nil
}

// --- Read helpers -----------------------------------------------------------

// getFile returns the file row for a file ID, or nil if it does not exist or
// has been soft-deleted.
func (fs *VirtualFS) getFile(ctx context.Context, fileID int64) (*db.File, error) {
	var f db.File
	err := fs.readDB.QueryRowContext(ctx,
		`SELECT id, project_id, version, filename, os, arch, ext, size_bytes, download_url,
		        local_path, checksum, status, error_message, created_at, retry_count, last_attempt_at,
		        source_type, category, storage_subdir, public_token
		 FROM files WHERE id = ?`, fileID).
		Scan(&f.ID, &f.ProjectID, &f.Version, &f.Filename, &f.OS, &f.Arch, &f.Ext, &f.SizeBytes,
			&f.DownloadURL, &f.LocalPath, &f.Checksum, &f.Status, &f.ErrorMessage, &f.CreatedAt, &f.RetryCount,
			&f.LastAttemptAt, &f.SourceType, &f.Category, &f.StorageSubdir, &f.PublicToken)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("vfs: reading file %d: %w", fileID, err)
	}
	if f.Status == "deleted" {
		return nil, nil
	}
	return &f, nil
}

// withinStorage reports whether a resolved physical path stays within the
// configured storage root (defense against path traversal).
func (fs *VirtualFS) withinStorage(fullPath string) bool {
	root := filepath.Clean(fs.storagePath)
	clean := filepath.Clean(fullPath)
	return clean == root || strings.HasPrefix(clean, root+string(os.PathSeparator))
}

// openDir returns a directory File whose Readdir lists the virtual children of
// the given node (or the view root when node is nil).
func (fs *VirtualFS) openDir(ctx context.Context, view *db.View, node *db.Node) webdav.File {
	children, err := fs.listChildren(ctx, view, node)
	return &virtualDir{view: view, node: node, children: children, err: err}
}

// listChildren returns the virtual child entries of a node (or view root).
// For rule_folder nodes it additionally lists the matched rule entries.
func (fs *VirtualFS) listChildren(ctx context.Context, view *db.View, node *db.Node) ([]os.FileInfo, error) {
	var parentID *int64
	if node != nil {
		parentID = &node.ID
	}
	nodes, err := fs.virtualRepo.ListNodesByParent(ctx, view.ID, parentID)
	if err != nil {
		return nil, fmt.Errorf("vfs: listing children: %w", err)
	}
	var infos []os.FileInfo
	for _, n := range nodes {
		if n.Status != "visible" {
			continue
		}
		switch n.NodeType {
		case "folder", "rule_folder":
			infos = append(infos, &virtualFileInfo{name: n.Name, mode: os.ModeDir | 0o755, modTime: n.CreatedAt})
		case "file_ref":
			if n.FileID == nil {
				continue
			}
			file, err := fs.getFile(ctx, *n.FileID)
			if err != nil {
				return nil, err
			}
			if file == nil {
				continue
			}
			infos = append(infos, &virtualFileInfo{name: n.Name, size: file.SizeBytes, mode: 0o644, modTime: file.CreatedAt})
		}
	}
	if node != nil && node.NodeType == "rule_folder" {
		entries, err := fs.listRuleEntries(ctx, node.ID)
		if err != nil {
			return nil, err
		}
		infos = append(infos, entries...)
	}
	return infos, nil
}

// listRuleEntries returns the matched files of a rule folder as virtual entries.
func (fs *VirtualFS) listRuleEntries(ctx context.Context, ruleNodeID int64) ([]os.FileInfo, error) {
	rows, err := fs.readDB.QueryContext(ctx,
		`SELECT f.filename, f.size_bytes, f.created_at
		 FROM virtual_rule_entries re
		 JOIN files f ON f.id = re.file_id
		 WHERE re.rule_node_id = ? AND f.status != 'deleted'`, ruleNodeID)
	if err != nil {
		return nil, fmt.Errorf("vfs: listing rule entries: %w", err)
	}
	defer rows.Close()
	var infos []os.FileInfo
	for rows.Next() {
		var filename string
		var size int64
		var createdAt time.Time
		if err := rows.Scan(&filename, &size, &createdAt); err != nil {
			return nil, fmt.Errorf("vfs: scanning rule entry: %w", err)
		}
		infos = append(infos, &virtualFileInfo{name: filename, size: size, mode: 0o644, modTime: createdAt})
	}
	return infos, rows.Err()
}

// openWrite handles write opens (PUT). It checks the view is writable, then
// delegates to the real manual-upload directory. The returned file registers
// the uploaded file in the file registry when it is closed.
func (fs *VirtualFS) openWrite(ctx context.Context, name string, flag int, perm os.FileMode) (webdav.File, error) {
	view, rest, err := fs.resolveView(ctx, name)
	if err != nil {
		return nil, err
	}
	if !view.Writable {
		return nil, os.ErrPermission
	}
	if rest == "" {
		return nil, os.ErrInvalid
	}
	dir := webdav.Dir(filepath.Join(fs.storagePath, manualUploadsSubdir))
	f, err := dir.OpenFile(ctx, rest, flag, perm)
	if err != nil {
		return nil, err
	}
	// local_path is stored in the DB in POSIX form (convention across the
	// codebase); rest is already slash-separated.
	localPath := path.Join(manualUploadsSubdir, rest)
	return &manualUploadFile{File: f, fs: fs, ctx: ctx, view: view, localPath: localPath}, nil
}

// createFolderNode reflects a newly created directory in the virtual tree.
func (fs *VirtualFS) createFolderNode(ctx context.Context, view *db.View, rest string) error {
	parent, err := fs.resolveParentNode(ctx, view, path.Dir(rest))
	if err != nil {
		return err
	}
	name := path.Base(rest)
	if fs.nodeExists(ctx, view.ID, parent, name) {
		return nil
	}
	_, err = fs.virtualRepo.CreateNode(ctx, &db.Node{
		ViewID:   view.ID,
		ParentID: parent,
		Name:     name,
		NodeType: "folder",
		Status:   "visible",
	})
	return err
}

// createFileRefNode registers a newly uploaded file as a file_ref node.
func (fs *VirtualFS) createFileRefNode(ctx context.Context, view *db.View, rest string, fileID int64) error {
	parent, err := fs.resolveParentNode(ctx, view, path.Dir(rest))
	if err != nil {
		return err
	}
	name := path.Base(rest)
	if fs.nodeExists(ctx, view.ID, parent, name) {
		return nil
	}
	_, err = fs.virtualRepo.CreateNode(ctx, &db.Node{
		ViewID:   view.ID,
		ParentID: parent,
		Name:     name,
		NodeType: "file_ref",
		FileID:   &fileID,
		Status:   "visible",
	})
	return err
}

// resolveParentNode resolves the parent node for a directory path relative to
// the view root. It returns nil when the parent is the view root or does not
// exist as a node.
func (fs *VirtualFS) resolveParentNode(ctx context.Context, view *db.View, dirPath string) (*int64, error) {
	if dirPath == "" || dirPath == "." || dirPath == "/" {
		return nil, nil
	}
	var node *db.Node
	var parentID *int64
	for _, seg := range strings.Split(path.Clean(dirPath), "/") {
		if seg == "" {
			continue
		}
		children, err := fs.virtualRepo.ListNodesByParent(ctx, view.ID, parentID)
		if err != nil {
			return nil, err
		}
		var found *db.Node
		for _, c := range children {
			if c.Status == "visible" && c.Name == seg {
				found = c
				break
			}
		}
		if found == nil {
			return nil, nil
		}
		node = found
		parentID = &found.ID
	}
	if node == nil {
		return nil, nil
	}
	return &node.ID, nil
}

// nodeExists reports whether a visible node with the given name already exists
// under the parent within a view.
func (fs *VirtualFS) nodeExists(ctx context.Context, viewID int64, parentID *int64, name string) bool {
	children, err := fs.virtualRepo.ListNodesByParent(ctx, viewID, parentID)
	if err != nil {
		return false
	}
	for _, c := range children {
		if c.Status == "visible" && c.Name == name {
			return true
		}
	}
	return false
}

// --- File implementations ----------------------------------------------------

// virtualFileInfo implements os.FileInfo for virtual nodes.
type virtualFileInfo struct {
	name    string
	size    int64
	mode    os.FileMode
	modTime time.Time
}

func (f *virtualFileInfo) Name() string       { return f.name }
func (f *virtualFileInfo) Size() int64        { return f.size }
func (f *virtualFileInfo) Mode() os.FileMode  { return f.mode }
func (f *virtualFileInfo) ModTime() time.Time { return f.modTime }
func (f *virtualFileInfo) IsDir() bool        { return f.mode.IsDir() }
func (f *virtualFileInfo) Sys() interface{}   { return nil }

// virtualFile wraps an *os.File for a file_ref node. Read/Seek/Write delegate
// directly to the underlying file (preserving sendfile zero-copy); Readdir
// returns an error because a file is not a directory.
type virtualFile struct {
	file *os.File
	name string
}

func (f *virtualFile) Close() error                     { return f.file.Close() }
func (f *virtualFile) Read(p []byte) (int, error)       { return f.file.Read(p) }
func (f *virtualFile) Seek(o int64, w int) (int64, error) { return f.file.Seek(o, w) }
func (f *virtualFile) Write(p []byte) (int, error)      { return f.file.Write(p) }
func (f *virtualFile) Readdir(count int) ([]os.FileInfo, error) {
	return nil, os.ErrInvalid
}
func (f *virtualFile) Stat() (os.FileInfo, error) { return f.file.Stat() }

// virtualDir is a directory File whose Readdir lists virtual children.
type virtualDir struct {
	view     *db.View
	node     *db.Node
	children []os.FileInfo
	err      error
	pos      int
}

func (d *virtualDir) Close() error { return nil }
func (d *virtualDir) Read(p []byte) (int, error) {
	return 0, os.ErrInvalid
}
func (d *virtualDir) Seek(o int64, w int) (int64, error) {
	return 0, os.ErrInvalid
}
func (d *virtualDir) Write(p []byte) (int, error) {
	return 0, os.ErrInvalid
}
func (d *virtualDir) Stat() (os.FileInfo, error) {
	if d.node == nil {
		return &virtualFileInfo{name: d.view.Slug, mode: os.ModeDir | 0o755, modTime: d.view.CreatedAt}, nil
	}
	return &virtualFileInfo{name: d.node.Name, mode: os.ModeDir | 0o755, modTime: d.node.CreatedAt}, nil
}
func (d *virtualDir) Readdir(count int) ([]os.FileInfo, error) {
	if d.err != nil {
		return nil, d.err
	}
	if d.pos >= len(d.children) {
		if count > 0 {
			return nil, io.EOF
		}
		return nil, nil
	}
	if count > 0 {
		end := d.pos + count
		if end > len(d.children) {
			end = len(d.children)
		}
		out := d.children[d.pos:end]
		d.pos = end
		return out, nil
	}
	out := d.children[d.pos:]
	d.pos = len(d.children)
	return out, nil
}

// manualUploadFile wraps a file opened for write in a writable view. On Close
// it registers the uploaded file in the file registry and creates a file_ref
// node so the upload becomes visible through the virtual tree.
type manualUploadFile struct {
	webdav.File
	fs         *VirtualFS
	ctx        context.Context
	view       *db.View
	localPath  string
	registered bool
}

func (f *manualUploadFile) Close() error {
	err := f.File.Close()
	if err != nil || f.registered {
		return err
	}
	f.registered = true
	fullPath := filepath.Join(f.fs.storagePath, f.localPath)
	info, serr := os.Stat(fullPath)
	if serr != nil {
		f.fs.logger.Warn("vfs: stat manual upload failed", "path", f.localPath, "error", serr)
		return err
	}
	projectID, perr := f.fs.getOrCreateManualProject()
	if perr != nil {
		f.fs.logger.Warn("vfs: manual project lookup failed", "error", perr)
		return err
	}
	fileID, rerr := service.RegisterFile(
		f.ctx, f.fs.writeDB, f.fs.bus,
		"manual_upload", "manual", "webdav", f.localPath,
		projectID, filepath.Base(f.localPath), "", info.Size(), "",
	)
	if rerr != nil {
		f.fs.logger.Warn("vfs: registering manual upload failed", "path", f.localPath, "error", rerr)
		return err
	}
	if cerr := f.fs.createFileRefNode(f.ctx, f.view, strings.TrimPrefix(f.localPath, manualUploadsSubdir+"/"), fileID); cerr != nil {
		f.fs.logger.Warn("vfs: creating file_ref node failed", "path", f.localPath, "error", cerr)
	}
	return err
}

// --- HTTP handler ------------------------------------------------------------

// noLockSystem is a no-op LockSystem that disables WebDAV locking.
type noLockSystem struct{}

func (noLockSystem) Confirm(now time.Time, name0, name1 string, conditions ...webdav.Condition) (func(), error) {
	return func() {}, nil
}
func (noLockSystem) Create(now time.Time, details webdav.LockDetails) (string, error) {
	return "opaquelocktoken:noop", nil
}
func (noLockSystem) Refresh(now time.Time, token string, duration time.Duration) (webdav.LockDetails, error) {
	return webdav.LockDetails{}, nil
}
func (noLockSystem) Unlock(now time.Time, token string) error {
	return nil
}

// VirtualHandler wraps the golang.org/x/net/webdav.Handler with a virtual
// filesystem, disables locking, rejects writes on non-writable views with 403,
// and truncates depth=infinity PROPFIND to depth 1 (DoS guard).
type VirtualHandler struct {
	fs      *VirtualFS
	handler *webdav.Handler
}

// NewVirtualHandler creates a VirtualHandler over the given VirtualFS.
func NewVirtualHandler(fs *VirtualFS) *VirtualHandler {
	return &VirtualHandler{
		fs: fs,
		handler: &webdav.Handler{
			FileSystem: fs,
			LockSystem: noLockSystem{},
		},
	}
}

// ServeHTTP implements http.Handler.
func (h *VirtualHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if isWriteMethod(r.Method) && !h.fs.checkWritable(r.Context(), r.URL.Path) {
		http.Error(w, "view is read-only", http.StatusForbidden)
		return
	}
	// Truncate depth=infinity PROPFIND to depth 1 to prevent unbounded recursion.
	if r.Method == "PROPFIND" && r.Header.Get("Depth") == "infinity" {
		r.Header.Set("Depth", "1")
	}
	h.handler.ServeHTTP(w, r)
}

// isWriteMethod reports whether the HTTP method mutates the filesystem.
func isWriteMethod(method string) bool {
	switch method {
	case http.MethodPut, http.MethodDelete, "MKCOL", "MOVE", "COPY", "PROPPATCH", "LOCK", "UNLOCK":
		return true
	}
	return false
}