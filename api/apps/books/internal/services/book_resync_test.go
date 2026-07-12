//nolint:testpackage // testing unexported service helpers
package services

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xdoubleu/essentia/v4/pkg/database"
	"github.com/xdoubleu/essentia/v4/pkg/logging"

	"tools.xdoubleu.com/apps/books/internal/models"
	"tools.xdoubleu.com/apps/books/internal/repositories"
	"tools.xdoubleu.com/apps/books/pkg/googlebooks"
	"tools.xdoubleu.com/apps/books/pkg/objectstore"
	"tools.xdoubleu.com/apps/books/pkg/openlibrary"
	"tools.xdoubleu.com/apps/books/pkg/unicat"
)

// refreshCall records a single call to fakeBooksResync.RefreshBookExternalData
// so tests can assert which fields were actually written to the DB.
type refreshCall struct {
	bookID         uuid.UUID
	coverURL       *string
	description    *string
	pageCount      *int
	isbn13         *string
	title          *string
	authors        []string
	metadataSource string
}

// scanStatusCall records a single call to
// fakeBooksResync.UpdateResyncScanStatus.
type scanStatusCall struct {
	bookID  uuid.UUID
	olFound *bool
	gbFound *bool
	ucFound *bool
}

// fakeBooksResync is a test stub for booksResyncSource.
type fakeBooksResync struct {
	books      []models.Book
	listErr    error
	getBookErr error

	mu           sync.Mutex
	replaced     map[uuid.UUID][]byte
	replaceErr   error
	proposalRows map[uuid.UUID]repositories.ResyncProposalRow
	deletedIDs   []uuid.UUID
	refreshCalls []refreshCall
	refreshErr   error

	scanStatusCalls []scanStatusCall
	scanStatusErr   error

	sourceStats    *repositories.SourceStats
	sourceStatsErr error

	uniqueBooks    []models.Book
	uniqueBooksErr error
}

func (f *fakeBooksResync) ListCatalogBooks(_ context.Context) ([]models.Book, error) {
	return f.books, f.listErr
}

func (f *fakeBooksResync) GetBookByID(
	_ context.Context,
	bookID uuid.UUID,
) (*models.Book, error) {
	if f.getBookErr != nil {
		return nil, f.getBookErr
	}
	for i := range f.books {
		if f.books[i].ID == bookID {
			return &f.books[i], nil
		}
	}
	return nil, database.ErrResourceNotFound
}

func (f *fakeBooksResync) RefreshBookExternalData(
	_ context.Context,
	bookID uuid.UUID,
	coverURL *string,
	description *string,
	pageCount *int,
	isbn13 *string,
	title *string,
	authors []string,
	metadataSource string,
) error {
	f.mu.Lock()
	f.refreshCalls = append(f.refreshCalls, refreshCall{
		bookID: bookID, coverURL: coverURL, description: description,
		pageCount: pageCount, isbn13: isbn13, title: title, authors: authors,
		metadataSource: metadataSource,
	})
	f.mu.Unlock()
	return f.refreshErr
}

func (f *fakeBooksResync) UpdateResyncScanStatus(
	_ context.Context,
	bookID uuid.UUID,
	olFound *bool,
	gbFound *bool,
	ucFound *bool,
) error {
	f.mu.Lock()
	f.scanStatusCalls = append(f.scanStatusCalls, scanStatusCall{
		bookID: bookID, olFound: olFound, gbFound: gbFound, ucFound: ucFound,
	})
	f.mu.Unlock()
	return f.scanStatusErr
}

func (f *fakeBooksResync) GetSourceStats(
	_ context.Context,
) (*repositories.SourceStats, error) {
	return f.sourceStats, f.sourceStatsErr
}

func (f *fakeBooksResync) ListBooksInExactSources(
	_ context.Context,
	_ []string,
) ([]models.Book, error) {
	return f.uniqueBooks, f.uniqueBooksErr
}

func (f *fakeBooksResync) ReplaceResyncProposals(
	_ context.Context,
	entries map[uuid.UUID][]byte,
) error {
	f.mu.Lock()
	f.replaced = entries
	f.mu.Unlock()
	return f.replaceErr
}

func (f *fakeBooksResync) ListResyncProposals(
	_ context.Context,
) ([]repositories.ResyncProposalRow, error) {
	out := make([]repositories.ResyncProposalRow, 0, len(f.proposalRows))
	for _, row := range f.proposalRows {
		out = append(out, row)
	}
	return out, nil
}

func (f *fakeBooksResync) GetResyncProposal(
	_ context.Context,
	bookID uuid.UUID,
) (*repositories.ResyncProposalRow, error) {
	row, ok := f.proposalRows[bookID]
	if !ok {
		return nil, database.ErrResourceNotFound
	}
	return &row, nil
}

func (f *fakeBooksResync) DeleteResyncProposal(
	_ context.Context,
	bookID uuid.UUID,
) error {
	f.mu.Lock()
	f.deletedIDs = append(f.deletedIDs, bookID)
	f.mu.Unlock()
	return nil
}

// fakeGBClient is a configurable googlebooks.Client stub. calls counts every
// GetByISBN/Search invocation, so tests can assert a call was skipped.
type fakeGBClient struct {
	searchResults []googlebooks.ExternalBook
	byISBN        *googlebooks.ExternalBook
	err           error

	calls atomic.Int32
}

func (f *fakeGBClient) Search(
	_ context.Context,
	_ string,
) ([]googlebooks.ExternalBook, error) {
	f.calls.Add(1)
	return f.searchResults, f.err
}

func (f *fakeGBClient) GetByISBN(
	_ context.Context,
	_ string,
) (*googlebooks.ExternalBook, error) {
	f.calls.Add(1)
	if f.err != nil {
		return nil, f.err
	}
	if f.byISBN == nil {
		return nil, googlebooks.ErrNotFound
	}
	return f.byISBN, nil
}

// fakeUCClient is a configurable unicat.Client stub.
type fakeUCClient struct {
	searchResults []unicat.ExternalBook
	byISBN        *unicat.ExternalBook
	err           error
}

func (f *fakeUCClient) GetByISBN(
	_ context.Context,
	_ string,
) (*unicat.ExternalBook, error) {
	if f.err != nil {
		return nil, f.err
	}
	if f.byISBN == nil {
		return nil, unicat.ErrNotFound
	}
	return f.byISBN, nil
}

