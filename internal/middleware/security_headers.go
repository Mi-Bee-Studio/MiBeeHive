package middleware

import (
	"net/http"
)

// SecurityHeaders adds security-related response headers to every HTTP response.
// It wraps the http.ResponseWriter to inject headers on the first WriteHeader
// call, ensuring headers are present even when downstream handlers omit explicit
// WriteHeader invocations.
//
// Headers added:
//   - X-Content-Type-Options: nosniff
//   - X-Frame-Options: DENY
//   - X-XSS-Protection: 1; mode=block
//   - Referrer-Policy: strict-origin-when-cross-origin
//   - Content-Security-Policy: default-src 'self'; ...
//
// HSTS is intentionally omitted — the server uses self-signed TLS certificates
// (no trusted public domain), making HSTS harmful by permanently locking users
// out on certificate changes.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sw := &securityHeadersWriter{ResponseWriter: w}
		next.ServeHTTP(sw, r)
	})
}

// securityHeadersWriter wraps http.ResponseWriter to inject security headers
// on the first call to WriteHeader (explicit or implicit via Write), preventing
// downstream handlers from removing or overriding them.
type securityHeadersWriter struct {
	http.ResponseWriter
	wroteHeader bool
}

func (w *securityHeadersWriter) WriteHeader(statusCode int) {
	if !w.wroteHeader {
		w.wroteHeader = true
		w.ResponseWriter.Header().Set("X-Content-Type-Options", "nosniff")
		w.ResponseWriter.Header().Set("X-Frame-Options", "DENY")
		w.ResponseWriter.Header().Set("X-XSS-Protection", "1; mode=block")
		w.ResponseWriter.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.ResponseWriter.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline' https://cdn.tailwindcss.com https://cdn.jsdelivr.net; style-src 'self' 'unsafe-inline' https://cdn.tailwindcss.com; img-src 'self' data:; font-src 'self'; connect-src 'self'")
	}
	w.ResponseWriter.WriteHeader(statusCode)
}

func (w *securityHeadersWriter) Write(b []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(b)
}
