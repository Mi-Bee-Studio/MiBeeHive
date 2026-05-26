package handler

import (
	"fmt"
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
	host := h.config.Server.BindAddr
	if host == "" || host == "0.0.0.0" {
		host = "localhost"
	}
	httpURL := fmt.Sprintf("http://%s:%d/webdav/", host, h.config.Server.Port)
	resp := model.WebDAVStatusResponse{
		Enabled:     true,
		HTTPURL:     httpURL,
		StoragePath: h.resolver.ResolveWebDAV(),
	}
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
