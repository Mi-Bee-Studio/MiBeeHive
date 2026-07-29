package rulesrc

import "strings"

// classify mirrors internal/crawler.parseFilename to detect os/arch/ext from a
// release filename. It is intentionally a local copy: the validation wants the
// rule engine to stand alone from the crawler package (parseFilename is
// package-private there anyway), and duplicating ~40 lines is cheaper than
// coupling the two packages. If the rule engine graduates from prototype, this
// should be lifted into a shared package (see REPORT.md).

var knownOS = map[string]bool{
	"linux":   true,
	"darwin":  true,
	"windows": true,
	"freebsd": true,
}

var knownArch = map[string]bool{
	"amd64":   true,
	"arm64":   true,
	"armv6":   true,
	"armv7":   true,
	"386":     true,
	"s390x":   true,
	"ppc64le": true,
}

// classification holds the detected os/arch/ext for a filename.
type classification struct {
	os   string
	arch string
	ext  string
}

// classify parses os, arch, and ext from a release asset filename.
//
//	prometheus-3.11.3.linux-arm64.tar.gz -> linux, arm64, tar.gz
//	consul_1.22.5_linux_arm64.zip        -> linux, arm64, zip
func classify(name string) classification {
	ext := compoundExt(name)
	base := name
	if ext != "" {
		base = strings.TrimSuffix(name, "."+ext)
	}

	var osVal, archVal string
	for _, p := range splitOn(name, base) {
		p = strings.ToLower(p)
		if knownOS[p] && osVal == "" {
			osVal = p
			continue
		}
		if knownArch[p] && archVal == "" {
			archVal = p
		}
	}
	return classification{os: osVal, arch: archVal, ext: ext}
}

// compoundExt returns compound extensions first (.tar.gz), else the last ext.
func compoundExt(name string) string {
	for _, ext := range []string{".tar.gz", ".tar.bz2", ".tar.xz"} {
		if strings.HasSuffix(name, ext) {
			return ext[1:] // strip leading dot
		}
	}
	if idx := strings.LastIndex(name, "."); idx >= 0 {
		return name[idx+1:]
	}
	return ""
}

// splitOn splits the filename-with-extension-stripped by common delimiters.
// It mirrors crawler.splitFilename: underscores and dots become hyphens so
// version separators and word separators normalize to one delimiter.
func splitOn(_, stripped string) []string {
	s := strings.ReplaceAll(stripped, "_", "-")
	s = strings.ReplaceAll(s, ".", "-")
	return strings.Split(s, "-")
}
