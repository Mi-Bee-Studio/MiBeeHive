// Package perf contains performance baseline benchmarks for the supply layer's
// hot paths: concurrent file downloads, virtual-path resolution, and WebDAV
// PROPFIND-style multistatus generation. It uses only the standard library.
//
// After the benchmarks run, results are written to .sisyphus/baseline.json so
// future runs can be compared against a known-good baseline.
package perf

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Mi-Bee-Studio/mibeehive/internal/cache"
)

// baselineResults accumulates benchmark results across runs so TestMain can
// persist them to .sisyphus/baseline.json after all benchmarks complete.
var (
	baselineMu     sync.Mutex
	baselineResults = map[string]float64{}
)

// recordBaseline stores the result of a completed benchmark.
func recordBaseline(name string, b *testing.B) {
	baselineMu.Lock()
	defer baselineMu.Unlock()
	baselineResults[name] = float64(b.Elapsed().Nanoseconds()) / float64(b.N)
}

// writeBaselineFile persists the collected results to .sisyphus/baseline.json
// relative to the repository root. It is a no-op if no benchmarks ran.
func writeBaselineFile() error {
	baselineMu.Lock()
	defer baselineMu.Unlock()
	if len(baselineResults) == 0 {
		return nil
	}
	path := filepath.Join(repoRoot(), ".sisyphus", "baseline.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create baseline dir: %w", err)
	}
	data, err := json.MarshalIndent(baselineResults, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal baseline: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write baseline: %w", err)
	}
	return nil
}

// repoRoot walks up from the current working directory to find the directory
// containing go.mod (the repository root). It falls back to the cwd if no
// marker is found, so the baseline is still written somewhere sensible.
func repoRoot() string {
	dir, err := os.Getwd()
	if err != nil {
		return "."
	}
	start := dir
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return start
}

// TestMain runs the benchmarks (via m.Run) and then persists the baseline.
func TestMain(m *testing.M) {
	code := m.Run()
	if err := writeBaselineFile(); err != nil {
		fmt.Fprintf(os.Stderr, "perf: failed to write baseline: %v\n", err)
	}
	os.Exit(code)
}

// BenchmarkConcurrentDownload simulates N goroutines each resolving a token to
// a file and streaming its content (the token→file→ServeContent hot path).
func BenchmarkConcurrentDownload(b *testing.B) {
	b.ReportAllocs()

	// Seed a temp file and a token→file mapping in the shared TokenCache.
	dir := b.TempDir()
	path := filepath.Join(dir, "payload.bin")
	payload := strings.Repeat("x", 64*1024) // 64KiB payload
	if err := os.WriteFile(path, []byte(payload), 0o644); err != nil {
		b.Fatalf("write temp payload: %v", err)
	}
	const token = "bench-token"
	cache.TokenCache.Put(token, 1)

	// Pre-warm the cache so the benchmark measures steady-state hits.
	if _, ok := cache.TokenCache.Get(token); !ok {
		b.Fatal("expected token cache hit after warm-up")
	}

	const workers = 16
	b.SetParallelism(workers)
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			// token → file_id lookup.
			if _, ok := cache.TokenCache.Get(token); !ok {
				b.Fatal("token cache miss during benchmark")
			}
			// ServeContent from the temp file.
			f, err := os.Open(path)
			if err != nil {
				b.Fatalf("open payload: %v", err)
			}
			rec := httptest.NewRecorder()
			http.ServeContent(rec, httptest.NewRequest(http.MethodGet, "/", nil), "payload.bin", time.Time{}, f)
			f.Close()
			if rec.Code != http.StatusOK {
				b.Fatalf("unexpected status %d", rec.Code)
			}
		}
	})
	recordBaseline("BenchmarkConcurrentDownload", b)
}

// BenchmarkResolvePath simulates N goroutines resolving virtual paths through
// the path cache (the WebDAV path→node resolution hot path).
func BenchmarkResolvePath(b *testing.B) {
	b.ReportAllocs()

	// Pre-populate the path cache with a set of virtual paths.
	const entries = 1024
	for i := 0; i < entries; i++ {
		cache.PathCache.Put(fmt.Sprintf("/virtual/dir-%d/file-%d", i%64, i), int64(i))
	}

	const workers = 16
	b.SetParallelism(workers)
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			key := fmt.Sprintf("/virtual/dir-%d/file-%d", i%64, i%entries)
			if _, ok := cache.PathCache.Get(key); !ok {
				b.Fatal("path cache miss during benchmark")
			}
			i++
		}
	})
	recordBaseline("BenchmarkResolvePath", b)
}

// BenchmarkPropfind simulates generating a WebDAV PROPFIND multistatus
// response body for a directory listing (the PROPFIND hot path).
func BenchmarkPropfind(b *testing.B) {
	b.ReportAllocs()

	names := make([]string, 64)
	for i := range names {
		names[i] = fmt.Sprintf("file-%d.bin", i)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var sb strings.Builder
		sb.WriteString(`<?xml version="1.0"?><D:multistatus xmlns:D="DAV:">`)
		for _, n := range names {
			sb.WriteString(`<D:response><D:href>/webdav/`)
			sb.WriteString(n)
			sb.WriteString(`</D:href><D:propstat><D:prop><D:getcontentlength>65536</D:getcontentlength></D:prop><D:status>HTTP/1.1 200 OK</D:status></D:propstat></D:response>`)
		}
		sb.WriteString(`</D:multistatus>`)
		_ = sb.String()
	}
	recordBaseline("BenchmarkPropfind", b)
}