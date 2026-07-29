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
	spec, ok := r.builtins[name]
	if !ok {
		return nil, fmt.Errorf("rule source %q: no fingerprint named %q registered", src.Name, name)
	}
	// Params other than "fingerprint" are forwarded as URL template values.
	params := make(map[string]string, len(src.Params))
	for k, v := range src.Params {
		if k != "fingerprint" {
			params[k] = v
		}
	}
	assets, err := r.rulesrc.FetchWithParams(ctx, spec, params)
	if err != nil {
		return nil, err
	}
	return rulesrc.ApplyFilter(assets, spec), nil
}
