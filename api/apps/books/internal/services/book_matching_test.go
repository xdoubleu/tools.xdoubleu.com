//nolint:testpackage // testing unexported service helpers
package services

import (
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"tools.xdoubleu.com/apps/books/internal/models"
	"tools.xdoubleu.com/apps/books/pkg/ebookmeta"
)

func TestNormalizeTitle_Basic(t *testing.T) {
	assert.Equal(t, "hobbit", normalizeTitle("The Hobbit"))
}

func TestNormalizeTitle_StripsSubtitle(t *testing.T) {
	assert.Equal(t, "hobbit", normalizeTitle("The Hobbit: An Unexpected Journey"))
}

func TestNormalizeTitle_Lowercase(t *testing.T) {
	assert.Equal(t, "hobbit", normalizeTitle("THE HOBBIT"))
}

func TestNormalizeTitle_StripsParenthetical(t *testing.T) {
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
	assert.Equal(t, normalizeTitle("Cafe"), normalizeTitle("Café"))
}

func TestNormalizeTitle_EmptyString(t *testing.T) {
	assert.Equal(t, "", normalizeTitle(""))
}

func TestNormalizeTitle_OnlySubtitle(t *testing.T) {
	assert.Equal(t, "", normalizeTitle(": A Subtitle Only"))
}

func TestNormalizeTitle_DistinguishesVolumeAfterColon(t *testing.T) {
	// A volume number in a subtitle must survive stripping.
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
	// "(Series, #1)" is a series position, not a volume: stays stripped.
	assert.Equal(
		t,
		normalizeTitle("Firekeeper's Daughter"),
		normalizeTitle("Firekeeper's Daughter (Firekeeper's Daughter, #1)"),
	)
}

func TestNormalizeTitle_NoDoubleCountWhenNumberAlreadyInMainTitle(t *testing.T) {
	// "2001" is already in the retained segment, so it isn't duplicated.
	assert.Equal(t, "2001", normalizeTitle("2001: A Space Odyssey"))
}

func TestNormalizeTitle_StripsPunctuation(t *testing.T) {
	assert.Equal(t, "helloworld", normalizeTitle("Hello, World!"))
}

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
	// High overlap, different volume: Jaccard alone would clear the threshold.
	a := titleTokens("Mistborn Saga Legendary Heroes Volume 1")
	b := titleTokens("Mistborn Saga Legendary Heroes Volume 2")
	assert.GreaterOrEqual(t, tokenSimilarity(a, b), titleSimilarityThreshold)
	assert.False(t, titlesFuzzyMatch(a, b))
}

