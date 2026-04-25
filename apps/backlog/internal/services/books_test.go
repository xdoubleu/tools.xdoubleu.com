//nolint:testpackage // testing unexported service helpers
package services

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"tools.xdoubleu.com/apps/backlog/pkg/hardcover"
)

func TestCountDatesOn(t *testing.T) {
	dates := []time.Time{
		time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
		time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC),
		time.Date(2024, 3, 10, 0, 0, 0, 0, time.UTC),
	}
	assert.Equal(t, 2, countDatesOn(dates, "2024-01-15"))
	assert.Equal(t, 1, countDatesOn(dates, "2024-03-10"))
	assert.Equal(t, 0, countDatesOn(dates, "2024-06-01"))
}

func TestCountDatesOn_Empty(t *testing.T) {
	assert.Equal(t, 0, countDatesOn(nil, "2024-01-15"))
}

func TestExternalToBook(t *testing.T) {
	isbn13 := "9780140449112"
	isbn10 := "0140449116"
	cover := "https://example.com/cover.jpg"
	desc := "A classic."

	ext := hardcover.ExternalBook{
		Provider:    "hardcover",
		ProviderID:  "42",
		Title:       "The Odyssey",
		Authors:     []string{"Homer"},
		ISBN13:      &isbn13,
		ISBN10:      &isbn10,
		CoverURL:    &cover,
		Description: &desc,
	}

	book := externalToBook(ext)

	assert.Equal(t, "The Odyssey", book.Title)
	assert.Equal(t, []string{"Homer"}, book.Authors)
	assert.Equal(t, &isbn13, book.ISBN13)
	assert.Equal(t, &isbn10, book.ISBN10)
	assert.Equal(t, &cover, book.CoverURL)
	assert.Equal(t, &desc, book.Description)
	assert.Equal(t, "42", book.ExternalRefs["hardcover"])
}

func TestExternalToBook_NilFields(t *testing.T) {
	ext := hardcover.ExternalBook{ //nolint:exhaustruct //optional fields nil
		Provider:   "manual",
		ProviderID: "1",
		Title:      "Untitled",
		Authors:    []string{},
	}

	book := externalToBook(ext)

	assert.Equal(t, "Untitled", book.Title)
	assert.Nil(t, book.ISBN13)
	assert.Nil(t, book.ISBN10)
	assert.Nil(t, book.CoverURL)
	assert.Nil(t, book.Description)
	assert.Equal(t, "1", book.ExternalRefs["manual"])
}
