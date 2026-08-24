package handler

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/Mi-Bee-Studio/mibeehive/internal/config"
	"github.com/Mi-Bee-Studio/mibeehive/internal/middleware"
	"github.com/Mi-Bee-Studio/mibeehive/internal/model"
	"github.com/Mi-Bee-Studio/mibeehive/internal/service"
)

// WebDAVAdminHandler handles admin WebDAV status and file listing endpoints.
type WebDAVAdminHandler struct {
	config   *config.Config
	resolver *service.StorageResolver
}

// NewWebDAVAdminHandler creates a new WebDAVAdminHandler.
func NewWebDAVAdminHandler(cfg *config.Config, resolver *service.StorageResolver) *WebDAVAdminHandler {
	return &WebDAVAdminHandler{
		config:   cfg,
		resolver: resolver,
	}
}

// WebDAVStatus handles GET /api/v1/admin/webdav/status.
func (h *WebDAVAdminHandler) WebDAVStatus(w http.ResponseWriter, r *http.Request) {
	// The generated URLs are connection guides pasted into OTHER machines'
	// WebDAV clients, so they must be reachable externally. The server's own
	// BindAddr is typically "0.0.0.0" (or empty), which is useless for that —
	// use the hostname the admin is currently browsing from (r.Host) instead,
	// since that is by definition an address that reaches this server.
	host := r.Host
	if host == "" {
		host = h.config.Server.BindAddr
	}
	// Strip any port from r.Host; we append the configured port below.
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	if host == "" || host == "0.0.0.0" {
		host = "localhost"
	}
	// WebDAV is HTTPS-only: the HTTP handler rejects /webdav with 404, so no
	// http:// URL is advertised. The service is reachable only when the HTTPS
	// port is configured.
	resp := model.WebDAVStatusResponse{Enabled: h.config.Server.HTTPSPort > 0}
	if h.config.Server.HTTPSPort > 0 {
		resp.HTTPSURL = fmt.Sprintf("https://%s:%d/webdav/", host, h.config.Server.HTTPSPort)
	}
	writeJSON(w, http.StatusOK, model.ApiResponse[model.WebDAVStatusResponse]{
		Success: true,
		Data:    resp,
	})
}

// WebDAVFileList handles GET /api/v1/admin/webdav/files.
// It lists files in the WebDAV storage directory with recursive navigation.
// Accepts optional ?path=subdirectory query parameter for browsing subdirectories.
func (h *WebDAVAdminHandler) WebDAVFileList(w http.ResponseWriter, r *http.Request) {
	webdavPath := h.resolver.ResolveWebDAV()
	absWebdavRoot, err := filepath.Abs(webdavPath)
	if err != nil {
		middleware.WriteError(w, http.StatusInternalServerError, model.ERR_INTERNAL, "操作失败，请稍后重试", err)
		return
	}
	subPath, absTarget, err := h.validateWebDAVPath(r.URL.Query().Get("path"), webdavPath, absWebdavRoot)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, model.ApiResponse[any]{Success: false, Message: err.Error()})
		return
	}
	files, err := h.readWebdavDir(absTarget)
	if err != nil {
		middleware.WriteError(w, http.StatusInternalServerError, model.ERR_INTERNAL, "操作失败，请稍后重试", err)
		return
	}

	limit, offset := parseWebdavPagination(r)
	paged, truncated := paginateWebdavEntries(files, limit, offset)
	writeJSON(w, http.StatusOK, model.ApiResponse[webdavListingResponse]{
		Success: true,
		Data: webdavListingResponse{
			CurrentPath: subPath,
			ParentPath:  computeWebdavParentPath(subPath),
			Files:       paged,
			Truncated:   truncated,
		},
	})
}

// validateWebDAVPath sanitizes the query path and ensures it stays within the WebDAV root.
func (h *WebDAVAdminHandler) validateWebDAVPath(rawQuery, webdavPath, absWebdavRoot string) (string, string, error) {
	var sub string
	if rawQuery != "" {
		decoded, err := url.QueryUnescape(rawQuery)
		if err != nil {
			return "", "", fmt.Errorf("invalid path encoding")
		}
		cleaned := filepath.Clean(decoded)
		if strings.Contains(cleaned, "..") {
			return "", "", fmt.Errorf("path traversal not allowed")
		}
		if cleaned == "." || cleaned == "/" {
			sub = ""
		} else {
			sub = strings.TrimPrefix(cleaned, "/")
		}
	}
	targetPath := filepath.Join(webdavPath, sub)
	abs, err := filepath.Abs(targetPath)
	if err != nil {
		return "", "", fmt.Errorf("invalid path")
	}
	if !strings.HasPrefix(abs+string(filepath.Separator), absWebdavRoot+string(filepath.Separator)) && abs != absWebdavRoot {
		return "", "", fmt.Errorf("path traversal not allowed")
	}
	return sub, abs, nil
}

// readWebdavDir reads the directory at targetPath and returns file entries.
func (h *WebDAVAdminHandler) readWebdavDir(targetPath string) ([]webdavFileEntry, error) {
	entries, err := os.ReadDir(targetPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []webdavFileEntry{}, nil
		}
		return nil, err
	}
	var files []webdavFileEntry
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}
		files = append(files, webdavFileEntry{
			Name:    entry.Name(),
			Size:    info.Size(),
			IsDir:   entry.IsDir(),
			ModTime: info.ModTime().Format("2006-01-02 15:04:05"),
		})
	}
	if files == nil {
		files = []webdavFileEntry{}
	}
	return files, nil
}
