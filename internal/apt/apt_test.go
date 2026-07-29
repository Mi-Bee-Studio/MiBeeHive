package apt

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"strconv"
	"strings"
	"testing"
	"time"
)

// makeControlTar builds a gzipped control.tar containing a control file with
// the given text. Used to assemble a minimal .deb for tests.
func makeControlTar(t *testing.T, controlText string) []byte {
	t.Helper()
	var tarBuf bytes.Buffer
	tw := tar.NewWriter(&tarBuf)
	control := []byte(controlText)
	if err := tw.WriteHeader(&tar.Header{Name: "./control", Mode: 0644, Size: int64(len(control)), Typeflag: tar.TypeReg}); err != nil {
		t.Fatalf("write control header: %v", err)
	}
	if _, err := tw.Write(control); err != nil {
		t.Fatalf("write control body: %v", err)
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("close tar: %v", err)
	}
	var gzBuf bytes.Buffer
	zw := gzip.NewWriter(&gzBuf)
	zw.Write(tarBuf.Bytes())
	zw.Close()
	return gzBuf.Bytes()
}

// makeDeb assembles a minimal .deb (ar archive) with a debian-binary member
// and a control.tar.gz member. data.tar is omitted (we only parse control).
func makeDeb(t *testing.T, controlText string) []byte {
	t.Helper()
	var ar bytes.Buffer
	ar.WriteString("!<arch>\n")
	writeArMember := func(name string, data []byte) {
		hdr := fmtArHeader(name, int64(len(data)))
		ar.WriteString(hdr)
		ar.Write(data)
		if len(data)%2 == 1 {
			ar.WriteByte('\n')
		}
	}
	writeArMember("debian-binary", []byte("2.0\n"))
	writeArMember("control.tar.gz", makeControlTar(t, controlText))
	return ar.Bytes()
}

// fmtArHeader builds a 60-byte ar member header (GNU style).
func fmtArHeader(name string, size int64) string {
	// name[16] modtime[12] uid[6] gid[6] mode[8] size[10] end[2]
	const fill = " "
	pad := func(s string, n int) string {
		if len(s) >= n {
			return s[:n]
		}
		return s + strings.Repeat(fill, n-len(s))
	}
	var b strings.Builder
	b.WriteString(pad(name+"/", 16))                // GNU style: trailing slash
	b.WriteString(pad("0", 12))                     // mtime
	b.WriteString(pad("0", 6))                      // uid
	b.WriteString(pad("0", 6))                      // gid
	b.WriteString(pad("100644", 8))                 // mode
	b.WriteString(pad(strconv.Itoa(int(size)), 10)) // size (decimal)
	b.WriteString("`\n")                            // end marker
	return b.String()
}

func TestParseDeb_ExtractsControl(t *testing.T) {
	control := `Package: node-exporter
Version: 1.8.2-1
Architecture: amd64
Maintainer: Test <t@example.com>
Installed-Size: 12345
Depends: libc6, adduser
Section: net
Priority: optional
Homepage: https://example.org
Description: Prometheus node exporter
 Runs on machines to export metrics.
`
	deb := makeDeb(t, control)
	info, err := ParseDeb(bytes.NewReader(deb))
	if err != nil {
		t.Fatalf("ParseDeb: %v", err)
	}
	if info.Package != "node-exporter" {
		t.Errorf("Package: want node-exporter, got %q", info.Package)
	}
	if info.Version != "1.8.2-1" {
		t.Errorf("Version: got %q", info.Version)
	}
	if info.Architecture != "amd64" {
		t.Errorf("Architecture: got %q", info.Architecture)
	}
	if info.Depends != "libc6, adduser" {
		t.Errorf("Depends: got %q", info.Depends)
	}
	// Multiline description folded with a newline continuation.
	if !strings.Contains(info.Description, "Prometheus node exporter") || !strings.Contains(info.Description, "Runs on machines") {
		t.Errorf("Description multiline not folded: %q", info.Description)
	}
}

func TestParseDeb_NoControlMember(t *testing.T) {
	// ar with only debian-binary, no control.tar.
	var ar bytes.Buffer
	ar.WriteString("!<arch>\n")
	ar.WriteString(fmtArHeader("debian-binary", 4))
	ar.WriteString("2.0\n")
	_, err := ParseDeb(bytes.NewReader(ar.Bytes()))
	if err == nil {
		t.Fatal("expected error when control.tar is missing")
	}
}

func TestGenerateRepo_PackagesFields(t *testing.T) {
	entries := []PackageEntry{
		{
			DebInfo:  DebInfo{Package: "node-exporter", Version: "1.8.2-1", Architecture: "amd64", Depends: "libc6"},
			Filename: "pool/main/n/node-exporter/node-exporter_1.8.2-1_amd64.deb", Size: 5000000, SHA256: "abc123",
		},
		{
			DebInfo:  DebInfo{Package: "vim", Version: "2:9.0-1", Architecture: "all", Description: "editor"},
			Filename: "pool/main/v/vim/vim_9.0-1_all.deb", Size: 1000000,
		},
	}
	files, err := GenerateRepo(entries, "stable", time.Date(2026, 7, 30, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("GenerateRepo: %v", err)
	}
	// Expect Packages for amd64 (includes the "all" vim package).
	pkg := findFile(files, "dists/stable/main/binary-amd64/Packages")
	if pkg == nil {
		t.Fatal("amd64 Packages file not generated")
	}
	s := string(pkg.Content)
	// Must contain both packages (all folded into amd64).
	if !strings.Contains(s, "Package: node-exporter") {
		t.Error("amd64 Packages missing node-exporter")
	}
	if !strings.Contains(s, "Package: vim") {
		t.Error("amd64 Packages missing vim (all-arch)")
	}
	// Size + Filename + SHA256 present for node-exporter.
	if !strings.Contains(s, "Size: 5000000") {
		t.Error("Size field missing")
	}
	if !strings.Contains(s, "Filename: pool/main/n/node-exporter/node-exporter_1.8.2-1_amd64.deb") {
		t.Error("Filename field missing/wrong")
	}
	if !strings.Contains(s, "SHA256: abc123") {
		t.Error("SHA256 field missing")
	}

	// Packages.gz must be valid gzip.
	gz := findFile(files, "dists/stable/main/binary-amd64/Packages.gz")
	if gz == nil {
		t.Fatal("Packages.gz not generated")
	}
	zr, err := gzip.NewReader(bytes.NewReader(gz.Content))
	if err != nil {
		t.Fatalf("Packages.gz not valid gzip: %v", err)
	}
	if _, err := io.ReadAll(zr); err != nil {
		t.Fatalf("decompress Packages.gz: %v", err)
	}

	// Suite Release must list checksums and the architectures.
	rel := findFile(files, "dists/stable/Release")
	if rel == nil {
		t.Fatal("suite Release not generated")
	}
	rs := string(rel.Content)
	if !strings.Contains(rs, "Architectures: amd64") {
		t.Error("Release missing Architectures")
	}
	if !strings.Contains(rs, "Components: main") {
		t.Error("Release missing Components")
	}
	if !strings.Contains(rs, "dists/stable/main/binary-amd64/Packages") {
		t.Error("Release missing Packages checksum line")
	}
}

func TestArReader_NonArchive(t *testing.T) {
	_, err := ReadAr(bytes.NewReader([]byte("not an ar file at all............")))
	if err == nil {
		t.Fatal("expected error for non-ar input")
	}
}

func findFile(files []RepoFile, path string) *RepoFile {
	for i := range files {
		if files[i].Path == path {
			return &files[i]
		}
	}
	return nil
}
