package source

import (
	"context"
	"fmt"

	"github.com/Mi-Bee-Studio/mibeehive/internal/crawler"
	"github.com/Mi-Bee-Studio/mibeehive/internal/model"
)

// LegacyAdapter wraps an existing internal/crawler.Crawler so it satisfies the
// new Fetcher interface WITHOUT changing the crawler itself. This is the
// transitional bridge (design Step 2): CrawlManager can route through Fetcher
// while the crawlers are migrated to fingerprints one at a time.
//
// It translates a Source.Params map into the (owner, repo) positional strings
// the old FetchReleases API expects. The semantics mirror the historical
// overloading documented in the design (owner/repo mean different things per
// source type); this adapter preserves that meaning exactly.
type LegacyAdapter struct {
	c crawler.Crawler
}

// NewLegacyAdapter wraps an existing Crawler as a Fetcher.
func NewLegacyAdapter(c crawler.Crawler) *LegacyAdapter { return &LegacyAdapter{c: c} }

// Sources reports the single source type the wrapped crawler handles.
func (a *LegacyAdapter) Sources() []string { return []string{string(a.c.SourceType())} }

// Fetch delegates to the wrapped crawler's FetchReleases, extracting owner/repo
// from Source.Params. Missing keys default to empty strings, matching today's
// behavior for sources that ignore one or both (e.g. Go ignores both).
func (a *LegacyAdapter) Fetch(ctx context.Context, src Source) ([]model.ReleaseAsset, error) {
	owner := src.Params["owner"]
	repo := src.Params["repo"]
	return a.c.FetchReleases(ctx, owner, repo)
}

// FromProject builds a Source from a db project's settings, translating the
// old GitHubOwner/GitHubRepo fields into Source.Params. This is the transitional
// mapper (design Step 3); it lets CrawlManager move to Fetcher without touching
// ProjectSettings or the DB yet.
func FromProject(name, sourceType string, settings model.ProjectSettings) Source {
	return Source{
		Name: name,
		Type: sourceType,
		Params: map[string]string{
			"owner": settings.GitHubOwner,
			"repo":  settings.GitHubRepo,
		},
	}
}

// AsParams is a small helper to build Params from common fields.
func AsParams(kv ...string) map[string]string {
	if len(kv)%2 != 0 {
		panic(fmt.Sprintf("source.AsParams: odd number of key/value args: %d", len(kv)))
	}
	m := make(map[string]string, len(kv)/2)
	for i := 0; i < len(kv); i += 2 {
		m[kv[i]] = kv[i+1]
	}
	return m
}
