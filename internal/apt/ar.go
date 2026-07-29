// Package apt implements Debian package (.deb) metadata parsing and APT
// repository layout generation for the supply layer. It lets MiBeeHive expose
// collected .deb files as a standard Debian repository that `apt` can consume.
//
// Pure stdlib: a minimal `ar` archive reader (stdlib has none), tar/gzip/zlib
// for the control archive, and RFC822-style control-file parsing.
package apt

import (
	"fmt"
	"io"
	"strings"
)

// arMagic is the 8-byte header that begins every ar archive (including .deb).
const arMagic = "!<arch>\n"

// ArEntry is one member of an ar archive (a .deb contains debian-binary,
// control.tar.*, data.tar.*).
type ArEntry struct {
	Name string
	Size int64
	Data []byte
}

// ReadAr parses an ar archive (e.g. a .deb file) and returns its members in
// order. Names have trailing spaces and version suffixes ("/0" etc.) stripped,
// matching how Debian tooling names members (e.g. "control.tar.gz").
func ReadAr(r io.Reader) ([]ArEntry, error) {
	// Read the whole archive: members are small except data.tar, which we don't
	// need for metadata. To bound memory, callers pass a .deb; if very large,
	// a streaming reader would be preferable. Metadata parsing only reads
	// control.tar.*, so a size cap is enforced by the caller (FileService limits).
	header := make([]byte, 8)
	if _, err := io.ReadFull(r, header); err != nil {
		return nil, fmt.Errorf("read ar magic: %w", err)
	}
	if string(header) != arMagic {
		return nil, fmt.Errorf("not an ar archive: bad magic %q", header)
	}

	var entries []ArEntry
	for {
		block := make([]byte, 60)
		n, err := io.ReadFull(r, block)
		if err == io.EOF || (err == io.ErrUnexpectedEOF && n == 0) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read ar header: %w", err)
		}
		name := strings.TrimSpace(strings.TrimRight(string(block[0:16]), " "))
		// Debian long-name member "//" lookup table: not handled; modern debs use
		// short names with a "#1" length-prefixed body. We handle the common case.
		sizeStr := strings.TrimSpace(string(block[48:58]))
		size, serr := parseOctalOrDecimal(sizeStr)
		if serr != nil {
			return nil, fmt.Errorf("parse ar member size %q: %w", sizeStr, serr)
		}

		data := make([]byte, size)
		if _, err := io.ReadFull(r, data); err != nil {
			return nil, fmt.Errorf("read ar member %q: %w", name, err)
		}
		entries = append(entries, ArEntry{Name: arName(name), Size: size, Data: data})

		// ar members are padded to a 2-byte boundary with "\n".
		if size%2 == 1 {
			io.CopyN(io.Discard, r, 1)
		}
	}
	return entries, nil
}

// arName normalizes a member name: strip BSD/Mac "#1/<len>" prefix data-name
// encoding by reading nothing (we rely on GNU short names used by dpkg-deb).
func arName(name string) string {
	// GNU style: "name/" → "name"; "control.tar.gz" stays.
	name = strings.TrimRight(name, "/")
	return name
}

// parseOctalOrDecimal parses ar size fields (decimal in GNU ar).
func parseOctalOrDecimal(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty size")
	}
	var n int64
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("non-digit %q", c)
		}
		n = n*10 + int64(c-'0')
	}
	return n, nil
}

// memberNamed returns the first ar member whose name starts with prefix.
func memberNamed(entries []ArEntry, prefix string) *ArEntry {
	for i := range entries {
		if strings.HasPrefix(entries[i].Name, prefix) {
			return &entries[i]
		}
	}
	return nil
}
