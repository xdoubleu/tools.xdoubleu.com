//nolint:testpackage // testing unexported service helpers
package services

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"tools.xdoubleu.com/apps/backlog/internal/models"
	"tools.xdoubleu.com/apps/backlog/pkg/ebookmeta"
)

// --- normalizeTitle ---

func TestNormalizeTitle_Basic(t *testing.T) {
	assert.Equal(t, "thehobbit", normalizeTitle("The Hobbit"))
}

func TestNormalizeTitle_StripsSubtitle(t *testing.T) {
	assert.Equal(t, "thehobbit", normalizeTitle("The Hobbit: An Unexpected Journey"))
}

func TestNormalizeTitle_Lowercase(t *testing.T) {
	assert.Equal(t, "thehobbit", normalizeTitle("THE HOBBIT"))
}

func TestNormalizeTitle_FoldsDiacritics(t *testing.T) {
	// "Café" should normalize the same as "Cafe"
	assert.Equal(t, normalizeTitle("Cafe"), normalizeTitle("Café"))
}

func TestNormalizeTitle_EmptyString(t *testing.T) {
	assert.Equal(t, "", normalizeTitle(""))
}

func TestNormalizeTitle_OnlySubtitle(t *testing.T) {
	// A title that is just a colon has nothing before the colon.
	assert.Equal(t, "", normalizeTitle(": A Subtitle Only"))
}

func TestNormalizeTitle_StripsPunctuation(t *testing.T) {
	// Punctuation other than the colon is also stripped.
	assert.Equal(t, "helloworld", normalizeTitle("Hello, World!"))
}

// --- normalizeAuthor ---

func TestNormalizeAuthor_FirstLast(t *testing.T) {
	// "J.R.R. Tolkien" → last token → "tolkien"
	assert.Equal(t, "tolkien", normalizeAuthor("J.R.R. Tolkien"))
}

func TestNormalizeAuthor_LastFirstComma(t *testing.T) {
	// "Tolkien, J.R.R." → before comma → "tolkien"
	assert.Equal(t, "tolkien", normalizeAuthor("Tolkien, J.R.R."))
}

func TestNormalizeAuthor_SingleName(t *testing.T) {
	assert.Equal(t, "homer", normalizeAuthor("Homer"))
}

func TestNormalizeAuthor_FoldsDiacritics(t *testing.T) {
	assert.Equal(t, normalizeAuthor("Bronte"), normalizeAuthor("Brontë"))
}

func TestNormalizeAuthor_Empty(t *testing.T) {
	assert.Equal(t, "", normalizeAuthor(""))
}

// --- matchLibraryByMetadata ---

func makeUserBook(title string, authors []string) models.UserBook {
	return models.UserBook{ //nolint:exhaustruct //only fields needed for matching
		ID:     uuid.New(),
		BookID: uuid.New(),
		Book: &models.Book{ //nolint:exhaustruct //only fields needed for matching
			Title:   title,
			Authors: authors,
		},
	}
}

func TestMatchLibraryByMetadata_ExactTitleAuthor(t *testing.T) {
	lib := []models.UserBook{makeUserBook("The Hobbit", []string{"J.R.R. Tolkien"})}
	meta := ebookmeta.Metadata{ //nolint:exhaustruct //only Title+Authors matter here
		Title:   "The Hobbit",
		Authors: []string{"J.R.R. Tolkien"},
	}
	got := matchLibraryByMetadata(lib, meta)
	assert.NotNil(t, got)
	assert.Equal(t, lib[0].BookID, got.BookID)
}

func TestMatchLibraryByMetadata_SubtitleInFile(t *testing.T) {
	// Library has base title; file carries subtitle.
	lib := []models.UserBook{makeUserBook("The Hobbit", []string{"J.R.R. Tolkien"})}
	meta := ebookmeta.Metadata{ //nolint:exhaustruct //only Title+Authors matter here
		Title:   "The Hobbit: There and Back Again",
		Authors: []string{"J.R.R. Tolkien"},
	}
	got := matchLibraryByMetadata(lib, meta)
	assert.NotNil(t, got)
	assert.Equal(t, lib[0].BookID, got.BookID)
}

