// Package registry provides a V2 Docker/OCI registry client with
// HTTP transport and auth token negotiation (Basic + Bearer).
package registry

import (
	"fmt"
	"strconv"
	"time"
)

// RegistryError represents an error returned by a registry API call.
type RegistryError struct {
	StatusCode int
	Message    string
	Detail     string
}

func (e *RegistryError) Error() string {
	if e.Detail != "" {
		return fmt.Sprintf("registry error %d: %s: %s", e.StatusCode, e.Message, e.Detail)
	}
	return fmt.Sprintf("registry error %d: %s", e.StatusCode, e.Message)
}

// AuthError indicates a 401 authentication failure.
type AuthError struct {
	RegistryError
}

// NewAuthError creates an AuthError from a status code and message.
func NewAuthError(statusCode int, message string) *AuthError {
	return &AuthError{
		RegistryError: RegistryError{
			StatusCode: statusCode,
			Message:    message,
		},
	}
}

// NotFoundError indicates a 404 resource not found.
type NotFoundError struct {
	RegistryError
}

// NewNotFoundError creates a NotFoundError from a status code and message.
func NewNotFoundError(statusCode int, message string) *NotFoundError {
	return &NotFoundError{
		RegistryError: RegistryError{
			StatusCode: statusCode,
			Message:    message,
		},
	}
}

// RateLimitError indicates a 429 rate limit response.
type RateLimitError struct {
	RegistryError
	RetryAfter time.Duration
}

// NewRateLimitError creates a RateLimitError.
func NewRateLimitError(statusCode int, message string, retryAfter time.Duration) *RateLimitError {
	return &RateLimitError{
		RegistryError: RegistryError{
			StatusCode: statusCode,
			Message:    message,
		},
		RetryAfter: retryAfter,
	}
}

// parseRetryAfter parses a Retry-After header value (seconds or HTTP date).
func parseRetryAfter(value string) time.Duration {
	if d, err := strconv.Atoi(value); err == nil {
		return time.Duration(d) * time.Second
	}

	return 60 * time.Second
}