func (f *fakeUCClient) Search(
	_ context.Context,
	_ string,
) ([]unicat.ExternalBook, error) {
	return f.searchResults, f.err
}

// fakeOLClientWithSearch wraps fakeOLClient but supports returning
// configurable Search results so we can exercise the search-match path.
type fakeOLClientWithSearch struct {
	searchResults []openlibrary.ExternalBook
	searchErr     error
	detail        *openlibrary.ExternalBook
	getErr        error
}

func (f *fakeOLClientWithSearch) Search(
	_ context.Context,
	_ string,
) ([]openlibrary.ExternalBook, error) {
	return f.searchResults, f.searchErr
}

func (f *fakeOLClientWithSearch) Get(
	_ context.Context,
	_ string,
) (*openlibrary.ExternalBook, error) {
	return f.detail, f.getErr
}

func (f *fakeOLClientWithSearch) GetByISBN(
	_ context.Context,
	_ string,
) (*openlibrary.ExternalBook, error) {
	return f.detail, f.getErr
}

func (f *fakeOLClientWithSearch) FetchCover(
	_ context.Context,
	_ string,
) ([]byte, string, error) {
	return nil, "", errors.New("not implemented")
}

// multiISBNOLClient returns a canned result per ISBN, used to give two books
// in the same test different provider responses.
type multiISBNOLClient struct {
	results map[string]*openlibrary.ExternalBook
}

func (f *multiISBNOLClient) Search(
	_ context.Context, _ string,
) ([]openlibrary.ExternalBook, error) {
	return nil, nil
}

func (f *multiISBNOLClient) Get(
	_ context.Context, id string,
) (*openlibrary.ExternalBook, error) {
	r, ok := f.results[id]
	if !ok {
		return nil, openlibrary.ErrNotFound
	}
	return r, nil
}

func (f *multiISBNOLClient) GetByISBN(
	_ context.Context, isbn string,
) (*openlibrary.ExternalBook, error) {
	r, ok := f.results[isbn]
	if !ok {
		return nil, openlibrary.ErrNotFound
	}
	return r, nil
}

func (f *multiISBNOLClient) FetchCover(
	_ context.Context, _ string,
) ([]byte, string, error) {
	return nil, "", errors.New("not implemented")
}

// failDeleteObjectStore is an objectstore.Client that always errors on Delete.
// All other methods delegate to a real FakeClient so Put/Get still work.
type failDeleteObjectStore struct {
	inner *objectstore.FakeClient
}

func (s failDeleteObjectStore) Put(
	ctx context.Context,
	key string,
	r io.Reader,
	size int64,
	contentType string,
) error {
	return s.inner.Put(ctx, key, r, size, contentType)
}

func (s failDeleteObjectStore) Get(
	ctx context.Context,
	key string,
) (io.ReadCloser, error) {
	return s.inner.Get(ctx, key)
}

func (s failDeleteObjectStore) PresignGet(
	ctx context.Context,
	key string,
	ttl time.Duration,
) (string, error) {
	return s.inner.PresignGet(ctx, key, ttl)
}

func (s failDeleteObjectStore) PresignPut(
	ctx context.Context,
	key string,
	ttl time.Duration,
	contentType string,
) (string, error) {
	return s.inner.PresignPut(ctx, key, ttl, contentType)
}

func (s failDeleteObjectStore) Delete(_ context.Context, _ string) error {
	return errors.New("delete failed")
}

func (s failDeleteObjectStore) Exists(
	ctx context.Context,
	key string,
) (bool, error) {
	return s.inner.Exists(ctx, key)
}

func (s failDeleteObjectStore) Copy(
	ctx context.Context,
	srcKey, dstKey string,
) error {
	return s.inner.Copy(ctx, srcKey, dstKey)
}

func (s failDeleteObjectStore) List(
	ctx context.Context,
	prefix string,
) ([]objectstore.ObjectInfo, error) {
	return s.inner.List(ctx, prefix)
}

// ---------------------------------------------------------------------------
// fetchByISBN: sources are kept independent, no gap-filling merge
// ---------------------------------------------------------------------------

func TestFetchByISBN_KeepsSourcesIndependent(t *testing.T) {
	isbn := "9780140449112"
	//nolint:exhaustruct // partial
	olDetail := &openlibrary.ExternalBook{
		Title:   "OL Title",
		Authors: []string{"OL Author"},
	}
	gbCover := "https://books.google.com/cover.jpg"
	gbDetail := &googlebooks.ExternalBook{ //nolint:exhaustruct // partial
		Title: "GB Title", CoverURL: &gbCover,
	}
	ucDetail := &unicat.ExternalBook{Title: "UC Title"} //nolint:exhaustruct // partial

	svc := &BookService{ //nolint:exhaustruct //only resync-path fields needed
		logger:      logging.NewNopLogger(),
		external:    &fakeOLClient{detail: olDetail}, //nolint:exhaustruct // partial
		googleBooks: &fakeGBClient{byISBN: gbDetail}, //nolint:exhaustruct // partial
		uniCat:      &fakeUCClient{byISBN: ucDetail}, //nolint:exhaustruct // partial
		objectStore: objectstore.NewFake(),
	}

	proposals, unresolved := svc.fetchByISBN(
		context.Background(), logging.NewNopLogger(), isbn, nil,
	)

	require.Len(t, proposals, 3, "all three providers returned a record; no merge")
	assert.Empty(t, unresolved, "every provider answered cleanly")
	assert.Equal(t, "openlibrary", proposals[0].Source)
	assert.Equal(t, "OL Title", proposals[0].Title)
	assert.Equal(t, "googlebooks", proposals[1].Source)
	assert.Equal(t, "GB Title", proposals[1].Title)
	assert.Equal(t, gbCover, proposals[1].CoverURL)
	assert.Equal(t, "unicat", proposals[2].Source)
	assert.Equal(t, "UC Title", proposals[2].Title)
}

func TestFetchByISBN_NotFound_Skipped(t *testing.T) {
	svc := &BookService{ //nolint:exhaustruct // partial
		logger: logging.NewNopLogger(),
		//nolint:exhaustruct // detail unused, err drives the not-found path
		external:    &fakeOLClient{err: openlibrary.ErrNotFound},
		objectStore: objectstore.NewFake(),
	}

	proposals, unresolved := svc.fetchByISBN(
		context.Background(),
		logging.NewNopLogger(),
		"9780140449112",
		nil,
	)
	assert.Empty(t, proposals)
	assert.Empty(t, unresolved, "a clean not-found is resolved, not unresolved")
}

