package apt

import (
	"bytes"
	"compress/gzip"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

// RepoFile is a generated APT repository file (path + contents). The supply
// handler serves these from memory.
type RepoFile struct {
	Path        string // e.g. "dists/stable/main/binary-amd64/Packages"
	ContentType string // e.g. "text/plain"
	Content     []byte
}

// PackageEntry is a DebInfo paired with the on-disk file metadata needed by the
// Packages index (filename relative to the repo root, byte size, SHA256).
type PackageEntry struct {
	DebInfo
	Filename string // pool-relative path, e.g. "pool/main/n/node-exporter/node-exporter_1.0.deb"
	Size     int64
	SHA256   string // hex; may be empty if not yet computed
}

// GenerateRepo builds the APT repository metadata for a flat single-component,
// single-suite ("stable") repo spanning the given architectures. It produces:
//
//	dists/stable/Release
//	dists/stable/main/binary-<arch>/Release
//	dists/stable/main/binary-<arch>/Packages
//	dists/stable/main/binary-<arch>/Packages.gz
//
// All packages are pooled under pool/main/ (served as raw file downloads by the
// supply handler, not generated here). Entries are grouped by Architecture;
// "all" packages are included in every arch index.
func GenerateRepo(entries []PackageEntry, suite string, date time.Time) ([]RepoFile, error) {
	archs := archsOf(entries)
	suite = orDefault(suite, "stable")

	// Build per-arch Packages stanzas once, then derive Packages/Packages.gz.
	byArchPackages := make(map[string][]byte) // arch -> Packages text
	for _, arch := range archs {
		byArchPackages[arch] = renderPackages(filterByArch(entries, arch))
	}

	var files []RepoFile
	// dists/<suite>/main/binary-<arch>/{Packages,Packages.gz,Release}
	for _, arch := range archs {
		dir := fmt.Sprintf("dists/%s/main/binary-%s", suite, arch)
		pkg := byArchPackages[arch]

		files = append(files, RepoFile{Path: dir + "/Packages", ContentType: "text/plain; charset=utf-8", Content: pkg})
		files = append(files, RepoFile{Path: dir + "/Packages.gz", ContentType: "application/gzip", Content: gzipBytes(pkg)})

		rel := renderArchRelease(dir, suite, arch, pkg)
		files = append(files, RepoFile{Path: dir + "/Release", ContentType: "text/plain; charset=utf-8", Content: []byte(rel)})
	}

	// dists/<suite>/Release (suite-level, with checksums over the above files)
	files = append(files, RepoFile{
		Path:        fmt.Sprintf("dists/%s/Release", suite),
		ContentType: "text/plain; charset=utf-8",
		Content:     []byte(renderSuiteRelease(suite, archs, files, date)),
	})
	return files, nil
}

// archsOf returns the distinct architectures present, always including "all"
// packages folded into each concrete arch. The distinct set here excludes
// "all" (it's merged per-arch).
func archsOf(entries []PackageEntry) []string {
	seen := make(map[string]bool)
	var archs []string
	for _, e := range entries {
		a := orDefault(e.Architecture, "all")
		if a != "all" && !seen[a] {
			seen[a] = true
			archs = append(archs, a)
		}
	}
	if len(archs) == 0 {
		// Only "all" (or empty) packages: emit an amd64 index as a sane default
		// so `apt` on the common arch finds them.
		return []string{"amd64"}
	}
	return archs
}

// filterByArch returns packages matching an architecture OR architecture "all".
func filterByArch(entries []PackageEntry, arch string) []PackageEntry {
	var out []PackageEntry
	for _, e := range entries {
		a := orDefault(e.Architecture, "all")
		if a == arch || a == "all" {
			out = append(out, e)
		}
	}
	return out
}

// renderPackages builds the Packages file text for a set of entries.
func renderPackages(entries []PackageEntry) []byte {
	var b strings.Builder
	for _, e := range entries {
		for _, p := range e.controlPairs() {
			fmt.Fprintf(&b, "%s: %s\n", p.k, p.v)
		}
		fmt.Fprintf(&b, "Filename: %s\n", e.Filename)
		fmt.Fprintf(&b, "Size: %d\n", e.Size)
		if e.SHA256 != "" {
			fmt.Fprintf(&b, "SHA256: %s\n", e.SHA256)
			fmt.Fprintf(&b, "Checksums-Sha256:\n %s %d %s\n", e.SHA256, e.Size, baseName(e.Filename))
		}
		b.WriteString("\n")
	}
	return []byte(b.String())
}

// renderArchRelease renders the per-arch Release file (Acquire metadata).
func renderArchRelease(dir, suite, arch string, packages []byte) string {
	return fmt.Sprintf(`Component: main
Origin: MiBeeHive
Label: MiBeeHive
Archive: %s
Architecture: %s
`, suite, arch)
}

// renderSuiteRelease renders the suite-level Release with checksums over all
// generated index files. The pool/*.deb files are intentionally NOT listed
// (apt fetches their checksums from Packages).
func renderSuiteRelease(suite string, archs []string, indexFiles []RepoFile, date time.Time) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Origin: MiBeeHive\n")
	fmt.Fprintf(&b, "Label: MiBeeHive\n")
	fmt.Fprintf(&b, "Suite: %s\n", suite)
	fmt.Fprintf(&b, "Codename: %s\n", suite)
	fmt.Fprintf(&b, "Date: %s\n", date.UTC().Format(time.RFC1123))
	fmt.Fprintf(&b, "Architectures: %s\n", strings.Join(archs, " "))
	fmt.Fprintf(&b, "Components: main\n")
	fmt.Fprintf(&b, "Description: MiBeeHive ops-tool supply repository\n")
	fmt.Fprintf(&b, "MD5Sum:\n")
	for _, f := range indexFiles {
		md5, size := checksums(f.Content)
		fmt.Fprintf(&b, " %s %8d %s\n", md5, size, f.Path)
	}
	fmt.Fprintf(&b, "SHA1:\n")
	for _, f := range indexFiles {
		_, sha1, size := sha1AndSize(f.Content)
		fmt.Fprintf(&b, " %s %8d %s\n", sha1, size, f.Path)
	}
	fmt.Fprintf(&b, "SHA256:\n")
	for _, f := range indexFiles {
		sha256, size := sha256AndSize(f.Content)
		fmt.Fprintf(&b, " %s %8d %s\n", sha256, size, f.Path)
	}
	return b.String()
}

