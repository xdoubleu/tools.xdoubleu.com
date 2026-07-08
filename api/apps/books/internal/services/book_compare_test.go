//nolint:testpackage // testing unexported service helpers
package services

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/books/internal/models"
	"tools.xdoubleu.com/apps/books/pkg/books"
)

// helpers ---------------------------------------------------------------

func cmpEntry(title, author string, isbn *string, status string) books.ParsedEntry {
	e := books.ParsedEntry{} //nolint:exhaustruct //only test-relevant fields set
	e.Book.Title = title
	e.Book.Authors = []string{author}
	e.Book.ISBN13 = isbn
	e.UserBook.Status = status
	return e
}

func cmpLibBook(title, author string, isbn *string, status string) models.UserBook {
	ub := models.UserBook{} //nolint:exhaustruct //only test-relevant fields set
	ub.BookID = uuid.New()
	ub.Status = status
	ub.Book = &models.Book{ //nolint:exhaustruct //only fields needed for compare
		ID:      uuid.New(),
		Title:   title,
		Authors: []string{author},
		ISBN13:  isbn,
	}
	return ub
}

func cmpEntryTags(
	title, author string,
	isbn *string,
	status string,
	tags []string,
) books.ParsedEntry {
	e := cmpEntry(title, author, isbn, status)
	e.UserBook.Tags = tags
	return e
}

func cmpLibBookTags(
	title, author string,
	isbn *string,
	status string,
	tags []string,
) models.UserBook {
	ub := cmpLibBook(title, author, isbn, status)
	ub.Tags = tags
	return ub
}

// tests -----------------------------------------------------------------

func TestCompareWithCSV_AllMatch(t *testing.T) {
	isbn := strPtr("9780000000001")
	entries := []books.ParsedEntry{
		cmpEntry("The Hobbit", "J.R.R. Tolkien", isbn, "read"),
	}
	lib := []models.UserBook{cmpLibBook("The Hobbit", "J.R.R. Tolkien", isbn, "read")}

	result := CompareWithCSV(entries, lib)

	assert.Equal(t, 1, result.CSVCount)
	assert.Equal(t, 1, result.LibraryCount)
	assert.Equal(t, 1, result.MatchedCount)
	assert.Empty(t, result.Mismatches)
}

func TestCompareWithCSV_MissingInLibrary(t *testing.T) {
	entries := []books.ParsedEntry{cmpEntry("Dune", "Frank Herbert", nil, "to-read")}
	lib := []models.UserBook{}

	result := CompareWithCSV(entries, lib)

	require.Len(t, result.Mismatches, 1)
	assert.Equal(t, []string{"missing-in-library"}, result.Mismatches[0].Differences)
	assert.NotNil(t, result.Mismatches[0].CSV)
	assert.Nil(t, result.Mismatches[0].Library)
	assert.Equal(t, "csv:0", result.Mismatches[0].ID)
	assert.NotNil(t, result.Mismatches[0].CSVEntry)
	assert.Nil(t, result.Mismatches[0].LibBook)
}

func TestCompareWithCSV_MissingInCSV(t *testing.T) {
	entries := []books.ParsedEntry{}
	lib := []models.UserBook{cmpLibBook("Neuromancer", "William Gibson", nil, "read")}

	result := CompareWithCSV(entries, lib)

	require.Len(t, result.Mismatches, 1)
	assert.Equal(t, []string{"missing-in-csv"}, result.Mismatches[0].Differences)
	assert.Nil(t, result.Mismatches[0].CSV)
	assert.NotNil(t, result.Mismatches[0].Library)
	assert.Equal(t, lib[0].BookID.String(), result.Mismatches[0].ID)
	assert.NotNil(t, result.Mismatches[0].LibBook)
	assert.Nil(t, result.Mismatches[0].CSVEntry)
}

func TestCompareWithCSV_StatusMismatch(t *testing.T) {
	isbn := strPtr("9780000000002")
	entries := []books.ParsedEntry{cmpEntry("Dune", "Frank Herbert", isbn, "to-read")}
	lib := []models.UserBook{cmpLibBook("Dune", "Frank Herbert", isbn, "read")}

	result := CompareWithCSV(entries, lib)

	require.Len(t, result.Mismatches, 1)
	assert.Contains(t, result.Mismatches[0].Differences, "status")
	assert.NotContains(t, result.Mismatches[0].Differences, "isbn")
	assert.NotContains(t, result.Mismatches[0].Differences, "title")
	assert.Equal(t, lib[0].BookID.String(), result.Mismatches[0].ID)
	assert.Same(t, &lib[0], result.Mismatches[0].LibBook)
	assert.NotNil(t, result.Mismatches[0].CSVEntry)
}

func TestCompareWithCSV_ISBNMismatch(t *testing.T) {
	entries := []books.ParsedEntry{
		cmpEntry("Dune", "Frank Herbert", strPtr("9780000000003"), "read"),
	}
	lib := []models.UserBook{
		cmpLibBook("Dune", "Frank Herbert", strPtr("9780000000004"), "read"),
	}

	result := CompareWithCSV(entries, lib)

	// matched by title+author, but ISBN differs
	assert.Equal(t, 1, result.MatchedCount)
	require.Len(t, result.Mismatches, 1)
	assert.Contains(t, result.Mismatches[0].Differences, "isbn")
}