// ---------------------------------------------------------------------------
// fetchByISBN: errored/skipped Google Books must be unresolved, not "false"
// (regression — a source that errors or is skipped must leave the DB flag
// untouched via recordScanStatus/UpdateResyncScanStatus's COALESCE-preserve,
// never overwrite a previously-known true with false).
// ---------------------------------------------------------------------------

// gbSvc builds a BookService with the given OL/GB fakes, wired for the
// fetchByISBN/fetchBySearch GB skip/breaker regression tests below.
func gbSvc(external openlibrary.Client, googleBooks *fakeGBClient) *BookService {
	//nolint:exhaustruct // test helper builds a partial service
	return &BookService{
		logger:      logging.NewNopLogger(),
		external:    external,
		googleBooks: googleBooks,
		objectStore: objectstore.NewFake(),
	}
}

func TestFetchByISBN_GoogleBooksErrors_MarkedUnresolved(t *testing.T) {
	//nolint:exhaustruct // partial
	olClient := &fakeOLClient{detail: &openlibrary.ExternalBook{Title: "OL Title"}}
	//nolint:exhaustruct // partial
	gb := &fakeGBClient{err: errors.New("boom")}
	svc := gbSvc(olClient, gb)

	proposals, unresolved := svc.fetchByISBN(
		context.Background(), logging.NewNopLogger(), "9780140449112", nil,
	)
	require.Len(t, proposals, 1, "OL still succeeds independently of GB's error")
	assert.True(t, unresolved["googlebooks"],
		"an errored source must be unresolved, not a false miss")
}

func TestFetchByISBN_GoogleBooksKnown_SkippedAndUnresolved(t *testing.T) {
	//nolint:exhaustruct // partial
	gb := &fakeGBClient{byISBN: &googlebooks.ExternalBook{Title: "GB Title"}}
	//nolint:exhaustruct // partial
	olClient := &fakeOLClient{err: openlibrary.ErrNotFound}
	svc := gbSvc(olClient, gb)
	opts := &scanOptions{
		known:      map[string]bool{"googlebooks": true},
		gbExceeded: &atomic.Bool{},
	}

	proposals, unresolved := svc.fetchByISBN(
		context.Background(), logging.NewNopLogger(), "9780140449112", opts,
	)
	assert.Empty(t, proposals)
	assert.True(t, unresolved["googlebooks"], "a skipped source must be unresolved")
	assert.Zero(t, gb.calls.Load(), "an already-known source must not be re-queried")
}

// TestBuildResyncProposals_ForceGoogleBooks_BypassesCache is a regression test
// for the bug where Google Books stopped being queried at all: once
// googlebooks_found is set (true or false) for every book — which happens
// after the very first scan — gbKnown is always non-nil and every later scan
// skips GB catalog-wide. forceGoogleBooks must bypass that cache so a stuck
// book (e.g. left "false" by a tripped rate-limit breaker) can be re-queried
// and pick up a fresh match.
func TestBuildResyncProposals_ForceGoogleBooks_BypassesCache(t *testing.T) {
	id := uuid.New()
	isbn := "9780140449112"
	gbFoundFalse := false
	book := models.Book{ //nolint:exhaustruct // partial
		ID: id, Title: "Stuck Book", ISBN13: &isbn, GoogleBooksFound: &gbFoundFalse,
	}
	repo := &fakeBooksResync{books: []models.Book{book}} //nolint:exhaustruct // partial
	//nolint:exhaustruct // partial
	olClient := &fakeOLClient{err: openlibrary.ErrNotFound}
	//nolint:exhaustruct // partial
	gb := &fakeGBClient{byISBN: &googlebooks.ExternalBook{Title: "GB Title"}}
	svc := &BookService{ //nolint:exhaustruct // partial
		logger:      logging.NewNopLogger(),
		booksResync: repo,
		external:    olClient,
		googleBooks: gb,
		objectStore: objectstore.NewFake(),
	}

	_, err := svc.BuildResyncProposals(
		context.Background(), logging.NewNopLogger(), nil, false,
	)
	require.NoError(t, err)
	assert.Zero(t, gb.calls.Load(),
		"without force, a known (even false) GB flag must keep skipping GB")

	_, err = svc.BuildResyncProposals(
		context.Background(), logging.NewNopLogger(), nil, true,
	)
	require.NoError(t, err)
	assert.Equal(t, int32(1), gb.calls.Load(),
		"force must bypass the skip-if-known cache and query GB")
}

// TestBuildResyncProposals_SkipsKnownOpenLibrary_UnlessForced verifies the
// skip-if-known cache now applies to OpenLibrary too, not just Google Books —
// a resolved (true or false) openlibrary_found flag skips the OL call on a
// normal run, and force bypasses it.
func TestBuildResyncProposals_SkipsKnownOpenLibrary_UnlessForced(t *testing.T) {
	id := uuid.New()
	isbn := "9780140449112"
	olFoundTrue := true
	book := models.Book{ //nolint:exhaustruct // partial
		ID: id, Title: "Known Book", ISBN13: &isbn, OpenLibraryFound: &olFoundTrue,
	}
	repo := &fakeBooksResync{books: []models.Book{book}} //nolint:exhaustruct // partial
	//nolint:exhaustruct // partial
	olClient := &fakeOLClient{detail: &openlibrary.ExternalBook{Title: "OL Title"}}
	svc := &BookService{ //nolint:exhaustruct // partial
		logger:      logging.NewNopLogger(),
		booksResync: repo,
		external:    olClient,
		objectStore: objectstore.NewFake(),
	}

	_, err := svc.BuildResyncProposals(
		context.Background(), logging.NewNopLogger(), nil, false,
	)
	require.NoError(t, err)
	assert.Zero(t, olClient.calls,
		"without force, a known OpenLibrary flag must skip the OL call")

	_, err = svc.BuildResyncProposals(
		context.Background(), logging.NewNopLogger(), nil, true,
	)
	require.NoError(t, err)
	assert.Equal(t, 1, olClient.calls,
		"force must bypass the skip-if-known cache and query OpenLibrary")
}

