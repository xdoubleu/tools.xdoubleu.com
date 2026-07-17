//nolint:testpackage // testing unexported service helpers
package services

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xdoubleu/essentia/v4/pkg/logging"

	"tools.xdoubleu.com/apps/reading/internal/models"
	"tools.xdoubleu.com/apps/reading/internal/repositories"
	"tools.xdoubleu.com/apps/reading/pkg/hardcover"
	"tools.xdoubleu.com/apps/reading/pkg/objectstore"
	"tools.xdoubleu.com/apps/reading/pkg/unicat"
)

// ---------------------------------------------------------------------------
// BuildResyncProposals: per-book scan status (found flags + last_resync_at)
// ---------------------------------------------------------------------------

func TestBuildResyncProposals_RecordsScanStatus(t *testing.T) {
	id := uuid.New()
	isbn := "9780140449112"
	repo := &fakeBooksResync{ //nolint:exhaustruct //zero values fine
		books: []models.Book{
			//nolint:exhaustruct // partial
			{ID: id, Title: "Found In UniCat Only", ISBN13: &isbn},
		},
	}
	svc := &BookService{ //nolint:exhaustruct // partial
		logger:      logging.NewNopLogger(),
		booksResync: repo,
		uniCat: &fakeUCClient{ //nolint:exhaustruct // partial
			//nolint:exhaustruct // partial
			byISBN: &unicat.ExternalBook{Title: "Found In UniCat Only"},
		},
		//nolint:exhaustruct // zero-value: byISBN nil -> ErrNotFound, empty
		// search fallback -> a clean, resolved "not found" (not unresolved)
		hardcover:   &fakeHCClient{},
		objectStore: objectstore.NewFake(),
	}

	_, err := svc.BuildResyncProposals(
		context.Background(), logging.NewNopLogger(), nil, false,
	)
	require.NoError(t, err)

	require.Len(t, repo.scanStatusCalls, 1)
	call := repo.scanStatusCalls[0]
	assert.Equal(t, id, call.bookID)
	require.NotNil(t, call.ucFound)
	assert.True(t, *call.ucFound)
	require.NotNil(t, call.hcFound)
	assert.False(t, *call.hcFound)
}

// TestBuildResyncProposals_Hardcover_FoundByISBN verifies the hardcover branch:
// a configured hardcover client that resolves the ISBN produces a "hardcover"
// proposal and records hardcover_found = true.
func TestBuildResyncProposals_Hardcover_FoundByISBN(t *testing.T) {
	id := uuid.New()
	isbn := "9780140449112"
	repo := &fakeBooksResync{ //nolint:exhaustruct //zero values fine
		books: []models.Book{
			//nolint:exhaustruct // partial
			{ID: id, Title: "The Odyssey", ISBN13: &isbn},
		},
	}
	hcTitle := "The Odyssey"
	svc := &BookService{ //nolint:exhaustruct // partial
		logger:      logging.NewNopLogger(),
		booksResync: repo,
		hardcover: &fakeHCClient{ //nolint:exhaustruct // partial
			byISBN: &hardcover.ExternalBook{ //nolint:exhaustruct // partial
				Title: hcTitle, Authors: []string{"Homer"}, ISBN13: &isbn,
			},
		},
		objectStore: objectstore.NewFake(),
	}

	_, err := svc.BuildResyncProposals(
		context.Background(), logging.NewNopLogger(), nil, false,
	)
	require.NoError(t, err)

	require.Len(t, repo.scanStatusCalls, 1)
	call := repo.scanStatusCalls[0]
	require.NotNil(t, call.hcFound)
	assert.True(t, *call.hcFound, "hardcover resolved the ISBN, so it's found")
}

func TestBuildResyncProposals_ScanStatus_UnsearchableAllNil(t *testing.T) {
	id := uuid.New()
	repo := &fakeBooksResync{ //nolint:exhaustruct //zero values fine
		books: []models.Book{{ID: id}}, //nolint:exhaustruct // no ISBN, no title
	}
	svc := &BookService{ //nolint:exhaustruct // partial
		logger:      logging.NewNopLogger(),
		booksResync: repo,
		objectStore: objectstore.NewFake(),
	}

	_, err := svc.BuildResyncProposals(
		context.Background(), logging.NewNopLogger(), nil, false,
	)
	require.NoError(t, err)

	require.Len(t, repo.scanStatusCalls, 1,
		"last_resync_at must still be bumped for unsearchable books")
	call := repo.scanStatusCalls[0]
	assert.Nil(t, call.ucFound)
	assert.Nil(t, call.hcFound)
}

