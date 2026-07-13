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

	"tools.xdoubleu.com/apps/books/internal/models"
	"tools.xdoubleu.com/apps/books/internal/repositories"
	"tools.xdoubleu.com/apps/books/pkg/googlebooks"
	"tools.xdoubleu.com/apps/books/pkg/hardcover"
	"tools.xdoubleu.com/apps/books/pkg/objectstore"
	"tools.xdoubleu.com/apps/books/pkg/openlibrary"
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
			{ID: id, Title: "Found In OL Only", ISBN13: &isbn},
		},
	}
	svc := &BookService{ //nolint:exhaustruct // partial
		logger:      logging.NewNopLogger(),
		booksResync: repo,
		external: &multiISBNOLClient{results: map[string]*openlibrary.ExternalBook{
			isbn: {Title: "Found In OL Only"},
		}},
		//nolint:exhaustruct // partial
		googleBooks: &fakeGBClient{err: googlebooks.ErrNotFound},
		objectStore: objectstore.NewFake(),
	}

	_, err := svc.BuildResyncProposals(
		context.Background(), logging.NewNopLogger(), nil, false,
	)
	require.NoError(t, err)

	require.Len(t, repo.scanStatusCalls, 1)
	call := repo.scanStatusCalls[0]
	assert.Equal(t, id, call.bookID)
	require.NotNil(t, call.olFound)
	assert.True(t, *call.olFound)
	require.NotNil(t, call.gbFound)
	assert.False(t, *call.gbFound)
	assert.Nil(t, call.ucFound, "unconfigured provider must record NULL")
	assert.Nil(t, call.hcFound, "unconfigured provider must record NULL")
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
		external: &fakeOLClient{ //nolint:exhaustruct // partial
			err: openlibrary.ErrNotFound,
		},
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
		//nolint:exhaustruct // partial
		external:    &fakeOLClient{err: openlibrary.ErrNotFound},
		objectStore: objectstore.NewFake(),
	}

	_, err := svc.BuildResyncProposals(
		context.Background(), logging.NewNopLogger(), nil, false,
	)
	require.NoError(t, err)

	require.Len(t, repo.scanStatusCalls, 1,
		"last_resync_at must still be bumped for unsearchable books")
	call := repo.scanStatusCalls[0]
	assert.Nil(t, call.olFound)
	assert.Nil(t, call.gbFound)
	assert.Nil(t, call.ucFound)
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
		//nolint:exhaustruct // partial
		external:    &fakeOLClient{err: openlibrary.ErrNotFound},
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
		external: &fakeOLClientWithSearch{ //nolint:exhaustruct // partial
			searchResults: []openlibrary.ExternalBook{
				//nolint:exhaustruct // partial
				{Title: "The Real Book", Authors: []string{"Real Author"}},
			},
			//nolint:exhaustruct // partial
			detail: &openlibrary.ExternalBook{Title: "ISBN Result"},
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
		external: &fakeOLClientWithSearch{ //nolint:exhaustruct // partial
			searchResults: []openlibrary.ExternalBook{
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
		external:    &fakeOLClientWithSearch{}, //nolint:exhaustruct // no results
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
		external: &fakeOLClientWithSearch{ //nolint:exhaustruct // partial
			searchResults: []openlibrary.ExternalBook{
				//nolint:exhaustruct // partial
				{Title: "Correct Title", Authors: []string{"Author"}},
			},
		},
		objectStore: objectstore.NewFake(),
	}

	err := svc.SyncBookSource(
		context.Background(), logging.NewNopLogger(), bookID, "openlibrary",
		"Correct Title", "",
	)
	require.NoError(t, err)

	require.Len(t, repo.refreshCalls, 1)
	rc := repo.refreshCalls[0]
	require.NotNil(t, rc.title)
	assert.Equal(t, "Correct Title", *rc.title)
	assert.Equal(t, "openlibrary", rc.metadataSource)
}

// ---------------------------------------------------------------------------
// externalToBook: creation provenance
// ---------------------------------------------------------------------------

func TestExternalToBook_MetadataSourceProvenance(t *testing.T) {
	//nolint:exhaustruct // partial
	fromOL := externalToBook(openlibrary.ExternalBook{
		Provider: "openlibrary",
		Title:    "T",
	})
	require.NotNil(t, fromOL.MetadataSource)
	assert.Equal(t, "openlibrary", *fromOL.MetadataSource)

	//nolint:exhaustruct // partial
	manual := externalToBook(openlibrary.ExternalBook{
		Provider: "manual",
		Title:    "T",
	})
	assert.Nil(t, manual.MetadataSource,
		"hand-entered books must not claim source provenance")
}

// ---------------------------------------------------------------------------
// GetSourceStats: service passthrough
// ---------------------------------------------------------------------------

func TestGetSourceStats_Passthrough(t *testing.T) {
	want := &repositories.SourceStats{ //nolint:exhaustruct // partial
		TotalBooks:        10,
		OpenLibraryFound:  7,
		OpenLibraryUnique: 3,
		NeverScanned:      2,
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
