package handler

import (
	"net/http"
	"path/filepath"
	"strconv"

	"github.com/Mi-Bee-Studio/mibeehive/internal/config"
	"github.com/Mi-Bee-Studio/mibeehive/internal/crawler"

	dbrepo "github.com/Mi-Bee-Studio/mibeehive/internal/db"
)

// AdminHandler handles admin API endpoints.
// It delegates to focused sub-handlers for project management, crawl control,
// configuration, and WebDAV admin.
type AdminHandler struct {
	*ProjectAdminHandler
	*CrawlControlHandler
	*ConfigHandler
	*WebDAVAdminHandler
}

// NewAdminHandler creates a new AdminHandler with all sub-handlers.
func NewAdminHandler(projectRepo *dbrepo.ProjectRepo, fileRepo *dbrepo.FileRepo, credRepo *dbrepo.SourceCredentialRepo, crawlManager *crawler.CrawlManager, cfg *config.Config, configPath string) *AdminHandler {
	return &AdminHandler{
		ProjectAdminHandler: NewProjectAdminHandler(projectRepo, fileRepo, crawlManager, cfg),
		CrawlControlHandler: NewCrawlControlHandler(projectRepo, credRepo, crawlManager),
		ConfigHandler:       NewConfigHandler(cfg, configPath),
		WebDAVAdminHandler:  NewWebDAVAdminHandler(cfg),
	}
}

// webdavFileEntry represents a single file or directory in a WebDAV listing.
type webdavFileEntry struct {
	Name    string `json:"name"`
	Size    int64  `json:"size"`
	IsDir   bool   `json:"is_dir"`
	ModTime string `json:"mod_time"`
}

// webdavListingResponse is the response payload for WebDAV directory listings.
type webdavListingResponse struct {
	CurrentPath string            `json:"currentPath"`
	ParentPath  *string           `json:"parentPath"`
	Files       []webdavFileEntry `json:"files"`
	Truncated   bool              `json:"truncated,omitempty"`
}

// paginateWebdavEntries applies limit/offset pagination to file entries.
func paginateWebdavEntries(entries []webdavFileEntry, limit, offset int) ([]webdavFileEntry, bool) {
	truncated := false
	end := offset + limit
	if end > len(entries) {
		end = len(entries)
	} else if end < len(entries) {
		truncated = true
	}
	if offset > len(entries) {
		offset = len(entries)
	}
	return entries[offset:end], truncated
}

// parseWebdavPagination extracts limit and offset from query parameters.
func parseWebdavPagination(r *http.Request) (int, int) {
	limit := 500
	offset := 0
	if l, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && l > 0 && l <= 1000 {
		limit = l
	}
	if o, err := strconv.Atoi(r.URL.Query().Get("offset")); err == nil && o >= 0 {
		offset = o
	}
	return limit, offset
}

// computeWebdavParentPath returns the parent path for navigation, or nil if at root.
func computeWebdavParentPath(subPath string) *string {
	if subPath == "" {
		return nil
	}
	p := filepath.Dir(subPath)
	if p == "." {
		empty := ""
		return &empty
	}
	return &p
}

// maskToken returns the token with all but the last 4 characters replaced by asterisks.
func maskToken(token string) string {
	if len(token) <= 4 {
		return "****"
	}
	masked := make([]byte, len(token))
	for i := range masked {
		masked[i] = '*'
	}
	copy(masked[len(masked)-4:], token[len(token)-4:])
	return string(masked)
}