func TestBuildResyncProposals_ScanStatusError_NonFatal(t *testing.T) {
	id := uuid.New()
	isbn := "9780140449112"
	repo := &fakeBooksResync{ //nolint:exhaustruct //zero values fine
		books: []models.Book{
			{ID: id, Title: "Some Book", ISBN13: &isbn}, //nolint:exhaustruct // partial
		},
		scanStatusErr: errors.New("db down"),
	}
	svc := &BookService{ //nolint:exhaustruct // partial
		logger:      logging.NewNopLogger(),
		booksResync: repo,
		objectStore: objectstore.NewFake(),
	}

	n, err := svc.BuildResyncProposals(
		context.Background(), logging.NewNopLogger(), nil, false,
	)
	require.ErrorContains(t, err, "record scan status")
	assert.Equal(t, 1, n,
		"a scan-status write failure must not abort the scan itself")
	assert.Contains(t, repo.replaced, id)
}

// ---------------------------------------------------------------------------
// Override search: manual title/author steering, guards skipped
// ---------------------------------------------------------------------------

func TestGetBookSources_Override_ForcesSearchAndSkipsGuards(t *testing.T) {
	bookID := uuid.New()
	isbn := "9780140449112"
	// Stored title is way off; the guard would reject the search result.
	book := models.Book{ //nolint:exhaustruct // partial
		ID:      bookID,
		Title:   "Completely Wrong Stored Title",
		Authors: []string{"Wrong Author"},
		ISBN13:  &isbn,
	}

	repo := &fakeBooksResync{books: []models.Book{book}} //nolint:exhaustruct // partial
	svc := &BookService{                                 //nolint:exhaustruct // partial
		booksResync: repo,
		hardcover: &fakeHCClient{ //nolint:exhaustruct // partial
			searchResults: []hardcover.ExternalBook{
				//nolint:exhaustruct // partial
				{Title: "The Real Book", Authors: []string{"Real Author"}},
			},
			//nolint:exhaustruct // partial
			byISBN: &hardcover.ExternalBook{Title: "ISBN Result"},
		},
		objectStore: objectstore.NewFake(),
	}

	proposal, err := svc.GetBookSources(
		context.Background(), logging.NewNopLogger(), bookID,
		"The Real Book", "Real Author",
	)
	require.NoError(t, err)
	require.Len(t, proposal.Sources, 1)
	assert.Equal(t, "The Real Book", proposal.Sources[0].Title,
		"an override must use the search path (top result), not the ISBN path")
}

func TestGetBookSources_OverrideAuthorOnly_UsesStoredTitle(t *testing.T) {
	bookID := uuid.New()
	book := models.Book{ //nolint:exhaustruct // partial
		ID:    bookID,
		Title: "Stored Title",
	}

	repo := &fakeBooksResync{books: []models.Book{book}} //nolint:exhaustruct // partial
	svc := &BookService{                                 //nolint:exhaustruct // partial
		booksResync: repo,
		hardcover: &fakeHCClient{ //nolint:exhaustruct // partial
			searchResults: []hardcover.ExternalBook{
				//nolint:exhaustruct // partial
				{Title: "Whatever The Provider Says", Authors: []string{"New Author"}},
			},
		},
		objectStore: objectstore.NewFake(),
	}

	proposal, err := svc.GetBookSources(
		context.Background(), logging.NewNopLogger(), bookID, "", "New Author",
	)
	require.NoError(t, err)
	require.Len(t, proposal.Sources, 1)
	assert.Equal(t, "Whatever The Provider Says", proposal.Sources[0].Title,
		"the guard must be skipped even when only the author is overridden")
}

