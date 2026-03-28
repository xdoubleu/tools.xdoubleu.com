package goodreads

import "context"

type Client interface {
	GetUserID(profileURL string) (*string, error)
	GetBooks(ctx context.Context, userID string) ([]Book, error)
}
