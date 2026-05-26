package service

import (
	"reflect"
	"testing"
)

func TestListProfiles(t *testing.T) {
	profiles := ListProfiles()
	if len(profiles) != 5 {
		t.Fatalf("ListProfiles() returned %d profiles, want 5", len(profiles))
	}

	// Verify each profile has required non-empty fields
	for _, p := range profiles {
		if p.ID == "" {
			t.Errorf("profile has empty ID: %+v", p)
		}
		if p.Name == "" {
			t.Errorf("profile %q has empty Name", p.ID)
		}
		if p.Distro == "" {
			t.Errorf("profile %q has empty Distro", p.ID)
		}
		if p.BaseURL == "" {
			t.Errorf("profile %q has empty BaseURL", p.ID)
		}
		if p.FilenamePattern == "" {
			t.Errorf("profile %q has empty FilenamePattern", p.ID)
		}
		if p.DefaultInterval <= 0 {
			t.Errorf("profile %q has DefaultInterval=%d, want > 0", p.ID, p.DefaultInterval)
		}
		if p.Arch == "" {
			t.Errorf("profile %q has empty Arch", p.ID)
		}
	}
}

func TestGetProfile(t *testing.T) {
	tests := []struct {
		name  string
		id    string
		found bool
	}{
		{"ubuntu server amd64", "ubuntu-server-amd64", true},
		{"ubuntu server arm64", "ubuntu-server-arm64", true},
		{"debian netinst amd64", "debian-netinst-amd64", true},
		{"rocky minimal amd64", "rocky-minimal-amd64", true},
		{"alpine standard amd64", "alpine-standard-amd64", true},
		{"unknown profile", "nonexistent-profile", false},
		{"empty string", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetProfile(tt.id)
			if tt.found {
				if got == nil {
					t.Errorf("GetProfile(%q) = nil, want non-nil", tt.id)
				} else if got.ID != tt.id {
					t.Errorf("GetProfile(%q).ID = %q, want %q", tt.id, got.ID, tt.id)
				}
			} else {
				if got != nil {
					t.Errorf("GetProfile(%q) = %+v, want nil", tt.id, got)
				}
			}
		})
	}
}

func TestListProfiles_ReturnsCopy(t *testing.T) {
	original := ListProfiles()
	// Modify the returned slice
	original[0].Name = "MODIFIED"
	// Fetch again — should be unchanged
	fresh := ListProfiles()
	if fresh[0].Name == "MODIFIED" {
		t.Error("ListProfiles() returned slice that shares backing array with internal data")
	}
	if fresh[0].Name == "" {
		t.Error("ListProfiles() returned profile with empty Name")
	}
}

func TestMirrorArch(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"amd64", "x86_64"},
		{"arm64", "aarch64"},
		{"x86_64", "x86_64"},
		{"aarch64", "aarch64"},
		{"i386", "i386"},
		{"", ""},
		{"unknown", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := MirrorArch(tt.input)
			if got != tt.want {
				t.Errorf("MirrorArch(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestGetProfile_NonNilFields(t *testing.T) {
	// Verify the first found profile (ubuntu-server-amd64) has all expected fields
	p := GetProfile("ubuntu-server-amd64")
	if p == nil {
		t.Fatal("GetProfile(\"ubuntu-server-amd64\") = nil")
	}

	// Check struct fields (not defaults)
	if p.ID != "ubuntu-server-amd64" {
		t.Errorf("ID = %q, want %q", p.ID, "ubuntu-server-amd64")
	}
	if p.Distro != "ubuntu" {
		t.Errorf("Distro = %q, want %q", p.Distro, "ubuntu")
	}
	if p.Variant != "server" {
		t.Errorf("Variant = %q, want %q", p.Variant, "server")
	}
	if p.Arch != "amd64" {
		t.Errorf("Arch = %q, want %q", p.Arch, "amd64")
	}
	if p.BaseURL != "https://releases.ubuntu.com/" {
		t.Errorf("BaseURL = %q, want %q", p.BaseURL, "https://releases.ubuntu.com/")
	}
	if p.VersionDirPattern != `\d{2}\.\d{2}` {
		t.Errorf("VersionDirPattern = %q, want %q", p.VersionDirPattern, `\d{2}\.\d{2}`)
	}
	if p.ISOPathTemplate != "{version}/" {
		t.Errorf("ISOPathTemplate = %q, want %q", p.ISOPathTemplate, "{version}/")
	}
	if p.FilenamePattern != `ubuntu-[\d.]+-live-server-amd64\.iso$` {
		t.Errorf("FilenamePattern = %q, want %q", p.FilenamePattern, `ubuntu-[\d.]+-live-server-amd64\.iso$`)
	}
	if p.DefaultInterval != 24 {
		t.Errorf("DefaultInterval = %d, want %d", p.DefaultInterval, 24)
	}
}

func TestDistroProfile_StructFields(t *testing.T) {
	// Verify that the DistroProfile struct has all expected fields via reflection
	p := DistroProfile{}
	typ := reflect.TypeOf(p)

	expectedFields := []string{
		"ID",
		"Name",
		"Distro",
		"Variant",
		"Arch",
		"BaseURL",
		"VersionDirPattern",
		"ISOPathTemplate",
		"FilenamePattern",
		"DefaultInterval",
	}

	for _, field := range expectedFields {
		if _, ok := typ.FieldByName(field); !ok {
			t.Errorf("DistroProfile missing field: %s", field)
		}
	}

	// Verify fields count (no unexpected fields)
	if typ.NumField() != len(expectedFields) {
		t.Errorf("DistroProfile has %d fields, want %d: %v", typ.NumField(), len(expectedFields), expectedFields)
	}
}