func TestGetBookSources_Override_NoResults_EmptySources(t *testing.T) {
	bookID := uuid.New()
	book := models.Book{ID: bookID, Title: "T"} //nolint:exhaustruct // partial

	repo := &fakeBooksResync{books: []models.Book{book}} //nolint:exhaustruct // partial
	svc := &BookService{                                 //nolint:exhaustruct // partial
		booksResync: repo,
		hardcover:   &fakeHCClient{}, //nolint:exhaustruct // no results
		objectStore: objectstore.NewFake(),
	}

	proposal, err := svc.GetBookSources(
		context.Background(), logging.NewNopLogger(), bookID, "Still Nothing", "",
	)
	require.NoError(t, err)
	assert.Empty(t, proposal.Sources)
}

func TestSyncBookSource_Override_AppliesTopResult(t *testing.T) {
	bookID := uuid.New()
	book := models.Book{ //nolint:exhaustruct // partial
		ID:    bookID,
		Title: "Misspelled Titel",
	}

	repo := &fakeBooksResync{books: []models.Book{book}} //nolint:exhaustruct // partial
	svc := &BookService{                                 //nolint:exhaustruct // partial
		booksResync: repo,
		hardcover: &fakeHCClient{ //nolint:exhaustruct // partial
			searchResults: []hardcover.ExternalBook{
				//nolint:exhaustruct // partial
				{Title: "Correct Title", Authors: []string{"Author"}},
			},
		},
		objectStore: objectstore.NewFake(),
	}

	err := svc.SyncBookSource(
		context.Background(), logging.NewNopLogger(), bookID, "hardcover", 0,
		"Correct Title", "",
	)
	require.NoError(t, err)

	require.Len(t, repo.refreshCalls, 1)
	rc := repo.refreshCalls[0]
	assert.Equal(t, "Correct Title", rc.title)
	assert.Equal(t, "hardcover", rc.metadataSource)
}

func TestGetBookSources_Override_ReturnsUpToFiveCandidatesPerSource(t *testing.T) {
	bookID := uuid.New()
	book := models.Book{ID: bookID, Title: "T"} //nolint:exhaustruct // partial

	repo := &fakeBooksResync{books: []models.Book{book}} //nolint:exhaustruct // partial
	svc := &BookService{                                 //nolint:exhaustruct // partial
		booksResync: repo,
		hardcover: &fakeHCClient{ //nolint:exhaustruct // partial

			searchResults: []hardcover.ExternalBook{
				{Title: "First Match"},           //nolint:exhaustruct // partial
				{Title: "Second Match"},          //nolint:exhaustruct // partial
				{Title: "Third Match"},           //nolint:exhaustruct // partial
				{Title: "Fourth Match"},          //nolint:exhaustruct // partial
				{Title: "Fifth Match"},           //nolint:exhaustruct // partial
				{Title: "Sixth Match (dropped)"}, //nolint:exhaustruct // partial
			},
		},
		objectStore: objectstore.NewFake(),
	}

	proposal, err := svc.GetBookSources(
		context.Background(), logging.NewNopLogger(), bookID, "T", "",
	)
	require.NoError(t, err)
	require.Len(t, proposal.Sources, 5,
		"override search must cap each source at 5 candidates")
	for i, source := range proposal.Sources {
		assert.Equal(t, "hardcover", source.Source)
		assert.Equal(t, i, source.Index,
			"candidates must be numbered by their position in the provider's results")
	}
	assert.Equal(t, "First Match", proposal.Sources[0].Title)
	assert.Equal(t, "Fifth Match", proposal.Sources[4].Title)
}

func TestSyncBookSource_Override_AppliesChosenIndexNotJustFirst(t *testing.T) {
	bookID := uuid.New()
	book := models.Book{ID: bookID, Title: "T"} //nolint:exhaustruct // partial

	repo := &fakeBooksResync{books: []models.Book{book}} //nolint:exhaustruct // partial
	svc := &BookService{                                 //nolint:exhaustruct // partial
		booksResync: repo,
		hardcover: &fakeHCClient{ //nolint:exhaustruct // partial

			searchResults: []hardcover.ExternalBook{
				{Title: "First Match"},  //nolint:exhaustruct // partial
				{Title: "Second Match"}, //nolint:exhaustruct // partial
			},
		},
		objectStore: objectstore.NewFake(),
	}

	err := svc.SyncBookSource(
		context.Background(), logging.NewNopLogger(), bookID, "hardcover", 1,
		"T", "",
	)
	require.NoError(t, err)

	require.Len(t, repo.refreshCalls, 1)
	assert.Equal(t, "Second Match", repo.refreshCalls[0].title,
		"index 1 must apply the second candidate, not the first")
}

