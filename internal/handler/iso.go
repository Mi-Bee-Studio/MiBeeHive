package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/Mi-Bee-Studio/mibeehive/internal/middleware"
	"github.com/Mi-Bee-Studio/mibeehive/internal/model"
	"github.com/Mi-Bee-Studio/mibeehive/internal/service"
	"github.com/golang-jwt/jwt/v5"
)

// ISOHandler handles admin API endpoints for ISO file management.
type ISOHandler struct {
	isoService     *service.ISOService
	catalogService *service.ISOCatalogService
	jwtSecret      string
}

// NewISOHandler creates a new ISOHandler.
func NewISOHandler(isoService *service.ISOService, catalogService *service.ISOCatalogService, jwtSecret string) *ISOHandler {
	return &ISOHandler{isoService: isoService, catalogService: catalogService, jwtSecret: jwtSecret}
}

type isoDownloadRequest struct {
	Filename string `json:"filename"`
	URL      string `json:"url"`
}

// TriggerDownload handles POST /api/v1/admin/os-install/iso/download.
// It validates inputs, checks disk space, and starts the download in a
// background goroutine, returning 202 Accepted immediately.
func (h *ISOHandler) TriggerDownload(w http.ResponseWriter, r *http.Request) {
	var req isoDownloadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, model.ApiResponse[any]{
			Success: false,
			Message: "invalid request body",
		})
		return
	}

	if req.Filename == "" {
		writeJSON(w, http.StatusBadRequest, model.ApiResponse[any]{
			Success: false,
			Message: "filename is required",
		})
		return
	}
	if strings.Contains(req.Filename, "..") || strings.ContainsAny(req.Filename, "/\\") {
		writeJSON(w, http.StatusBadRequest, model.ApiResponse[any]{
			Success: false,
			Message: "filename contains invalid characters",
		})
		return
	}

	if req.URL == "" {
		writeJSON(w, http.StatusBadRequest, model.ApiResponse[any]{
			Success: false,
			Message: "url is required",
		})
		return
	}
	parsedURL, err := url.Parse(req.URL)
	if err != nil || (parsedURL.Scheme != "http" && parsedURL.Scheme != "https") {
		writeJSON(w, http.StatusBadRequest, model.ApiResponse[any]{
			Success: false,
			Message: "url must be a valid http(s) URL",
		})
		return
	}

	// Check if file already exists.
	existing, err := h.isoService.ListISOs()
	if err != nil {
		slog.Warn("failed to list ISOs for duplicate check", "error", err)
	} else {
		for _, iso := range existing {
			if iso.Name == req.Filename {
				writeJSON(w, http.StatusConflict, model.ApiResponse[any]{
					Success: false,
					Message: fmt.Sprintf("ISO file %q already exists", req.Filename),
				})
				return
			}
		}
	}

	// Launch download in background.
	go func() {
		if err := h.isoService.DownloadISO(r.Context(), req.Filename, req.URL, ""); err != nil {
			slog.Error("ISO download failed", "filename", req.Filename, "error", err)
		}
	}()

	writeJSON(w, http.StatusAccepted, model.ApiResponse[any]{
		Success: true,
		Message: fmt.Sprintf("download started for %q", req.Filename),
	})
}

// ListISOs handles GET /api/v1/admin/os-install/isos.
func (h *ISOHandler) ListISOs(w http.ResponseWriter, r *http.Request) {
	isos, err := h.isoService.ListISOs()
	if err != nil {
		middleware.WriteError(w, http.StatusInternalServerError, model.ERR_INTERNAL, "failed to list ISOs", err)
		return
	}

	writeJSON(w, http.StatusOK, model.ApiResponse[[]service.ISOInfo]{
		Success: true,
		Data:    isos,
	})
}

// DeleteISO handles DELETE /api/v1/admin/os-install/isos/{name}.
func (h *ISOHandler) DeleteISO(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		writeJSON(w, http.StatusBadRequest, model.ApiResponse[any]{
			Success: false,
			Message: "ISO name is required",
		})
		return
	}

	if err := h.isoService.DeleteISO(name); err != nil {
		middleware.WriteError(w, http.StatusNotFound, model.ERR_NOT_FOUND, "ISO file not found", err)
		return
	}

	writeJSON(w, http.StatusOK, model.ApiResponse[any]{
		Success: true,
		Message: fmt.Sprintf("ISO %q deleted", name),
	})
}

