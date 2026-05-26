package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/Mi-Bee-Studio/mibeehive/internal/model"
)

type contextKey string

const claimsKey contextKey = "jwt_claims"

// AuthMiddleware returns a middleware that validates JWT tokens.
// It skips authentication for the /api/v1/auth/login path.
// If getPasswordChangedAt is provided, tokens issued before the password change are rejected.
func AuthMiddleware(jwtSecret string, getPasswordChangedAt ...func() time.Time) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/api/v1/auth/login" || r.URL.Path == "/api/v1/auth/refresh" {
				next.ServeHTTP(w, r)
				return
			}

			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				WriteError(w, http.StatusUnauthorized, model.ERR_UNAUTHORIZED, "unauthorized", nil)
				return
			}

			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || parts[0] != "Bearer" {
				WriteError(w, http.StatusUnauthorized, model.ERR_UNAUTHORIZED, "unauthorized", nil)
				return
			}

			tokenString := parts[1]
			token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
				if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, jwt.ErrSignatureInvalid
				}
				return []byte(jwtSecret), nil
			})

			if err != nil || !token.Valid {
				// Differentiate expired vs invalid for better UX.
				code := model.ERR_UNAUTHORIZED
				if err != nil {
					if strings.Contains(err.Error(), "token is expired") {
						code = model.ERR_TOKEN_EXPIRED
					}
				}
				WriteError(w, http.StatusUnauthorized, code, "unauthorized", nil)
				return
			}

			claims, ok := token.Claims.(jwt.MapClaims)
			if !ok {
				WriteError(w, http.StatusUnauthorized, model.ERR_UNAUTHORIZED, "unauthorized", nil)
				return
			}

			// Check iat against password_changed_at if provided.
			if len(getPasswordChangedAt) > 0 && getPasswordChangedAt[0] != nil {
				if iatFloat, ok := claims["iat"].(float64); ok {
					tokenIssuedAt := time.Unix(int64(iatFloat), 0)
					pwdChangedAt := getPasswordChangedAt[0]()
					if !pwdChangedAt.IsZero() && tokenIssuedAt.Before(pwdChangedAt) {
						slog.Warn("token issued before password change, rejecting", "iat", tokenIssuedAt, "password_changed_at", pwdChangedAt)
						WriteError(w, http.StatusUnauthorized, model.ERR_PASSWORD_CHANGED, "session expired due to password change", nil)
						return
					}
				}
			}

			ctx := context.WithValue(r.Context(), claimsKey, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetTokenClaims extracts JWT claims from the request context.
func GetTokenClaims(r *http.Request) (jwt.MapClaims, bool) {
	claims, ok := r.Context().Value(claimsKey).(jwt.MapClaims)
	return claims, ok
}
