package middleware

import (
	"compress/gzip"
	"io"
	"net/http"
	"path"
	"strings"
	"sync"
)

// gzipWriterPool pools gzip.Writer instances to reduce allocations.
// Writers are initialized with io.Discard and reset per-request via Reset().
var gzipWriterPool = sync.Pool{
	New: func() any {
		w, _ := gzip.NewWriterLevel(io.Discard, gzip.BestSpeed)
		return w
	},
}

// gzipResponseWriter wraps http.ResponseWriter to compress responses with gzip.
// It intercepts Write calls to route through the gzip writer.
type gzipResponseWriter struct {
	http.ResponseWriter
	writer *gzip.Writer
}

func (g *gzipResponseWriter) Write(b []byte) (int, error) {
	return g.writer.Write(b)
}

// CacheAndGzip adds Cache-Control headers and gzip compression for static files.
//
// Cache-Control behavior:
//   - .css, .js → public, max-age=86400 (1 day — versioned by binary rebuild)
//   - .html or root path → no-cache (SPA router needs fresh HTML)
//
// Gzip compression:
//   - Applied to text-based files (.css, .js, .html, .svg) when the client
//     sends Accept-Encoding: gzip
//   - Uses gzip.BestSpeed (level 1) for low-memory environments
//   - gzip.Writer instances are pooled via sync.Pool to reduce allocations
func CacheAndGzip(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Determine file extension from URL path.
		ext := path.Ext(r.URL.Path)
		if ext == "" || r.URL.Path == "/" {
			ext = ".html"
		}

		// Set Cache-Control header based on file type.
		switch ext {
		case ".css", ".js":
			w.Header().Set("Cache-Control", "public, max-age=86400")
		case ".html":
			w.Header().Set("Cache-Control", "no-cache")
		}

		// Gzip text-based files when client supports it.
		acceptsGzip := strings.Contains(r.Header.Get("Accept-Encoding"), "gzip")
		isTextFile := ext == ".css" || ext == ".js" || ext == ".html" || ext == ".svg"

		if acceptsGzip && isTextFile {
			w.Header().Set("Content-Encoding", "gzip")
			w.Header().Set("Vary", "Accept-Encoding")

			gw := gzipWriterPool.Get().(*gzip.Writer)
			gw.Reset(w)
			defer func() {
				gw.Close()
				gzipWriterPool.Put(gw)
			}()

			gzw := &gzipResponseWriter{
				ResponseWriter: w,
				writer:         gw,
			}
			next.ServeHTTP(gzw, r)
			return
		}

		next.ServeHTTP(w, r)
	})
}
