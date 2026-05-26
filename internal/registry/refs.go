package registry

import "strings"

// ParseImageRef parses a container image reference into repository and tag (or digest).
//
// Supported formats:
//   - "alpine"                      → repo="alpine", tag="latest"
//   - "alpine:3.18"                 → repo="alpine", tag="3.18"
//   - "alpine@sha256:abc123"        → repo="alpine", tag="sha256:abc123"
//   - "library/alpine:latest"       → repo="library/alpine", tag="latest"
//   - "myrepo/myimage:v1"           → repo="myrepo/myimage", tag="v1"
func ParseImageRef(ref string) (repo, tag string) {
	// Check for digest reference (repo@sha256:...)
	if atIdx := strings.Index(ref, "@"); atIdx != -1 {
		repo = ref[:atIdx]
		tag = ref[atIdx+1:]
		return repo, tag
	}

	// Check for tag reference (repo:tag).
	// The colon must be after the last slash to distinguish from registry ports.
	if colonIdx := strings.LastIndex(ref, ":"); colonIdx != -1 {
		slashIdx := strings.LastIndex(ref, "/")
		if colonIdx > slashIdx {
			repo = ref[:colonIdx]
			tag = ref[colonIdx+1:]
			return repo, tag
		}
	}

	// No tag or digest — default to latest.
	return ref, "latest"
}