// TestBuildResyncProposals_OnProgress_ReportsGoogleBooksQuotaReached verifies
// the gbExceeded breaker state is threaded through onProgress live, so
// callers (the resync job / progress WebSocket) can surface a quota-reached
// notice without waiting for the run to finish.
func TestBuildResyncProposals_OnProgress_ReportsGoogleBooksQuotaReached(t *testing.T) {
	isbn := "9780140449112"
	book := models.Book{ //nolint:exhaustruct // partial
		ID: uuid.New(), Title: "Rate Limited Book", ISBN13: &isbn,
	}
	repo := &fakeBooksResync{books: []models.Book{book}} //nolint:exhaustruct // partial
	//nolint:exhaustruct // partial
	olClient := &fakeOLClient{err: openlibrary.ErrNotFound}
	//nolint:exhaustruct // partial
	gb := &fakeGBClient{err: googlebooks.ErrRateLimited}
	svc := &BookService{ //nolint:exhaustruct // partial
		logger:      logging.NewNopLogger(),
		booksResync: repo,
		external:    olClient,
		googleBooks: gb,
		objectStore: objectstore.NewFake(),
	}

	var quotaCalls []bool
	_, err := svc.BuildResyncProposals(
		context.Background(), logging.NewNopLogger(),
		func(_, _ int, gbQuotaReached bool) { quotaCalls = append(quotaCalls, gbQuotaReached) },
		false,
	)
	require.NoError(t, err)
	require.Len(t, quotaCalls, 2, "one initial (0,total) call plus one per book")
	assert.False(t, quotaCalls[0], "the initial call precedes any GB lookup")
	assert.True(t, quotaCalls[1], "the 429 must trip the breaker before the per-book call")
}

func TestFetchByISBN_GoogleBooksBreakerTripped_SkipsCall(t *testing.T) {
	//nolint:exhaustruct // partial
	gb := &fakeGBClient{byISBN: &googlebooks.ExternalBook{Title: "GB Title"}}
	//nolint:exhaustruct // partial
	olClient := &fakeOLClient{err: openlibrary.ErrNotFound}
	svc := gbSvc(olClient, gb)
	tripped := &atomic.Bool{}
	tripped.Store(true)
	opts := &scanOptions{gbExceeded: tripped} //nolint:exhaustruct // partial

	_, unresolved := svc.fetchByISBN(
		context.Background(), logging.NewNopLogger(), "9780140449112", opts,
	)
	assert.True(t, unresolved["googlebooks"])
	assert.Zero(t, gb.calls.Load(), "a tripped breaker must skip the call entirely")
}

func TestFetchByISBN_GoogleBooksRateLimited_TripsBreaker(t *testing.T) {
	gb := &fakeGBClient{err: googlebooks.ErrRateLimited} //nolint:exhaustruct // partial
	//nolint:exhaustruct // partial
	olClient := &fakeOLClient{err: openlibrary.ErrNotFound}
	svc := gbSvc(olClient, gb)
	opts := &scanOptions{gbExceeded: &atomic.Bool{}} //nolint:exhaustruct // partial

	_, unresolved := svc.fetchByISBN(
		context.Background(), logging.NewNopLogger(), "9780140449112", opts,
	)
	assert.True(t, unresolved["googlebooks"])
	assert.True(
		t,
		opts.gbExceeded.Load(),
		"a 429 must trip the breaker for the rest of the run",
	)
}

func TestFetchSourceProposals_DispatchesOnISBNPresence(t *testing.T) {
	//nolint:exhaustruct // partial
	svc := &BookService{
		logger: logging.NewNopLogger(),
		//nolint:exhaustruct // partial
		external:    &fakeOLClient{err: openlibrary.ErrNotFound},
		objectStore: objectstore.NewFake(),
	}
	ctx := context.Background()

	isbn := "9780140449112"
	withISBN := models.Book{ISBN13: &isbn} //nolint:exhaustruct // partial
	// Has an ISBN: fetchByISBN's path runs (proves it by observing the OL call
	// outcome — ErrNotFound yields no proposals, same as a direct call would).
	proposals, _ := svc.fetchSourceProposals(ctx, logging.NewNopLogger(), withISBN, nil)
	assert.Empty(t, proposals)

	// No ISBN and no title: neither lookup path can run.
	bare := models.Book{} //nolint:exhaustruct // partial
	proposals, _ = svc.fetchSourceProposals(ctx, logging.NewNopLogger(), bare, nil)
	assert.Empty(t, proposals)
}

// ---------------------------------------------------------------------------
// fetchBySearch: match guards
// ---------------------------------------------------------------------------

func TestFetchBySearch_TitleAuthorMatch_Accepted(t *testing.T) {
	book := models.Book{ //nolint:exhaustruct // partial
		Title: "Dune", Authors: []string{"Frank Herbert"},
	}
	olFake := &fakeOLClientWithSearch{ //nolint:exhaustruct //only relevant fields
		searchResults: []openlibrary.ExternalBook{
			//nolint:exhaustruct // title/authors are all this test checks
			{Title: "Dune", Authors: []string{"Frank Herbert"}},
		},
	}
	svc := &BookService{ //nolint:exhaustruct // partial
		logger:      logging.NewNopLogger(),
		external:    olFake,
		objectStore: objectstore.NewFake(),
	}

	proposals, _ := svc.fetchBySearch(
		context.Background(),
		logging.NewNopLogger(),
		book,
		nil,
	)
	require.Len(t, proposals, 1)
	assert.Equal(t, "openlibrary", proposals[0].Source)
}

func TestFetchBySearch_TitleMismatch_Rejected(t *testing.T) {
	book := models.Book{ //nolint:exhaustruct // partial
		Title: "Dune", Authors: []string{"Frank Herbert"},
	}
	olFake := &fakeOLClientWithSearch{ //nolint:exhaustruct //only relevant fields
		searchResults: []openlibrary.ExternalBook{
			{ //nolint:exhaustruct // title/authors are all this test checks
				Title:   "Totally Different Book",
				Authors: []string{"Frank Herbert"},
			},
		},
	}
	svc := &BookService{ //nolint:exhaustruct // partial
		logger:      logging.NewNopLogger(),
		external:    olFake,
		objectStore: objectstore.NewFake(),
	}

	proposals, _ := svc.fetchBySearch(
		context.Background(),
		logging.NewNopLogger(),
		book,
		nil,
	)
	assert.Empty(t, proposals)
}

