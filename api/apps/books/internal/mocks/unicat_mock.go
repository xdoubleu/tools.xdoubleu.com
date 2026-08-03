package mocks

import (
	"context"

	"tools.xdoubleu.com/apps/books/pkg/unicat"
)

// MockUniCatClient returns the same canned book from both Search and
// GetByISBN. Used for happy-path tests that require the UniCat client.
type MockUniCatClient struct{}

func (m MockUniCatClient) Search(
	_ context.Context,
	_ string,
) ([]unicat.ExternalBook, error) {
	book := ucOdysseyBook()
	return []unicat.ExternalBook{book}, nil
}

func (m MockUniCatClient) GetByISBN(
	_ context.Context,
	_ string,
) (*unicat.ExternalBook, error) {
	book := ucOdysseyBook()
	return &book, nil
}

func NewMockUniCatClient() unicat.Client {
	return MockUniCatClient{}
}

func ucOdysseyBook() unicat.ExternalBook {
	isbn := testBookISBN13
	desc := testBookDesc
	return unicat.ExternalBook{ //nolint:exhaustruct //UniCat has no cover images
		Title:       testBookTitle,
		Authors:     []string{testBookAuthor},
		ISBN13:      &isbn,
		Description: &desc,
	}
}

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
