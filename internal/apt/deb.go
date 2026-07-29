package apt

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"strings"
)

// DebInfo holds the control metadata of a .deb package — what APT's Packages
// index needs. Fields not present in the control file stay empty.
type DebInfo struct {
	Package       string
	Version       string
	Architecture  string
	Maintainer    string
	InstalledSize string
	Description   string
	Depends       string
	PreDepends    string
	Recommends    string
	Suggests      string
	Conflicts     string
	Breaks        string
	Replaces      string
	Provides      string
	Section       string
	Priority      string
	Homepage      string
	// Filename/Size/SHA256 are set by the repo generator (from the FileRepo
	// row), not the control file, so they are not parsed here.
}

// ParseDeb reads a .deb's control metadata. It opens the ar archive, locates
// control.tar.{gz,xz,zst}, extracts the ./control file, and parses its
// RFC822-style fields. Returns an error if the control member is missing.
//
// Supported control archive compressions: gzip. xz/zstd are detected and
// reported with a clear error if unsupported (dpkg-deb defaults vary by distro;
// gzip is the safe, universally-supported choice for our served packages).
func ParseDeb(r io.Reader) (*DebInfo, error) {
	entries, err := ReadAr(r)
	if err != nil {
		return nil, fmt.Errorf("read ar: %w", err)
	}
	ctrl := memberNamed(entries, "control.tar")
	if ctrl == nil {
		return nil, fmt.Errorf("deb has no control.tar.* member")
	}

	tarBytes, derr := decompressControl(ctrl)
	if derr != nil {
		return nil, derr
	}
	controlText, perr := extractControlFile(tarBytes)
	if perr != nil {
		return nil, perr
	}
	return parseControl(controlText), nil
}

// decompressControl decompresses a control.tar.* member to raw tar bytes based
// on the member's name suffix.
func decompressControl(entry *ArEntry) ([]byte, error) {
	switch {
	case strings.HasSuffix(entry.Name, ".gz"):
		zr, err := gzip.NewReader(bytes.NewReader(entry.Data))
		if err != nil {
			return nil, fmt.Errorf("gzip control.tar: %w", err)
		}
		defer zr.Close()
		return io.ReadAll(zr)
	case strings.HasSuffix(entry.Name, ".xz"):
		return nil, fmt.Errorf("xz-compressed control.tar not supported yet (rebuild the deb or repackage as gzip)")
	case strings.HasSuffix(entry.Name, ".zst"):
		return nil, fmt.Errorf("zstd-compressed control.tar not supported yet")
	default:
		// Uncompressed control.tar (rare but valid).
		return entry.Data, nil
	}
}

// extractControlFile reads a tar stream and returns the contents of the
// "control" (or "./control", "./") member: the package control file.
func extractControlFile(tarBytes []byte) (string, error) {
	tr := tar.NewReader(bytes.NewReader(tarBytes))
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return "", fmt.Errorf("control file not found in control.tar")
		}
		if err != nil {
			return "", fmt.Errorf("read control.tar: %w", err)
		}
		name := strings.TrimPrefix(hdr.Name, "./")
		// The control file is named "control" at the archive root. Some tools
		// name it "./" — treat the first regular file named "control" as it.
		if name == "control" && hdr.Typeflag == tar.TypeReg {
			b, rerr := io.ReadAll(tr)
			if rerr != nil {
				return "", fmt.Errorf("read control: %w", rerr)
			}
			return string(b), nil
		}
	}
}

// parseControl parses an RFC822-style Debian control file (the package stanza)
// into a DebInfo. It reads only the first paragraph (the source/binary stanza).
// Multiline continuations (lines beginning with whitespace) are folded into the
// preceding field (used by Description, Depends, etc.).
func parseControl(text string) *DebInfo {
	info := &DebInfo{}
	fields := parseRFC822(text)
	info.Package = fields["Package"]
	info.Version = fields["Version"]
	info.Architecture = fields["Architecture"]
	info.Maintainer = fields["Maintainer"]
	info.InstalledSize = fields["Installed-Size"]
	info.Description = fields["Description"]
	info.Depends = fields["Depends"]
	info.PreDepends = fields["Pre-Depends"]
	info.Recommends = fields["Recommends"]
	info.Suggests = fields["Suggests"]
	info.Conflicts = fields["Conflicts"]
	info.Breaks = fields["Breaks"]
	info.Replaces = fields["Replaces"]
	info.Provides = fields["Provides"]
	info.Section = fields["Section"]
	info.Priority = fields["Priority"]
	info.Homepage = fields["Homepage"]
	return info
}

// parseRFC822 parses one Debian control stanza into a field map, folding
// continuation lines (those beginning with space/tab) into the prior field.
func parseRFC822(text string) map[string]string {
	out := make(map[string]string)
	var lastKey string
	for _, raw := range strings.Split(text, "\n") {
		if strings.TrimSpace(raw) == "" {
			if lastKey != "" {
				// Blank line ends a stanza; we only want the first.
				break
			}
			continue
		}
		// Continuation line?
		if raw[0] == ' ' || raw[0] == '\t' {
			if lastKey != "" {
				out[lastKey] += "\n" + raw
			}
			continue
		}
		idx := strings.IndexByte(raw, ':')
		if idx < 0 {
			continue
		}
		key := strings.TrimSpace(raw[:idx])
		val := strings.TrimSpace(raw[idx+1:])
		out[key] = val
		lastKey = key
	}
	return out
}

// SortedFieldKeys returns DebInfo field names in canonical Packages-file order,
// for deterministic index generation. Only non-empty fields are included by the
// caller.
var canonicalOrder = []string{
	"Package", "Version", "Architecture", "Maintainer", "Installed-Size",
	"Depends", "Pre-Depends", "Recommends", "Suggests", "Conflicts",
	"Breaks", "Replaces", "Provides", "Section", "Priority", "Homepage",
	"Description",
}

// controlPairs returns the DebInfo fields as ordered (key,value) pairs in
// canonical APT Packages-file order, skipping empty values.
func (d *DebInfo) controlPairs() []struct{ k, v string } {
	m := map[string]string{
		"Package": d.Package, "Version": d.Version, "Architecture": d.Architecture,
		"Maintainer": d.Maintainer, "Installed-Size": d.InstalledSize,
		"Description": d.Description, "Depends": d.Depends, "Pre-Depends": d.PreDepends,
		"Recommends": d.Recommends, "Suggests": d.Suggests, "Conflicts": d.Conflicts,
		"Breaks": d.Breaks, "Replaces": d.Replaces, "Provides": d.Provides,
		"Section": d.Section, "Priority": d.Priority, "Homepage": d.Homepage,
	}
	var pairs []struct{ k, v string }
	for _, k := range canonicalOrder {
		if m[k] != "" {
			pairs = append(pairs, struct{ k, v string }{k, m[k]})
		}
	}
	return pairs
}