func TestFetchBySearch_TitleOnly_AmbiguousDisjointAuthors_Rejected(t *testing.T) {
	book := models.Book{ //nolint:exhaustruct // partial
		Title: "Emma",
	}
	isbn1, isbn2 := "9780141439587", "9780385340069"
	olFake := &fakeOLClientWithSearch{ //nolint:exhaustruct //only searchResults
		searchResults: []openlibrary.ExternalBook{
			{ //nolint:exhaustruct // title/authors/isbn13 are all this test checks
				Title:   "Emma",
				Authors: []string{"Jane Austen"},
				ISBN13:  &isbn1,
			},
			{ //nolint:exhaustruct // title/authors/isbn13 are all this test checks
				Title:   "Emma",
				Authors: []string{"Someone Else"},
				ISBN13:  &isbn2,
			},
		},
	}
	svc := &BookService{ //nolint:exhaustruct // partial
		logger:      logging.NewNopLogger(),
		external:    olFake,
		objectStore: objectstore.NewFake(),
	}

	proposals, _ := svc.fetchBySearch(
		context.Background(),
		logging.NewNopLogger(),
		book,
		nil,
	)
	assert.Empty(t, proposals, "disjoint-author title matches must not be proposed")
}

func TestFetchBySearch_GoogleBooksAndUniCat_AlsoMatch(t *testing.T) {
	//nolint:exhaustruct // partial
	book := models.Book{Title: "Dune", Authors: []string{"Frank Herbert"}}
	//nolint:exhaustruct // no OL match; proves GB/UC still run
	olFake := &fakeOLClientWithSearch{}
	gbFake := &fakeGBClient{ //nolint:exhaustruct // partial
		searchResults: []googlebooks.ExternalBook{
			{ //nolint:exhaustruct // title/authors are all this test checks
				Title:   "Dune",
				Authors: []string{"Frank Herbert"},
			},
		},
	}
	ucFake := &fakeUCClient{ //nolint:exhaustruct // partial
		searchResults: []unicat.ExternalBook{
			{ //nolint:exhaustruct // title/authors are all this test checks
				Title:   "Dune",
				Authors: []string{"Frank Herbert"},
			},
		},
	}
	svc := &BookService{ //nolint:exhaustruct // partial
		logger:      logging.NewNopLogger(),
		external:    olFake,
		googleBooks: gbFake,
		uniCat:      ucFake,
		objectStore: objectstore.NewFake(),
	}

	proposals, _ := svc.fetchBySearch(
		context.Background(),
		logging.NewNopLogger(),
		book,
		nil,
	)
	require.Len(t, proposals, 2, "both GB and UC matched")
	assert.Equal(t, "googlebooks", proposals[0].Source)
	assert.Equal(t, "unicat", proposals[1].Source)
}

// ---------------------------------------------------------------------------
// searchProviders: the same GB skip/breaker rules apply on the search path
// (a book with no ISBN), not just fetchByISBN.
// ---------------------------------------------------------------------------

func TestFetchBySearch_GoogleBooksKnown_SkippedAndUnresolved(t *testing.T) {
	book := models.Book{Title: "Dune"} //nolint:exhaustruct // partial
	//nolint:exhaustruct // partial
	gb := &fakeGBClient{
		searchResults: []googlebooks.ExternalBook{{Title: "Dune"}},
	} //nolint:exhaustruct // partial
	//nolint:exhaustruct // no OL match; proves GB still runs
	olClient := &fakeOLClientWithSearch{}
	svc := gbSvc(olClient, gb)
	opts := &scanOptions{
		known:      map[string]bool{"googlebooks": true},
		gbExceeded: &atomic.Bool{},
	}

	proposals, unresolved := svc.fetchBySearch(
		context.Background(), logging.NewNopLogger(), book, opts,
	)
	assert.Empty(t, proposals)
	assert.True(t, unresolved["googlebooks"])
	assert.Zero(t, gb.calls.Load(), "an already-known source must not be re-queried")
}

func TestFetchBySearch_GoogleBooksBreakerTripped_SkipsCall(t *testing.T) {
	book := models.Book{Title: "Dune"} //nolint:exhaustruct // partial
	//nolint:exhaustruct // partial
	gb := &fakeGBClient{
		searchResults: []googlebooks.ExternalBook{{Title: "Dune"}},
	} //nolint:exhaustruct // partial
	//nolint:exhaustruct // no OL match; proves GB still runs
	olClient := &fakeOLClientWithSearch{}
	svc := gbSvc(olClient, gb)
	tripped := &atomic.Bool{}
	tripped.Store(true)
	opts := &scanOptions{gbExceeded: tripped} //nolint:exhaustruct // partial

	_, unresolved := svc.fetchBySearch(
		context.Background(), logging.NewNopLogger(), book, opts,
	)
	assert.True(t, unresolved["googlebooks"])
	assert.Zero(t, gb.calls.Load(), "a tripped breaker must skip the call entirely")
}

func TestFetchBySearch_GoogleBooksRateLimited_TripsBreaker(t *testing.T) {
	book := models.Book{Title: "Dune"}                   //nolint:exhaustruct // partial
	gb := &fakeGBClient{err: googlebooks.ErrRateLimited} //nolint:exhaustruct // partial
	//nolint:exhaustruct // no OL match; proves GB still runs
	olClient := &fakeOLClientWithSearch{}
	svc := gbSvc(olClient, gb)
	opts := &scanOptions{gbExceeded: &atomic.Bool{}} //nolint:exhaustruct // partial

	_, unresolved := svc.fetchBySearch(
		context.Background(), logging.NewNopLogger(), book, opts,
	)
	assert.True(t, unresolved["googlebooks"])
	assert.True(
		t,
		opts.gbExceeded.Load(),
		"a 429 must trip the breaker for the rest of the run",
	)
}

// ---------------------------------------------------------------------------
// computeDifferences
// ---------------------------------------------------------------------------

