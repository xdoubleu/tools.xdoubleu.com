package openlibrary

import "context"

type Client interface {
	Search(ctx context.Context, query string) ([]ExternalBook, error)
	GetByISBN(ctx context.Context, isbn string) (*ExternalBook, error)
}
