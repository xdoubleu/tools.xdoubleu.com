package hardcover

import "context"

type Client interface {
	Search(ctx context.Context, query string) ([]ExternalBook, error)
	GetByID(ctx context.Context, id string) (*ExternalBook, error)
}