func TestComputeDifferences_Rules(t *testing.T) {
	existingCover := "https://example.com/cover.jpg"
	existingISBN := "9780140449112"
	pages := 100
	book := models.Book{ //nolint:exhaustruct // partial
		Title:     "Dune",
		Authors:   []string{"Frank Herbert"},
		CoverURL:  &existingCover,
		ISBN13:    &existingISBN,
		PageCount: &pages,
	}

	// A source that agrees on everything it offers: no diff.
	agree := SourceProposal{ //nolint:exhaustruct // partial
		Title: "Dune", Authors: []string{"Frank Herbert"}, PageCount: pages,
	}
	assert.Empty(t, computeDifferences(book, agree))

	// Title differs.
	titleDiff := SourceProposal{ //nolint:exhaustruct // partial
		Title: "Different Title",
	}
	assert.Contains(t, computeDifferences(book, titleDiff), "title")

	// Page count differs.
	pageDiff := SourceProposal{PageCount: 999} //nolint:exhaustruct // partial
	assert.Contains(t, computeDifferences(book, pageDiff), "page_count")

	// Description differs (library has none — any non-empty source value counts).
	//nolint:exhaustruct // partial
	descDiff := SourceProposal{Description: "A new description."}
	assert.Contains(t, computeDifferences(book, descDiff), "description")

	// Cover: never flagged when the library already has one.
	//nolint:exhaustruct // partial
	coverDiff := SourceProposal{CoverURL: "https://elsewhere.example.com/x.jpg"}
	assert.NotContains(t, computeDifferences(book, coverDiff), "cover_url")

	// ISBN: never flagged when the library already has one.
	isbnDiff := SourceProposal{ISBN13: "9780062316097"} //nolint:exhaustruct // partial
	assert.NotContains(t, computeDifferences(book, isbnDiff), "isbn13")

	// A book missing cover/ISBN does flag a source that supplies one.
	//nolint:exhaustruct // partial
	bareBook := models.Book{Title: "Dune", Authors: []string{"Frank Herbert"}}
	gapFill := SourceProposal{ //nolint:exhaustruct // partial
		CoverURL: "https://example.com/new.jpg",
		ISBN13:   "9780062316097",
	}
	diffs := computeDifferences(bareBook, gapFill)
	assert.Contains(t, diffs, "cover_url")
	assert.Contains(t, diffs, "isbn13")
}

// ---------------------------------------------------------------------------
// BuildResyncProposals
// ---------------------------------------------------------------------------

func TestBuildResyncProposals_FlagsOnlyDiffering(t *testing.T) {
	idAgree, idDiffer := uuid.New(), uuid.New()
	isbnA, isbnB := "9780140449112", "9780062316097"
	// The library already has a cover, matching the ISBN-keyed fallback OL
	// falls back to, so the "agree" book truly agrees on every field.
	coverA := openlibrary.CoverURLByISBN(&isbnA)

	repo := &fakeBooksResync{ //nolint:exhaustruct //zero values fine
		books: []models.Book{
			//nolint:exhaustruct // partial
			{ID: idAgree, Title: "Agreeing Book", ISBN13: &isbnA, CoverURL: &coverA},
			//nolint:exhaustruct // partial
			{ID: idDiffer, Title: "Differing Book", ISBN13: &isbnB},
		},
	}

	// OL returns the same title for the "agree" book but a different one for
	// the "differ" book.
	svc := &BookService{ //nolint:exhaustruct // partial
		logger:      logging.NewNopLogger(),
		booksResync: repo,
		external: &multiISBNOLClient{results: map[string]*openlibrary.ExternalBook{
			isbnA: {Title: "Agreeing Book"},
			isbnB: {Title: "A Totally Different Title"},
		}},
		objectStore: objectstore.NewFake(),
	}

	var calls [][2]int
	n, err := svc.BuildResyncProposals(
		context.Background(), logging.NewNopLogger(),
		func(processed, total int, _ bool) { calls = append(calls, [2]int{processed, total}) },
		false,
	)
	require.NoError(t, err)
	assert.Equal(t, 1, n, "only the differing book should be flagged")
	require.Len(t, calls, 3, "one (0,total) call plus one per book")
	assert.Equal(t, [2]int{0, 2}, calls[0])

	require.Len(t, repo.replaced, 1)
	_, ok := repo.replaced[idDiffer]
	assert.True(t, ok, "the differing book must be in the replacement set")
	_, agreeStillThere := repo.replaced[idAgree]
	assert.False(t, agreeStillThere, "the agreeing book must not be flagged")
}

// TestBuildResyncProposals_FlagsNotFoundAnywhere verifies that a searchable
// book (has an ISBN) every configured source returns ErrNotFound for is still
// flagged — with zero stored sources — so the admin wizard can surface
// coverage gaps, distinct from a book that agrees with every source (which is
// never flagged at all).
func TestBuildResyncProposals_FlagsNotFoundAnywhere(t *testing.T) {
	id := uuid.New()
	isbn := "9780140449112"
	repo := &fakeBooksResync{ //nolint:exhaustruct //zero values fine
		books: []models.Book{
			{ //nolint:exhaustruct // partial
				ID:     id,
				Title:  "Obscure Book",
				ISBN13: &isbn,
			},
		},
	}
	svc := &BookService{ //nolint:exhaustruct // partial
		logger:      logging.NewNopLogger(),
		booksResync: repo,
		//nolint:exhaustruct // partial
		external:    &fakeOLClient{err: openlibrary.ErrNotFound},
		objectStore: objectstore.NewFake(),
	}

	n, err := svc.BuildResyncProposals(
		context.Background(),
		logging.NewNopLogger(),
		nil,
		false,
	)
	require.NoError(t, err)
	assert.Equal(t, 1, n, "a book no source could find must still be flagged")

	require.Contains(t, repo.replaced, id)
	var sources []SourceProposal
	require.NoError(t, json.Unmarshal(repo.replaced[id], &sources))
	assert.Empty(t, sources, "no source data to store when nothing was found")
}

// TestBuildResyncProposals_NeverAttempted_NotFlagged verifies that a book
// with neither an ISBN nor a title (nothing could be searched) is never
// flagged — unlike the "not found anywhere" case, no lookup was attempted.
func TestBuildResyncProposals_NeverAttempted_NotFlagged(t *testing.T) {
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

	n, err := svc.BuildResyncProposals(
		context.Background(),
		logging.NewNopLogger(),
		nil,
		false,
	)
	require.NoError(t, err)
	assert.Equal(t, 0, n, "a book nothing could be searched for must not be flagged")
}