func TestTitlesFuzzyMatch_SwappedNumbersNeverMatch(t *testing.T) {
	// Same digits, different positions: comparison must be positional.
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
	// Roman numerals distinguish "Programmer I" from "Programmer II".
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

func TestNormalizeAuthor_FirstLast(t *testing.T) {
	assert.Equal(t, "tolkien", normalizeAuthor("J.R.R. Tolkien"))
}

func TestNormalizeAuthor_LastFirstComma(t *testing.T) {
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

func TestMatchCatalogByMetadata_ExactMatchNotInCallersLibrary(t *testing.T) {
	// The catalog entry is another user's; still a valid attach target.
	catalog := []models.UserBook{makeUserBook("The Hobbit", []string{"J.R.R. Tolkien"})}
	meta := ebookmeta.Metadata{ //nolint:exhaustruct //only Title+Authors matter here
		Title:   "The Hobbit",
		Authors: []string{"J.R.R. Tolkien"},
	}
	got := matchCatalogByMetadata(catalog, meta)
	assert.NotNil(t, got)
	assert.Equal(t, catalog[0].BookID, got.BookID)
}

func TestMatchCatalogByMetadata_FuzzyReorderedTitle(t *testing.T) {
	// Exact matching misses this; the fuzzy fallback must catch it.
	catalog := []models.UserBook{
		makeUserBook("Fellowship of the Ring, The", []string{"J.R.R. Tolkien"}),
	}
	meta := ebookmeta.Metadata{ //nolint:exhaustruct //only Title+Authors matter here
		Title:   "The Fellowship of the Ring",
		Authors: []string{"J.R.R. Tolkien"},
	}
	got := matchCatalogByMetadata(catalog, meta)
	assert.NotNil(t, got)
	assert.Equal(t, catalog[0].BookID, got.BookID)
}

func TestMatchCatalogByMetadata_FuzzyDoesNotMergeDifferentVolumes(t *testing.T) {
	catalog := []models.UserBook{
		makeUserBook(
			"Mistborn Saga Legendary Heroes Volume 1",
			[]string{"Brandon Sanderson"},
		),
	}
	meta := ebookmeta.Metadata{ //nolint:exhaustruct //only Title+Authors matter here
		Title:   "Mistborn Saga Legendary Heroes Volume 2",
		Authors: []string{"Brandon Sanderson"},
	}
	got := matchCatalogByMetadata(catalog, meta)
	assert.Nil(t, got)
}

func TestMatchCatalogByMetadata_FuzzyRequiresAuthorOverlap(t *testing.T) {
	catalog := []models.UserBook{
		makeUserBook("Fellowship of the Ring, The", []string{"J.R.R. Tolkien"}),
	}
	meta := ebookmeta.Metadata{ //nolint:exhaustruct //only Title+Authors matter here
		Title:   "The Fellowship of the Ring",
		Authors: []string{"George Orwell"},
	}
	got := matchCatalogByMetadata(catalog, meta)
	assert.Nil(t, got)
}

func TestMatchCatalogByMetadata_BelowThreshold_NoMatch(t *testing.T) {
	catalog := []models.UserBook{
		makeUserBook("The Fellowship of the Ring", []string{"J.R.R. Tolkien"}),
	}
	meta := ebookmeta.Metadata{ //nolint:exhaustruct //only Title+Authors matter here
		Title:   "The Return of the King",
		Authors: []string{"J.R.R. Tolkien"},
	}
	got := matchCatalogByMetadata(catalog, meta)
	assert.Nil(t, got)
}

func TestMatchCatalogByMetadata_EmptyTitle_NoMatch(t *testing.T) {
	catalog := []models.UserBook{makeUserBook("The Hobbit", []string{"J.R.R. Tolkien"})}
	meta := ebookmeta.Metadata{ //nolint:exhaustruct //only Title+Authors matter here
		Title:   "",
		Authors: []string{"J.R.R. Tolkien"},
	}
	got := matchCatalogByMetadata(catalog, meta)
	assert.Nil(t, got)
}

func TestMatchCatalogByMetadata_EmptyAuthors_NoMatch(t *testing.T) {
	catalog := []models.UserBook{makeUserBook("The Hobbit", []string{"J.R.R. Tolkien"})}
	meta := ebookmeta.Metadata{ //nolint:exhaustruct //only Title+Authors matter here
		Title:   "The Fellowship of the Ring",
		Authors: []string{},
	}
	got := matchCatalogByMetadata(catalog, meta)
	assert.Nil(t, got)
}

func TestMatchCatalogByMetadata_EmptyCatalog(t *testing.T) {
	meta := ebookmeta.Metadata{ //nolint:exhaustruct //only Title+Authors matter here
		Title:   "The Hobbit",
		Authors: []string{"J.R.R. Tolkien"},
	}
	got := matchCatalogByMetadata(nil, meta)
	assert.Nil(t, got)
}

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
	assert.Equal(t, models.StatusRead, groups[0].Entries[0].Status)
}

func TestFindDuplicateGroups_DoesNotGroupByISBN10Only(t *testing.T) {
	// ISBN-10 is not a matching signal.
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
	// Series suffix without ISBN vs clean title with ISBN: same book.
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
	// Same author, different books sharing "of"/"the": must stay separate.
	a := makeUserBook("The Fellowship of the Ring", []string{"J.R.R. Tolkien"})
	b := makeUserBook("The Return of the King", []string{"J.R.R. Tolkien"})
	lib := []models.UserBook{a, b}
	assert.Nil(t, FindDuplicateGroups(lib))
}

func TestFindDuplicateGroups_DoesNotMergeDifferentVolumes(t *testing.T) {
	a := makeUserBook("System Design Interview: Volume 1", []string{"Alex Xu"})
	b := makeUserBook("System Design Interview: Volume 2", []string{"Alex Xu"})
	lib := []models.UserBook{a, b}
	assert.Nil(t, FindDuplicateGroups(lib))
}

func TestFindDuplicateGroups_DoesNotMergeDifferentRomanVolumes(t *testing.T) {
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
	// Both ISBN13 and title+author match: the reason must be the stronger isbn13.
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
	// A nil Book must not panic and is excluded.
	realBook := makeUserBook("Dune", []string{"Frank Herbert"})
	nilBook := models.UserBook{ //nolint:exhaustruct // only testing nil-Book guard
		ID:     uuid.New(),
		BookID: uuid.New(),
		Book:   nil,
	}
	lib := []models.UserBook{realBook, nilBook}
	assert.Nil(t, FindDuplicateGroups(lib))
}

func TestFindDuplicateGroups_LargeLibrary(t *testing.T) {
	// 5000 books with 50 planted ISBN13 and 50 title+author pairs; also guards
	// against an O(n^2) regression timing out.
	const (
		uniqueBooks    = 4900
		isbn13Pairs    = 50
		titleAuthPairs = 50
	)

	lib := make([]models.UserBook, 0, uniqueBooks+isbn13Pairs*2+titleAuthPairs*2)

	for i := range uniqueBooks {
		lib = append(lib, makeUserBook(
			"Unique Book "+fmt.Sprint(i),
			[]string{"Author" + fmt.Sprint(i)},
		))
	}

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

	for i := range titleAuthPairs {
		title := fmt.Sprintf("Duplicate Title %d", i)
		author := fmt.Sprintf("Shared Author %d", i)
		a := makeUserBook(title, []string{author})
		b := makeUserBook(title+": A Subtitle", []string{author})
		lib = append(lib, a, b)
	}

	groups := FindDuplicateGroups(lib)

	assert.Len(t, groups, isbn13Pairs+titleAuthPairs)

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
	assert.Equal(t, models.StatusReading, groups[0].Entries[0].Status)
}

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
	sparse := makeUBWithBook(
		models.Book{ //nolint:exhaustruct // only fields needed for matching
			Title:   "Dune",
			Authors: []string{"Herbert"},
			ISBN13:  isbn,
		},
		models.StatusRead, // higher status than rich — completeness must still win
		[]string{"epub", "pdf", "mobi"},
	)

	lib := []models.UserBook{sparse, rich} // sparse first in slice
	groups := FindDuplicateGroups(lib)
	assert.Len(t, groups, 1)
	// Metadata completeness beats status and formats.
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
	assert.Equal(t, noFormats.BookID, groups[0].Entries[0].BookID)
}

// makeDupGroup returns two UserBooks sharing isbn13 with the given title.
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
	// Expected order: isbn13 groups (Alpha, Beta) before title+author (Gamma).

	a1, a2 := makeDupGroup("Alpha", "9780000000001")
	b1, b2 := makeDupGroup("Beta", "9780000000002")
	c1 := makeUserBook("Gamma", []string{"AuthorC"})
	c2 := makeUserBook("Gamma: A Subtitle", []string{"AuthorC"})

	lib := []models.UserBook{c1, b1, a2, c2, a1, b2} // intentionally shuffled

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

	assert.Equal(t, "isbn13", first[0].Reason)
	assert.Equal(t, "isbn13", first[1].Reason)
	assert.Equal(t, "title+author", first[2].Reason)

	title0 := first[0].Entries[0].Book.Title
	title1 := first[1].Entries[0].Book.Title
	assert.Less(t, title0, title1, "isbn13 groups should be sorted by winner title")
}

func TestFindDuplicateGroups_GroupOrderStableOnShuffledInput(t *testing.T) {
	// Order must not depend on input order.
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

	// The within-group winner is a UUID tiebreak, so check any entry's title.
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