func TestSyncBookSource_Override_UnknownIndexNotFound(t *testing.T) {
	bookID := uuid.New()
	book := models.Book{ID: bookID, Title: "T"} //nolint:exhaustruct // partial

	repo := &fakeBooksResync{books: []models.Book{book}} //nolint:exhaustruct // partial
	svc := &BookService{                                 //nolint:exhaustruct // partial
		booksResync: repo,
		hardcover: &fakeHCClient{ //nolint:exhaustruct // partial
			searchResults: []hardcover.ExternalBook{
				//nolint:exhaustruct // partial
				{Title: "Only Match"},
			},
		},
		objectStore: objectstore.NewFake(),
	}

	err := svc.SyncBookSource(
		context.Background(), logging.NewNopLogger(), bookID, "hardcover", 3,
		"T", "",
	)
	require.ErrorIs(t, err, ErrProposalNotFound)
}

// TestGetBookSources_Override_Hardcover_FiltersByAuthor: Hardcover's Typesense
// query is title-only (see pkg/hardcover extractSearchTerms), so the author
// must be applied as a post-fetch filter — without it the override search
// shows same-titled books by unrelated authors (the "The Fall" / Albert Camus
// regression: UniCat filters server-side via inauthor:, Hardcover can't).
func TestGetBookSources_Override_Hardcover_FiltersByAuthor(t *testing.T) {
	bookID := uuid.New()
	book := models.Book{ID: bookID, Title: "The Fall"} //nolint:exhaustruct // partial

	repo := &fakeBooksResync{books: []models.Book{book}} //nolint:exhaustruct // partial
	svc := &BookService{                                 //nolint:exhaustruct // partial
		booksResync: repo,
		hardcover: &fakeHCClient{ //nolint:exhaustruct // partial
			searchResults: []hardcover.ExternalBook{
				//nolint:exhaustruct // partial
				{Title: "The Fall", Authors: []string{"Guillermo del Toro"}},
				//nolint:exhaustruct // partial
				{Title: "The Fall", Authors: []string{"T.J. Newman"}},
				//nolint:exhaustruct // partial
				{Title: "The Fall", Authors: []string{"Albert Camus"}},
				//nolint:exhaustruct // partial
				{Title: "The Fall of Hyperion", Authors: []string{"Dan Simmons"}},
			},
		},
		objectStore: objectstore.NewFake(),
	}

	proposal, err := svc.GetBookSources(
		context.Background(), logging.NewNopLogger(), bookID,
		"The Fall", "Albert Camus",
	)
	require.NoError(t, err)
	require.Len(t, proposal.Sources, 1,
		"only the author-matching hardcover candidate must be proposed")
	assert.Equal(t, "hardcover", proposal.Sources[0].Source)
	assert.Equal(t, []string{"Albert Camus"}, proposal.Sources[0].Authors)
}

// TestGetBookSources_Override_Hardcover_NoAuthor_Unfiltered: with no author
// anywhere (book has none, no override), there is nothing to filter on — the
// override search keeps Hardcover's relevance-ordered candidates as-is.
func TestGetBookSources_Override_Hardcover_NoAuthor_Unfiltered(t *testing.T) {
	bookID := uuid.New()
	book := models.Book{ID: bookID, Title: "The Fall"} //nolint:exhaustruct // partial

	repo := &fakeBooksResync{books: []models.Book{book}} //nolint:exhaustruct // partial
	svc := &BookService{                                 //nolint:exhaustruct // partial
		booksResync: repo,
		hardcover: &fakeHCClient{ //nolint:exhaustruct // partial
			searchResults: []hardcover.ExternalBook{
				//nolint:exhaustruct // partial
				{Title: "The Fall", Authors: []string{"Guillermo del Toro"}},
				//nolint:exhaustruct // partial
				{Title: "The Fall", Authors: []string{"Albert Camus"}},
			},
		},
		objectStore: objectstore.NewFake(),
	}

	proposal, err := svc.GetBookSources(
		context.Background(), logging.NewNopLogger(), bookID, "The Fall", "",
	)
	require.NoError(t, err)
	assert.Len(t, proposal.Sources, 2,
		"no author to filter on: all hardcover candidates must be kept")
}

