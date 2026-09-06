package service

import (
	"fmt"
	"path/filepath"
	"strings"
)

// DefaultStorageSubdir returns the default per-project storage subdirectory
// relative to storage.base_path. It mirrors the historical convention of
// storing crawled artifacts under {base}/oss/{project_name}.
func DefaultStorageSubdir(projectName string) string {
	return "oss/" + projectName
}

// ValidateStorageSubdir ensures a project's storage_subdir stays within the
// configured storage base path. An empty subdir is allowed (the caller falls
// back to DefaultStorageSubdir). Absolute paths and any path that resolves
// outside basePath (path traversal) are rejected.
func ValidateStorageSubdir(basePath, subdir string) error {
	if subdir == "" {
		return nil
	}
	// Reject POSIX-absolute subdirs explicitly: filepath.IsAbs alone is
	// platform-dependent (it returns false for "/etc" on Windows), while the
	// storage path convention is POSIX on every OS.
	if strings.HasPrefix(subdir, "/") || strings.HasPrefix(subdir, `\`) || filepath.IsAbs(subdir) {
		return fmt.Errorf("storage_subdir must be relative: %q", subdir)
	}

	cleanBase := filepath.Clean(basePath)
	full := filepath.Join(cleanBase, subdir)
	rel, err := filepath.Rel(cleanBase, full)
	if err != nil {
		return fmt.Errorf("computing relative path for storage_subdir %q: %w", subdir, err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return fmt.Errorf("storage_subdir %q escapes storage base path %q", subdir, basePath)
	}
	return nil
}