// ISOCatalogList handles GET /api/v1/admin/os-install/catalog.
func (h *ISOHandler) ISOCatalogList(w http.ResponseWriter, r *http.Request) {
	entries, err := h.catalogService.ListCatalog(r.Context())
	if err != nil {
		middleware.WriteError(w, http.StatusInternalServerError, model.ERR_INTERNAL, "failed to list catalog entries", err)
		return
	}

	writeJSON(w, http.StatusOK, model.ApiResponse[[]model.ISOCatalogEntry]{
		Success: true,
		Data:    entries,
	})
}

// ISOCatalogCreate handles POST /api/v1/admin/os-install/catalog.
func (h *ISOHandler) ISOCatalogCreate(w http.ResponseWriter, r *http.Request) {
	var req model.ISOCatalogCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, model.ApiResponse[any]{
			Success: false,
			Message: "invalid request body",
		})
		return
	}

	var missingFields []string
	if req.Name == "" {
		missingFields = append(missingFields, "name")
	}
	if req.Distro == "" {
		missingFields = append(missingFields, "distro")
	}
	if req.Arch == "" {
		missingFields = append(missingFields, "arch")
	}
	if req.CheckURL == "" && req.BaseURL == "" {
		missingFields = append(missingFields, "check_url or base_url")
	}
	if req.FilenamePattern == "" {
		missingFields = append(missingFields, "filename_pattern")
	}
	if len(missingFields) > 0 {
		writeJSON(w, http.StatusBadRequest, model.ApiResponse[any]{
			Success: false,
			Message: fmt.Sprintf("missing required fields: %s", strings.Join(missingFields, ", ")),
		})
		return
	}

	id, err := h.catalogService.CreateCatalogEntry(r.Context(), req)
	if err != nil {
		middleware.WriteError(w, http.StatusInternalServerError, model.ERR_INTERNAL, "failed to create catalog entry", err)
		return
	}

	writeJSON(w, http.StatusCreated, model.ApiResponse[any]{
		Success: true,
		Message: fmt.Sprintf("catalog entry created with id %d", id),
	})
}

// ISOCatalogUpdate handles PUT /api/v1/admin/os-install/catalog/{id}.
func (h *ISOHandler) ISOCatalogUpdate(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, model.ApiResponse[any]{
			Success: false,
			Message: "invalid catalog entry id",
		})
		return
	}

	var req model.ISOCatalogUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, model.ApiResponse[any]{
			Success: false,
			Message: "invalid request body",
		})
		return
	}

	if err := h.catalogService.UpdateCatalogEntry(r.Context(), id, req); err != nil {
		middleware.WriteError(w, http.StatusInternalServerError, model.ERR_INTERNAL, "failed to update catalog entry", err)
		return
	}

	writeJSON(w, http.StatusOK, model.ApiResponse[any]{
		Success: true,
		Message: "catalog entry updated",
	})
}

// ISOCatalogDelete handles DELETE /api/v1/admin/os-install/catalog/{id}.
func (h *ISOHandler) ISOCatalogDelete(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, model.ApiResponse[any]{
			Success: false,
			Message: "invalid catalog entry id",
		})
		return
	}

	if err := h.catalogService.DeleteCatalogEntry(r.Context(), id); err != nil {
		middleware.WriteError(w, http.StatusInternalServerError, model.ERR_INTERNAL, "failed to delete catalog entry", err)
		return
	}

	writeJSON(w, http.StatusOK, model.ApiResponse[any]{
		Success: true,
		Message: "catalog entry deleted",
	})
}

// ISOCatalogCheck handles POST /api/v1/admin/os-install/catalog/{id}/check.
func (h *ISOHandler) ISOCatalogCheck(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, model.ApiResponse[any]{
			Success: false,
			Message: "invalid catalog entry id",
		})
		return
	}

	result, err := h.catalogService.CheckVersion(r.Context(), id)
	if err != nil {
		middleware.WriteError(w, http.StatusInternalServerError, model.ERR_INTERNAL, "failed to check version", err)
		return
	}

	writeJSON(w, http.StatusOK, model.ApiResponse[*model.ISOCatalogCheckResponse]{
		Success: true,
		Data:    result,
	})
}

