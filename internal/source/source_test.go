package source

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Mi-Bee-Studio/mibeehive/internal/model"
	"github.com/Mi-Bee-Studio/mibeehive/internal/rulesrc"
)

// stubServer serves a recorded GitHub-releases JSON for the migration
// equivalence test (no real network).
var stubServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`[
	  {"tag_name":"v3.11.3","prerelease":false,"draft":false,"assets":[
	    {"name":"prometheus-3.11.3.linux-amd64.tar.gz","browser_download_url":"https://gh/p/3.11.3/amd64.tar.gz","size":100},
	    {"name":"prometheus-3.11.3.linux-arm64.tar.gz","browser_download_url":"https://gh/p/3.11.3/arm64.tar.gz","size":90}
	  ]},
	  {"tag_name":"v3.12.0-rc.1","prerelease":true,"draft":false,"assets":[
	    {"name":"ignored.tar.gz","browser_download_url":"https://gh/ignored","size":1}
	  ]}
	]`))
}))

// stubCrawler is a minimal crawler.Crawler for testing the LegacyAdapter.
type stubCrawler struct {
	typ    model.SourceType
	owner  string
	repo   string
	assets []model.ReleaseAsset
	err    error
	called bool
}

func (s *stubCrawler) Name() string                 { return string(s.typ) }
func (s *stubCrawler) SourceType() model.SourceType { return s.typ }
func (s *stubCrawler) FetchReleases(_ context.Context, owner, repo string) ([]model.ReleaseAsset, error) {
	s.called = true
	s.owner, s.repo = owner, repo
	return s.assets, s.err
}

func TestRegistryRoutesByType(t *testing.T) {
	reg := NewRegistry()
	a := NewLegacyAdapter(&stubCrawler{typ: "github"})
	reg.Register(a)

	got, ok := reg.Get("github")
	if !ok {
		t.Fatal("expected github fetcher registered")
	}
	if got != a {
		t.Error("Get returned a different fetcher instance")
	}
	if _, ok := reg.Get("nope"); ok {
		t.Error("expected miss for unregistered type")
	}
}

func TestLegacyAdapterTranslatesParams(t *testing.T) {
	sc := &stubCrawler{typ: "github", assets: []model.ReleaseAsset{{Filename: "x"}}}
	a := NewLegacyAdapter(sc)

	assets, err := a.Fetch(context.Background(), Source{
		Name: "p", Type: "github", Params: AsParams("owner", "prometheus", "repo", "prometheus"),
	})
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if len(assets) != 1 || assets[0].Filename != "x" {
		t.Fatalf("unexpected assets: %+v", assets)
	}
	// Verify owner/repo were passed through from Params.
	if sc.owner != "prometheus" || sc.repo != "prometheus" {
		t.Errorf("owner/repo not translated: got %q/%q", sc.owner, sc.repo)
	}
	if !sc.called {
		t.Error("wrapped crawler was not called")
	}
}

func TestLegacyAdapterMissingParamsDefaultEmpty(t *testing.T) {
	// Sources that ignore owner/repo (e.g. Go) pass an empty map; adapter must
	// not panic and must pass empty strings.
	sc := &stubCrawler{typ: "go"}
	a := NewLegacyAdapter(sc)
	if _, err := a.Fetch(context.Background(), Source{Name: "g", Type: "go", Params: map[string]string{}}); err != nil {
		t.Fatalf("Fetch with empty params: %v", err)
	}
	if sc.owner != "" || sc.repo != "" {
		t.Errorf("expected empty owner/repo, got %q/%q", sc.owner, sc.repo)
	}
}

func TestLegacyAdapterPropagatesError(t *testing.T) {
	want := errors.New("boom")
	a := NewLegacyAdapter(&stubCrawler{typ: "npm", err: want})
	if _, err := a.Fetch(context.Background(), Source{Name: "n", Type: "npm"}); err != want {
		t.Errorf("expected error propagated, got %v", err)
	}
}

func TestFromProjectMapsSettings(t *testing.T) {
	s := FromProject("p", "github", model.ProjectSettings{GitHubOwner: "o", GitHubRepo: "r"})
	if s.Name != "p" || s.Type != "github" || s.Params["owner"] != "o" || s.Params["repo"] != "r" {
		t.Errorf("FromProject mapping wrong: %+v", s)
	}
}

func TestRegistryTypes(t *testing.T) {
	reg := NewRegistry()
	reg.Register(NewLegacyAdapter(&stubCrawler{typ: "github"}))
	reg.Register(NewLegacyAdapter(&stubCrawler{typ: "npm"}))
	types := reg.Types()
	if len(types) != 2 {
		t.Fatalf("expected 2 types, got %d (%v)", len(types), types)
	}
}

// TestRuleFetcherLoadsBuiltins verifies the embedded github fingerprint loads
// and is addressable by name. This is the migration-readiness check: a builtin
// fingerprint exists that can replace the GitHub crawler.
func TestRuleFetcherLoadsBuiltins(t *testing.T) {
	rf, err := NewRuleFetcher()
	if err != nil {
		t.Fatalf("NewRuleFetcher: %v", err)
	}
	sources := rf.Sources()
	// Sources = builtin fingerprint names + "rulesrc".
	if !contains(sources, "github") {
		t.Errorf("expected sources to include 'github', got %v", sources)
	}
	if !contains(sources, "rulesrc") {
		t.Errorf("expected sources to include 'rulesrc', got %v", sources)
	}
	// The github fingerprint must be registered by name.
	if _, ok := rf.builtins["github"]; !ok {
		t.Error("expected builtin fingerprint named 'github'")
	}
}

