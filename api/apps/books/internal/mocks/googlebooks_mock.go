package mocks

import (
	"context"

	"tools.xdoubleu.com/apps/books/pkg/googlebooks"
)

// MockGoogleBooksClient returns the same canned book from both Search and
// GetByISBN. Used for happy-path tests that require the Google Books client.
type MockGoogleBooksClient struct{}

func (m MockGoogleBooksClient) Search(
	_ context.Context,
	_ string,
) ([]googlebooks.ExternalBook, error) {
	book := gbOdysseyBook()
	return []googlebooks.ExternalBook{book}, nil
}

func (m MockGoogleBooksClient) GetByISBN(
	_ context.Context,
	_ string,
) (*googlebooks.ExternalBook, error) {
	book := gbOdysseyBook()
	return &book, nil
}

func NewMockGoogleBooksClient() googlebooks.Client {
	return MockGoogleBooksClient{}
}

func gbOdysseyBook() googlebooks.ExternalBook {
	isbn := "9780140447934"
	cover := "https://books.google.com/cover.jpg"
	desc := "A test book."
	pages := 300
	return googlebooks.ExternalBook{
		Title:       "The Odyssey",
		Authors:     []string{"Homer"},
		ISBN13:      &isbn,
		CoverURL:    &cover,
		Description: &desc,
		PageCount:   &pages,
	}
}

// MockEmptyGoogleBooksClient returns no results and ErrNotFound.
type MockEmptyGoogleBooksClient struct{}

func (m MockEmptyGoogleBooksClient) Search(
	_ context.Context,
	_ string,
) ([]googlebooks.ExternalBook, error) {
	return nil, nil
}

func (m MockEmptyGoogleBooksClient) GetByISBN(
	_ context.Context,
	_ string,
) (*googlebooks.ExternalBook, error) {
	return nil, googlebooks.ErrNotFound
}

func NewMockEmptyGoogleBooksClient() googlebooks.Client {
	return MockEmptyGoogleBooksClient{}
}
