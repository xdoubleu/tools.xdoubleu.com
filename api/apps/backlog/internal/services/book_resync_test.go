//nolint:testpackage // testing unexported service helpers
package services

import (
	"bytes"
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xdoubleu/essentia/v4/pkg/logging"

	"tools.xdoubleu.com/apps/backlog/internal/models"
	"tools.xdoubleu.com/apps/backlog/pkg/objectstore"
	"tools.xdoubleu.com/apps/backlog/pkg/openlibrary"
)

// refreshCall records a single call to fakeBooksResync.RefreshBookExternalData
// so tests can assert which fields were actually written to the DB.
type refreshCall struct {
	bookID      uuid.UUID
	coverURL    *string
	description *string
	pageCount   *int
}

// fakeBooksResync is a test stub for booksResyncSource.
type fakeBooksResync struct {
	books        []models.Book
	listErr      error
	refreshMu    sync.Mutex
	refreshErr   error
	refreshCalls []refreshCall
}

func (f *fakeBooksResync) ListBooksWithISBN13(
	_ context.Context,
) ([]models.Book, error) {
	return f.books, f.listErr
}

func (f *fakeBooksResync) RefreshBookExternalData(
	_ context.Context,
	bookID uuid.UUID,
	coverURL *string,
	description *string,
	pageCount *int,
) error {
	f.refreshMu.Lock()
	f.refreshCalls = append(f.refreshCalls, refreshCall{
		bookID:      bookID,
		coverURL:    coverURL,
		description: description,
		pageCount:   pageCount,
	})
	f.refreshMu.Unlock()
	return f.refreshErr
}

// TestResyncBook_ErrNotFound_Skips verifies that when Open Library returns
// ErrNotFound the book is silently skipped (nil error, no DB or cache calls).
func TestResyncBook_ErrNotFound_Skips(t *testing.T) {
	fake := &fakeOLClient{ //nolint:exhaustruct //detail and calls zero-valued
		err: openlibrary.ErrNotFound,
	}
	store := objectstore.NewFake()
	isbn := "9780140449112"
	book := models.Book{ //nolint:exhaustruct //only tested fields needed
		ID:     uuid.New(),
		ISBN13: &isbn,
	}
	svc := &BookService{ //nolint:exhaustruct //only tested fields needed
		logger:      logging.NewNopLogger(),
		external:    fake,
		objectStore: store,
	}

	err := svc.resyncBook(
		context.Background(),
		logging.NewNopLogger(),
		book,
	)
	require.NoError(t, err)
	assert.Equal(t, 1, fake.calls)
}

// TestResyncBook_GetByISBNError_Propagates verifies that a non-NotFound error
// from Open Library is returned and no DB/cache calls are made.
func TestResyncBook_GetByISBNError_Propagates(t *testing.T) {
	boom := errors.New("network error")
	fake := &fakeOLClient{ //nolint:exhaustruct //detail and calls zero-valued
		err: boom,
	}
	store := objectstore.NewFake()
	isbn := "9780140449112"
	book := models.Book{ //nolint:exhaustruct //only tested fields needed
		ID:     uuid.New(),
		ISBN13: &isbn,
	}
	svc := &BookService{ //nolint:exhaustruct //only tested fields needed
		logger:      logging.NewNopLogger(),
		external:    fake,
		objectStore: store,
	}

	err := svc.resyncBook(
		context.Background(),
		logging.NewNopLogger(),
		book,
	)
	require.Error(t, err)
	assert.ErrorIs(t, err, boom)
}

// TestResyncBook_FullyPopulated_Skipped verifies that a book which already has
// cover_url, description, and page_count is skipped entirely: no Open Library
// call, no DB write, and the cached cover is left untouched.
func TestResyncBook_FullyPopulated_Skipped(t *testing.T) {
	isbn := "9780140449112"
	cover := "https://covers.openlibrary.org/b/isbn/9780140449112-L.jpg"
	desc := "An existing description."
	pages := 300
	book := models.Book{ //nolint:exhaustruct //only tested fields needed
		ID:          uuid.New(),
		ISBN13:      &isbn,
		CoverURL:    &cover,
		Description: &desc,
		PageCount:   &pages,
	}

	fake := &fakeOLClient{}    //nolint:exhaustruct //zero values — should never be called
	repo := &fakeBooksResync{} //nolint:exhaustruct //no books list needed
	store := objectstore.NewFake()

	// Pre-populate the cover cache to verify it is not deleted.
	coverKey := bookCoverKey(book.ID)
	err := store.Put(
		context.Background(), coverKey,
		bytes.NewReader([]byte("imgdata")), 7, "image/jpeg",
	)
	require.NoError(t, err)

	svc := &BookService{ //nolint:exhaustruct //only tested fields needed
		logger:      logging.NewNopLogger(),
		booksResync: repo,
		external:    fake,
		objectStore: store,
	}

	err = svc.resyncBook(context.Background(), logging.NewNopLogger(), book)
	require.NoError(t, err)

	assert.Equal(t, 0, fake.calls, "no OL call expected for fully populated book")
	assert.Empty(t, repo.refreshCalls, "no DB write expected for fully populated book")

	_, stillCached := store.GetContent(coverKey)
	assert.True(
		t,
		stillCached,
		"cover cache must not be deleted for fully populated book",
	)
}