func contains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

// TestRuleFetcherGitHubEquivalent is the migration validation: a github Source
// served via the RuleFetcher must produce the same ReleaseAsset shape the old
// GitHubCrawler did (version stripped of "v", os/arch classified), using a
// recorded GitHub-releases fixture. This proves the rule track can replace the
// code track for single-page sources.
func TestRuleFetcherGitHubEquivalent(t *testing.T) {
	// Build a RuleFetcher with an in-process github fingerprint pointed at a
	// stub server, to avoid the network. Override the URL via a custom spec.
	spec := &rulesrc.Spec{
		APIVersion: "rulesrc/v1", Kind: "Source", Name: "github",
		Request: rulesrc.Request{
			URL:    stubServer.URL + "/repos/{owner}/{repo}/releases?per_page=10",
			Format: rulesrc.FormatJSON,
			Header: []rulesrc.Field{{Name: "Accept", Value: "application/vnd.github+json"}},
		},
		List: rulesrc.List{Path: "[]", Skip: []string{".prerelease", ".draft"}},
		Extract: rulesrc.Extract{
			Version: ".tag_name", VersionStrip: "v", Classify: "filename-parser",
			Assets: &rulesrc.AssetExtract{
				Path: ".assets[]", Filename: ".name",
				DownloadURL: ".browser_download_url", Size: ".size",
			},
		},
	}
	rf := &RuleFetcher{rulesrc: rulesrc.NewFetcher(), builtins: map[string]*rulesrc.Spec{"github": spec}}

	assets, err := rf.Fetch(context.Background(), Source{
		Name: "prometheus", Type: "rulesrc",
		Params: AsParams("fingerprint", "github", "owner", "prometheus", "repo", "prometheus"),
	})
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	// Two stable assets (prerelease skipped), matching the fixture.
	if len(assets) != 2 {
		t.Fatalf("want 2 assets, got %d: %+v", len(assets), assets)
	}
	a := assets[0]
	if a.Version != "3.11.3" { // "v" stripped
		t.Errorf("version: want 3.11.3, got %q", a.Version)
	}
	if a.OS != "linux" || a.Arch != "amd64" {
		t.Errorf("classify linux/amd64, got %s/%s", a.OS, a.Arch)
	}
}

// TestRuleFetcherTokenInjection verifies that a token resolved by
// SetTokenResolver is sent as an Authorization header on the fingerprint
// request — without mutating the shared builtin spec (concurrency-safe).
func TestRuleFetcherTokenInjection(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"tag_name":"v1.0.0","prerelease":false,"draft":false,"assets":[]}]`))
	}))
	defer srv.Close()

	spec := &rulesrc.Spec{
		APIVersion: "rulesrc/v1", Kind: "Source", Name: "github",
		Request: rulesrc.Request{URL: srv.URL + "/repos/{owner}/{repo}/releases", Format: rulesrc.FormatJSON},
		List:    rulesrc.List{Path: "[]", Skip: []string{".prerelease", ".draft"}},
		Extract: rulesrc.Extract{Version: ".tag_name", VersionStrip: "v"},
	}
	rf := &RuleFetcher{rulesrc: rulesrc.NewFetcher(), builtins: map[string]*rulesrc.Spec{"github": spec}}
	rf.SetTokenResolver(func(sourceType string) string {
		if sourceType == "github" {
			return "secret-pat-123"
		}
		return ""
	})

	if _, err := rf.Fetch(context.Background(), Source{
		Name: "p", Type: "github", Params: AsParams("owner", "o", "repo", "r"),
	}); err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if gotAuth != "token secret-pat-123" {
		t.Errorf("Authorization header: want %q, got %q", "token secret-pat-123", gotAuth)
	}
	// The builtin spec must NOT be mutated (no header persisted on the shared spec).
	if len(spec.Request.Header) != 0 {
		t.Errorf("shared builtin spec was mutated, header=%v", spec.Request.Header)
	}
}

// TestRuleFetcherNoTokenWhenResolverNil verifies that without a resolver (or an
// empty token), no Authorization header is added (the default/anonymous case).
func TestRuleFetcherNoTokenWhenResolverNil(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	spec := &rulesrc.Spec{
		APIVersion: "rulesrc/v1", Kind: "Source", Name: "github",
		Request: rulesrc.Request{URL: srv.URL, Format: rulesrc.FormatJSON},
		List:    rulesrc.List{Path: "[]"},
		Extract: rulesrc.Extract{Version: ".tag_name"},
	}
	rf := &RuleFetcher{rulesrc: rulesrc.NewFetcher(), builtins: map[string]*rulesrc.Spec{"github": spec}}
	// No SetTokenResolver.
	if _, err := rf.Fetch(context.Background(), Source{Name: "p", Type: "github"}); err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if gotAuth != "" {
		t.Errorf("expected no Authorization header, got %q", gotAuth)
	}
}
