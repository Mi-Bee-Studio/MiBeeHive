package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/Mi-Bee-Studio/mibeehive/internal/middleware"
	"github.com/Mi-Bee-Studio/mibeehive/internal/model"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

// AuthHandler handles authentication endpoints.
type AuthHandler struct {
	passwordHash         string
	jwtSecret            string
	isDefaultPassword    bool
	getPasswordChangedAt func() time.Time
}

// NewAuthHandler creates a new AuthHandler.
func NewAuthHandler(passwordHash, jwtSecret string, getPasswordChangedAt ...func() time.Time) *AuthHandler {
	h := &AuthHandler{
		passwordHash:      passwordHash,
		jwtSecret:         jwtSecret,
		isDefaultPassword: passwordHash == defaultPasswordHash,
	}
	if len(getPasswordChangedAt) > 0 && getPasswordChangedAt[0] != nil {
		h.getPasswordChangedAt = getPasswordChangedAt[0]
	}
	return h
}

// Login handles POST /api/v1/auth/login.
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, model.ApiResponse[any]{
			Success: false,
			Message: "method not allowed",
		})
		return
	}
	// Validate Content-Type.
	contentType := r.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "application/json") {
		writeJSON(w, http.StatusUnsupportedMediaType, model.ApiResponse[any]{
			Success: false,
			Message: "unsupported media type, expected application/json",
		})
		return
	}

	var req model.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, model.ApiResponse[any]{
			Success: false,
			Message: "invalid request body",
		})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(h.passwordHash), []byte(req.Password)); err != nil {
		middleware.WriteError(w, http.StatusUnauthorized, model.ERR_UNAUTHORIZED, "invalid password", nil)
		return
	}

	claims := jwt.MapClaims{
		"sub": "admin",
		"exp": time.Now().Add(24 * time.Hour).Unix(),
		"iat": time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(h.jwtSecret))
	if err != nil {
		middleware.WriteError(w, http.StatusInternalServerError, model.ERR_INTERNAL, "操作失败，请稍后重试", err)
		return
	}

	if h.isDefaultPassword {
		writeJSON(w, http.StatusConflict, model.ApiResponse[map[string]any]{
			Success: false,
			Message: "Password change required",
			Data: map[string]any{
				"require_password_change": true,
				"token":                   tokenString,
			},
		})
		return
	}

	writeJSON(w, http.StatusOK, model.ApiResponse[model.LoginResponse]{
		Success: true,
		Data: model.LoginResponse{
			Token: tokenString,
		},
	})
}

const defaultPasswordHash = "$2a$10$rsBcy69QYKw.zW5UloqBoOMFPOk0pmRfuEBCESiEbYijCRBAst0DG"

// PasswordStatus handles GET /api/v1/auth/password-status.
func (h *AuthHandler) PasswordStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, model.ApiResponse[model.PasswordStatusResponse]{
		Success: true,
		Data: model.PasswordStatusResponse{
			IsDefault:     h.isDefaultPassword,
			RequireChange: h.isDefaultPassword,
		},
	})
}

// RefreshToken handles POST /api/v1/auth/refresh.
// It accepts a valid, non-expired JWT and returns a new one with 24h expiry.
func (h *AuthHandler) RefreshToken(w http.ResponseWriter, r *http.Request) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		middleware.WriteError(w, http.StatusUnauthorized, model.ERR_UNAUTHORIZED, "unauthorized", nil)
		return
	}

	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || parts[0] != "Bearer" {
		middleware.WriteError(w, http.StatusUnauthorized, model.ERR_UNAUTHORIZED, "unauthorized", nil)
		return
	}

	tokenString := parts[1]
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return []byte(h.jwtSecret), nil
	})

	if err != nil || !token.Valid {
		code := model.ERR_UNAUTHORIZED
		if err != nil && strings.Contains(err.Error(), "token is expired") {
			code = model.ERR_TOKEN_EXPIRED
		}
		middleware.WriteError(w, http.StatusUnauthorized, code, "invalid or expired token", nil)
		return
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		middleware.WriteError(w, http.StatusUnauthorized, model.ERR_UNAUTHORIZED, "invalid token claims", nil)
		return
	}

	// Reject tokens issued before last password change.
	if h.getPasswordChangedAt != nil {
		if iatFloat, ok := claims["iat"].(float64); ok {
			tokenIssuedAt := time.Unix(int64(iatFloat), 0)
			pwdChangedAt := h.getPasswordChangedAt()
			if !pwdChangedAt.IsZero() && tokenIssuedAt.Before(pwdChangedAt) {
				slog.Warn("refresh rejected: token issued before password change")
				middleware.WriteError(w, http.StatusUnauthorized, model.ERR_PASSWORD_CHANGED, "session expired due to password change", nil)
				return
			}
		}
	}

	// Generate new token with 24h expiry.
	newClaims := jwt.MapClaims{
		"sub": "admin",
		"exp": time.Now().Add(24 * time.Hour).Unix(),
		"iat": time.Now().Unix(),
	}
	newToken := jwt.NewWithClaims(jwt.SigningMethodHS256, newClaims)
	newTokenString, err := newToken.SignedString([]byte(h.jwtSecret))
	if err != nil {
		middleware.WriteError(w, http.StatusInternalServerError, model.ERR_INTERNAL, "操作失败，请稍后重试", err)
		return
	}

	writeJSON(w, http.StatusOK, model.ApiResponse[model.LoginResponse]{
		Success: true,
		Data: model.LoginResponse{
			Token: newTokenString,
		},
	})
}
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
