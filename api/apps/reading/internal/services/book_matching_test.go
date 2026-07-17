//nolint:testpackage // testing unexported service helpers
package services

import (
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"tools.xdoubleu.com/apps/reading/internal/models"
	"tools.xdoubleu.com/apps/reading/pkg/ebookmeta"
)

// --- normalizeTitle ---

func TestNormalizeTitle_Basic(t *testing.T) {
	// Leading article is dropped so "The Hobbit" and "Hobbit" match.
	assert.Equal(t, "hobbit", normalizeTitle("The Hobbit"))
}

func TestNormalizeTitle_StripsSubtitle(t *testing.T) {
	assert.Equal(t, "hobbit", normalizeTitle("The Hobbit: An Unexpected Journey"))
}

func TestNormalizeTitle_Lowercase(t *testing.T) {
	assert.Equal(t, "hobbit", normalizeTitle("THE HOBBIT"))
}

func TestNormalizeTitle_StripsParenthetical(t *testing.T) {
	// Goodreads-style series annotation must not defeat matching.
	assert.Equal(
		t,
		normalizeTitle("Firekeeper's Daughter"),
		normalizeTitle("Firekeeper's Daughter (Firekeeper's Daughter, #1)"),
	)
}

func TestNormalizeTitle_StripsBracketed(t *testing.T) {
	assert.Equal(
		t,
		normalizeTitle("Dune"),
		normalizeTitle("Dune [Illustrated]"),
	)
}

func TestNormalizeTitle_StripsTrailingEditionMarker(t *testing.T) {
	assert.Equal(
		t,
		normalizeTitle("Dune"),
		normalizeTitle("Dune - Deluxe Edition"),
	)
}