// ISOCatalogDownload handles POST /api/v1/admin/os-install/catalog/{id}/download.
func (h *ISOHandler) ISOCatalogDownload(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, model.ApiResponse[any]{
			Success: false,
			Message: "invalid catalog entry id",
		})
		return
	}

	if err := h.catalogService.DownloadFromCatalog(r.Context(), id); err != nil {
		var diskErr *service.InsufficientStorageError
		if errors.As(err, &diskErr) {
			writeJSON(w, http.StatusInsufficientStorage, model.ApiResponse[any]{
				Success: false,
				Message: diskErr.Error(),
			})
			return
		}
		middleware.WriteError(w, http.StatusInternalServerError, model.ERR_INTERNAL, "failed to download from catalog", err)
		return
	}

	writeJSON(w, http.StatusOK, model.ApiResponse[any]{
		Success: true,
		Message: "download completed",
	})
}

// ISOCatalogRetry handles POST /api/v1/admin/os-install/catalog/{id}/retry.
func (h *ISOHandler) ISOCatalogRetry(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, model.ApiResponse[any]{
			Success: false,
			Message: "invalid catalog entry id",
		})
		return
	}

	if err := h.catalogService.RetryCatalogDownload(r.Context(), id); err != nil {
		errMsg := err.Error()
		if strings.Contains(errMsg, "not found") {
			writeJSON(w, http.StatusNotFound, model.ApiResponse[any]{
				Success: false,
				Message: "catalog entry not found",
			})
			return
		}
		if strings.Contains(errMsg, "expected") {
			writeJSON(w, http.StatusBadRequest, model.ApiResponse[any]{
				Success: false,
				Message: "catalog entry status must be 'error' to retry",
			})
			return
		}
		middleware.WriteError(w, http.StatusInternalServerError, model.ERR_INTERNAL, "failed to retry download", err)
		return
	}

	writeJSON(w, http.StatusOK, model.ApiResponse[any]{
		Success: true,
		Message: "retry queued",
	})
}

// ISOCatalogCancel handles POST /api/v1/admin/os-install/catalog/{id}/cancel.
func (h *ISOHandler) ISOCatalogCancel(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, model.ApiResponse[any]{
			Success: false,
			Message: "invalid catalog entry id",
		})
		return
	}

	err = h.catalogService.CancelDownload(r.Context(), id)
	if err != nil {
		errMsg := err.Error()
		if strings.Contains(errMsg, "not found") {
			writeJSON(w, http.StatusNotFound, model.ApiResponse[any]{
				Success: false,
				Message: "catalog entry not found",
			})
			return
		}
		if strings.Contains(errMsg, "not currently downloading") {
			writeJSON(w, http.StatusBadRequest, model.ApiResponse[any]{
				Success: false,
				Message: "catalog entry is not currently downloading",
			})
			return
		}
		middleware.WriteError(w, http.StatusInternalServerError, model.ERR_INTERNAL, "failed to cancel download", err)
		return
	}

	writeJSON(w, http.StatusOK, model.ApiResponse[any]{
		Success: true,
		Message: "download cancelled",
	})
}

// ISOCatalogCheckAll handles POST /api/v1/admin/os-install/catalog/check-all.
func (h *ISOHandler) ISOCatalogCheckAll(w http.ResponseWriter, r *http.Request) {
	if err := h.catalogService.CheckAllAutoUpdate(r.Context()); err != nil {
		middleware.WriteError(w, http.StatusInternalServerError, model.ERR_INTERNAL, "failed to check all entries", err)
		return
	}

	writeJSON(w, http.StatusOK, model.ApiResponse[any]{
		Success: true,
		Message: "all auto-update entries checked",
	})
}

// ISOCatalogQueue handles GET /api/v1/admin/os-install/catalog/queue.
func (h *ISOHandler) ISOCatalogQueue(w http.ResponseWriter, r *http.Request) {
	stats, err := h.catalogService.GetQueueStats(r.Context())
	if err != nil {
		middleware.WriteError(w, http.StatusInternalServerError, model.ERR_INTERNAL, "failed to get queue stats", err)
		return
	}
	items, err := h.catalogService.GetQueueList(r.Context())
	if err != nil {
		middleware.WriteError(w, http.StatusInternalServerError, model.ERR_INTERNAL, "failed to get queue list", err)
		return
	}
	type queueResponse struct {
		Stats model.ISOQueueStats  `json:"stats"`
		Items []model.ISOQueueItem `json:"items"`
	}
	writeJSON(w, http.StatusOK, model.ApiResponse[queueResponse]{
		Success: true,
		Data:    queueResponse{Stats: *stats, Items: items},
	})
}