func TestCompareWithCSV_TitleMismatch_MatchedByISBN(t *testing.T) {
	isbn := strPtr("9780000000005")
	entries := []books.ParsedEntry{
		cmpEntry("Dune Messiah", "Frank Herbert", isbn, "read"),
	}
	lib := []models.UserBook{cmpLibBook("Dune", "Frank Herbert", isbn, "read")}

	result := CompareWithCSV(entries, lib)

	// matched by ISBN, but normalized title differs
	assert.Equal(t, 1, result.MatchedCount)
	require.Len(t, result.Mismatches, 1)
	assert.Contains(t, result.Mismatches[0].Differences, "title")
}

func TestCompareWithCSV_ISBNPrecedenceOverTitle(t *testing.T) {
	// CSV: isbn1 + "Title A"; Library: isbn1 + "Title B". Matched by ISBN.
	isbn := strPtr("9780000000006")
	entries := []books.ParsedEntry{cmpEntry("Title A", "Author X", isbn, "read")}
	lib := []models.UserBook{cmpLibBook("Title B", "Author X", isbn, "read")}

	result := CompareWithCSV(entries, lib)

	assert.Equal(t, 1, result.MatchedCount)
	require.Len(t, result.Mismatches, 1)
	assert.Contains(t, result.Mismatches[0].Differences, "title")
}

func TestCompareWithCSV_MatchByTitleAuthorWhenNoISBN(t *testing.T) {
	entries := []books.ParsedEntry{cmpEntry("Foundation", "Isaac Asimov", nil, "read")}
	lib := []models.UserBook{cmpLibBook("Foundation", "Isaac Asimov", nil, "read")}

	result := CompareWithCSV(entries, lib)

	assert.Equal(t, 1, result.MatchedCount)
	assert.Empty(t, result.Mismatches)
}

func TestCompareWithCSV_MatchByFuzzyTitleWhenNoISBN(t *testing.T) {
	// Same book, no ISBN on either side, title word order differs (common
	// Goodreads "Title, The" export format vs "The Title" library entry).
	entries := []books.ParsedEntry{
		cmpEntry("Fellowship of the Ring, The", "J.R.R. Tolkien", nil, "read"),
	}
	lib := []models.UserBook{
		cmpLibBook("The Fellowship of the Ring", "J.R.R. Tolkien", nil, "read"),
	}

	result := CompareWithCSV(entries, lib)

	assert.Equal(t, 1, result.MatchedCount)
}

func TestCompareWithCSV_FuzzyDoesNotMatchDifferentSeriesVolume(t *testing.T) {
	// Same author, high word overlap, but a different book in the series —
	// must surface as missing rather than being fuzzy-matched.
	entries := []books.ParsedEntry{
		cmpEntry("The Return of the King", "J.R.R. Tolkien", nil, "read"),
	}
	lib := []models.UserBook{
		cmpLibBook("The Fellowship of the Ring", "J.R.R. Tolkien", nil, "read"),
	}

	result := CompareWithCSV(entries, lib)

	assert.Equal(t, 0, result.MatchedCount)
	require.Len(t, result.Mismatches, 2) // csv missing-in-library + lib missing-in-csv
}

func TestCompareWithCSV_Counts(t *testing.T) {
	isbn1 := strPtr("9780000000010")
	isbn2 := strPtr("9780000000011")
	entries := []books.ParsedEntry{
		cmpEntry("Book A", "Author A", isbn1, "read"),
		cmpEntry("Book B", "Author B", isbn2, "to-read"),
		cmpEntry("Book C", "Author C", nil, "read"),
	}
	lib := []models.UserBook{
		cmpLibBook("Book A", "Author A", isbn1, "read"),
		cmpLibBook("Book D", "Author D", nil, "read"),
	}

	result := CompareWithCSV(entries, lib)

	assert.Equal(t, 3, result.CSVCount)
	assert.Equal(t, 2, result.LibraryCount)
	assert.Equal(t, 1, result.MatchedCount) // only Book A matched
}

func TestCompareWithCSV_TagsMismatch(t *testing.T) {
	isbn := strPtr("9780000000012")
	entries := []books.ParsedEntry{
		cmpEntryTags(
			"Dune",
			"Frank Herbert",
			isbn,
			"read",
			[]string{"technical", "own-physical"},
		),
	}
	lib := []models.UserBook{
		cmpLibBookTags("Dune", "Frank Herbert", isbn, "read", []string{"technical"}),
	}

	result := CompareWithCSV(entries, lib)

	require.Len(t, result.Mismatches, 1)
	assert.Contains(t, result.Mismatches[0].Differences, "tags")
}

func TestCompareWithCSV_TagsSameSetDifferentOrder_NoDiff(t *testing.T) {
	isbn := strPtr("9780000000013")
	entries := []books.ParsedEntry{
		cmpEntryTags(
			"Dune",
			"Frank Herbert",
			isbn,
			"read",
			[]string{"own-physical", "technical"},
		),
	}
	lib := []models.UserBook{
		cmpLibBookTags(
			"Dune",
			"Frank Herbert",
			isbn,
			"read",
			[]string{"technical", "own-physical"},
		),
	}

	result := CompareWithCSV(entries, lib)

	assert.Empty(t, result.Mismatches)
}
