package unicat

import (
	"context"
	"errors"
)

// ErrNotFound is returned by GetByISBN and Search when no matching book exists
// in the UniCat catalog.
var ErrNotFound = errors.New("unicat: book not found")

// Client is the subset of the UniCat SRU API used for metadata enrichment.
// UniCat is the Belgian union catalog (https://www.unicat.be) and provides
// good coverage for Dutch- and French-language Belgian/Flemish titles that
// OpenLibrary frequently misses.
type Client interface {
	// GetByISBN returns metadata for the book with the given ISBN-13.
	// Returns ErrNotFound when no matching record exists in UniCat.
	GetByISBN(ctx context.Context, isbn string) (*ExternalBook, error)
	// Search queries UniCat by title and optional author using SRU/CQL.
	// The query is expected in the same "intitle:... inauthor:..." format as
	// buildSearchQuery in the resync service. Returns up to searchLimit results.
	Search(ctx context.Context, query string) ([]ExternalBook, error)
}
