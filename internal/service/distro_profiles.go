package service

// DistroProfile defines a built-in ISO distribution profile template.
// Each profile captures the metadata and URL patterns needed to discover
// and download ISO images for a specific distro/variant/arch combination.
type DistroProfile struct {
	// ID is the unique identifier, e.g. "ubuntu-server-amd64".
	ID string `json:"id"`

	// Name is the human-readable display name, e.g. "Ubuntu Server (amd64)".
	Name string `json:"name"`

	// Distro is the distribution name, e.g. "ubuntu", "debian", "rocky", "alpine".
	Distro string `json:"distro"`

	// Variant identifies the edition, e.g. "server", "netinst", "minimal", "standard".
	Variant string `json:"variant"`

	// Arch is the CPU architecture as used by the DB, e.g. "amd64", "arm64".
	Arch string `json:"arch"`

	// BaseURL is the top-level mirror URL used for version directory discovery.
	BaseURL string `json:"base_url"`

	// VersionDirPattern is the regex pattern to identify version subdirectories
	// when scraping the BaseURL directory listing. Empty means single-level (no version discovery).
	VersionDirPattern string `json:"version_dir_pattern"`

	// ISOPathTemplate is the relative path template under BaseURL where ISO files live.
	// The "{version}" placeholder is replaced with the discovered version string.
	ISOPathTemplate string `json:"iso_path_template"`

	// FilenamePattern is the regex pattern used to match ISO filenames on the mirror.
	FilenamePattern string `json:"filename_pattern"`

	// DefaultInterval is the default polling interval in hours for version checks.
	DefaultInterval int `json:"default_interval"`
}

// distroProfiles is the built-in registry of distribution profiles.
// These are hardcoded compile-time templates — no CRUD or hot-reload.
var distroProfiles = []DistroProfile{
	{
		ID:                "ubuntu-server-amd64",
		Name:              "Ubuntu Server (amd64)",
		Distro:            "ubuntu",
		Variant:           "server",
		Arch:              "amd64",
		BaseURL:           "https://releases.ubuntu.com/",
		VersionDirPattern: `\d{2}\.\d{2}`,
		ISOPathTemplate:   "{version}/",
		FilenamePattern:   `ubuntu-[\d.]+-live-server-amd64\.iso$`,
		DefaultInterval:   24,
	},
	{
		ID:                "ubuntu-server-arm64",
		Name:              "Ubuntu Server (arm64)",
		Distro:            "ubuntu",
		Variant:           "server",
		Arch:              "arm64",
		BaseURL:           "https://cdimage.ubuntu.com/releases/",
		VersionDirPattern: `\d{2}\.\d{2}`,
		ISOPathTemplate:   "{version}/release/",
		FilenamePattern:   `ubuntu-[\d.]+-live-server-arm64\.iso$`,
		DefaultInterval:   24,
	},
	{
		ID:                "debian-netinst-amd64",
		Name:              "Debian Netinst (amd64)",
		Distro:            "debian",
		Variant:           "netinst",
		Arch:              "amd64",
		BaseURL:           "https://cdimage.debian.org/debian-cd/",
		VersionDirPattern: "",
		ISOPathTemplate:   "current/amd64/iso-cd/",
		FilenamePattern:   `debian-[\d.]+-amd64-netinst\.iso$`,
		DefaultInterval:   24,
	},
	{
		ID:                "rocky-minimal-amd64",
		Name:              "Rocky Minimal (amd64)",
		Distro:            "rocky",
		Variant:           "minimal",
		Arch:              "amd64",
		BaseURL:           "https://download.rockylinux.org/pub/rocky/",
		VersionDirPattern: `\d+`,
		ISOPathTemplate:   "{version}/isos/x86_64/",
		FilenamePattern:   `Rocky-[\d.]+-x86_64-minimal\.iso$`,
		DefaultInterval:   24,
	},
	{
		ID:                "alpine-standard-amd64",
		Name:              "Alpine Standard (amd64)",
		Distro:            "alpine",
		Variant:           "standard",
		Arch:              "amd64",
		BaseURL:           "https://dl-cdn.alpinelinux.org/alpine/",
		VersionDirPattern: `v\d+\.\d+`,
		ISOPathTemplate:   "{version}/releases/x86_64/",
		FilenamePattern:   `alpine-standard-[\d.]+-x86_64\.iso$`,
		DefaultInterval:   24,
	},
}

// ListProfiles returns a copy of all built-in distro profiles.
// The returned slice is safe to modify by the caller.
func ListProfiles() []DistroProfile {
	result := make([]DistroProfile, len(distroProfiles))
	copy(result, distroProfiles)
	return result
}

// GetProfile looks up a built-in distro profile by ID.
// Returns nil if no profile with the given ID exists.
func GetProfile(id string) *DistroProfile {
	for _, p := range distroProfiles {
		if p.ID == id {
			// Return a copy to prevent caller from mutating internal data
			cp := p
			return &cp
		}
	}
	return nil
}

// mirrorArchMap maps DB arch names to mirror directory arch names.
var mirrorArchMap = map[string]string{
	"amd64": "x86_64",
	"arm64": "aarch64",
}

// MirrorArch converts a DB architecture name to the corresponding
// mirror directory naming convention. Returns the input unchanged
// if no mapping exists.
func MirrorArch(dbArch string) string {
	if mapped, ok := mirrorArchMap[dbArch]; ok {
		return mapped
	}
	return dbArch
}
