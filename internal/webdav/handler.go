package webdav

import (
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"golang.org/x/net/webdav"
)

// MaxUploadSize is the maximum allowed WebDAV PUT body size (2 GB).
const MaxUploadSize int64 = 2 * 1024 * 1024 * 1024

// MaxConcurrentUploads is the maximum number of concurrent PUT requests.
const MaxConcurrentUploads = 3

// Handler wraps the golang.org/x/net/webdav handler with safety controls.
//
// Known limitations of golang.org/x/net/webdav.NewMemLS:
//   - ISSUE-020: Lock timeout defaults to infinite when client omits Timeout header.
//     The stdlib parseTimeout() returns infiniteTimeout for empty headers. Customizing
//     this requires implementing the full LockSystem interface.
//   - ISSUE-030: Lock tokens are simple incrementing integers, not opaquelocktoken: URIs.
//     The stdlib nextToken() uses strconv.FormatUint. Using UUID-based tokens requires
//     a custom LockSystem implementation or forking the package.
type Handler struct {
	handler   *webdav.Handler
	putSem    chan struct{} // concurrency limiter for PUT
	stripPath string      // URL prefix stripped by http.StripPrefix (e.g. "/webdav")
}

// NewHandler creates a new WebDAV handler serving files from storagePath.
// stripPath is the URL prefix that http.StripPrefix removes from incoming requests
// (e.g. "/webdav"). It is used to fix the Destination header in COPY/MOVE requests.
func NewHandler(storagePath, stripPath string) *Handler {
	return &Handler{
		handler: &webdav.Handler{
			FileSystem: webdav.Dir(storagePath),
			LockSystem: webdav.NewMemLS(),
		},
		putSem:    make(chan struct{}, MaxConcurrentUploads),
		stripPath: stripPath,
	}
}

// ServeHTTP implements http.Handler.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	slog.Debug("webdav request", "method", r.Method, "path", r.URL.Path)

	// ISSUE-002: Strip the mount prefix from the Destination header in COPY/MOVE.
	// http.StripPrefix removes /webdav from r.URL.Path, but the Destination header
	// still contains the full URL (e.g. http://host/webdav/path). The underlying
	// webdav.Handler parses the Destination header and calls stripPrefix on its path,
	// but since we don't set Handler.Prefix (http.StripPrefix handles it), the path
	// won't be stripped. Fix it here before delegating.
	if r.Method == "COPY" || r.Method == "MOVE" {
		if dest := r.Header.Get("Destination"); dest != "" {
			if fixed, ok := stripPrefixFromDestination(dest, h.stripPath); ok {
				r.Header.Set("Destination", fixed)
			}
		}
	}

	// Enforce upload body size limit on PUT
	if r.Method == http.MethodPut {
		if r.ContentLength > MaxUploadSize {
			http.Error(w, fmt.Sprintf("request body too large: maximum size is %d bytes", MaxUploadSize), http.StatusRequestEntityTooLarge)
			return
		}

		// Acquire concurrency slot (non-blocking)
		select {
		case h.putSem <- struct{}{}:
			defer func() { <-h.putSem }()
		default:
			http.Error(w, "too many concurrent uploads", http.StatusTooManyRequests)
			return
		}
	}

	h.handler.ServeHTTP(w, r)
}

// stripPrefixFromDestination removes the given prefix from the path component of
// a Destination header URL. Returns the fixed URL and true if the prefix was stripped.
func stripPrefixFromDestination(dest, prefix string) (string, bool) {
	if prefix == "" {
		return dest, false
	}
	u, err := url.Parse(dest)
	if err != nil {
		return dest, false
	}
	if !strings.HasPrefix(u.Path, prefix) {
		return dest, false
	}
	u.Path = strings.TrimPrefix(u.Path, prefix)
	// Ensure path starts with /
	if u.Path == "" || u.Path[0] != '/' {
		u.Path = "/" + u.Path
	}
	return u.String(), true
}