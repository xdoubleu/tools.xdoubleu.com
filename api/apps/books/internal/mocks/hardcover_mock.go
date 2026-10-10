package mocks

import (
	"context"

	"tools.xdoubleu.com/apps/books/pkg/hardcover"
)

// MockHardcoverClient returns the same canned book from both Search and
// GetByISBN. Used for happy-path tests that require the Hardcover client.
type MockHardcoverClient struct{}

func (m MockHardcoverClient) Search(
	_ context.Context,
	_ string,
) ([]hardcover.ExternalBook, error) {
	book := hcOdysseyBook()
	return []hardcover.ExternalBook{book}, nil
}

func (m MockHardcoverClient) GetByISBN(
	_ context.Context,
	_ string,
) (*hardcover.ExternalBook, error) {
	book := hcOdysseyBook()
	return &book, nil
}

// GetSeries returns the canned book as the series' only volume.
func (m MockHardcoverClient) GetSeries(
	_ context.Context,
	name string,
) (*hardcover.Series, error) {
	total := 2
	return &hardcover.Series{
		Name:  name,
		Total: &total,
		Books: []hardcover.SeriesBook{{Position: 1, Book: hcOdysseyBook()}},
	}, nil
}

func NewMockHardcoverClient() hardcover.Client {
	return MockHardcoverClient{}
}

func hcOdysseyBook() hardcover.ExternalBook {
	isbn := testBookISBN13
	cover := "https://hardcover.app/cover.jpg"
	desc := testBookDesc
	pages := 300
	return hardcover.ExternalBook{
		Title:       testBookTitle,
		Authors:     []string{testBookAuthor},
		ISBN13:      &isbn,
		CoverURL:    &cover,
		Description: &desc,
		PageCount:   &pages,
		Series:      nil,
	}
}

// MockEmptyHardcoverClient returns no results and ErrNotFound — used to keep
// Hardcover "configured but confirmed absent" in tests without claiming every
// test book.
type MockEmptyHardcoverClient struct{}

func (m MockEmptyHardcoverClient) Search(
	_ context.Context,
	_ string,
) ([]hardcover.ExternalBook, error) {
	return nil, nil
}

func (m MockEmptyHardcoverClient) GetByISBN(
	_ context.Context,
	_ string,
) (*hardcover.ExternalBook, error) {
	return nil, hardcover.ErrNotFound
}

func (m MockEmptyHardcoverClient) GetSeries(
	_ context.Context,
	_ string,
) (*hardcover.Series, error) {
	return nil, hardcover.ErrNotFound
}

func NewMockEmptyHardcoverClient() hardcover.Client {
	return MockEmptyHardcoverClient{}
}
