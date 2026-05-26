package crawler

import (
	"context"

	"github.com/Mi-Bee-Studio/mibeehive/internal/model"
	)

	// UserAgent is sent with all HTTP requests to avoid being blocked by API gateways.
const UserAgent = "MiBeeHive/1.0"

// Crawler defines the interface for fetching releases from a source.
type Crawler interface {
	// Name returns the human-readable name of this crawler.
	Name() string
	// SourceType returns the type of source this crawler handles.
	SourceType() model.SourceType
	// FetchReleases fetches the latest release assets for the given owner/repo.
	FetchReleases(ctx context.Context, owner, repo string) ([]model.ReleaseAsset, error)
}
