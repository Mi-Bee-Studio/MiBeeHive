package crawler

import "testing"

func TestExtractVersionFromFilename(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		want     string
	}{
		{
			name:     "hashicorp pattern",
			filename: "consul_1.22.2_darwin_amd64.zip",
			want:     "1.22.2",
		},
		{
			name:     "prometheus tarball",
			filename: "prometheus-3.11.3.linux-arm64.tar.gz",
			want:     "3.11.3",
		},
		{
			name:     "go tarball strips go prefix",
			filename: "go1.23.4.linux-arm64.tar.gz",
			want:     "1.23.4",
		},
		{
			name:     "terraform pre-release",
			filename: "terraform_1.9.0-rc1_linux_arm64.zip",
			want:     "1.9.0-rc1",
		},
		{
			name:     "node tarball",
			filename: "node-v20.11.0-linux-arm64.tar.gz",
			want:     "20.11.0",
		},
		{
			name:     "no version",
			filename: "config.yaml",
			want:     "",
		},
		{
			name:     "grafana tarball",
			filename: "grafana-11.1.0.darwin-amd64.tar.gz",
			want:     "11.1.0",
		},
		{
			name:     "simple zip",
			filename: "hello-algo-1.3.0.zip",
			want:     "1.3.0",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractVersionFromFilename(tt.filename)
			if got != tt.want {
				t.Errorf("ExtractVersionFromFilename(%q) = %q, want %q", tt.filename, got, tt.want)
			}
		})
	}
}

func TestParseVersionGroup(t *testing.T) {
	tests := []struct {
		name    string
		version string
		pattern string
		want    string
	}{
		{
			name:    "standard semver",
			version: "1.22.3",
			pattern: "semver",
			want:    "1.22.x",
		},
		{
			name:    "three-part semver",
			version: "3.11.3",
			pattern: "semver",
			want:    "3.11.x",
		},
		{
			name:    "pre-release stripped",
			version: "1.9.0-rc1",
			pattern: "semver",
			want:    "1.9.x",
		},
		{
			name:    "gover strips go prefix",
			version: "go1.23.4",
			pattern: "gover",
			want:    "1.23.x",
		},
		{
			name:    "empty version",
			version: "",
			pattern: "semver",
			want:    "",
		},
		{
			name:    "two-part version",
			version: "2.0",
			pattern: "semver",
			want:    "2.0.x",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseVersionGroup(tt.version, tt.pattern)
			if got != tt.want {
				t.Errorf("ParseVersionGroup(%q, %q) = %q, want %q", tt.version, tt.pattern, got, tt.want)
			}
		})
	}
}

func TestVersionGroupKey(t *testing.T) {
	got := VersionGroupKey("1.22.3")
	want := "1.22.x"
	if got != want {
		t.Errorf("VersionGroupKey(%q) = %q, want %q", "1.22.3", got, want)
	}
}

func TestSourceTypeVersionPattern(t *testing.T) {
	tests := []struct {
		name       string
		sourceType string
		want       string
	}{
		{
			name:       "hashicorp",
			sourceType: "hashicorp",
			want:       "semver",
		},
		{
			name:       "go",
			sourceType: "go",
			want:       "gover",
		},
		{
			name:       "github",
			sourceType: "github",
			want:       "semver",
		},
		{
			name:       "unknown",
			sourceType: "unknown",
			want:       "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SourceTypeVersionPattern(tt.sourceType)
			if got != tt.want {
				t.Errorf("SourceTypeVersionPattern(%q) = %q, want %q", tt.sourceType, got, tt.want)
			}
		})
	}
}
