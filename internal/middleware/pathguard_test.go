package middleware

import (
	"errors"
	"os"
	"testing"
)

func TestValidatePhysicalPath_CleanPathsPass(t *testing.T) {
	basePath := "/var/lib/mibeehive/storage"

	tests := []struct {
		name         string
		physicalPath string
	}{
		{
			name:         "simple path within base",
			physicalPath: "/var/lib/mibeehive/storage/webdav/file.tar.gz",
		},
		{
			name:         "path with trailing slash",
			physicalPath: "/var/lib/mibeehive/storage/webdav/",
		},
		{
			name:         "path with single dot",
			physicalPath: "/var/lib/mibeehive/storage/./webdav/file.tar.gz",
		},
		{
			name:         "nested path",
			physicalPath: "/var/lib/mibeehive/storage/webdav/subdir/deep/file.tar.gz",
		},
		{
			name:         "exact base path",
			physicalPath: "/var/lib/mibeehive/storage",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePhysicalPath(tt.physicalPath, basePath)
			if err != nil {
				t.Errorf("ValidatePhysicalPath(%q, %q) returned unexpected error: %v", tt.physicalPath, basePath, err)
			}
		})
	}
}

func TestValidatePhysicalPath_TraversalRejected(t *testing.T) {
	basePath := "/var/lib/mibeehive/storage"

	tests := []struct {
		name         string
		physicalPath string
	}{
		{
			name:         "parent directory traversal",
			physicalPath: "/var/lib/mibeehive/storage/../other/file.tar.gz",
		},
		{
			name:         "deep traversal",
			physicalPath: "/var/lib/mibeehive/storage/../../../etc/passwd",
		},
		{
			name:         "multiple parent traversals",
			physicalPath: "/var/lib/mibeehive/storage/../../..",
		},
		{
			name:         "traversal to root",
			physicalPath: "/var/lib/mibeehive/storage/../../../../",
		},
		{
			name:         "absolute path outside base",
			physicalPath: "/etc/passwd",
		},
		{
			name:         "traversal with nested subdir",
			physicalPath: "/var/lib/mibeehive/storage/webdav/../../../etc/shadow",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePhysicalPath(tt.physicalPath, basePath)
			if err == nil {
				t.Errorf("ValidatePhysicalPath(%q, %q) expected error, got nil", tt.physicalPath, basePath)
			}
		})
	}
}


func TestRejectVirtualPathTraversal_ValidPaths(t *testing.T) {
	tests := []string{
		"webdav/file.tar.gz",
		"webdav/subdir/file.tar.gz",
		"webdav",
		"file.tar.gz",
		".",
		"webdav/./file.tar.gz",
	}

	for _, tt := range tests {
		t.Run(tt, func(t *testing.T) {
			err := RejectVirtualPathTraversal(tt)
			if err != nil {
				t.Errorf("RejectVirtualPathTraversal(%q) returned unexpected error: %v", tt, err)
			}
		})
	}
}

func TestRejectVirtualPathTraversal_TraversalRejected(t *testing.T) {
	tests := []string{
		"../etc/passwd",
		"webdav/../../etc/passwd",
		"webdav/../../../etc/shadow",
		"../webdav/file.tar.gz",
		"webdav/..",
		"..",
		"webdav/../../..",
	}

	for _, tt := range tests {
		t.Run(tt, func(t *testing.T) {
			err := RejectVirtualPathTraversal(tt)
			if err == nil {
				t.Errorf("RejectVirtualPathTraversal(%q) expected error, got nil", tt)
			}
			if err != nil && !errors.Is(err, os.ErrPermission) {
				t.Errorf("RejectVirtualPathTraversal(%q) expected os.ErrPermission, got: %v", tt, err)
			}
		})
	}
}

func TestRejectVirtualPathTraversal_EmptyPath(t *testing.T) {
	// Empty path should be allowed (it's a valid edge case)
	err := RejectVirtualPathTraversal("")
	if err != nil {
		t.Errorf("RejectVirtualPathTraversal(\"\") returned unexpected error: %v", err)
	}
}