// ISOCatalogDownloadAll handles POST /api/v1/admin/os-install/catalog/download-all.
func (h *ISOHandler) ISOCatalogDownloadAll(w http.ResponseWriter, r *http.Request) {
	if err := h.catalogService.QueueDownloadAll(r.Context()); err != nil {
		middleware.WriteError(w, http.StatusInternalServerError, model.ERR_INTERNAL, "failed to queue all downloads", err)
		return
	}
	writeJSON(w, http.StatusAccepted, model.ApiResponse[any]{
		Success: true,
		Message: "all available ISOs queued for download",
	})
}

// ISOQueueProgress handles GET /api/v1/admin/os-install/catalog/progress.
// Returns real-time download progress for all actively downloading ISOs.
func (h *ISOHandler) ISOQueueProgress(w http.ResponseWriter, r *http.Request) {
	progress := h.isoService.GetActiveProgress()
	type isoProgressItem struct {
		Filename   string `json:"filename"`
		BytesRead  int64  `json:"bytes_read"`
		TotalBytes int64  `json:"total_bytes"`
		Percent    int    `json:"percent"`
		Speed      int64  `json:"speed"`
		ETA        int64  `json:"eta"`
	}
	resp := make([]isoProgressItem, 0, len(progress))
	for filename, p := range progress {
		pct := 0
		if p.Total > 0 {
			pct = int(p.BytesRead * 100 / p.Total)
		}
		resp = append(resp, isoProgressItem{
			Filename:   filename,
			BytesRead:  p.BytesRead,
			TotalBytes: p.Total,
			Percent:    pct,
			Speed:      p.Speed,
			ETA:        p.ETA,
		})
	}
	writeJSON(w, http.StatusOK, model.ApiResponse[[]isoProgressItem]{
		Success: true,
		Data:    resp,
	})
}

// ISOCatalogProfiles handles GET /api/v1/admin/os-install/catalog/profiles.
// Returns the list of built-in distro profiles available as catalog templates.
func (h *ISOHandler) ISOCatalogProfiles(w http.ResponseWriter, r *http.Request) {
	profiles := service.ListProfiles()

	writeJSON(w, http.StatusOK, model.ApiResponse[[]service.DistroProfile]{
		Success: true,
		Data:    profiles,
	})
}

// PublicListISOs handles GET /api/v1/isos.
// Public endpoint that returns the list of ISO files available for download.
// Unlike the admin endpoint, this does not require auth middleware.
func (h *ISOHandler) PublicListISOs(w http.ResponseWriter, r *http.Request) {
	isos, err := h.isoService.ListISOs()
	if err != nil {
		middleware.WriteError(w, http.StatusInternalServerError, model.ERR_INTERNAL, "failed to list ISOs", err)
		return
	}

	writeJSON(w, http.StatusOK, model.ApiResponse[[]service.ISOInfo]{
		Success: true,
		Data:    isos,
	})
}

// DownloadISO handles GET /api/v1/isos/{name}/download.
// Auth: JWT via Authorization header or ?token= query param (same pattern as FileHandler.Download).
func (h *ISOHandler) DownloadISO(w http.ResponseWriter, r *http.Request) {
	// Auth: check Authorization header first, then ?token= query param
	tokenString := ""
	if authHeader := r.Header.Get("Authorization"); strings.HasPrefix(authHeader, "Bearer ") {
		tokenString = strings.TrimPrefix(authHeader, "Bearer ")
	} else if tok := r.URL.Query().Get("token"); tok != "" {
		tokenString = tok
	} else {
		middleware.WriteError(w, http.StatusUnauthorized, model.ERR_UNAUTHORIZED, "unauthorized", nil)
		return
	}
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return []byte(h.jwtSecret), nil
	})
	if err != nil || !token.Valid {
		middleware.WriteError(w, http.StatusUnauthorized, model.ERR_UNAUTHORIZED, "invalid token", nil)
		return
	}

	name := r.PathValue("name")
	if name == "" {
		writeJSON(w, http.StatusBadRequest, model.ApiResponse[any]{
			Success: false,
			Message: "ISO name is required",
		})
		return
	}

	if err := h.isoService.StreamISO(w, name); err != nil {
		if !strings.Contains(err.Error(), "broken pipe") {
			middleware.WriteError(w, http.StatusInternalServerError, model.ERR_INTERNAL, "failed to stream ISO", err)
		}
	}
}
