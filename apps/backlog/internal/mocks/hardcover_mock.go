package mocks

import (
	"context"

	"tools.xdoubleu.com/apps/backlog/pkg/hardcover"
)

type MockHardcoverClient struct {
}

// GetByID implements [hardcover.Client].
func (m MockHardcoverClient) GetByID(ctx context.Context, id string) (*hardcover.ExternalBook, error) {
	panic("unimplemented")
}

// Search implements [hardcover.Client].
func (m MockHardcoverClient) Search(ctx context.Context, query string) ([]hardcover.ExternalBook, error) {
	panic("unimplemented")
}

func NewMockHardcoverClient() hardcover.Client {
	return MockHardcoverClient{}
}