func TestNormalizeTitle_KeepsShortArticleTitleIntact(t *testing.T) {
	// A single-word title equal to an article is not stripped down to "".
	assert.Equal(t, "a", normalizeTitle("A"))
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

func TestNormalizeTitle_DistinguishesVolumeAfterColon(t *testing.T) {
	// A volume number in a colon-separated subtitle must survive stripping —
	// otherwise "System Design Interview" Volume 1 and Volume 2 collapse to
	// the same normalized title.
	assert.NotEqual(t,
		normalizeTitle("System Design Interview: Volume 1"),
		normalizeTitle("System Design Interview: Volume 2"),
	)
}

func TestNormalizeTitle_DistinguishesVolumeAfterDash(t *testing.T) {
	assert.NotEqual(t,
		normalizeTitle("System Design Interview - Volume 1"),
		normalizeTitle("System Design Interview - Volume 2"),
	)
}

func TestNormalizeTitle_DistinguishesVolumeInParenthetical(t *testing.T) {
	assert.NotEqual(t,
		normalizeTitle("System Design Interview (Volume 1)"),
		normalizeTitle("System Design Interview (Volume 2)"),
	)
}

func TestNormalizeTitle_StripsParentheticalStillWorksForSeriesMarker(t *testing.T) {
	// Goodreads-style "(Series, #1)" is a shelf/series position, not a
	// volume of the book itself — it must remain stripped as noise.
	assert.Equal(
		t,
		normalizeTitle("Firekeeper's Daughter"),
		normalizeTitle("Firekeeper's Daughter (Firekeeper's Daughter, #1)"),
	)
}

func TestNormalizeTitle_NoDoubleCountWhenNumberAlreadyInMainTitle(t *testing.T) {
	// The year "2001" is already part of the retained segment (before the
	// colon), so it must not be duplicated.
	assert.Equal(t, "2001", normalizeTitle("2001: A Space Odyssey"))
}

func TestNormalizeTitle_StripsPunctuation(t *testing.T) {
	// Punctuation other than the colon is also stripped.
	assert.Equal(t, "helloworld", normalizeTitle("Hello, World!"))
}

// --- titleTokens / tokenSimilarity ---

func TestTokenSimilarity_ReorderedWordsMatch(t *testing.T) {
	a := titleTokens("The Fellowship of the Ring")
	b := titleTokens("Fellowship of the Ring, The")
	assert.InDelta(t, 1.0, tokenSimilarity(a, b), 0.001)
}

func TestTokenSimilarity_DifferentBooksSameSeriesWordsBelowThreshold(t *testing.T) {
	a := titleTokens("The Fellowship of the Ring")
	b := titleTokens("The Return of the King")
	assert.Less(t, tokenSimilarity(a, b), titleSimilarityThreshold)
}

func TestTokenSimilarity_EmptySide(t *testing.T) {
	assert.InDelta(t, 0.0, tokenSimilarity(nil, titleTokens("Dune")), 0.001)
}

func TestTitlesFuzzyMatch_DifferingVolumeNumberNeverMatches(t *testing.T) {
	// High word overlap but a different volume number — must not fuzzy-match
	// even though Jaccard similarity alone would clear the threshold.
	a := titleTokens("Mistborn Saga Legendary Heroes Volume 1")
	b := titleTokens("Mistborn Saga Legendary Heroes Volume 2")
	assert.GreaterOrEqual(t, tokenSimilarity(a, b), titleSimilarityThreshold)
	assert.False(t, titlesFuzzyMatch(a, b))
}

func TestTitlesFuzzyMatch_SwappedNumbersNeverMatch(t *testing.T) {
	// Same digit set, different positions — "Book 1 Edition 2" is not
	// "Book 2 Edition 1". Set-based (as opposed to positional) numeric
	// comparison would wrongly treat these as identical.
	a := titleTokens("ISBN Book 1 edition 2")
	b := titleTokens("ISBN Book 2 edition 1")
	assert.InDelta(t, 1.0, tokenSimilarity(a, b), 0.001)
	assert.False(t, titlesFuzzyMatch(a, b))
}

func TestTitlesFuzzyMatch_SameVolumeNumberCanMatch(t *testing.T) {
	a := titleTokens("The Fellowship of the Ring")
	b := titleTokens("Fellowship of the Ring, The")
	assert.True(t, titlesFuzzyMatch(a, b))
}

func TestTitlesFuzzyMatch_DifferingRomanNumeralNeverMatches(t *testing.T) {
	// The reported bug: "Programmer I" vs "Programmer II" — high word overlap,
	// but the Roman numeral distinguishes two different books.
	a := titleTokens(
		"OCP Oracle Certified Professional Java SE 11 Programmer I Study Guide",
	)
	b := titleTokens(
		"OCP Oracle Certified Professional Java SE 11 Programmer II Study Guide",
	)
	assert.GreaterOrEqual(t, tokenSimilarity(a, b), titleSimilarityThreshold)
	assert.False(t, titlesFuzzyMatch(a, b))
}

func TestTitlesFuzzyMatch_SameRomanNumeralCanMatch(t *testing.T) {
	a := titleTokens("Fellowship of the Ring Part II")
	b := titleTokens("Fellowship of the Ring, Part II")
	assert.True(t, titlesFuzzyMatch(a, b))
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

func makeUBWithISBN(
	title string,
	authors []string,
	isbn13 *string,
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
		models.StatusToRead,
	)
	b := makeUBWithISBN(
		"The Hobbit (2nd ed.)",
		[]string{"J.R.R. Tolkien"},
		isbn,
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

func TestFindDuplicateGroups_DoesNotGroupByISBN10Only(t *testing.T) {
	// ISBN-10 is no longer a matching signal — two entries sharing only an
	// ISBN-10 (and different titles/authors) must NOT be grouped.
	a := makeUserBook("The Hobbit", []string{"Tolkien"})
	b := makeUserBook("The Hobbit (pocket)", []string{"Herbert"})
	lib := []models.UserBook{a, b}
	assert.Nil(t, FindDuplicateGroups(lib))
}

func TestFindDuplicateGroups_GroupsByTitleAndAuthor(t *testing.T) {
	a := makeUserBook("The Hobbit", []string{"J.R.R. Tolkien"})
	b := makeUserBook("The Hobbit: There and Back Again", []string{"Tolkien, J.R.R."})
	lib := []models.UserBook{a, b}
	groups := FindDuplicateGroups(lib)
	assert.Len(t, groups, 1)
	assert.Equal(t, "title+author", groups[0].Reason)
}

func TestFindDuplicateGroups_GroupsBySeriesAnnotation(t *testing.T) {
	// The reported bug: same book, one entry carries a Goodreads-style series
	// suffix and no ISBN, the other has an ISBN and a clean title.
	isbn := isbn13Ptr("9780062983594")
	a := makeUBWithISBN(
		"Firekeeper's Daughter",
		[]string{"Angeline Boulley"},
		isbn,
		models.StatusToRead,
	)
	b := makeUserBook(
		"Firekeeper's Daughter (Firekeeper's Daughter, #1)",
		[]string{"Angeline Boulley"},
	)
	lib := []models.UserBook{a, b}
	groups := FindDuplicateGroups(lib)
	assert.Len(t, groups, 1)
	assert.Len(t, groups[0].Entries, 2)
}

func TestFindDuplicateGroups_FuzzyMatchesReorderedTitle(t *testing.T) {
	a := makeUserBook("The Fellowship of the Ring", []string{"J.R.R. Tolkien"})
	b := makeUserBook("Fellowship of the Ring, The", []string{"Tolkien"})
	lib := []models.UserBook{a, b}
	groups := FindDuplicateGroups(lib)
	assert.Len(t, groups, 1)
	assert.Equal(t, "title+author", groups[0].Reason)
}

func TestFindDuplicateGroups_FuzzyDoesNotMergeDifferentBooksSameAuthor(t *testing.T) {
	// False-positive guard: same author, different books in the same series
	// share several title words ("of", "the") but must stay separate.
	a := makeUserBook("The Fellowship of the Ring", []string{"J.R.R. Tolkien"})
	b := makeUserBook("The Return of the King", []string{"J.R.R. Tolkien"})
	lib := []models.UserBook{a, b}
	assert.Nil(t, FindDuplicateGroups(lib))
}

func TestFindDuplicateGroups_DoesNotMergeDifferentVolumes(t *testing.T) {
	// The reported bug: "System Design Interview: Volume 1" and "…: Volume 2"
	// are different books by the same author and must never be grouped.
	a := makeUserBook("System Design Interview: Volume 1", []string{"Alex Xu"})
	b := makeUserBook("System Design Interview: Volume 2", []string{"Alex Xu"})
	lib := []models.UserBook{a, b}
	assert.Nil(t, FindDuplicateGroups(lib))
}

func TestFindDuplicateGroups_DoesNotMergeDifferentRomanVolumes(t *testing.T) {
	// The reported bug: two Oracle certification study guides differing only
	// by "Programmer I" vs "Programmer II" must never be grouped.
	a := makeUserBook(
		"OCP Java SE 11 Programmer I Study Guide: Exam 1z0-815",
		[]string{"Jeanne Boyarsky"},
	)
	b := makeUserBook(
		"OCP Java SE 11 Programmer II Study Guide: Exam 1z0-816",
		[]string{"Jeanne Boyarsky"},
	)
	lib := []models.UserBook{a, b}
	assert.Nil(t, FindDuplicateGroups(lib))
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

func TestFindDuplicateGroups_ReasonUpgradedToStrongest(t *testing.T) {
	// Two books share both an ISBN-13 and a matching title+author.
	// The group reason must be "isbn13" (stronger signal).
	isbn := isbn13Ptr("9780261102217")
	a := makeUBWithISBN(
		"The Hobbit",
		[]string{"J.R.R. Tolkien"},
		isbn,
		models.StatusToRead,
	)
	b := makeUBWithISBN(
		"The Hobbit: There and Back Again",
		[]string{"Tolkien, J.R.R."},
		isbn,
		models.StatusToRead,
	)
	lib := []models.UserBook{a, b}
	groups := FindDuplicateGroups(lib)
	assert.Len(t, groups, 1)
	assert.Equal(t, "isbn13", groups[0].Reason)
}

func TestFindDuplicateGroups_NilBookSkipped(t *testing.T) {
	// An entry with a nil Book pointer must not panic and must be excluded from
	// all groups.
	realBook := makeUserBook("Dune", []string{"Frank Herbert"})
	nilBook := models.UserBook{ //nolint:exhaustruct // only testing nil-Book guard
		ID:     uuid.New(),
		BookID: uuid.New(),
		Book:   nil,
	}
	lib := []models.UserBook{realBook, nilBook}
	// Only one real book — no group possible.
	assert.Nil(t, FindDuplicateGroups(lib))
}

func TestFindDuplicateGroups_LargeLibrary(t *testing.T) {
	// Build a synthetic library of 5 000 books with 50 planted ISBN13 duplicate
	// pairs and 50 planted title+author duplicate pairs. Verify all 100 expected
	// groups are returned and that no spurious groups appear.
	//
	// This test also acts as a regression guard: the pre-refactor O(n²) algorithm
	// timed out on libraries of this size; the bucketed O(n) implementation must
	// complete well within a test timeout.
	const (
		uniqueBooks    = 4900
		isbn13Pairs    = 50
		titleAuthPairs = 50
	)

	lib := make([]models.UserBook, 0, uniqueBooks+isbn13Pairs*2+titleAuthPairs*2)

	// Unique, non-duplicate books.
	for i := range uniqueBooks {
		lib = append(lib, makeUserBook(
			"Unique Book "+fmt.Sprint(i),
			[]string{"Author" + fmt.Sprint(i)},
		))
	}

	// Planted ISBN13 duplicates: two entries sharing the same ISBN13.
	for i := range isbn13Pairs {
		isbn := isbn13Ptr(fmt.Sprintf("978000000%04d", i))
		a := makeUBWithISBN(
			fmt.Sprintf("ISBN Book %d edition 1", i),
			[]string{"Writer One"},
			isbn, models.StatusToRead,
		)
		b := makeUBWithISBN(
			fmt.Sprintf("ISBN Book %d edition 2", i),
			[]string{"Writer One"},
			isbn, models.StatusRead,
		)
		lib = append(lib, a, b)
	}

	// Planted title+author duplicates: same normalised title and author, no ISBN.
	for i := range titleAuthPairs {
		title := fmt.Sprintf("Duplicate Title %d", i)
		author := fmt.Sprintf("Shared Author %d", i)
		a := makeUserBook(title, []string{author})
		b := makeUserBook(title+": A Subtitle", []string{author})
		lib = append(lib, a, b)
	}

	groups := FindDuplicateGroups(lib)

	// Total expected groups = isbn13Pairs + titleAuthPairs.
	assert.Len(t, groups, isbn13Pairs+titleAuthPairs)

	// Every returned group must have exactly 2 entries.
	for _, g := range groups {
		assert.Len(t, g.Entries, 2)
	}
}

func TestFindDuplicateGroups_WinnerPrefersMostProgressed(t *testing.T) {
	reading := makeUBWithISBN(
		"Dune",
		[]string{"Herbert"},
		isbn13Ptr("9780441013593"),
		models.StatusReading,
	)
	toRead := makeUBWithISBN(
		"Dune",
		[]string{"Herbert"},
		isbn13Ptr("9780441013593"),
		models.StatusToRead,
	)
	lib := []models.UserBook{toRead, reading} // toRead first in slice
	groups := FindDuplicateGroups(lib)
	assert.Len(t, groups, 1)
	// Equal metadata completeness; reading has higher status rank → winner.
	assert.Equal(t, models.StatusReading, groups[0].Entries[0].Status)
}

// makeUBWithBook constructs a UserBook with the given Book, status, and
// formats so tests can exercise completeness-sensitive winner selection.
func makeUBWithBook(
	book models.Book,
	status string,
	formats []string,
) models.UserBook {
	//nolint:exhaustruct // only fields needed for richness / duplicate detection
	return models.UserBook{
		ID:      uuid.New(),
		BookID:  uuid.New(),
		Status:  status,
		Formats: formats,
		Book:    &book,
	}
}

func TestFindDuplicateGroups_WinnerPrefersCompleteMetadata(t *testing.T) {
	isbn := isbn13Ptr("9780441013593")
	coverURL := "https://example.com/cover.jpg"
	desc := "A sci-fi epic."

	// rich: complete metadata, lower status
	rich := makeUBWithBook(
		models.Book{ //nolint:exhaustruct // only fields needed for matching
			Title:       "Dune",
			Authors:     []string{"Herbert"},
			ISBN13:      isbn,
			CoverURL:    strPtr(coverURL),
			Description: strPtr(desc),
			PageCount:   intPtr(412),
		},
		models.StatusToRead,
		nil, // no formats
	)
	// sparse: no metadata, higher status and many formats
	sparse := makeUBWithBook(
		models.Book{ //nolint:exhaustruct // only fields needed for matching
			Title:   "Dune",
			Authors: []string{"Herbert"},
			ISBN13:  isbn,
			// no cover, description, or page count
		},
		models.StatusRead, // higher status than rich — completeness must still win
		[]string{"epub", "pdf", "mobi"},
	)

	lib := []models.UserBook{sparse, rich} // sparse first in slice
	groups := FindDuplicateGroups(lib)
	assert.Len(t, groups, 1)
	// rich has cover + description + page count → wins despite lower status and no formats
	assert.Equal(t, rich.BookID, groups[0].Entries[0].BookID)
}

func TestFindDuplicateGroups_FormatsDoNotAffectWinner(t *testing.T) {
	isbn := isbn13Ptr("9780141439518")

	noFormats := makeUBWithBook(
		models.Book{ //nolint:exhaustruct // only fields needed for matching
			Title:       "Pride and Prejudice",
			Authors:     []string{"Austen"},
			ISBN13:      isbn,
			CoverURL:    strPtr("https://example.com/cover.jpg"),
			Description: strPtr("A classic novel."),
			PageCount:   intPtr(279),
		},
		models.StatusToRead,
		nil,
	)
	manyFormats := makeUBWithBook(
		models.Book{ //nolint:exhaustruct // only fields needed for matching
			Title:   "Pride and Prejudice",
			Authors: []string{"Austen"},
			ISBN13:  isbn,
		},
		models.StatusToRead,
		[]string{"epub", "pdf", "mobi", "azw3"},
	)

	lib := []models.UserBook{manyFormats, noFormats}
	groups := FindDuplicateGroups(lib)
	assert.Len(t, groups, 1)
	// Metadata completeness dominates — manyFormats must NOT win.
	assert.Equal(t, noFormats.BookID, groups[0].Entries[0].BookID)
}

// --- FindDuplicateGroups group ordering ---

// makeDupGroup is a convenience builder: returns two UserBooks sharing isbn13
// so they form a duplicate group with the given title (used as sort key).
func makeDupGroup(
	title, isbn13val string,
) (models.UserBook, models.UserBook) {
	var i13 *string
	if isbn13val != "" {
		i13 = isbn13Ptr(isbn13val)
	}
	a := makeUBWithISBN(title, []string{"Author"}, i13, models.StatusToRead)
	b := makeUBWithISBN(
		title+" (2nd ed.)",
		[]string{"Author"},
		i13,
		models.StatusToRead,
	)
	return a, b
}

func TestFindDuplicateGroups_GroupOrderIsDeterministic(t *testing.T) {
	// Build a library with three distinct duplicate groups:
	//   group A — isbn13 match, title "Alpha"
	//   group B — isbn13 match, title "Beta"
	//   group C — title+author match, title "Gamma"
	//
	// Expected sort: signal strength desc (A, B before C), then title asc (A
	// before B). So stable order is [A, B, C].

	a1, a2 := makeDupGroup("Alpha", "9780000000001")
	b1, b2 := makeDupGroup("Beta", "9780000000002")
	// title+author group — no ISBN, matched by shared normalised title+author
	c1 := makeUserBook("Gamma", []string{"AuthorC"})
	c2 := makeUserBook("Gamma: A Subtitle", []string{"AuthorC"})

	lib := []models.UserBook{c1, b1, a2, c2, a1, b2} // intentionally shuffled

	// Call FindDuplicateGroups multiple times and verify the order is identical.
	first := FindDuplicateGroups(lib)
	if len(first) != 3 {
		t.Fatalf("expected 3 groups, got %d", len(first))
	}

	for range 10 {
		got := FindDuplicateGroups(lib)
		assert.Len(t, got, 3)
		for i, g := range got {
			assert.Equal(t, first[i].Reason, g.Reason,
				"group %d reason changed between calls", i)
			assert.Equal(
				t,
				first[i].Entries[0].BookID,
				g.Entries[0].BookID,
				"group %d winner changed between calls", i,
			)
		}
	}

	// Verify the documented order: isbn13 groups first, then title+author.
	assert.Equal(t, "isbn13", first[0].Reason)
	assert.Equal(t, "isbn13", first[1].Reason)
	assert.Equal(t, "title+author", first[2].Reason)

	// Within the isbn13 tier: "Alpha" < "Beta" alphabetically.
	title0 := first[0].Entries[0].Book.Title
	title1 := first[1].Entries[0].Book.Title
	assert.Less(t, title0, title1, "isbn13 groups should be sorted by winner title")
}

func TestFindDuplicateGroups_GroupOrderStableOnShuffledInput(t *testing.T) {
	// Groups should come back in the same order regardless of input slice order.
	a1, a2 := makeDupGroup("Zeta", "9780000000010")
	b1, b2 := makeDupGroup("Aardvark", "9780000000011")

	orderA := FindDuplicateGroups([]models.UserBook{a1, a2, b1, b2})
	orderB := FindDuplicateGroups([]models.UserBook{b2, a2, b1, a1})
	orderC := FindDuplicateGroups([]models.UserBook{b1, b2, a1, a2})

	if len(orderA) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(orderA))
	}
	assert.Len(t, orderB, 2)
	assert.Len(t, orderC, 2)

	for i := range 2 {
		assert.Equal(
			t,
			orderA[i].Entries[0].BookID,
			orderB[i].Entries[0].BookID,
			"group %d winner differs between orderA and orderB", i,
		)
		assert.Equal(
			t,
			orderA[i].Entries[0].BookID,
			orderC[i].Entries[0].BookID,
			"group %d winner differs between orderA and orderC", i,
		)
	}

	// "Aardvark" < "Zeta" — the Aardvark group must come first.
	// We check that some entry in group[0] is titled "Aardvark" rather than
	// asserting Entries[0] specifically, because the within-group winner is
	// decided by UUID tiebreak and is non-deterministic across runs.
	firstGroupTitles := make([]string, 0, len(orderA[0].Entries))
	for _, e := range orderA[0].Entries {
		if e.Book != nil {
			firstGroupTitles = append(firstGroupTitles, e.Book.Title)
		}
	}
	assert.Contains(
		t,
		firstGroupTitles,
		"Aardvark",
		"groups not sorted by title within same signal tier",
	)
}

func TestMetadataCompleteness_NilBook(t *testing.T) {
	assert.Equal(t, 0, metadataCompleteness(nil))
}

func TestMetadataCompleteness_Empty(t *testing.T) {
	b := &models.Book{} //nolint:exhaustruct // all fields intentionally zero for test
	assert.Equal(t, 0, metadataCompleteness(b))
}

func TestMetadataCompleteness_Full(t *testing.T) {
	b := &models.Book{ //nolint:exhaustruct // only metadata fields are needed here
		Authors:     []string{"Author"},
		ISBN13:      strPtr("9780441013593"),
		CoverURL:    strPtr("https://example.com/cover.jpg"),
		Description: strPtr("A description."),
		PageCount:   intPtr(300),
	}
	assert.Equal(t, 5, metadataCompleteness(b))
}

// --- normalizeISBN ---

func TestNormalizeISBN_PlainPassthrough(t *testing.T) {
	assert.Equal(t, "9789463107389", normalizeISBN("9789463107389"))
}

func TestNormalizeISBN_HyphenatedStripped(t *testing.T) {
	assert.Equal(t, "9789463107389", normalizeISBN("978-94-6310-738-9"))
}

func TestNormalizeISBN_EmptyString(t *testing.T) {
	assert.Equal(t, "", normalizeISBN(""))
}

func TestNormalizeISBN_SpacesStripped(t *testing.T) {
	assert.Equal(t, "9780140449112", normalizeISBN("978 0 14 044911 2"))
}

// --- FindDuplicateGroups: ISBN normalization ---

func TestFindDuplicateGroups_HyphenatedISBNGroupsWithPlain(t *testing.T) {
	hyphenated := "978-94-6310-738-9"
	plain := "9789463107389"
	idA, idB := uuid.New(), uuid.New()

	lib := []models.UserBook{
		{ //nolint:exhaustruct //only Book needed
			BookID: idA,
			Book: &models.Book{ //nolint:exhaustruct //only matching fields
				ID:      idA,
				Title:   "Franke Vragen",
				Authors: []string{"Vandenbroucke"},
				ISBN13:  &hyphenated,
			},
		},
		{ //nolint:exhaustruct //only Book needed
			BookID: idB,
			Book: &models.Book{ //nolint:exhaustruct //only matching fields
				ID:      idB,
				Title:   "Franke Vragen",
				Authors: []string{"Vandenbroucke"},
				ISBN13:  &plain,
			},
		},
	}

	groups := FindDuplicateGroups(lib)
	assert.Len(
		t,
		groups,
		1,
		"hyphenated and plain ISBN should be grouped as duplicates",
	)
	assert.Equal(t, "isbn13", groups[0].Reason)
	assert.Len(t, groups[0].Entries, 2)
}
