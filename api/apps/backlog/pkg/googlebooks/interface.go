package googlebooks

import (
	"context"
	"errors"
)

// ErrNotFound is returned by GetByISBN when no book matches the given ISBN.
var ErrNotFound = errors.New("googlebooks: book not found")

// Client is the subset of the Google Books API used for metadata enrichment.
type Client interface {
	// Search queries the Google Books API for volumes matching query (free text,
	// typically "intitle:<title> inauthor:<author>"). Returns up to searchLimit
	// results ordered by relevance.
	Search(ctx context.Context, query string) ([]ExternalBook, error)
	// GetByISBN returns the single best-matching volume for the given ISBN-13.
	// Returns ErrNotFound when no match exists.
	GetByISBN(ctx context.Context, isbn string) (*ExternalBook, error)
}
