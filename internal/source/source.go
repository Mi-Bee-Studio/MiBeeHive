// Package source defines the two-track crawl abstraction that replaces the
// GitHub-shaped internal/crawler.Crawler interface.
//
// Three orthogonal concepts (see docs/roadmap/crawl-two-track-design.md):
//
//	Source   — a persisted definition of a crawlable source (name, type, params)
//	Fetcher  — fetches []model.ReleaseAsset for a Source. Two implementations:
//	           a rule-fingerprint engine (internal/rulesrc) and protocol adapters.
//	Asset    — unchanged: model.ReleaseAsset. The download/store pipeline is untouched.
//
// A Registry maps a Source.Type (a free-form string, no DB CHECK) to the Fetcher
// that handles it. This is the runtime validation that replaces the old schema
// constraint.
package source

import (
	"context"
	"fmt"

	"github.com/Mi-Bee-Studio/mibeehive/internal/model"
)

// Source is the resolved definition of a crawlable source. It is the
// type-safe replacement for the old (project, ProjectSettings{GitHubOwner,
// GitHubRepo}, SourceType) tuple: type-specific values live in Params instead
// of being overloaded onto two positional strings.
type Source struct {
	Name   string            // project name (maps to a projects.name)
	Type   string            // registry key: "github", "rulesrc", "hashicorp", ...
	Params map[string]string // type-specific, e.g. {"owner","repo"} or {"fingerprint":"..."}
}

// Fetcher fetches assets for a Source. A Fetcher may handle one or more source
// types (see Sources); the Registry routes by Source.Type.
type Fetcher interface {
	// Fetch returns the release assets for the source.
	Fetch(ctx context.Context, src Source) ([]model.ReleaseAsset, error)
	// Sources returns the source-type keys this Fetcher handles.
	Sources() []string
}

// Registry maps source-type keys to their Fetcher. It is the runtime
// replacement for the old Scheduler.crawlers map and the DB CHECK constraint:
// validity is "does a Fetcher handle this type?", not a schema rule.
type Registry struct {
	fetchers map[string]Fetcher
}

// NewRegistry returns an empty Registry.
func NewRegistry() *Registry { return &Registry{fetchers: make(map[string]Fetcher)} }

// Register adds a Fetcher for each of its source types. A later registration
// for an already-registered type overwrites the earlier one.
func (r *Registry) Register(f Fetcher) {
	for _, t := range f.Sources() {
		r.fetchers[t] = f
	}
}

// Get returns the Fetcher for a source type, or false if none is registered.
func (r *Registry) Get(sourceType string) (Fetcher, bool) {
	f, ok := r.fetchers[sourceType]
	return f, ok
}

// Fetch resolves a source type to its Fetcher and calls Fetch. It matches the
// crawler.FetchFunc signature so init.go can wire it into
// CrawlManager.SetFetchFunc without an import cycle. Returns an error if no
// Fetcher is registered for the type (runtime validation, replacing the old
// DB CHECK constraint).
func (r *Registry) Fetch(ctx context.Context, name, sourceType string, params map[string]string) ([]model.ReleaseAsset, error) {
	f, ok := r.Get(sourceType)
	if !ok {
		return nil, ErrNoFetcherFor(sourceType)
	}
	return f.Fetch(ctx, Source{Name: name, Type: sourceType, Params: params})
}

// Types returns all registered source-type keys.
func (r *Registry) Types() []string {
	types := make([]string, 0, len(r.fetchers))
	for t := range r.fetchers {
		types = append(types, t)
	}
	return types
}

// ErrNoFetcher is returned when no Fetcher is registered for a source type.
type errNoFetcher struct{ typ string }

func (e *errNoFetcher) Error() string {
	return fmt.Sprintf("no fetcher registered for source type %q", e.typ)
}

// ErrNoFetcherFor returns a sentinel error for an unregistered source type.
func ErrNoFetcherFor(typ string) error { return &errNoFetcher{typ: typ} }