// TestGetBookSources_Hardcover_WutheringHeights_Regression pins the #374
// scenario end to end: Hardcover's index ranks a critical companion whose
// *title* contains the author name above the real novel. The post-fetch
// author filter must keep the real novel (diacritic-folded "Brontë" matches
// the stored "Bronte") and drop the critic's companion — on both the guarded
// no-override path and the manual override path.
func TestGetBookSources_Hardcover_WutheringHeights_Regression(t *testing.T) {
	bookID := uuid.New()
	book := models.Book{ //nolint:exhaustruct // partial
		ID:      bookID,
		Title:   "Wuthering Heights",
		Authors: []string{"Emily Bronte"},
	}

	repo := &fakeBooksResync{books: []models.Book{book}} //nolint:exhaustruct // partial
	svc := &BookService{                                 //nolint:exhaustruct // partial
		booksResync: repo,
		hardcover: &fakeHCClient{ //nolint:exhaustruct // partial
			searchResults: []hardcover.ExternalBook{
				//nolint:exhaustruct // partial
				{
					Title:   "Emily Brontë: Wuthering Heights",
					Authors: []string{"Patsy Stoneman"},
				},
				//nolint:exhaustruct // partial
				{Title: "Wuthering Heights", Authors: []string{"Emily Brontë"}},
			},
		},
		objectStore: objectstore.NewFake(),
	}

	// Guarded path (no override).
	proposal, err := svc.GetBookSources(
		context.Background(), logging.NewNopLogger(), bookID, "", "",
	)
	require.NoError(t, err)
	require.Len(t, proposal.Sources, 1)
	assert.Equal(t, "hardcover", proposal.Sources[0].Source)
	assert.Equal(t, "Wuthering Heights", proposal.Sources[0].Title)

	// Manual override path.
	proposal, err = svc.GetBookSources(
		context.Background(), logging.NewNopLogger(), bookID,
		"Wuthering Heights", "Emily Bronte",
	)
	require.NoError(t, err)
	require.Len(t, proposal.Sources, 1,
		"the critic's companion must be filtered out by author")
	assert.Equal(t, "Wuthering Heights", proposal.Sources[0].Title)
}

// ---------------------------------------------------------------------------
// externalToBook: creation provenance
// ---------------------------------------------------------------------------

func TestExternalToBook_MetadataSourceProvenance(t *testing.T) {
	//nolint:exhaustruct // partial
	fromHC := externalToBook(SourceProposal{
		Source: "hardcover",
		Title:  "T",
	})
	require.NotNil(t, fromHC.MetadataSource)
	assert.Equal(t, "hardcover", *fromHC.MetadataSource)

	//nolint:exhaustruct // partial
	manual := externalToBook(SourceProposal{
		Source: "manual",
		Title:  "T",
	})
	assert.Nil(t, manual.MetadataSource,
		"hand-entered books must not claim source provenance")
}

// ---------------------------------------------------------------------------
// GetSourceStats: service passthrough
// ---------------------------------------------------------------------------

func TestGetSourceStats_Passthrough(t *testing.T) {
	want := &repositories.SourceStats{ //nolint:exhaustruct // partial
		TotalBooks:   10,
		UniCatFound:  7,
		UniCatUnique: 3,
		NeverScanned: 2,
	}
	repo := &fakeBooksResync{sourceStats: want} //nolint:exhaustruct // partial
	svc := &BookService{booksResync: repo}      //nolint:exhaustruct // partial

	got, err := svc.GetSourceStats(context.Background())
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestListBooksInExactSources_Passthrough(t *testing.T) {
	//nolint:exhaustruct // partial
	want := []models.Book{{Title: "Unique Book"}}
	repo := &fakeBooksResync{uniqueBooks: want} //nolint:exhaustruct // partial
	svc := &BookService{booksResync: repo}      //nolint:exhaustruct // partial

	got, err := svc.ListBooksInExactSources(context.Background(), []string{"unicat"})
	require.NoError(t, err)
	assert.Equal(t, want, got)
}
