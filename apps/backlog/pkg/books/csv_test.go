package books_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"tools.xdoubleu.com/apps/backlog/internal/models"
	"tools.xdoubleu.com/apps/backlog/pkg/books"
)

// Goodreads encodes ISBNs as ="VALUE" inside a CSV-quoted field.
// "Bookshelves with positions" lists all shelves: exclusive shelf + non-exclusive (tags).
//
//nolint:lll // CSV header and data rows are inherently long
const goodreadsCSV = `Book Id,Title,Author,ISBN,ISBN13,My Rating,Exclusive Shelf,Bookshelves with positions,Date Read
12345,The Odyssey,Homer,"=""0140449116""","=""9780140449112""",5,read,"read (#1), own-physical",2023/06/15
67890,Dune,Frank Herbert,"=""0441013597""","=""9780441013593""",0,to-read,"to-read (#5), own-digital",
11111,Foundation,Isaac Asimov,,,3,currently-reading,"currently-reading (#2)",
`

func TestParseCSV_HappyPath(t *testing.T) {
	entries, err := books.ParseCSV(strings.NewReader(goodreadsCSV))
	require.NoError(t, err)
	require.Len(t, entries, 3)

	// Finished book
	e0 := entries[0]
	assert.Equal(t, "The Odyssey", e0.Book.Title)
	assert.Equal(t, []string{"Homer"}, e0.Book.Authors)
	assert.Equal(t, "9780140449112", *e0.Book.ISBN13)
	assert.Equal(t, "0140449116", *e0.Book.ISBN10)
	assert.Equal(t, "12345", e0.Book.ExternalRefs["goodreads"])
	assert.Equal(t, models.StatusRead, e0.UserBook.Status)
	assert.NotEmpty(t, e0.UserBook.FinishedAt)
	assert.EqualValues(t, 5, *e0.UserBook.Rating)
	assert.NotContains(
		t,
		e0.UserBook.Tags,
		"read",
	) // exclusive shelf excluded from tags
	assert.Contains(t, e0.UserBook.Tags, "own-physical")

	// To-read book
	e1 := entries[1]
	assert.Equal(t, "Dune", e1.Book.Title)
	assert.Equal(t, models.StatusToRead, e1.UserBook.Status)
	assert.Nil(t, e1.UserBook.Rating) // rating=0 → nil
	assert.Empty(t, e1.UserBook.FinishedAt)
	assert.Contains(t, e1.UserBook.Tags, "own-digital")

	// Reading book
	e2 := entries[2]
	assert.Equal(t, "Foundation", e2.Book.Title)
	assert.Equal(t, models.StatusReading, e2.UserBook.Status)
	assert.Nil(t, e2.Book.ISBN13) // empty ISBN
	assert.Nil(t, e2.Book.ISBN10)
	assert.EqualValues(t, 3, *e2.UserBook.Rating)
}

func TestParseCSV_MissingRequiredColumn(t *testing.T) {
	csv := "Title,Author\nFoo,Bar\n"
	_, err := books.ParseCSV(strings.NewReader(csv))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing required column")
}

const csvHeader = "Book Id,Title,Author,ISBN,ISBN13,My Rating," + //nolint:lll // CSV header is inherently long
	"Exclusive Shelf,Bookshelves with positions,Date Read"

func TestParseCSV_EmptyDateRead(t *testing.T) {
	// Empty date read and no ISBN — fields are blank, not Goodreads =""-style.
	row := "99999,Test Book,Test Author,,,0,read,read (#1),"
	entries, err := books.ParseCSV(strings.NewReader(csvHeader + "\n" + row + "\n"))
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Empty(t, entries[0].UserBook.FinishedAt)
	assert.Nil(t, entries[0].Book.ISBN13)
	assert.Nil(t, entries[0].Book.ISBN10)
}

func TestParseCSV_SkipsInvalidBookID(t *testing.T) {
	rows := "not-a-number,Bad Row,Author,,,0,read,read (#1),\n" +
		"99999,Good Book,Author,,,0,read,read (#1),"
	entries, err := books.ParseCSV(strings.NewReader(csvHeader + "\n" + rows + "\n"))
	require.NoError(t, err)
	assert.Len(t, entries, 1)
	assert.Equal(t, "Good Book", entries[0].Book.Title)
}