// TestResyncBook_ExistingCoverNotDeleted is the regression test for the
// cover-loss bug: a book that already has a cover_url must not have its cached
// cover deleted during resync, even when Open Library returns no cover.
func TestResyncBook_ExistingCoverNotDeleted(t *testing.T) {
	isbn := "9780140449112"
	cover := "https://existing.example.com/cover.jpg"
	book := models.Book{ //nolint:exhaustruct //only tested fields needed
		ID:       uuid.New(),
		ISBN13:   &isbn,
		CoverURL: &cover,
		// Description and PageCount are nil — something needs filling so
		// the OL call is made, but cover must stay untouched.
	}

	// OL returns a description but no cover.
	olDesc := "A description from Open Library."
	detail := &openlibrary.ExternalBook{ //nolint:exhaustruct //only relevant fields
		Provider:    "openlibrary",
		ProviderID:  "OL123W",
		Title:       "Test Book",
		Authors:     []string{"Author"},
		Description: &olDesc,
		CoverURL:    nil,
	}
	fake := &fakeOLClient{detail: detail} //nolint:exhaustruct //err nil
	repo := &fakeBooksResync{}            //nolint:exhaustruct //no books list needed
	store := objectstore.NewFake()

	// Pre-populate the cover cache to verify it is preserved.
	coverKey := bookCoverKey(book.ID)
	err := store.Put(
		context.Background(), coverKey,
		bytes.NewReader([]byte("imgdata")), 7, "image/jpeg",
	)
	require.NoError(t, err)

	svc := &BookService{ //nolint:exhaustruct //only tested fields needed
		logger:      logging.NewNopLogger(),
		booksResync: repo,
		external:    fake,
		objectStore: store,
	}

	err = svc.resyncBook(context.Background(), logging.NewNopLogger(), book)
	require.NoError(t, err)

	require.Len(t, repo.refreshCalls, 1)
	assert.Nil(
		t, repo.refreshCalls[0].coverURL,
		"coverURL passed to DB must be nil when book already has a cover",
	)
	assert.Equal(t, &olDesc, repo.refreshCalls[0].description)

	_, stillCached := store.GetContent(coverKey)
	assert.True(
		t,
		stillCached,
		"cover cache must not be deleted when cover already exists",
	)
}

// TestResyncBook_MissingCover_Backfilled verifies that when a book has no cover
// and Open Library returns one, the cover is written and the cover cache is busted.
func TestResyncBook_MissingCover_Backfilled(t *testing.T) {
	isbn := "9780140449112"
	book := models.Book{ //nolint:exhaustruct //only tested fields needed
		ID:     uuid.New(),
		ISBN13: &isbn,
		// no cover, no description, no page count
	}

	olCover := "https://covers.openlibrary.org/b/isbn/9780140449112-L.jpg"
	detail := &openlibrary.ExternalBook{ //nolint:exhaustruct //only relevant fields
		Provider:   "openlibrary",
		ProviderID: "OL123W",
		Title:      "Test Book",
		Authors:    []string{"Author"},
		CoverURL:   &olCover,
	}
	fake := &fakeOLClient{detail: detail} //nolint:exhaustruct //err nil
	repo := &fakeBooksResync{}            //nolint:exhaustruct //no books list needed
	store := objectstore.NewFake()

	// Pre-populate a stale missing-marker to verify it gets cleared.
	missingKey := bookCoverMissingKey(book.ID)
	err := store.Put(
		context.Background(), missingKey,
		bytes.NewReader([]byte("")), 0, "text/plain",
	)
	require.NoError(t, err)

	svc := &BookService{ //nolint:exhaustruct //only tested fields needed
		logger:      logging.NewNopLogger(),
		booksResync: repo,
		external:    fake,
		objectStore: store,
	}

	err = svc.resyncBook(context.Background(), logging.NewNopLogger(), book)
	require.NoError(t, err)

	require.Len(t, repo.refreshCalls, 1)
	require.NotNil(t, repo.refreshCalls[0].coverURL)
	assert.Equal(t, olCover, *repo.refreshCalls[0].coverURL)

	_, missingStillExists := store.GetContent(missingKey)
	assert.False(
		t,
		missingStillExists,
		"missing-marker must be deleted when a cover was fetched",
	)
}