// gzipBytes gzips the input.
func gzipBytes(in []byte) []byte {
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	_, _ = zw.Write(in)
	_ = zw.Close()
	return buf.Bytes()
}

// checksums returns md5 hex + byte size. (Kept for Release MD5Sum lines; SHA256
// is the authoritative one.) Implemented without extra deps via stdlib.
func checksums(in []byte) (string, int) {
	return md5Hex(in), len(in)
}

func sha1AndSize(in []byte) (string, string, int) {
	return "", sha1Hex(in), len(in)
}

func sha256AndSize(in []byte) (string, int) {
	return sha256Hex(in), len(in)
}

func sha256Hex(in []byte) string {
	sum := sha256.Sum256(in)
	return hex.EncodeToString(sum[:])
}

func sha1Hex(in []byte) string {
	sum := sha1.Sum(in)
	return hex.EncodeToString(sum[:])
}

func md5Hex(in []byte) string {
	sum := md5.Sum(in)
	return hex.EncodeToString(sum[:])
}

// orDefault returns s if non-empty, else d.
func orDefault(s, d string) string {
	if s == "" {
		return d
	}
	return s
}

func baseName(path string) string {
	if i := strings.LastIndexByte(path, '/'); i >= 0 {
		return path[i+1:]
	}
	return path
}