func TestBuildResyncProposals_EmptyLibrary(t *testing.T) {
	repo := &fakeBooksResync{} //nolint:exhaustruct //zero values fine
	svc := &BookService{       //nolint:exhaustruct // partial
		logger:      logging.NewNopLogger(),
		booksResync: repo,
		external:    &fakeOLClient{}, //nolint:exhaustruct //zero values fine
		objectStore: objectstore.NewFake(),
	}

	var calls [][2]int
	n, err := svc.BuildResyncProposals(
		context.Background(), logging.NewNopLogger(),
		func(processed, total int, _ bool) { calls = append(calls, [2]int{processed, total}) },
		false,
	)
	require.NoError(t, err)
	assert.Equal(t, 0, n)
	require.Len(t, calls, 1)
	assert.Equal(t, [2]int{0, 0}, calls[0])
}

func TestBuildResyncProposals_ListError(t *testing.T) {
	listErr := errors.New("connection refused")
	repo := &fakeBooksResync{listErr: listErr} //nolint:exhaustruct // partial
	svc := &BookService{                       //nolint:exhaustruct // partial
		logger:      logging.NewNopLogger(),
		booksResync: repo,
		objectStore: objectstore.NewFake(),
	}

	n, err := svc.BuildResyncProposals(
		context.Background(),
		logging.NewNopLogger(),
		nil,
		false,
	)
	require.ErrorIs(t, err, listErr)
	assert.Equal(t, 0, n)
}

// ---------------------------------------------------------------------------
// ListResyncProposals: Differs recomputed at read time
// ---------------------------------------------------------------------------

func TestListResyncProposals_RecomputesDiffers(t *testing.T) {
	bookID := uuid.New()
	book := models.Book{ID: bookID, Title: "Dune"} //nolint:exhaustruct // partial

	raw, err := json.Marshal([]SourceProposal{
		{ //nolint:exhaustruct // partial
			Source: "openlibrary",
			Title:  "Different Title",
		},
	})
	require.NoError(t, err)

	repo := &fakeBooksResync{ //nolint:exhaustruct //zero values fine
		proposalRows: map[uuid.UUID]repositories.ResyncProposalRow{
			bookID: {Book: book, ProposalsJSON: raw},
		},
	}
	svc := &BookService{booksResync: repo} //nolint:exhaustruct // partial

	proposals, err := svc.ListResyncProposals(context.Background())
	require.NoError(t, err)
	require.Len(t, proposals, 1)
	require.Len(t, proposals[0].Sources, 1)
	assert.Contains(t, proposals[0].Sources[0].Differs, "title")
	assert.Equal(t, "Dune", proposals[0].Library.Title)
}

// ---------------------------------------------------------------------------
// ApplyResyncChoice
// ---------------------------------------------------------------------------

func TestApplyResyncChoice_KeepLibrary_DismissesWithoutWriting(t *testing.T) {
	bookID := uuid.New()
	book := models.Book{ID: bookID, Title: "Dune"} //nolint:exhaustruct // partial
	raw, err := json.Marshal([]SourceProposal{
		{Source: "openlibrary", Title: "Other Title"}, //nolint:exhaustruct // partial
	})
	require.NoError(t, err)

	repo := &fakeBooksResync{ //nolint:exhaustruct //zero values fine
		proposalRows: map[uuid.UUID]repositories.ResyncProposalRow{
			bookID: {Book: book, ProposalsJSON: raw},
		},
	}
	svc := &BookService{booksResync: repo} //nolint:exhaustruct // partial

	err = svc.ApplyResyncChoice(
		context.Background(),
		logging.NewNopLogger(),
		bookID,
		"",
	)
	require.NoError(t, err)
	assert.Empty(t, repo.refreshCalls, "keeping the library value must not write")
	assert.Equal(t, []uuid.UUID{bookID}, repo.deletedIDs)
}

func TestApplyResyncChoice_ChosenSource_WritesFields(t *testing.T) {
	bookID := uuid.New()
	book := models.Book{ID: bookID, Title: "Old Title"} //nolint:exhaustruct // partial
	raw, err := json.Marshal([]SourceProposal{
		{ //nolint:exhaustruct // partial
			Source: "openlibrary", Title: "New Title", Description: "New desc",
			PageCount: 42, CoverURL: "https://example.com/c.jpg",
		},
	})
	require.NoError(t, err)

	repo := &fakeBooksResync{ //nolint:exhaustruct //zero values fine
		proposalRows: map[uuid.UUID]repositories.ResyncProposalRow{
			bookID: {Book: book, ProposalsJSON: raw},
		},
	}
	svc := &BookService{ //nolint:exhaustruct // partial
		booksResync: repo,
		objectStore: objectstore.NewFake(),
	}

	err = svc.ApplyResyncChoice(
		context.Background(), logging.NewNopLogger(), bookID, "openlibrary",
	)
	require.NoError(t, err)

	require.Len(t, repo.refreshCalls, 1)
	rc := repo.refreshCalls[0]
	require.NotNil(t, rc.title)
	assert.Equal(t, "New Title", *rc.title)
	require.NotNil(t, rc.description)
	assert.Equal(t, "New desc", *rc.description)
	require.NotNil(t, rc.pageCount)
	assert.Equal(t, 42, *rc.pageCount)
	assert.Equal(t, "openlibrary", rc.metadataSource,
		"applying a source must record it as the book's metadata source")
	assert.Equal(t, []uuid.UUID{bookID}, repo.deletedIDs)
}

func TestApplyResyncChoice_NeverOverwritesExistingISBN(t *testing.T) {
	bookID := uuid.New()
	existingISBN := "9780140449112"
	book := models.Book{ //nolint:exhaustruct // partial
		ID:     bookID,
		ISBN13: &existingISBN,
	}
	raw, err := json.Marshal([]SourceProposal{
		{ //nolint:exhaustruct // partial
			Source: "openlibrary",
			ISBN13: "9780062316097",
		},
	})
	require.NoError(t, err)

	repo := &fakeBooksResync{ //nolint:exhaustruct //zero values fine
		proposalRows: map[uuid.UUID]repositories.ResyncProposalRow{
			bookID: {Book: book, ProposalsJSON: raw},
		},
	}
	svc := &BookService{ //nolint:exhaustruct // partial
		booksResync: repo,
		objectStore: objectstore.NewFake(),
	}

	err = svc.ApplyResyncChoice(
		context.Background(), logging.NewNopLogger(), bookID, "openlibrary",
	)
	require.NoError(t, err)

	require.Len(t, repo.refreshCalls, 1)
	assert.Nil(t, repo.refreshCalls[0].isbn13,
		"an existing ISBN must never be overwritten by a resync choice")
}

