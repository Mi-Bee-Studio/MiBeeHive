package source

import (
	"context"
	"embed"
	"fmt"
	"io/fs"

	"github.com/Mi-Bee-Studio/mibeehive/internal/model"
	"github.com/Mi-Bee-Studio/mibeehive/internal/rulesrc"
)

// builtinFingerprints embeds the built-in fingerprint YAMLs (single-page
// sources). Adding a source of this kind = adding a YAML here, no recompile of
// crawl logic. (User-defined fingerprints will live in the DB later.)
//
//go:embed fingerprints/*.yaml
var builtinFingerprints embed.FS

// RuleFetcher is the "rule track" Fetcher: it runs a YAML fingerprint via the
// internal/rulesrc engine. A single instance serves all "rulesrc"-type sources,
// resolving which fingerprint to use from Source.Params["fingerprint"].
//
// Built-in fingerprints are registered by name (see RegisterFingerprint); this
// is how single-page sources get added WITHOUT recompiling the binary (the
// fingerprints can be embedded YAML or, later, user-defined in the DB).
type RuleFetcher struct {
	rulesrc  *rulesrc.Fetcher
	builtins map[string]*rulesrc.Spec // name -> fingerprint
	// tokens resolves an optional auth token for a source type (e.g. a GitHub
	// PAT read from source_credentials). When non-nil and non-empty for a type,
	// the token is injected as an Authorization header at fetch time so secrets
	// never live in the fingerprint YAML. Set via SetTokenResolver.
	tokens func(sourceType string) string
}

// NewRuleFetcher builds a RuleFetcher over the rulesrc engine and loads all
// built-in fingerprints embedded under fingerprints/*.yaml.
func NewRuleFetcher() (*RuleFetcher, error) {
	rf := &RuleFetcher{rulesrc: rulesrc.NewFetcher(), builtins: make(map[string]*rulesrc.Spec)}
	if err := rf.loadBuiltins(); err != nil {
		return nil, err
	}
	return rf, nil
}

// SetTokenResolver wires per-source-type auth token resolution. Optional; call
// before serving requests. Used to inject API tokens (GitHub/HashiCorp PATs)
// into fingerprint requests without storing secrets in YAML.
func (r *RuleFetcher) SetTokenResolver(f func(sourceType string) string) { r.tokens = f }

// loadBuiltins parses every embedded fingerprints/*.yaml and registers it.
func (r *RuleFetcher) loadBuiltins() error {
	entries, err := fs.ReadDir(builtinFingerprints, "fingerprints")
	if err != nil {
		return fmt.Errorf("read builtin fingerprints: %w", err)
	}
	for _, e := range entries {
		data, err := builtinFingerprints.ReadFile("fingerprints/" + e.Name())
		if err != nil {
			return fmt.Errorf("read fingerprint %s: %w", e.Name(), err)
		}
		spec, err := rulesrc.ParseSpec(data)
		if err != nil {
			return fmt.Errorf("parse fingerprint %s: %w", e.Name(), err)
		}
		r.builtins[spec.Name] = spec
	}
	return nil
}

// RegisterFingerprint makes a fingerprint available by name at runtime
// (used for user-defined fingerprints in future, and tests).
func (r *RuleFetcher) RegisterFingerprint(spec *rulesrc.Spec) {
	r.builtins[spec.Name] = spec
}

// Sources reports the names of the built-in fingerprints this Fetcher serves.
// A source_type matching a fingerprint name (e.g. "github") routes here, so a
// project with source_type "github" is served by the github fingerprint.
// Additionally the literal "rulesrc" type is always handled, with the specific
// fingerprint chosen by Source.Params["fingerprint"] (for user-defined sources).
func (r *RuleFetcher) Sources() []string {
	out := make([]string, 0, len(r.builtins)+1)
	for name := range r.builtins {
		out = append(out, name)
	}
	out = append(out, "rulesrc")
	return out
}

// Fetch resolves the fingerprint for the source and runs it through the rulesrc
// engine (with URL template substitution from Params), then applies filters.
// For a builtin type (e.g. source_type "github") the fingerprint named by the
// type is used. For the generic "rulesrc" type, the fingerprint is named by
// Source.Params["fingerprint"].
func (r *RuleFetcher) Fetch(ctx context.Context, src Source) ([]model.ReleaseAsset, error) {
	name := src.Params["fingerprint"]
	if name == "" {
		name = src.Type // builtin type name == fingerprint name
	}

	// Inline YAML: if the caller provides a full fingerprint in
	// Params["fingerprint_yaml"], parse and use it directly (bypassing
	// builtins). This enables adding new sources at runtime without
	// recompiling — the YAML lives in the project config (DB).
	if inlineYAML := src.Params["fingerprint_yaml"]; inlineYAML != "" {
		spec, err := rulesrc.ParseSpec([]byte(inlineYAML))
		if err != nil {
			return nil, fmt.Errorf("rule source %q: invalid inline fingerprint: %w", src.Name, err)
		}
		return r.fetchWithSpec(ctx, spec, src, name)
	}

	spec, ok := r.builtins[name]
	if !ok {
		return nil, fmt.Errorf("rule source %q: no fingerprint named %q registered", src.Name, name)
	}
	return r.fetchWithSpec(ctx, spec, src, name)
}

// fetchWithSpec runs a resolved spec through the rulesrc engine with token
// injection and URL template substitution.
func (r *RuleFetcher) fetchWithSpec(ctx context.Context, spec *rulesrc.Spec, src Source, tokenKey string) ([]model.ReleaseAsset, error) {
	// Inject an auth token if a resolver is wired and yields one for this type.
	if r.tokens != nil {
		if tok := r.tokens(tokenKey); tok != "" {
			spec = withAuthHeader(spec, "token "+tok)
		}
	}

	// Params other than "fingerprint" and "fingerprint_yaml" are forwarded as URL template values.
	params := make(map[string]string, len(src.Params))
	for k, v := range src.Params {
		if k != "fingerprint" && k != "fingerprint_yaml" {
			params[k] = v
		}
	}
	assets, err := r.rulesrc.FetchWithParams(ctx, spec, params)
	if err != nil {
		return nil, err
	}
	return rulesrc.ApplyFilter(assets, spec), nil
}

// withAuthHeader returns a shallow copy of spec whose request carries an extra
// Authorization header, leaving the original builtin untouched (concurrency-safe).
func withAuthHeader(spec *rulesrc.Spec, value string) *rulesrc.Spec {
	cp := *spec
	cp.Request = spec.Request
	cp.Request.Header = append([]rulesrc.Field{}, spec.Request.Header...)
	cp.Request.Header = append(cp.Request.Header, rulesrc.Field{Name: "Authorization", Value: value})
	return &cp
}