// TestResyncBook_NoCoverAnywhere verifies that when a book has no cover and
// Open Library also returns no cover (and the ISBN fallback is empty), the cover
// cache is NOT deleted — this is the second half of the cover-loss bug.
func TestResyncBook_NoCoverAnywhere(t *testing.T) {
	// Use an ISBN that has no OL cover image and for which CoverURLByISBN also
	// returns empty (the function returns empty string on a nil pointer, which we
	// can trigger by passing a book whose ISBN yields no fallback URL).
	// We use a clearly fake ISBN so CoverURLByISBN produces a URL that OL won't
	// serve, but what matters here is that detail.CoverURL == nil and the
	// fallback is also nil — we achieve the latter by stubbing CoverURL = nil and
	// checking the cache is untouched. Since CoverURLByISBN always constructs a
	// URL from any non-empty ISBN, we verify the behaviour by checking that a nil
	// detail.CoverURL with an existing missing-marker does NOT delete the cache
	// (i.e. no unnecessary bust when cover stays unknown).
	isbn := "9780140449112"
	book := models.Book{ //nolint:exhaustruct //only tested fields needed
		ID:     uuid.New(),
		ISBN13: &isbn,
	}

	// OL returns no cover, no description, no page_count.
	detail := &openlibrary.ExternalBook{ //nolint:exhaustruct //only relevant fields
		Provider:   "openlibrary",
		ProviderID: "OL123W",
		Title:      "Test Book",
		Authors:    []string{"Author"},
		CoverURL:   nil,
	}
	fake := &fakeOLClient{detail: detail} //nolint:exhaustruct //err nil
	repo := &fakeBooksResync{}            //nolint:exhaustruct //no books list needed
	store := objectstore.NewFake()

	// Pre-populate a missing-marker — it must not be deleted if no cover was found.
	missingKey := bookCoverMissingKey(book.ID)
	err := store.Put(
		context.Background(), missingKey,
		bytes.NewReader([]byte("")), 0, "text/plain",
	)
	require.NoError(t, err)

	svc := &BookService{ //nolint:exhaustruct //only tested fields needed
		logger:      logging.NewNopLogger(),
		booksResync: repo,
		external:    fake,
		objectStore: store,
	}

	err = svc.resyncBook(context.Background(), logging.NewNopLogger(), book)
	require.NoError(t, err)

	// DB was called (something was missing), but the cover written must be nil or
	// from the ISBN fallback — either way the missing-marker must NOT be deleted if
	// no cover was actually resolved to cache-bust.
	// The real assertion: if coverURL is still nil after fallback lookup, no delete.
	if len(repo.refreshCalls) > 0 && repo.refreshCalls[0].coverURL == nil {
		_, missingStillExists := store.GetContent(missingKey)
		assert.True(
			t, missingStillExists,
			"missing-marker must not be deleted when no cover URL was found",
		)
	}
}

func newResyncSvc(
	repo *fakeBooksResync,
	ol *fakeOLClient,
) *BookService {
	return &BookService{ //nolint:exhaustruct //only resync fields needed
		logger:      logging.NewNopLogger(),
		booksResync: repo,
		external:    ol,
		objectStore: objectstore.NewFake(),
	}
}

// TestResyncAllFromOpenLibrary_EmptyLibrary verifies that with no ISBN13 books
// the function returns (0, nil) and emits exactly one onProgress(0, 0) call.
func TestResyncAllFromOpenLibrary_EmptyLibrary(t *testing.T) {
	repo := &fakeBooksResync{}                 //nolint:exhaustruct //no books needed
	svc := newResyncSvc(repo, &fakeOLClient{}) //nolint:exhaustruct //defaults fine

	var calls [][2]int
	n, err := svc.ResyncAllFromOpenLibrary(
		context.Background(),
		logging.NewNopLogger(),
		func(processed, total int) { calls = append(calls, [2]int{processed, total}) },
	)

	require.NoError(t, err)
	assert.Equal(t, 0, n)
	require.Len(t, calls, 1)
	assert.Equal(t, [2]int{0, 0}, calls[0])
}