func TestMatchLibraryByMetadata_SubtitleInLibrary(t *testing.T) {
	// Library has full title with subtitle; file carries only base title.
	lib := []models.UserBook{
		makeUserBook("The Hobbit: There and Back Again", []string{"J.R.R. Tolkien"}),
	}
	meta := ebookmeta.Metadata{ //nolint:exhaustruct //only Title+Authors matter here
		Title:   "The Hobbit",
		Authors: []string{"J.R.R. Tolkien"},
	}
	got := matchLibraryByMetadata(lib, meta)
	assert.NotNil(t, got)
	assert.Equal(t, lib[0].BookID, got.BookID)
}

func TestMatchLibraryByMetadata_AuthorLastFirstVsFirstLast(t *testing.T) {
	// Library has "First Last"; file has "Last, First".
	lib := []models.UserBook{makeUserBook("The Hobbit", []string{"J.R.R. Tolkien"})}
	meta := ebookmeta.Metadata{ //nolint:exhaustruct //only Title+Authors matter here
		Title:   "The Hobbit",
		Authors: []string{"Tolkien, J.R.R."},
	}
	got := matchLibraryByMetadata(lib, meta)
	assert.NotNil(t, got)
	assert.Equal(t, lib[0].BookID, got.BookID)
}

func TestMatchLibraryByMetadata_DiacriticDifference(t *testing.T) {
	lib := []models.UserBook{makeUserBook("Café Society", []string{"Pierre Dupont"})}
	meta := ebookmeta.Metadata{ //nolint:exhaustruct //only Title+Authors matter here
		Title:   "Cafe Society",
		Authors: []string{"Pierre Dupont"},
	}
	got := matchLibraryByMetadata(lib, meta)
	assert.NotNil(t, got)
}

func TestMatchLibraryByMetadata_NoMatchWrongAuthor(t *testing.T) {
	// Same title, different author — must NOT link (false-positive guard).
	lib := []models.UserBook{makeUserBook("The Hobbit", []string{"J.R.R. Tolkien"})}
	meta := ebookmeta.Metadata{ //nolint:exhaustruct //only Title+Authors matter here
		Title:   "The Hobbit",
		Authors: []string{"George Orwell"},
	}
	got := matchLibraryByMetadata(lib, meta)
	assert.Nil(t, got)
}

func TestMatchLibraryByMetadata_EmptyTitle_NoMatch(t *testing.T) {
	lib := []models.UserBook{makeUserBook("The Hobbit", []string{"J.R.R. Tolkien"})}
	meta := ebookmeta.Metadata{ //nolint:exhaustruct //only Title+Authors matter here
		Title:   "",
		Authors: []string{"J.R.R. Tolkien"},
	}
	got := matchLibraryByMetadata(lib, meta)
	assert.Nil(t, got)
}

func TestMatchLibraryByMetadata_EmptyAuthors_NoMatch(t *testing.T) {
	// No authors in file metadata → cannot verify author overlap → no match.
	lib := []models.UserBook{makeUserBook("The Hobbit", []string{"J.R.R. Tolkien"})}
	meta := ebookmeta.Metadata{ //nolint:exhaustruct //only Title+Authors matter here
		Title:   "The Hobbit",
		Authors: []string{},
	}
	got := matchLibraryByMetadata(lib, meta)
	assert.Nil(t, got)
}

func TestMatchLibraryByMetadata_EmptyLibrary(t *testing.T) {
	meta := ebookmeta.Metadata{ //nolint:exhaustruct //only Title+Authors matter here
		Title:   "The Hobbit",
		Authors: []string{"J.R.R. Tolkien"},
	}
	got := matchLibraryByMetadata(nil, meta)
	assert.Nil(t, got)
}

// --- FindDuplicateGroups ---

func isbn13Ptr(s string) *string { return &s }
func isbn10Ptr(s string) *string { return &s }

func makeUBWithISBN(
	title string,
	authors []string,
	isbn13, isbn10 *string,
	status string,
) models.UserBook {
	//nolint:exhaustruct // only fields needed for duplicate detection
	return models.UserBook{
		ID:     uuid.New(),
		BookID: uuid.New(),
		Status: status,
		Book: &models.Book{ //nolint:exhaustruct // only fields needed for matching
			Title:   title,
			Authors: authors,
			ISBN13:  isbn13,
			ISBN10:  isbn10,
		},
	}
}

