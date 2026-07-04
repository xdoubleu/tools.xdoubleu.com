package mocks

import (
	"context"

	"tools.xdoubleu.com/apps/books/pkg/openlibrary"
)

// MockOpenLibraryClient returns a single canned book from Search and resolves
// GetByISBN to the same book. Used for the happy-path external-search tests.
type MockOpenLibraryClient struct{}

func (m MockOpenLibraryClient) Search(
	_ context.Context,
	_ string,
) ([]openlibrary.ExternalBook, error) {
	return []openlibrary.ExternalBook{odysseyBook()}, nil
}

func (m MockOpenLibraryClient) GetByISBN(
	_ context.Context,
	_ string,
) (*openlibrary.ExternalBook, error) {
	book := odysseyBook()
	return &book, nil
}

func (m MockOpenLibraryClient) FetchCover(
	_ context.Context,
	_ string,
) ([]byte, string, error) {
	return []byte("fake-cover-bytes"), "image/jpeg", nil
}

func NewMockOpenLibraryClient() openlibrary.Client {
	return MockOpenLibraryClient{}
}

func odysseyBook() openlibrary.ExternalBook {
	isbn := "9780140447934"
	cover := "https://example.com/cover.jpg"
	desc := "A test book."
	return openlibrary.ExternalBook{ //nolint:exhaustruct //ISBN10 not needed for mock
		Provider:    "openlibrary",
		ProviderID:  "OL1W",
		Title:       "The Odyssey",
		Authors:     []string{"Homer"},
		ISBN13:      &isbn,
		CoverURL:    &cover,
		Description: &desc,
	}
}

// MockEmptyOpenLibraryClient returns no results from Search and not-found from
// GetByISBN. Used to drive the upload "unrecognized book" rejection path.
type MockEmptyOpenLibraryClient struct{}

func (m MockEmptyOpenLibraryClient) Search(
	_ context.Context,
	_ string,
) ([]openlibrary.ExternalBook, error) {
	return nil, nil
}

func (m MockEmptyOpenLibraryClient) GetByISBN(
	_ context.Context,
	_ string,
) (*openlibrary.ExternalBook, error) {
	return nil, openlibrary.ErrNotFound
}

func (m MockEmptyOpenLibraryClient) FetchCover(
	_ context.Context,
	_ string,
) ([]byte, string, error) {
	return nil, "", openlibrary.ErrCoverNotFound
}

func NewMockEmptyOpenLibraryClient() openlibrary.Client {
	return MockEmptyOpenLibraryClient{}
}
