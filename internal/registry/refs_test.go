package registry

import "testing"

func TestParseImageRef(t *testing.T) {
	tests := []struct {
		input   string
		wantRepo string
		wantTag  string
	}{
		{"alpine", "alpine", "latest"},
		{"alpine:3.18", "alpine", "3.18"},
		{"alpine@sha256:abc123def456", "alpine", "sha256:abc123def456"},
		{"library/alpine:latest", "library/alpine", "latest"},
		{"myrepo/myimage:v1", "myrepo/myimage", "v1"},
		{"registry.example.com:5000/myimage:v2", "registry.example.com:5000/myimage", "v2"},
		{"org/repo:tag-name", "org/repo", "tag-name"},
		{"ubuntu:22.04", "ubuntu", "22.04"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			repo, tag := ParseImageRef(tt.input)
			if repo != tt.wantRepo {
				t.Errorf("repo = %q, want %q", repo, tt.wantRepo)
			}
			if tag != tt.wantTag {
				t.Errorf("tag = %q, want %q", tag, tt.wantTag)
			}
		})
	}
}
