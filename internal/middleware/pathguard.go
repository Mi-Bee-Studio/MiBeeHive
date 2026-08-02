package middleware

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ValidatePhysicalPath validates that a resolved physical path stays within
// the specified base path. This prevents path traversal attacks where an
// attacker could escape the intended directory.
//
// The function cleans both paths and checks if the resolved path is within
// the base directory. Returns an error if path traversal is detected.
func ValidatePhysicalPath(physicalPath, basePath string) error {
	// Clean both paths to resolve any . or .. components and normalize separators
	cleanPath := filepath.Clean(physicalPath)
	cleanBase := filepath.Clean(basePath)

	// Get the relative path from base to the target
	rel, err := filepath.Rel(cleanBase, cleanPath)
	if err != nil {
		return fmt.Errorf("failed to compute relative path: %w", err)
	}

	// If the relative path starts with "..", it means the path is outside the base
	// filepath.Rel on Windows can return absolute paths, so check for that too
	if strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) {
		return fmt.Errorf("path traversal detected: path %q escapes base %q", physicalPath, basePath)
	}

	return nil
}

// RejectVirtualPathTraversal checks if a virtual path contains path traversal
// sequences (..) and rejects it with os.ErrPermission if found.
//
// This is used to validate user-provided virtual paths before they are
// processed by the application.
func RejectVirtualPathTraversal(virtualPath string) error {
	// Split the path into elements and check for ".."
	elements := strings.Split(virtualPath, string(filepath.Separator))
	for _, elem := range elements {
		if elem == ".." {
			return fmt.Errorf("path traversal rejected: %w", os.ErrPermission)
		}
	}

	// Also check for ".." in the raw path (handles cases like "/../" or "foo/../bar")
	if strings.Contains(virtualPath, "/..") || strings.HasPrefix(virtualPath, "../") {
		return fmt.Errorf("path traversal rejected: %w", os.ErrPermission)
	}

	return nil
}