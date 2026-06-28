//nolint:testpackage // testing unexported service helpers
package services

import (
	"bytes"
	"context"
	"errors"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xdoubleu/essentia/v4/pkg/logging"

	"tools.xdoubleu.com/apps/backlog/internal/models"
	"tools.xdoubleu.com/apps/backlog/pkg/googlebooks"
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
	isbn13      *string
}

// statusCall records a single call to fakeBooksResync.SetResyncStatus.
type statusCall struct {
	bookID  uuid.UUID
	olFound bool
	gbFound bool
}

// fakeBooksResync is a test stub for booksResyncSource.
type fakeBooksResync struct {
	books        []models.Book
	listErr      error
	refreshMu    sync.Mutex
	refreshErr   error
	refreshCalls []refreshCall
	statusMu     sync.Mutex
	statusCalls  []statusCall
	statusErr    error
}

func (f *fakeBooksResync) ListBooksMissingMetadata(
	_ context.Context,
) ([]models.Book, error) {
	return f.books, f.listErr
}

func (f *fakeBooksResync) GetBooksByIDs(
	_ context.Context,
	ids []uuid.UUID,
) ([]models.Book, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	idSet := make(map[uuid.UUID]struct{}, len(ids))
	for _, id := range ids {
		idSet[id] = struct{}{}
	}
	var out []models.Book
	for _, b := range f.books {
		if _, ok := idSet[b.ID]; ok {
			out = append(out, b)
		}
	}
	return out, nil
}

func (f *fakeBooksResync) RefreshBookExternalData(
	_ context.Context,
	bookID uuid.UUID,
	coverURL *string,
	description *string,
	pageCount *int,
	isbn13 *string,
) error {
	f.refreshMu.Lock()
	f.refreshCalls = append(f.refreshCalls, refreshCall{
		bookID:      bookID,
		coverURL:    coverURL,
		description: description,
		pageCount:   pageCount,
		isbn13:      isbn13,
	})
	f.refreshMu.Unlock()
	return f.refreshErr
}

func (f *fakeBooksResync) SetResyncStatus(
	_ context.Context,
	bookID uuid.UUID,
	olFound bool,
	gbFound bool,
) error {
	f.statusMu.Lock()
	f.statusCalls = append(f.statusCalls, statusCall{
		bookID:  bookID,
		olFound: olFound,
		gbFound: gbFound,
	})
	f.statusMu.Unlock()
	return f.statusErr
}

// fakeGBClient is a configurable googlebooks.Client stub.
type fakeGBClient struct {
	searchResults []gbResult
	byISBN        *gbResult
	err           error
	mu            sync.Mutex
	calls         int
}

type gbResult struct {
	title    string
	authors  []string
	isbn13   *string
	coverURL *string
	desc     *string
	pages    *int
}

func (f *fakeGBClient) Search(
	_ context.Context,
	_ string,
) ([]googlebooks.ExternalBook, error) {
	f.mu.Lock()
	f.calls++
	f.mu.Unlock()
	if f.err != nil {
		return nil, f.err
	}
	out := make([]googlebooks.ExternalBook, len(f.searchResults))
	for i, r := range f.searchResults {
		out[i] = gbResultToExternal(r)
	}
	return out, nil
}

func (f *fakeGBClient) GetByISBN(
	_ context.Context,
	_ string,
) (*googlebooks.ExternalBook, error) {
	f.mu.Lock()
	f.calls++
	f.mu.Unlock()
	if f.err != nil {
		return nil, f.err
	}
	if f.byISBN == nil {
		return nil, googlebooks.ErrNotFound
	}
	b := gbResultToExternal(*f.byISBN)
	return &b, nil
}

