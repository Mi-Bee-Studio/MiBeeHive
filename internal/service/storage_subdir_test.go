package service

import "testing"

func TestStorageSubdirDefault(t *testing.T) {
	// Empty subdir is allowed and falls back to the default.
	if err := ValidateStorageSubdir("/data", ""); err != nil {
		t.Errorf("ValidateStorageSubdir(empty) error = %v, want nil", err)
	}
	if got := DefaultStorageSubdir("prometheus"); got != "oss/prometheus" {
		t.Errorf("DefaultStorageSubdir(prometheus) = %q, want %q", got, "oss/prometheus")
	}
}

func TestStorageSubdirCustom(t *testing.T) {
	base := "/data"
	cases := []string{
		"oss/prometheus",
		"custom/tools",
		"a/b/c",
		"oss",
	}
	for _, subdir := range cases {
		if err := ValidateStorageSubdir(base, subdir); err != nil {
			t.Errorf("ValidateStorageSubdir(%q) error = %v, want nil", subdir, err)
		}
	}
}

func TestStorageSubdirTraversal(t *testing.T) {
	base := "/data"
	cases := []string{
		"../../etc",
		"../secret",
		"oss/../../etc",
		"/etc",
		"oss/../../../tmp",
	}
	for _, subdir := range cases {
		if err := ValidateStorageSubdir(base, subdir); err == nil {
			t.Errorf("ValidateStorageSubdir(%q) = nil, want error (traversal)", subdir)
		}
	}
}