func TestFindDuplicateGroups_EmptyLibrary(t *testing.T) {
	assert.Nil(t, FindDuplicateGroups(nil))
}

func TestFindDuplicateGroups_SingleEntry(t *testing.T) {
	lib := []models.UserBook{makeUserBook("The Hobbit", []string{"J.R.R. Tolkien"})}
	assert.Nil(t, FindDuplicateGroups(lib))
}

func TestFindDuplicateGroups_GroupsByISBN13(t *testing.T) {
	isbn := isbn13Ptr("9780261102217")
	a := makeUBWithISBN(
		"The Hobbit",
		[]string{"Tolkien"},
		isbn,
		nil,
		models.StatusToRead,
	)
	b := makeUBWithISBN(
		"The Hobbit (2nd ed.)",
		[]string{"J.R.R. Tolkien"},
		isbn,
		nil,
		models.StatusRead,
	)
	lib := []models.UserBook{a, b}
	groups := FindDuplicateGroups(lib)
	assert.Len(t, groups, 1)
	assert.Len(t, groups[0].Entries, 2)
	assert.Equal(t, "isbn13", groups[0].Reason)
	// Winner should be the "read" entry (higher status rank).
	assert.Equal(t, models.StatusRead, groups[0].Entries[0].Status)
}

func TestFindDuplicateGroups_GroupsByISBN10(t *testing.T) {
	isbn := isbn10Ptr("0261102214")
	a := makeUBWithISBN(
		"The Hobbit",
		[]string{"Tolkien"},
		nil,
		isbn,
		models.StatusToRead,
	)
	b := makeUBWithISBN(
		"The Hobbit (pocket)",
		[]string{"J.R.R. Tolkien"},
		nil,
		isbn,
		models.StatusToRead,
	)
	lib := []models.UserBook{a, b}
	groups := FindDuplicateGroups(lib)
	assert.Len(t, groups, 1)
	assert.Equal(t, "isbn10", groups[0].Reason)
}

func TestFindDuplicateGroups_GroupsByTitleAndAuthor(t *testing.T) {
	a := makeUserBook("The Hobbit", []string{"J.R.R. Tolkien"})
	b := makeUserBook("The Hobbit: There and Back Again", []string{"Tolkien, J.R.R."})
	lib := []models.UserBook{a, b}
	groups := FindDuplicateGroups(lib)
	assert.Len(t, groups, 1)
	assert.Equal(t, "title+author", groups[0].Reason)
}

func TestFindDuplicateGroups_NoGroupSameTitleDifferentAuthor(t *testing.T) {
	// False-positive guard: same title, different author must NOT be grouped.
	a := makeUserBook("Foundation", []string{"Isaac Asimov"})
	b := makeUserBook("Foundation", []string{"Someone Else"})
	lib := []models.UserBook{a, b}
	assert.Nil(t, FindDuplicateGroups(lib))
}

func TestFindDuplicateGroups_NoGroupDifferentBooks(t *testing.T) {
	a := makeUserBook("The Hobbit", []string{"J.R.R. Tolkien"})
	b := makeUserBook("Dune", []string{"Frank Herbert"})
	lib := []models.UserBook{a, b}
	assert.Nil(t, FindDuplicateGroups(lib))
}

func TestFindDuplicateGroups_WinnerPrefersMostProgressed(t *testing.T) {
	reading := makeUBWithISBN(
		"Dune",
		[]string{"Herbert"},
		isbn13Ptr("9780441013593"),
		nil,
		models.StatusReading,
	)
	toRead := makeUBWithISBN(
		"Dune",
		[]string{"Herbert"},
		isbn13Ptr("9780441013593"),
		nil,
		models.StatusToRead,
	)
	lib := []models.UserBook{toRead, reading} // toRead first in slice
	groups := FindDuplicateGroups(lib)
	assert.Len(t, groups, 1)
	// reading has higher status rank → should be winner (entries[0])
	assert.Equal(t, models.StatusReading, groups[0].Entries[0].Status)
}
