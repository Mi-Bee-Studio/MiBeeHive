package model

// Error codes for API responses. Frontend uses these to show localized messages.
const (
	ERR_NOT_FOUND          = "NOT_FOUND"
	ERR_UNAUTHORIZED       = "UNAUTHORIZED"
	ERR_TOKEN_EXPIRED      = "TOKEN_EXPIRED"
	ERR_PASSWORD_CHANGED   = "PASSWORD_CHANGED"
	ERR_VALIDATION         = "VALIDATION"
	ERR_DOCKER_UNAVAILABLE = "DOCKER_UNAVAILABLE"
	ERR_DISK_FULL          = "DISK_FULL"
	ERR_NETWORK            = "NETWORK"
	ERR_DUPLICATE          = "DUPLICATE"
	ERR_INTERNAL           = "INTERNAL_ERROR"
)
