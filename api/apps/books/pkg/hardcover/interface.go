package hardcover

import (
	"context"
	"errors"
)

// ErrNotFound is returned by GetByISBN when no edition matches the given ISBN.
var ErrNotFound = errors.New("hardcover: book not found")

// Client is the subset of the Hardcover GraphQL API used for metadata
// enrichment. Hardcover (https://hardcover.app) exposes a Hasura GraphQL API at
// https://api.hardcover.app/v1/graphql. It has no daily quota — only a
// documented 1 req/s API ceiling, enforced client-side by a rate limiter
// (the resync throughput floor, see book_resync.go). A free API key (a
// Bearer JWT, taken from the account settings page) is required; the key
// expires roughly yearly and must be refreshed.
type Client interface {
	// Search queries Hardcover for books matching query (the same
	// "intitle:<title> inauthor:<author>" format produced by buildSearchQuery
	// in the resync service). Only the title is used to filter server-side;
	// author disambiguation happens in the resync match layer over the
	// returned candidates. Returns up to searchLimit results.
	Search(ctx context.Context, query string) ([]ExternalBook, error)
	// GetByISBN returns the single best-matching edition for the given ISBN-13.
	// Returns ErrNotFound when Hardcover has no matching edition.
	GetByISBN(ctx context.Context, isbn string) (*ExternalBook, error)
}
