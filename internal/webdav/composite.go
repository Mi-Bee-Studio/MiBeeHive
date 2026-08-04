package webdav

import "net/http"

// recordingWriter wraps http.ResponseWriter to capture the status code
// without committing it to the underlying writer on 404. This lets the
// composite handler suppress the virtual handler's 404 response and let
// the legacy handler try instead.
type recordingWriter struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (w *recordingWriter) WriteHeader(code int) {
	if w.wroteHeader {
		return
	}
	w.status = code
	w.wroteHeader = true
	if code != http.StatusNotFound {
		w.ResponseWriter.WriteHeader(code)
	}
}

func (w *recordingWriter) Write(b []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	if w.status == http.StatusNotFound {
		return len(b), nil // suppress body for 404
	}
	return w.ResponseWriter.Write(b)
}

// CompositeHandler tries the virtual handler first. If the virtual handler
// returns 404 (path doesn't match any channel/view), the request falls
// through to the legacy flat-file handler so existing WebDAV clients that
// address files directly under /webdav/ keep working.
type CompositeHandler struct {
	virtual http.Handler
	legacy  http.Handler
}

// NewCompositeHandler creates a handler that tries virtual first, legacy fallback.
func NewCompositeHandler(virtual, legacy http.Handler) *CompositeHandler {
	return &CompositeHandler{virtual: virtual, legacy: legacy}
}

func (h *CompositeHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	rec := &recordingWriter{ResponseWriter: w, status: http.StatusOK}
	h.virtual.ServeHTTP(rec, r)
	if rec.status == http.StatusNotFound {
		h.legacy.ServeHTTP(w, r)
	}
}