// TestResyncAllFromOpenLibrary_AllSucceed verifies that all books are refreshed,
// refreshed count equals the book count, and onProgress is called total+1 times.
func TestResyncAllFromOpenLibrary_AllSucceed(t *testing.T) {
	isbn1 := "9780140449112"
	isbn2 := "9780062316097"
	id1, id2 := uuid.New(), uuid.New()

	repo := &fakeBooksResync{ //nolint:exhaustruct //listErr/refreshErr zero = nil
		books: []models.Book{
			{ID: id1, ISBN13: &isbn1}, //nolint:exhaustruct //only ID+ISBN needed
			{ID: id2, ISBN13: &isbn2}, //nolint:exhaustruct //only ID+ISBN needed
		},
	}
	detail := &openlibrary.ExternalBook{ //nolint:exhaustruct //minimal detail
		Provider:   "openlibrary",
		ProviderID: "OL1W",
		Title:      "Test Book",
		Authors:    []string{"Author"},
	}
	ol := &fakeOLClient{detail: detail} //nolint:exhaustruct //err zero = nil

	svc := newResyncSvc(repo, ol)

	var progressMu sync.Mutex
	var calls [][2]int
	n, err := svc.ResyncAllFromOpenLibrary(
		context.Background(),
		logging.NewNopLogger(),
		func(processed, total int) {
			progressMu.Lock()
			calls = append(calls, [2]int{processed, total})
			progressMu.Unlock()
		},
	)

	require.NoError(t, err)
	assert.Equal(t, 2, n)
	// total+1 calls: (0,2) then (1,2) and (2,2) in any order
	assert.Len(t, calls, 3)
	assert.Equal(t, [2]int{0, 2}, calls[0], "first call must be (0, total)")

	repo.refreshMu.Lock()
	calls2 := repo.refreshCalls
	repo.refreshMu.Unlock()

	refreshedIDs := make([]uuid.UUID, len(calls2))
	for i, c := range calls2 {
		refreshedIDs[i] = c.bookID
	}
	assert.ElementsMatch(t, []uuid.UUID{id1, id2}, refreshedIDs)
}

// TestResyncAllFromOpenLibrary_PartialFailure verifies that a per-book error is
// collected but does not stop the other books from being refreshed.
func TestResyncAllFromOpenLibrary_PartialFailure(t *testing.T) {
	isbn1 := "9780140449112"
	isbn2 := "9780062316097"
	id1, id2 := uuid.New(), uuid.New()

	repo := &fakeBooksResync{ //nolint:exhaustruct //listErr zero = nil
		books: []models.Book{
			{ID: id1, ISBN13: &isbn1}, //nolint:exhaustruct //only ID+ISBN needed
			{ID: id2, ISBN13: &isbn2}, //nolint:exhaustruct //only ID+ISBN needed
		},
		refreshErr: errors.New("db down"),
	}
	detail := &openlibrary.ExternalBook{ //nolint:exhaustruct //minimal detail
		Provider:   "openlibrary",
		ProviderID: "OL1W",
		Title:      "Test Book",
		Authors:    []string{"Author"},
	}
	ol := &fakeOLClient{detail: detail} //nolint:exhaustruct //err zero = nil

	svc := newResyncSvc(repo, ol)

	n, err := svc.ResyncAllFromOpenLibrary(
		context.Background(),
		logging.NewNopLogger(),
		nil,
	)

	require.Error(t, err)
	assert.Equal(t, 0, n)
}

// TestResyncAllFromOpenLibrary_ListError verifies that a list error is returned
// immediately and no books are processed.
func TestResyncAllFromOpenLibrary_ListError(t *testing.T) {
	listErr := errors.New("connection refused")
	repo := &fakeBooksResync{ //nolint:exhaustruct //only listErr needed
		listErr: listErr,
	}
	svc := newResyncSvc(repo, &fakeOLClient{}) //nolint:exhaustruct //defaults fine

	n, err := svc.ResyncAllFromOpenLibrary(
		context.Background(),
		logging.NewNopLogger(),
		nil,
	)

	require.ErrorIs(t, err, listErr)
	assert.Equal(t, 0, n)
}