func TestApplyResyncChoice_UnknownBook_ErrProposalNotFound(t *testing.T) {
	repo := &fakeBooksResync{}             //nolint:exhaustruct //zero values fine
	svc := &BookService{booksResync: repo} //nolint:exhaustruct // partial

	err := svc.ApplyResyncChoice(
		context.Background(), logging.NewNopLogger(), uuid.New(), "openlibrary",
	)
	require.ErrorIs(t, err, ErrProposalNotFound)
}

func TestApplyResyncChoice_UnknownSource_ErrProposalNotFound(t *testing.T) {
	bookID := uuid.New()
	book := models.Book{ID: bookID} //nolint:exhaustruct // partial
	raw, err := json.Marshal([]SourceProposal{
		{Source: "openlibrary", Title: "X"}, //nolint:exhaustruct // partial
	})
	require.NoError(t, err)

	repo := &fakeBooksResync{ //nolint:exhaustruct //zero values fine
		proposalRows: map[uuid.UUID]repositories.ResyncProposalRow{
			bookID: {Book: book, ProposalsJSON: raw},
		},
	}
	svc := &BookService{booksResync: repo} //nolint:exhaustruct // partial

	err = svc.ApplyResyncChoice(
		context.Background(), logging.NewNopLogger(), bookID, "googlebooks",
	)
	require.ErrorIs(t, err, ErrProposalNotFound)
}

// ---------------------------------------------------------------------------
// GetBookSources / SyncBookSource: live per-book fetch, no prior scan needed
// ---------------------------------------------------------------------------

func TestGetBookSources_ReturnsLiveProposal(t *testing.T) {
	bookID := uuid.New()
	book := models.Book{ID: bookID, Title: "Dune"} //nolint:exhaustruct // partial
	//nolint:exhaustruct // partial
	olDetail := &openlibrary.ExternalBook{
		Title:   "Dune",
		Authors: []string{"Frank Herbert"},
	}

	repo := &fakeBooksResync{ //nolint:exhaustruct //zero values fine
		books: []models.Book{book},
	}
	svc := &BookService{ //nolint:exhaustruct // partial
		booksResync: repo,
		external: &fakeOLClientWithSearch{ //nolint:exhaustruct //only relevant fields
			searchResults: []openlibrary.ExternalBook{*olDetail},
		},
		objectStore: objectstore.NewFake(),
	}

	proposal, err := svc.GetBookSources(
		context.Background(),
		logging.NewNopLogger(),
		bookID,
		"",
		"",
	)
	require.NoError(t, err)
	assert.Equal(t, bookID.String(), proposal.BookID)
	require.Len(t, proposal.Sources, 1)
	assert.Equal(t, "openlibrary", proposal.Sources[0].Source)
	assert.Contains(t, proposal.Sources[0].Differs, "authors")
}

func TestGetBookSources_UnknownBook_ErrProposalNotFound(t *testing.T) {
	repo := &fakeBooksResync{}             //nolint:exhaustruct //zero values fine
	svc := &BookService{booksResync: repo} //nolint:exhaustruct // partial

	_, err := svc.GetBookSources(
		context.Background(),
		logging.NewNopLogger(),
		uuid.New(),
		"",
		"",
	)
	require.ErrorIs(t, err, ErrProposalNotFound)
}

func TestSyncBookSource_AppliesLiveFetchAndClearsPendingProposal(t *testing.T) {
	bookID := uuid.New()
	isbn := "9780140449112"
	book := models.Book{ //nolint:exhaustruct // partial
		ID:     bookID,
		Title:  "Old Title",
		ISBN13: &isbn,
	}
	//nolint:exhaustruct // partial
	olDetail := &openlibrary.ExternalBook{
		Title:   "New Title",
		Authors: []string{"Author"},
	}

	repo := &fakeBooksResync{ //nolint:exhaustruct //zero values fine
		books: []models.Book{book},
		proposalRows: map[uuid.UUID]repositories.ResyncProposalRow{
			bookID: {Book: book, ProposalsJSON: []byte("[]")},
		},
	}
	svc := &BookService{ //nolint:exhaustruct // partial
		booksResync: repo,
		external:    &fakeOLClient{detail: olDetail}, //nolint:exhaustruct // partial
		objectStore: objectstore.NewFake(),
	}

	err := svc.SyncBookSource(
		context.Background(), logging.NewNopLogger(), bookID, "openlibrary", "", "",
	)
	require.NoError(t, err)

	require.Len(t, repo.refreshCalls, 1)
	require.NotNil(t, repo.refreshCalls[0].title)
	assert.Equal(t, "New Title", *repo.refreshCalls[0].title)
	assert.Equal(t, "openlibrary", repo.refreshCalls[0].metadataSource)
	assert.Equal(t, []uuid.UUID{bookID}, repo.deletedIDs,
		"applying live should also clear any pending wizard proposal")
}

func TestSyncBookSource_UnknownSource_ErrProposalNotFound(t *testing.T) {
	bookID := uuid.New()
	book := models.Book{ID: bookID} //nolint:exhaustruct // partial
	repo := &fakeBooksResync{       //nolint:exhaustruct //zero values fine
		books: []models.Book{book},
	}
	svc := &BookService{ //nolint:exhaustruct // partial
		booksResync: repo,
		//nolint:exhaustruct // detail unused, err drives the not-found path
		external:    &fakeOLClient{err: openlibrary.ErrNotFound},
		objectStore: objectstore.NewFake(),
	}

	err := svc.SyncBookSource(
		context.Background(), logging.NewNopLogger(), bookID, "openlibrary", "", "",
	)
	require.ErrorIs(t, err, ErrProposalNotFound)
}

// ---------------------------------------------------------------------------
// bustCoverCache: Delete failures are non-fatal
// ---------------------------------------------------------------------------

func TestBustCoverCache_DeleteErrors(t *testing.T) {
	bookID := uuid.New()
	store := failDeleteObjectStore{inner: objectstore.NewFake()}
	svc := &BookService{ //nolint:exhaustruct // partial
		logger:      logging.NewNopLogger(),
		objectStore: store,
	}

	assert.NotPanics(t, func() {
		svc.bustCoverCache(context.Background(), logging.NewNopLogger(), bookID)
	})
}
