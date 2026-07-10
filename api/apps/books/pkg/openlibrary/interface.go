package openlibrary

import (
	"context"
	"errors"
)

// ErrCoverNotFound is returned by FetchCover when Open Library has no cover
// for the given URL (i.e. it responds with a 404 when ?default=false is set).
var ErrCoverNotFound = errors.New("openlibrary: cover not found")

type Client interface {
	Search(ctx context.Context, query string) ([]ExternalBook, error)
	// Get fetches a single work by its Open Library work ID (the ProviderID
	// returned by Search, e.g. "OL123W"). Returns ErrNotFound if no work
	// matches.
	Get(ctx context.Context, providerID string) (*ExternalBook, error)
	GetByISBN(ctx context.Context, isbn string) (*ExternalBook, error)
	// FetchCover downloads the raw image bytes for the given Open Library cover
	// URL. It appends ?default=false so that Open Library returns HTTP 404
	// instead of a blank placeholder image when no cover exists.
	// Returns ErrCoverNotFound when the cover does not exist.
	FetchCover(ctx context.Context, coverURL string) ([]byte, string, error)
}
