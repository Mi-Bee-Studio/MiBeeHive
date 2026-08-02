package handler

import (
	"net/http"
)

// WebDAVRedirectHandler redirects the legacy /webdav/ root to the new default
// view /webdav/public/default/ with a 301 Moved Permanently. All other
// /webdav/ sub-paths are passed through unchanged to next, so existing clients
// that address files under /webdav/ keep working without disruption.
func WebDAVRedirectHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/webdav/" {
			http.Redirect(w, r, "/webdav/public/default/", http.StatusMovedPermanently)
			return
		}
		next.ServeHTTP(w, r)
	})
}