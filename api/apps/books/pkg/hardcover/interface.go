package hardcover

import (
	"context"
	"errors"
)

// ErrNotFound is returned by GetByISBN when no edition matches the given ISBN.
var ErrNotFound = errors.New("hardcover: book not found")

// Client is the subset of Hardcover's Hasura GraphQL API
// (https://api.hardcover.app/v1/graphql) used for metadata. No daily quota,
// a 1 req/s ceiling enforced client-side. Requires a Bearer JWT API key that
// expires roughly yearly.
type Client interface {
	// Search takes a buildSearchQuery-style query but searches by title only;
	// author disambiguation is the caller's. Returns up to searchLimit results.
	Search(ctx context.Context, query string) ([]ExternalBook, error)
	// GetByISBN returns the single best-matching edition for the given ISBN-13.
	// Returns ErrNotFound when Hardcover has no matching edition.
	GetByISBN(ctx context.Context, isbn string) (*ExternalBook, error)
}
