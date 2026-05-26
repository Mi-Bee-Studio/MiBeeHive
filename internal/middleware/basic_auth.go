package middleware

import (
  "log/slog"
  "net/http"

  "golang.org/x/crypto/bcrypt"
)

// BasicAuthMiddleware returns middleware that allows anonymous read and requires auth for write.
func BasicAuthMiddleware(passwordHash string) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            method := r.Method

            // OPTIONS passes through (WebDAV capability discovery)
            if method == http.MethodOptions {
                next.ServeHTTP(w, r)
                return
            }

            // Read-only methods pass through without auth (anonymous read)
            switch method {
            case http.MethodGet, http.MethodHead, "PROPFIND":
                next.ServeHTTP(w, r)
                return
            }

            // Write methods require Basic Auth
            username, password, ok := r.BasicAuth()
            if !ok || username != "admin" {
                w.Header().Set("WWW-Authenticate", "Basic realm=\"MiBeeHive\"")
                w.WriteHeader(http.StatusUnauthorized)
                return
            }

            if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(password)); err != nil {
                slog.Debug("webdav auth failed", "username", username)
                w.Header().Set("WWW-Authenticate", "Basic realm=\"MiBeeHive\"")
                w.WriteHeader(http.StatusUnauthorized)
                return
            }

            slog.Debug("webdav auth success", "username", username)
            next.ServeHTTP(w, r)
        })
    }
}