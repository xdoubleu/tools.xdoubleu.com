package mocks

import (
	"context"

	"tools.xdoubleu.com/apps/books/pkg/unicat"
)

// MockEmptyUniCatClient returns no results and ErrNotFound — used to keep
// UniCat "configured but confirmed absent" in tests, mirroring production
// (all three sources are always configured there) without claiming every
// test book.
type MockEmptyUniCatClient struct{}

func (m MockEmptyUniCatClient) GetByISBN(
	_ context.Context,
	_ string,
) (*unicat.ExternalBook, error) {
	return nil, unicat.ErrNotFound
}

func (m MockEmptyUniCatClient) Search(
	_ context.Context,
	_ string,
) ([]unicat.ExternalBook, error) {
	return nil, nil
}

func NewMockEmptyUniCatClient() unicat.Client {
	return MockEmptyUniCatClient{}
}
