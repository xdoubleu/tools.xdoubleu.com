// Package arxiv is a minimal client for the arXiv Atom API
// (https://info.arxiv.org/help/api/), used to turn a pasted arXiv URL into
// paper metadata and a PDF location.
package arxiv

import (
	"context"
	"errors"
)

// ErrNotFound is returned when the id does not resolve to a paper.
var ErrNotFound = errors.New("arxiv: paper not found")

// Client fetches paper metadata by arXiv id.
type Client interface {
	GetByID(ctx context.Context, id string) (*Paper, error)
}