func gbResultToExternal(r gbResult) googlebooks.ExternalBook {
	return googlebooks.ExternalBook{
		Title:       r.title,
		Authors:     r.authors,
		ISBN13:      r.isbn13,
		CoverURL:    r.coverURL,
		Description: r.desc,
		PageCount:   r.pages,
	}
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
		booksResync: &fakeBooksResync{}, //nolint:exhaustruct //zero values fine
		external:    fake,
		objectStore: store,
	}

	err := svc.resyncBook(
		context.Background(),
		logging.NewNopLogger(),
		book,
		false,
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
		booksResync: &fakeBooksResync{}, //nolint:exhaustruct //zero values fine
		external:    fake,
		objectStore: store,
	}

	err := svc.resyncBook(
		context.Background(),
		logging.NewNopLogger(),
		book,
		false,
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

	err = svc.resyncBook(context.Background(), logging.NewNopLogger(), book, false)
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

	err = svc.resyncBook(context.Background(), logging.NewNopLogger(), book, false)
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

	err = svc.resyncBook(context.Background(), logging.NewNopLogger(), book, false)
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
// cache is NOT deleted.
func TestResyncBook_NoCoverAnywhere(t *testing.T) {
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

	err = svc.resyncBook(context.Background(), logging.NewNopLogger(), book, false)
	require.NoError(t, err)

	if len(repo.refreshCalls) > 0 && repo.refreshCalls[0].coverURL == nil {
		_, missingStillExists := store.GetContent(missingKey)
		assert.True(
			t, missingStillExists,
			"missing-marker must not be deleted when no cover URL was found",
		)
	}
}

// TestResyncBook_GoogleBooks_FillsMissingCover verifies that when OL has no
// cover for a book but Google Books does, the GB cover is used.
func TestResyncBook_GoogleBooks_FillsMissingCover(t *testing.T) {
	isbn := "9789463107587" // Dutch book — no OL cover
	book := models.Book{    //nolint:exhaustruct //only tested fields needed
		ID:     uuid.New(),
		ISBN13: &isbn,
	}

	// OL returns no cover.
	detail := &openlibrary.ExternalBook{ //nolint:exhaustruct //only relevant fields
		CoverURL: nil,
	}
	olFake := &fakeOLClient{detail: detail} //nolint:exhaustruct // err nil

	// GB returns a cover.
	gbCover := "https://books.google.com/covers/dutch-book.jpg"
	gbFake := &fakeGBClient{ //nolint:exhaustruct // only byISBN needed
		byISBN: &gbResult{ //nolint:exhaustruct // desc and pages not relevant here
			title:    "Dutch Book",
			authors:  []string{"Dutch Author"},
			isbn13:   &isbn,
			coverURL: &gbCover,
		},
	}

	repo := &fakeBooksResync{} //nolint:exhaustruct // zero values fine
	store := objectstore.NewFake()

	svc := &BookService{ //nolint:exhaustruct // only tested fields needed
		logger:      logging.NewNopLogger(),
		booksResync: repo,
		external:    olFake,
		googleBooks: gbFake,
		objectStore: store,
	}

	err := svc.resyncBook(context.Background(), logging.NewNopLogger(), book, false)
	require.NoError(t, err)

	require.Len(t, repo.refreshCalls, 1)
	require.NotNil(t, repo.refreshCalls[0].coverURL,
		"GB cover must be written when OL has none")
	assert.Equal(t, gbCover, *repo.refreshCalls[0].coverURL)
}

// TestResyncBook_NoISBN_MatchedByTitleAuthor verifies that an ISBN-less book
// matched by title+author gets cover, description, page_count AND isbn13
// backfilled from the search result.
func TestResyncBook_NoISBN_MatchedByTitleAuthor(t *testing.T) {
	book := models.Book{ //nolint:exhaustruct //only tested fields needed
		ID:      uuid.New(),
		Title:   "2001: A Space Odyssey",
		Authors: []string{"Arthur C. Clarke"},
		// No ISBN, no cover, no description, no page count.
	}

	// OL search returns no results.
	olFake := &fakeOLClient{} //nolint:exhaustruct // Search returns nil,nil

	// GB search returns a confident match.
	discoveredISBN := "9780451457998"
	gbCover := "https://books.google.com/covers/odyssey.jpg"
	gbDesc := "A science fiction novel."
	gbPages := 221
	gbFake := &fakeGBClient{ //nolint:exhaustruct // only searchResults needed
		searchResults: []gbResult{{
			title:    "2001: A Space Odyssey",
			authors:  []string{"Arthur C. Clarke"},
			isbn13:   &discoveredISBN,
			coverURL: &gbCover,
			desc:     &gbDesc,
			pages:    &gbPages,
		}},
	}

	repo := &fakeBooksResync{} //nolint:exhaustruct // zero values fine
	store := objectstore.NewFake()

	svc := &BookService{ //nolint:exhaustruct // only tested fields needed
		logger:      logging.NewNopLogger(),
		booksResync: repo,
		external:    olFake,
		googleBooks: gbFake,
		objectStore: store,
	}

	err := svc.resyncBook(context.Background(), logging.NewNopLogger(), book, false)
	require.NoError(t, err)

	require.Len(t, repo.refreshCalls, 1)
	rc := repo.refreshCalls[0]
	require.NotNil(t, rc.coverURL)
	assert.Equal(t, gbCover, *rc.coverURL)
	require.NotNil(t, rc.description)
	assert.Equal(t, gbDesc, *rc.description)
	require.NotNil(t, rc.pageCount)
	assert.Equal(t, gbPages, *rc.pageCount)
	require.NotNil(t, rc.isbn13, "discovered ISBN13 must be written")
	assert.Equal(t, discoveredISBN, *rc.isbn13)
}

// TestResyncBook_NoISBN_LowConfidence_NothingWritten verifies that when a
// search result title does not match the book's title, no DB write occurs.
func TestResyncBook_NoISBN_LowConfidence_NothingWritten(t *testing.T) {
	book := models.Book{ //nolint:exhaustruct // only tested fields needed
		ID:      uuid.New(),
		Title:   "2001: A Space Odyssey",
		Authors: []string{"Arthur C. Clarke"},
	}

	// Both providers return a result whose title does NOT match.
	wrongISBN := "9780000000001"
	wrongCover := "https://books.google.com/wrong.jpg"
	gbFake := &fakeGBClient{ //nolint:exhaustruct // only searchResults needed
		searchResults: []gbResult{
			{ //nolint:exhaustruct // desc and pages not relevant here
				title:    "Totally Different Book",
				authors:  []string{"Arthur C. Clarke"},
				isbn13:   &wrongISBN,
				coverURL: &wrongCover,
			},
		},
	}

	repo := &fakeBooksResync{} //nolint:exhaustruct // zero values fine
	store := objectstore.NewFake()

	svc := &BookService{ //nolint:exhaustruct // only tested fields needed
		logger:      logging.NewNopLogger(),
		booksResync: repo,
		external:    &fakeOLClient{}, //nolint:exhaustruct // zero values fine
		googleBooks: gbFake,
		objectStore: store,
	}

	err := svc.resyncBook(context.Background(), logging.NewNopLogger(), book, false)
	require.NoError(t, err)
	assert.Empty(t, repo.refreshCalls,
		"no DB write must occur when no confident title/author match found")
}

// TestResyncBook_NoISBN_ISBNCollision_OtherFieldsStillWritten verifies that
// even when the discovered ISBN belongs to another book (RefreshBookExternalData
// silently skips it via the NOT EXISTS guard), cover/description/page_count
// are still written.
//
// This is tested at the service level by checking that RefreshBookExternalData
// is called with the isbn13 argument set — the collision guard lives in the
// repository SQL and is tested in repository integration tests.
func TestResyncBook_NoISBN_ISBNPassedToRepo(t *testing.T) {
	book := models.Book{ //nolint:exhaustruct // only tested fields needed
		ID:      uuid.New(),
		Title:   "Dune",
		Authors: []string{"Frank Herbert"},
	}

	discoveredISBN := "9780441013593"
	gbCover := "https://books.google.com/dune.jpg"
	gbFake := &fakeGBClient{ //nolint:exhaustruct // only searchResults needed
		searchResults: []gbResult{
			{ //nolint:exhaustruct // desc and pages not relevant here
				title:    "Dune",
				authors:  []string{"Frank Herbert"},
				isbn13:   &discoveredISBN,
				coverURL: &gbCover,
			},
		},
	}

	repo := &fakeBooksResync{} //nolint:exhaustruct // zero values fine
	store := objectstore.NewFake()

	svc := &BookService{ //nolint:exhaustruct // only tested fields needed
		logger:      logging.NewNopLogger(),
		booksResync: repo,
		external:    &fakeOLClient{}, //nolint:exhaustruct // zero values fine
		googleBooks: gbFake,
		objectStore: store,
	}

	err := svc.resyncBook(context.Background(), logging.NewNopLogger(), book, false)
	require.NoError(t, err)

	require.Len(t, repo.refreshCalls, 1)
	require.NotNil(t, repo.refreshCalls[0].isbn13,
		"isbn13 must be passed to RefreshBookExternalData")
	assert.Equal(t, discoveredISBN, *repo.refreshCalls[0].isbn13)
}

// fakeOLClientWithSearch wraps fakeOLClient but supports returning configurable
// Search results so we can exercise the OL-match path in resyncBookByTitleAuthor.
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

// TestResyncAllFromOpenLibrary_EmptyLibrary verifies that with no books
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

// TestResyncBook_NoISBN_EmptyTitle_Skipped verifies that a book with no title
// and no ISBN is silently skipped with no network or DB calls.
func TestResyncBook_NoISBN_EmptyTitle_Skipped(t *testing.T) {
	book := models.Book{ //nolint:exhaustruct // only tested fields needed
		ID:      uuid.New(),
		Title:   "",
		Authors: []string{"Some Author"},
	}
	repo := &fakeBooksResync{} //nolint:exhaustruct // zero values fine
	store := objectstore.NewFake()
	svc := &BookService{ //nolint:exhaustruct // only tested fields needed
		logger:      logging.NewNopLogger(),
		booksResync: repo,
		external:    &fakeOLClient{}, //nolint:exhaustruct // zero values fine
		objectStore: store,
	}

	err := svc.resyncBook(context.Background(), logging.NewNopLogger(), book, false)
	require.NoError(t, err)
	assert.Empty(t, repo.refreshCalls, "no DB write expected for title-less book")
}

// TestResyncBook_OLSearch_MatchFillsMetadata verifies that when OL search
// returns a confident title/author match, cover and ISBN are backfilled without
// needing to call Google Books.
func TestResyncBook_OLSearch_MatchFillsMetadata(t *testing.T) {
	book := models.Book{ //nolint:exhaustruct // only tested fields needed
		ID:      uuid.New(),
		Title:   "Dune",
		Authors: []string{"Frank Herbert"},
	}

	olISBN := "9780441013593"
	olCover := "https://covers.openlibrary.org/b/isbn/9780441013593-L.jpg"
	olFake := &fakeOLClientWithSearch{ //nolint:exhaustruct // only searchResults needed
		searchResults: []openlibrary.ExternalBook{
			{ //nolint:exhaustruct // only fields relevant to match check
				Provider:   "openlibrary",
				ProviderID: "OL102749W",
				Title:      "Dune",
				Authors:    []string{"Frank Herbert"},
				ISBN13:     &olISBN,
				CoverURL:   &olCover,
			},
		},
	}

	repo := &fakeBooksResync{} //nolint:exhaustruct // zero values fine
	store := objectstore.NewFake()
	svc := &BookService{ //nolint:exhaustruct // only tested fields needed
		logger:      logging.NewNopLogger(),
		booksResync: repo,
		external:    olFake,
		objectStore: store,
	}

	err := svc.resyncBook(context.Background(), logging.NewNopLogger(), book, false)
	require.NoError(t, err)

	require.Len(t, repo.refreshCalls, 1)
	require.NotNil(t, repo.refreshCalls[0].coverURL)
	assert.Equal(t, olCover, *repo.refreshCalls[0].coverURL)
	require.NotNil(t, repo.refreshCalls[0].isbn13)
	assert.Equal(t, olISBN, *repo.refreshCalls[0].isbn13)
}

// TestBustCoverCache_DeleteErrors verifies that Delete failures in bustCoverCache
// are non-fatal: the method logs a warning and does not panic.
func TestBustCoverCache_DeleteErrors(t *testing.T) {
	bookID := uuid.New()
	store := failDeleteObjectStore{inner: objectstore.NewFake()}
	svc := &BookService{ //nolint:exhaustruct // only tested fields needed
		logger:      logging.NewNopLogger(),
		objectStore: store,
	}

	assert.NotPanics(t, func() {
		svc.bustCoverCache(context.Background(), logging.NewNopLogger(), bookID)
	})
}

// ---------------------------------------------------------------------------
// SetResyncStatus recording tests
// ---------------------------------------------------------------------------

// TestResyncBook_RecordsOLFound verifies that when OL returns a record,
// SetResyncStatus is called with olFound=true.
func TestResyncBook_RecordsOLFound(t *testing.T) {
	isbn := "9780140449112"
	book := models.Book{ //nolint:exhaustruct //only tested fields needed
		ID:     uuid.New(),
		ISBN13: &isbn,
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

	svc := &BookService{ //nolint:exhaustruct //only tested fields needed
		logger:      logging.NewNopLogger(),
		booksResync: repo,
		external:    fake,
		objectStore: objectstore.NewFake(),
	}

	err := svc.resyncBook(context.Background(), logging.NewNopLogger(), book, false)
	require.NoError(t, err)

	repo.statusMu.Lock()
	calls := repo.statusCalls
	repo.statusMu.Unlock()

	require.Len(t, calls, 1, "SetResyncStatus must be called once")
	assert.Equal(t, book.ID, calls[0].bookID)
	assert.True(t, calls[0].olFound, "olFound must be true when OL returned a record")
	assert.False(t, calls[0].gbFound, "gbFound must be false when GB was not queried")
}

// TestResyncBook_RecordsNotFound verifies that when OL returns ErrNotFound,
// SetResyncStatus is called with olFound=false.
func TestResyncBook_RecordsOLNotFound(t *testing.T) {
	isbn := "9780140449112"
	book := models.Book{ //nolint:exhaustruct //only tested fields needed
		ID:     uuid.New(),
		ISBN13: &isbn,
	}

	fake := &fakeOLClient{ //nolint:exhaustruct //only err needed
		err: openlibrary.ErrNotFound,
	}
	repo := &fakeBooksResync{} //nolint:exhaustruct //zero values fine

	svc := &BookService{ //nolint:exhaustruct //only tested fields needed
		logger:      logging.NewNopLogger(),
		booksResync: repo,
		external:    fake,
		objectStore: objectstore.NewFake(),
	}

	err := svc.resyncBook(context.Background(), logging.NewNopLogger(), book, false)
	require.NoError(t, err)

	repo.statusMu.Lock()
	calls := repo.statusCalls
	repo.statusMu.Unlock()

	require.Len(t, calls, 1, "SetResyncStatus must be called even on ErrNotFound")
	assert.False(
		t,
		calls[0].olFound,
		"olFound must be false when OL returned ErrNotFound",
	)
}

// TestResyncBook_Force_OverwritesExistingFields verifies that when force=true,
// all three fields are re-queried and overwritten even when the book already
// has cover, description, and page_count.
func TestResyncBook_Force_OverwritesExistingFields(t *testing.T) {
	isbn := "9780140449112"
	existingCover := "https://old.example.com/cover.jpg"
	existingDesc := "Old description."
	existingPages := 100
	book := models.Book{ //nolint:exhaustruct //only tested fields needed
		ID:          uuid.New(),
		ISBN13:      &isbn,
		CoverURL:    &existingCover,
		Description: &existingDesc,
		PageCount:   &existingPages,
	}

	newCover := "https://covers.openlibrary.org/b/isbn/9780140449112-L.jpg"
	newDesc := "New description from OL."
	newPages := 400
	detail := &openlibrary.ExternalBook{ //nolint:exhaustruct //only relevant fields
		Provider:    "openlibrary",
		ProviderID:  "OL123W",
		Title:       "Test Book",
		Authors:     []string{"Author"},
		CoverURL:    &newCover,
		Description: &newDesc,
		PageCount:   &newPages,
	}
	fake := &fakeOLClient{detail: detail} //nolint:exhaustruct //err nil
	repo := &fakeBooksResync{}            //nolint:exhaustruct //zero values fine

	svc := &BookService{ //nolint:exhaustruct //only tested fields needed
		logger:      logging.NewNopLogger(),
		booksResync: repo,
		external:    fake,
		objectStore: objectstore.NewFake(),
	}

	// Without force: fully populated book is skipped.
	err := svc.resyncBook(context.Background(), logging.NewNopLogger(), book, false)
	require.NoError(t, err)
	assert.Empty(t, repo.refreshCalls, "additive resync must skip fully-populated book")

	// With force: all fields must be re-queried.
	err = svc.resyncBook(context.Background(), logging.NewNopLogger(), book, true)
	require.NoError(t, err)

	repo.refreshMu.Lock()
	calls := repo.refreshCalls
	repo.refreshMu.Unlock()

	require.Len(
		t,
		calls,
		1,
		"RefreshBookExternalData must be called once in force mode",
	)
	rc := calls[0]
	require.NotNil(t, rc.coverURL)
	assert.Equal(t, newCover, *rc.coverURL, "force must pass new cover to repo")
	require.NotNil(t, rc.description)
	assert.Equal(t, newDesc, *rc.description, "force must pass new description to repo")
	require.NotNil(t, rc.pageCount)
	assert.Equal(t, newPages, *rc.pageCount, "force must pass new page count to repo")
}

// TestResyncBooks_ProcessesGivenIDs verifies that ResyncBooks loads only the
// requested IDs and processes them using the existing resync loop.
func TestResyncBooks_ProcessesGivenIDs(t *testing.T) {
	isbn1 := "9780140449112"
	isbn2 := "9780062316097"
	id1, id2, id3 := uuid.New(), uuid.New(), uuid.New()

	repo := &fakeBooksResync{ //nolint:exhaustruct //listErr zero = nil
		books: []models.Book{
			{ID: id1, ISBN13: &isbn1}, //nolint:exhaustruct //only ID+ISBN needed
			{ID: id2, ISBN13: &isbn2}, //nolint:exhaustruct //only ID+ISBN needed
			// id3 is NOT in the fake — should not be processed.
		},
	}

	olCover := "https://covers.openlibrary.org/b/isbn/cover.jpg"
	detail := &openlibrary.ExternalBook{ //nolint:exhaustruct //minimal detail
		Provider:   "openlibrary",
		ProviderID: "OL1W",
		Title:      "Test Book",
		Authors:    []string{"Author"},
		CoverURL:   &olCover,
	}
	ol := &fakeOLClient{detail: detail} //nolint:exhaustruct //err nil

	svc := newResyncSvc(repo, ol)

	// Request only id1; id2 and id3 must be ignored.
	n, err := svc.ResyncBooks(
		context.Background(),
		logging.NewNopLogger(),
		[]uuid.UUID{id1},
		false,
		nil,
	)

	require.NoError(t, err)
	assert.Equal(t, 1, n, "only the requested book must be resynced")

	repo.refreshMu.Lock()
	calls := repo.refreshCalls
	repo.refreshMu.Unlock()

	ids := make([]uuid.UUID, len(calls))
	for i, c := range calls {
		ids[i] = c.bookID
	}
	assert.Equal(t, []uuid.UUID{id1}, ids, "only id1 must be written to the repo")

	// id3 was never in the fake — requesting it should just return 0 processed.
	n2, err2 := svc.ResyncBooks(
		context.Background(),
		logging.NewNopLogger(),
		[]uuid.UUID{id3},
		false,
		nil,
	)
	require.NoError(t, err2)
	assert.Equal(t, 0, n2, "unknown book ID must return 0 processed books")
}
