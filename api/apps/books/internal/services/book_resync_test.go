//nolint:testpackage // testing unexported service helpers
package services

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/books/internal/models"
	"tools.xdoubleu.com/apps/books/internal/repositories"
	"tools.xdoubleu.com/apps/books/pkg/hardcover"
	"tools.xdoubleu.com/apps/books/pkg/objectstore"
	"tools.xdoubleu.com/apps/books/pkg/unicat"
	"tools.xdoubleu.com/internal/database"
	"tools.xdoubleu.com/internal/logging"
)

type refreshCall struct {
	bookID         uuid.UUID
	coverURL       string
	description    string
	pageCount      int
	isbn13         string
	title          string
	authors        []string
	metadataSource string
}

type scanStatusCall struct {
	bookID  uuid.UUID
	ucFound *bool
	hcFound *bool
}

type fakeBooksResync struct {
	books      []models.Book
	listErr    error
	getBookErr error

	mu           sync.Mutex
	replaced     map[uuid.UUID][]byte
	replaceErr   error
	proposalRows map[uuid.UUID]repositories.ResyncProposalRow
	deletedIDs   []uuid.UUID
	deleteErr    error
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
	coverURL string,
	description string,
	pageCount int,
	isbn13 string,
	title string,
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
	ucFound *bool,
	hcFound *bool,
) error {
	f.mu.Lock()
	f.scanStatusCalls = append(f.scanStatusCalls, scanStatusCall{
		bookID: bookID, ucFound: ucFound, hcFound: hcFound,
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
	return f.deleteErr
}

// fakeUCClient is a unicat.Client stub; calls is mutex-guarded since sources
// are queried concurrently.
type fakeUCClient struct {
	searchResults []unicat.ExternalBook
	byISBN        *unicat.ExternalBook
	err           error

	mu    sync.Mutex
	calls int
}

func (f *fakeUCClient) GetByISBN(
	_ context.Context,
	_ string,
) (*unicat.ExternalBook, error) {
	f.mu.Lock()
	f.calls++
	f.mu.Unlock()
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
	f.mu.Lock()
	f.calls++
	f.mu.Unlock()
	return f.searchResults, f.err
}

// fakeHCClient is a hardcover.Client stub; calls is mutex-guarded. byISBNMap
// keys responses per ISBN; byISBN takes priority when both are set.
type fakeHCClient struct {
	searchResults []hardcover.ExternalBook
	byISBN        *hardcover.ExternalBook
	byISBNMap     map[string]*hardcover.ExternalBook
	err           error

	mu    sync.Mutex
	calls int
}

func (f *fakeHCClient) GetByISBN(
	_ context.Context,
	isbn string,
) (*hardcover.ExternalBook, error) {
	f.mu.Lock()
	f.calls++
	f.mu.Unlock()
	if f.err != nil {
		return nil, f.err
	}
	if f.byISBN != nil {
		return f.byISBN, nil
	}
	if f.byISBNMap != nil {
		if r, ok := f.byISBNMap[isbn]; ok {
			return r, nil
		}
		return nil, hardcover.ErrNotFound
	}
	return nil, hardcover.ErrNotFound
}

func (f *fakeHCClient) Search(
	_ context.Context,
	_ string,
) ([]hardcover.ExternalBook, error) {
	f.mu.Lock()
	f.calls++
	f.mu.Unlock()
	return f.searchResults, f.err
}

// failDeleteObjectStore errors on Delete and delegates everything else.
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

// fetchByISBN: sources stay independent, no gap-filling merge.

func TestFetchByISBN_KeepsSourcesIndependent(t *testing.T) {
	isbn := "9780140449112"
	ucDetail := &unicat.ExternalBook{Title: "UC Title"} //nolint:exhaustruct // partial
	hcCover := "https://hardcover.app/cover.jpg"
	hcDetail := &hardcover.ExternalBook{ //nolint:exhaustruct // partial
		Title: "HC Title", CoverURL: &hcCover,
	}

	svc := &BookService{ //nolint:exhaustruct //only resync-path fields needed
		logger:      logging.NewNopLogger(),
		uniCat:      &fakeUCClient{byISBN: ucDetail}, //nolint:exhaustruct // partial
		hardcover:   &fakeHCClient{byISBN: hcDetail}, //nolint:exhaustruct // partial
		objectStore: objectstore.NewFake(),
	}

	proposals, unresolved := svc.fetchByISBN(
		context.Background(), logging.NewNopLogger(),
		models.Book{ISBN13: &isbn}, nil, //nolint:exhaustruct // partial
	)

	require.Len(t, proposals, 2, "both providers returned a record; no merge")
	assert.Empty(t, unresolved, "every provider answered cleanly")
	assert.Equal(t, "unicat", proposals[0].Source)
	assert.Equal(t, "UC Title", proposals[0].Title)
	assert.Equal(t, "hardcover", proposals[1].Source)
	assert.Equal(t, "HC Title", proposals[1].Title)
	assert.Equal(t, hcCover, proposals[1].CoverURL)
}

func TestFetchByISBN_NotFound_Skipped(t *testing.T) {
	svc := &BookService{ //nolint:exhaustruct // partial
		logger: logging.NewNopLogger(),
		//nolint:exhaustruct // byISBN unused, err drives the not-found path
		hardcover:   &fakeHCClient{err: hardcover.ErrNotFound},
		objectStore: objectstore.NewFake(),
	}

	isbn := "9780140449112"
	proposals, unresolved := svc.fetchByISBN(
		context.Background(),
		logging.NewNopLogger(),
		models.Book{ISBN13: &isbn}, //nolint:exhaustruct // partial
		nil,
	)
	assert.Empty(t, proposals)
	assert.Empty(t, unresolved, "a clean not-found is resolved, not unresolved")
}

// fetchByISBN: an errored/skipped source is unresolved, never "false", so the
// DB flag is preserved.

func TestFetchByISBN_HardcoverErrors_MarkedUnresolved(t *testing.T) {
	//nolint:exhaustruct // partial
	uc := &fakeUCClient{byISBN: &unicat.ExternalBook{Title: "UC Title"}}
	//nolint:exhaustruct // partial
	hc := &fakeHCClient{err: errors.New("boom")}
	svc := &BookService{ //nolint:exhaustruct // partial
		logger:      logging.NewNopLogger(),
		uniCat:      uc,
		hardcover:   hc,
		objectStore: objectstore.NewFake(),
	}

	isbn := "9780140449112"
	proposals, unresolved := svc.fetchByISBN(
		context.Background(), logging.NewNopLogger(),
		models.Book{ISBN13: &isbn}, nil, //nolint:exhaustruct // partial
	)
	require.Len(t, proposals, 1, "UniCat still succeeds independently of HC's error")
	assert.True(t, unresolved["hardcover"],
		"an errored source must be unresolved, not a false miss")
}

func TestFetchByISBN_HardcoverKnown_SkippedAndUnresolved(t *testing.T) {
	//nolint:exhaustruct // partial
	hc := &fakeHCClient{byISBN: &hardcover.ExternalBook{Title: "HC Title"}}
	svc := &BookService{ //nolint:exhaustruct // partial
		logger:      logging.NewNopLogger(),
		hardcover:   hc,
		objectStore: objectstore.NewFake(),
	}
	opts := &scanOptions{
		known: map[string]bool{"hardcover": true},
	}

	isbn := "9780140449112"
	proposals, unresolved := svc.fetchByISBN(
		context.Background(), logging.NewNopLogger(),
		models.Book{ISBN13: &isbn}, opts, //nolint:exhaustruct // partial
	)
	assert.Empty(t, proposals)
	assert.True(t, unresolved["hardcover"], "a skipped source must be unresolved")
	hc.mu.Lock()
	defer hc.mu.Unlock()
	assert.Zero(t, hc.calls, "an already-known source must not be re-queried")
}

// TestBuildResyncProposals_ForceHardcover_BypassesCache: after the first scan
// every book has a known hardcover_found, so force must bypass the cache.
func TestBuildResyncProposals_ForceHardcover_BypassesCache(t *testing.T) {
	id := uuid.New()
	isbn := "9780140449112"
	hcFoundFalse := false
	book := models.Book{ //nolint:exhaustruct // partial
		ID: id, Title: "Stuck Book", ISBN13: &isbn, HardcoverFound: &hcFoundFalse,
	}
	repo := &fakeBooksResync{books: []models.Book{book}} //nolint:exhaustruct // partial
	//nolint:exhaustruct // partial
	hc := &fakeHCClient{byISBN: &hardcover.ExternalBook{Title: "HC Title"}}
	svc := &BookService{ //nolint:exhaustruct // partial
		logger:       logging.NewNopLogger(),
		resyncSource: repo,
		hardcover:    hc,
		objectStore:  objectstore.NewFake(),
	}

	_, err := svc.BuildResyncProposals(
		context.Background(), logging.NewNopLogger(), nil, false,
	)
	require.NoError(t, err)
	hc.mu.Lock()
	callsBeforeForce := hc.calls
	hc.mu.Unlock()
	assert.Zero(t, callsBeforeForce,
		"without force, a known (even false) HC flag must keep skipping HC")

	_, err = svc.BuildResyncProposals(
		context.Background(), logging.NewNopLogger(), nil, true,
	)
	require.NoError(t, err)
	hc.mu.Lock()
	defer hc.mu.Unlock()
	assert.Equal(t, 1, hc.calls,
		"force must bypass the skip-if-known cache and query HC")
}

// TestBuildResyncProposals_SkipsKnownUniCat_UnlessForced: the cache applies to
// UniCat too.
func TestBuildResyncProposals_SkipsKnownUniCat_UnlessForced(t *testing.T) {
	id := uuid.New()
	isbn := "9780140449112"
	ucFoundTrue := true
	book := models.Book{ //nolint:exhaustruct // partial
		ID: id, Title: "Known Book", ISBN13: &isbn, UniCatFound: &ucFoundTrue,
	}
	repo := &fakeBooksResync{books: []models.Book{book}} //nolint:exhaustruct // partial
	//nolint:exhaustruct // partial
	ucClient := &fakeUCClient{byISBN: &unicat.ExternalBook{Title: "UC Title"}}
	svc := &BookService{ //nolint:exhaustruct // partial
		logger:       logging.NewNopLogger(),
		resyncSource: repo,
		uniCat:       ucClient,
		objectStore:  objectstore.NewFake(),
	}

	_, err := svc.BuildResyncProposals(
		context.Background(), logging.NewNopLogger(), nil, false,
	)
	require.NoError(t, err)
	assert.Zero(t, ucClient.calls,
		"without force, a known UniCat flag must skip the UniCat call")

	_, err = svc.BuildResyncProposals(
		context.Background(), logging.NewNopLogger(), nil, true,
	)
	require.NoError(t, err)
	assert.Equal(t, 1, ucClient.calls,
		"force must bypass the skip-if-known cache and query UniCat")
}

func TestFetchSourceProposals_DispatchesOnISBNPresence(t *testing.T) {
	//nolint:exhaustruct // partial
	svc := &BookService{
		logger: logging.NewNopLogger(),
		//nolint:exhaustruct // byISBN unused, err drives the not-found path
		hardcover:   &fakeHCClient{err: hardcover.ErrNotFound},
		objectStore: objectstore.NewFake(),
	}
	ctx := context.Background()

	isbn := "9780140449112"
	withISBN := models.Book{ISBN13: &isbn} //nolint:exhaustruct // partial
	proposals, _ := svc.fetchSourceProposals(ctx, logging.NewNopLogger(), withISBN, nil)
	assert.Empty(t, proposals)

	bare := models.Book{} //nolint:exhaustruct // partial
	proposals, _ = svc.fetchSourceProposals(ctx, logging.NewNopLogger(), bare, nil)
	assert.Empty(t, proposals)
}

// fetchBySearch: match guards.

func TestFetchBySearch_TitleAuthorMatch_Accepted(t *testing.T) {
	book := models.Book{ //nolint:exhaustruct // partial
		Title: "Dune", Authors: []string{"Frank Herbert"},
	}
	hcFake := &fakeHCClient{ //nolint:exhaustruct //only relevant fields
		searchResults: []hardcover.ExternalBook{
			//nolint:exhaustruct // title/authors are all this test checks
			{Title: "Dune", Authors: []string{"Frank Herbert"}},
		},
	}
	svc := &BookService{ //nolint:exhaustruct // partial
		logger:      logging.NewNopLogger(),
		hardcover:   hcFake,
		objectStore: objectstore.NewFake(),
	}

	proposals, _ := svc.fetchBySearch(
		context.Background(),
		logging.NewNopLogger(),
		book,
		nil,
	)
	require.Len(t, proposals, 1)
	assert.Equal(t, "hardcover", proposals[0].Source)
}

func TestFetchBySearch_TitleMismatch_Rejected(t *testing.T) {
	book := models.Book{ //nolint:exhaustruct // partial
		Title: "Dune", Authors: []string{"Frank Herbert"},
	}
	hcFake := &fakeHCClient{ //nolint:exhaustruct //only relevant fields
		searchResults: []hardcover.ExternalBook{
			{ //nolint:exhaustruct // title/authors are all this test checks
				Title:   "Totally Different Book",
				Authors: []string{"Frank Herbert"},
			},
		},
	}
	svc := &BookService{ //nolint:exhaustruct // partial
		logger:      logging.NewNopLogger(),
		hardcover:   hcFake,
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
	hcFake := &fakeHCClient{ //nolint:exhaustruct //only searchResults
		searchResults: []hardcover.ExternalBook{
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
		hardcover:   hcFake,
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

func TestFetchBySearch_UniCatAndHardcover_BothMatch(t *testing.T) {
	//nolint:exhaustruct // partial
	book := models.Book{Title: "Dune", Authors: []string{"Frank Herbert"}}
	ucFake := &fakeUCClient{ //nolint:exhaustruct // partial
		searchResults: []unicat.ExternalBook{
			{ //nolint:exhaustruct // title/authors are all this test checks
				Title:   "Dune",
				Authors: []string{"Frank Herbert"},
			},
		},
	}
	hcFake := &fakeHCClient{ //nolint:exhaustruct // partial
		searchResults: []hardcover.ExternalBook{
			{ //nolint:exhaustruct // title/authors are all this test checks
				Title:   "Dune",
				Authors: []string{"Frank Herbert"},
			},
		},
	}
	svc := &BookService{ //nolint:exhaustruct // partial
		logger:      logging.NewNopLogger(),
		uniCat:      ucFake,
		hardcover:   hcFake,
		objectStore: objectstore.NewFake(),
	}

	proposals, _ := svc.fetchBySearch(
		context.Background(),
		logging.NewNopLogger(),
		book,
		nil,
	)
	require.Len(t, proposals, 2, "both UC and HC matched")
	assert.Equal(t, "unicat", proposals[0].Source)
	assert.Equal(t, "hardcover", proposals[1].Source)
}

// searchProviders: skip-if-known applies on the search path too.

func TestFetchBySearch_HardcoverKnown_SkippedAndUnresolved(t *testing.T) {
	book := models.Book{Title: "Dune"} //nolint:exhaustruct // partial
	//nolint:exhaustruct // partial
	hc := &fakeHCClient{
		searchResults: []hardcover.ExternalBook{
			{Title: "Dune"}, //nolint:exhaustruct // partial
		},
	}
	svc := &BookService{ //nolint:exhaustruct // partial
		logger:      logging.NewNopLogger(),
		hardcover:   hc,
		objectStore: objectstore.NewFake(),
	}
	opts := &scanOptions{
		known: map[string]bool{"hardcover": true},
	}

	proposals, unresolved := svc.fetchBySearch(
		context.Background(), logging.NewNopLogger(), book, opts,
	)
	assert.Empty(t, proposals)
	assert.True(t, unresolved["hardcover"])
	hc.mu.Lock()
	defer hc.mu.Unlock()
	assert.Zero(t, hc.calls, "an already-known source must not be re-queried")
}

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

	agree := SourceProposal{ //nolint:exhaustruct // partial
		Title: "Dune", Authors: []string{"Frank Herbert"}, PageCount: pages,
	}
	assert.Empty(t, computeDifferences(book, agree))

	titleDiff := SourceProposal{ //nolint:exhaustruct // partial
		Title: "Different Title",
	}
	assert.Contains(t, computeDifferences(book, titleDiff), "title")

	pageDiff := SourceProposal{PageCount: 999} //nolint:exhaustruct // partial
	assert.Contains(t, computeDifferences(book, pageDiff), "page_count")

	// Library has no description: any source value counts.
	//nolint:exhaustruct // partial
	descDiff := SourceProposal{Description: "A new description."}
	assert.Contains(t, computeDifferences(book, descDiff), "description")

	//nolint:exhaustruct // partial
	coverDiff := SourceProposal{CoverURL: "https://elsewhere.example.com/x.jpg"}
	assert.NotContains(t, computeDifferences(book, coverDiff), "cover_url")

	isbnDiff := SourceProposal{ISBN13: "9780062316097"} //nolint:exhaustruct // partial
	assert.NotContains(t, computeDifferences(book, isbnDiff), "isbn13")

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

// TestBuildResyncProposals_FlagsOnlyMoreCompleteSource: a merely different
// source is never flagged, only a strictly more complete one.
func TestBuildResyncProposals_FlagsOnlyMoreCompleteSource(t *testing.T) {
	idMereDiff, idMoreComplete := uuid.New(), uuid.New()
	isbnA, isbnB := "9780140449112", "9780062316097"
	existingDesc := "Already has a description."

	repo := &fakeBooksResync{ //nolint:exhaustruct //zero values fine
		books: []models.Book{
			{ //nolint:exhaustruct // partial
				ID: idMereDiff, Title: "Has A Description", ISBN13: &isbnA,
				Description: &existingDesc,
			},
			//nolint:exhaustruct // partial
			{ID: idMoreComplete, Title: "Missing A Description", ISBN13: &isbnB},
		},
	}

	svc := &BookService{ //nolint:exhaustruct // partial
		logger:       logging.NewNopLogger(),
		resyncSource: repo,
		hardcover: &fakeHCClient{ //nolint:exhaustruct // partial
			byISBNMap: map[string]*hardcover.ExternalBook{
				isbnA: {
					Title: "A Totally Different Title",
				},
				isbnB: {
					Title:       "Missing A Description",
					Description: strPtr("A description HC actually has."),
					ISBN13:      &isbnB,
				},
			},
		},
		objectStore: objectstore.NewFake(),
	}

	// onProgress runs from concurrent goroutines.
	var callsMu sync.Mutex
	var calls [][2]int
	n, err := svc.BuildResyncProposals(
		context.Background(),
		logging.NewNopLogger(),
		func(processed, total int) {
			callsMu.Lock()
			defer callsMu.Unlock()
			calls = append(calls, [2]int{processed, total})
		},
		false,
	)
	require.NoError(t, err)
	assert.Equal(
		t,
		1,
		n,
		"only the book a source can genuinely complete should be flagged",
	)
	require.Len(t, calls, 3, "one (0,total) call plus one per book")
	assert.Equal(t, [2]int{0, 2}, calls[0])

	require.Len(t, repo.replaced, 1)
	_, ok := repo.replaced[idMoreComplete]
	assert.True(t, ok, "the more-complete-source book must be in the replacement set")
	_, mereDiffStillThere := repo.replaced[idMereDiff]
	assert.False(t, mereDiffStillThere,
		"a source that merely differs, without adding fields, must not be flagged")
}

// TestBuildResyncProposals_FlagsNotFoundAnywhere: a searchable book no source
// finds is flagged with zero sources (a coverage gap).
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
		logger:       logging.NewNopLogger(),
		resyncSource: repo,
		//nolint:exhaustruct // partial
		hardcover:   &fakeHCClient{err: hardcover.ErrNotFound},
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

// TestBuildResyncProposals_NeverAttempted_NotFlagged: an unsearchable book is
// never flagged.
func TestBuildResyncProposals_NeverAttempted_NotFlagged(t *testing.T) {
	id := uuid.New()
	repo := &fakeBooksResync{ //nolint:exhaustruct //zero values fine
		books: []models.Book{{ID: id}}, //nolint:exhaustruct // no ISBN, no title
	}
	svc := &BookService{ //nolint:exhaustruct // partial
		logger:       logging.NewNopLogger(),
		resyncSource: repo,
		//nolint:exhaustruct // partial
		hardcover:   &fakeHCClient{err: hardcover.ErrNotFound},
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
		logger:       logging.NewNopLogger(),
		resyncSource: repo,
		hardcover:    &fakeHCClient{}, //nolint:exhaustruct //zero values fine
		objectStore:  objectstore.NewFake(),
	}

	var calls [][2]int
	n, err := svc.BuildResyncProposals(
		context.Background(),
		logging.NewNopLogger(),
		func(processed, total int) {
			calls = append(calls, [2]int{processed, total})
		},
		false,
	)
	require.NoError(t, err)
	assert.Equal(t, 0, n)
	require.Len(t, calls, 1)
	assert.Equal(t, [2]int{0, 0}, calls[0])
}

// TestBuildResyncProposals_Cancelled_SkipsProposalsReplace: cancel must not
// overwrite the proposals table.
func TestBuildResyncProposals_Cancelled_SkipsProposalsReplace(t *testing.T) {
	book := models.Book{ID: uuid.New(), Title: "Dune"}   //nolint:exhaustruct // partial
	repo := &fakeBooksResync{books: []models.Book{book}} //nolint:exhaustruct // partial
	svc := &BookService{                                 //nolint:exhaustruct // partial
		logger:       logging.NewNopLogger(),
		resyncSource: repo,
		hardcover:    &fakeHCClient{}, //nolint:exhaustruct //zero values fine
		objectStore:  objectstore.NewFake(),
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	n, err := svc.BuildResyncProposals(ctx, logging.NewNopLogger(), nil, false)
	require.NoError(t, err, "a cancel is not a failure")
	assert.Equal(t, 0, n)
	assert.Nil(
		t,
		repo.replaced,
		"ReplaceResyncProposals must not run on a cancelled scan",
	)
}

func TestBuildResyncProposals_ListError(t *testing.T) {
	listErr := errors.New("connection refused")
	repo := &fakeBooksResync{listErr: listErr} //nolint:exhaustruct // partial
	svc := &BookService{                       //nolint:exhaustruct // partial
		logger:       logging.NewNopLogger(),
		resyncSource: repo,
		objectStore:  objectstore.NewFake(),
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

func TestBuildResyncProposals_ReplaceError(t *testing.T) {
	replaceErr := errors.New("write failed")
	book := models.Book{ID: uuid.New(), Title: "Dune"} //nolint:exhaustruct // partial
	repo := &fakeBooksResync{                          //nolint:exhaustruct // partial
		books:      []models.Book{book},
		replaceErr: replaceErr,
	}
	svc := &BookService{ //nolint:exhaustruct // partial
		logger:       logging.NewNopLogger(),
		resyncSource: repo,
		hardcover:    &fakeHCClient{}, //nolint:exhaustruct //zero values fine
		objectStore:  objectstore.NewFake(),
	}

	n, err := svc.BuildResyncProposals(
		context.Background(),
		logging.NewNopLogger(),
		nil,
		false,
	)
	require.ErrorIs(t, err, replaceErr)
	assert.Equal(t, 0, n)
}

// ListResyncProposals: Differs recomputed at read time.

func TestListResyncProposals_RecomputesDiffers(t *testing.T) {
	bookID := uuid.New()
	book := models.Book{ID: bookID, Title: "Dune"} //nolint:exhaustruct // partial

	raw, err := json.Marshal([]SourceProposal{
		{ //nolint:exhaustruct // partial
			Source: "hardcover",
			Title:  "Different Title",
		},
	})
	require.NoError(t, err)

	repo := &fakeBooksResync{ //nolint:exhaustruct //zero values fine
		proposalRows: map[uuid.UUID]repositories.ResyncProposalRow{
			bookID: {Book: book, ProposalsJSON: raw},
		},
	}
	svc := &BookService{resyncSource: repo} //nolint:exhaustruct // partial

	proposals, err := svc.ListResyncProposals(context.Background())
	require.NoError(t, err)
	require.Len(t, proposals, 1)
	require.Len(t, proposals[0].Sources, 1)
	assert.Contains(t, proposals[0].Sources[0].Differs, "title")
	assert.Equal(t, "Dune", proposals[0].Library.Title)
}

func TestApplyResyncChoice_KeepLibrary_DismissesWithoutWriting(t *testing.T) {
	bookID := uuid.New()
	book := models.Book{ID: bookID, Title: "Dune"} //nolint:exhaustruct // partial
	raw, err := json.Marshal([]SourceProposal{
		{Source: "hardcover", Title: "Other Title"}, //nolint:exhaustruct // partial
	})
	require.NoError(t, err)

	repo := &fakeBooksResync{ //nolint:exhaustruct //zero values fine
		proposalRows: map[uuid.UUID]repositories.ResyncProposalRow{
			bookID: {Book: book, ProposalsJSON: raw},
		},
	}
	svc := &BookService{resyncSource: repo} //nolint:exhaustruct // partial

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
			Source: "hardcover", Title: "New Title", Description: "New desc",
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
		resyncSource: repo,
		objectStore:  objectstore.NewFake(),
	}

	err = svc.ApplyResyncChoice(
		context.Background(), logging.NewNopLogger(), bookID, "hardcover",
	)
	require.NoError(t, err)

	require.Len(t, repo.refreshCalls, 1)
	rc := repo.refreshCalls[0]
	assert.Equal(t, "New Title", rc.title)
	assert.Equal(t, "New desc", rc.description)
	assert.Equal(t, 42, rc.pageCount)
	assert.Equal(t, "hardcover", rc.metadataSource,
		"applying a source must record it as the book's metadata source")
	assert.Equal(t, []uuid.UUID{bookID}, repo.deletedIDs)
}

func TestApplyResyncChoice_ChosenSource_BlanksFieldsSourceLacks(t *testing.T) {
	bookID := uuid.New()
	oldDesc := "Old desc"
	oldPages := 99
	book := models.Book{ //nolint:exhaustruct // partial
		ID:          bookID,
		Title:       "Old Title",
		Description: &oldDesc,
		PageCount:   &oldPages,
	}
	// Blank fields from the chosen source must overwrite existing values.
	raw, err := json.Marshal([]SourceProposal{
		{Source: "unicat", Title: "New Title"}, //nolint:exhaustruct // partial
	})
	require.NoError(t, err)

	repo := &fakeBooksResync{ //nolint:exhaustruct //zero values fine
		proposalRows: map[uuid.UUID]repositories.ResyncProposalRow{
			bookID: {Book: book, ProposalsJSON: raw},
		},
	}
	svc := &BookService{ //nolint:exhaustruct // partial
		resyncSource: repo,
		objectStore:  objectstore.NewFake(),
	}

	err = svc.ApplyResyncChoice(
		context.Background(), logging.NewNopLogger(), bookID, "unicat",
	)
	require.NoError(t, err)

	require.Len(t, repo.refreshCalls, 1)
	rc := repo.refreshCalls[0]
	assert.Equal(t, "New Title", rc.title)
	assert.Empty(t, rc.description,
		"a field the chosen source doesn't supply must be blanked, not kept")
	assert.Zero(t, rc.pageCount,
		"a field the chosen source doesn't supply must be blanked, not kept")
}

func TestApplyResyncChoice_PassesSourceISBNThrough(t *testing.T) {
	bookID := uuid.New()
	existingISBN := "9780140449112"
	book := models.Book{ //nolint:exhaustruct // partial
		ID:     bookID,
		ISBN13: &existingISBN,
	}
	raw, err := json.Marshal([]SourceProposal{
		{ //nolint:exhaustruct // partial
			Source: "hardcover",
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
		resyncSource: repo,
		objectStore:  objectstore.NewFake(),
	}

	err = svc.ApplyResyncChoice(
		context.Background(), logging.NewNopLogger(), bookID, "hardcover",
	)
	require.NoError(t, err)

	// The duplicate-ISBN guard lives in RefreshBookExternalData's SQL, not here.
	require.Len(t, repo.refreshCalls, 1)
	assert.Equal(t, "9780062316097", repo.refreshCalls[0].isbn13)
}

func TestApplyResyncChoice_UnknownBook_ErrProposalNotFound(t *testing.T) {
	repo := &fakeBooksResync{}              //nolint:exhaustruct //zero values fine
	svc := &BookService{resyncSource: repo} //nolint:exhaustruct // partial

	err := svc.ApplyResyncChoice(
		context.Background(), logging.NewNopLogger(), uuid.New(), "hardcover",
	)
	require.ErrorIs(t, err, ErrProposalNotFound)
}

func TestApplyResyncChoice_UnknownSource_ErrProposalNotFound(t *testing.T) {
	bookID := uuid.New()
	book := models.Book{ID: bookID} //nolint:exhaustruct // partial
	raw, err := json.Marshal([]SourceProposal{
		{Source: "hardcover", Title: "X"}, //nolint:exhaustruct // partial
	})
	require.NoError(t, err)

	repo := &fakeBooksResync{ //nolint:exhaustruct //zero values fine
		proposalRows: map[uuid.UUID]repositories.ResyncProposalRow{
			bookID: {Book: book, ProposalsJSON: raw},
		},
	}
	svc := &BookService{resyncSource: repo} //nolint:exhaustruct // partial

	err = svc.ApplyResyncChoice(
		context.Background(), logging.NewNopLogger(), bookID, "unknownsource",
	)
	require.ErrorIs(t, err, ErrProposalNotFound)
}

// GetBookSources / SyncBookSource: live per-book fetch, no prior scan needed.

func TestGetBookSources_ReturnsLiveProposal(t *testing.T) {
	bookID := uuid.New()
	book := models.Book{ID: bookID, Title: "Dune"} //nolint:exhaustruct // partial
	//nolint:exhaustruct // partial
	hcDetail := hardcover.ExternalBook{
		Title:   "Dune",
		Authors: []string{"Frank Herbert"},
	}

	repo := &fakeBooksResync{ //nolint:exhaustruct //zero values fine
		books: []models.Book{book},
	}
	svc := &BookService{ //nolint:exhaustruct // partial
		resyncSource: repo,
		hardcover: &fakeHCClient{ //nolint:exhaustruct //only relevant fields
			searchResults: []hardcover.ExternalBook{hcDetail},
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
	assert.Equal(t, "hardcover", proposal.Sources[0].Source)
	assert.Contains(t, proposal.Sources[0].Differs, "authors")
}

func TestGetBookSources_UnknownBook_ErrProposalNotFound(t *testing.T) {
	repo := &fakeBooksResync{}              //nolint:exhaustruct //zero values fine
	svc := &BookService{resyncSource: repo} //nolint:exhaustruct // partial

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
	// The on-demand path always searches by title+author, so the title must match.
	//nolint:exhaustruct // partial
	hcDetail := hardcover.ExternalBook{
		Title:   "Old Title",
		Authors: []string{"Author"},
	}

	repo := &fakeBooksResync{ //nolint:exhaustruct //zero values fine
		books: []models.Book{book},
		proposalRows: map[uuid.UUID]repositories.ResyncProposalRow{
			bookID: {Book: book, ProposalsJSON: []byte("[]")},
		},
	}
	svc := &BookService{ //nolint:exhaustruct // partial
		resyncSource: repo,
		hardcover: &fakeHCClient{ //nolint:exhaustruct //only relevant fields
			searchResults: []hardcover.ExternalBook{hcDetail},
		},
		objectStore: objectstore.NewFake(),
	}

	err := svc.SyncBookSource(
		context.Background(), logging.NewNopLogger(), bookID, "hardcover", 0, "", "",
	)
	require.NoError(t, err)

	require.Len(t, repo.refreshCalls, 1)
	assert.Equal(t, "Old Title", repo.refreshCalls[0].title)
	assert.Equal(t, []string{"Author"}, repo.refreshCalls[0].authors)
	assert.Equal(t, "hardcover", repo.refreshCalls[0].metadataSource)
	assert.Equal(t, []uuid.UUID{bookID}, repo.deletedIDs,
		"applying live should also clear any pending wizard proposal")
}

func TestSyncBookSource_ClearPendingProposalError_NonFatal(t *testing.T) {
	bookID := uuid.New()
	book := models.Book{ID: bookID, Title: "Old Title"} //nolint:exhaustruct // partial
	//nolint:exhaustruct // partial
	hcDetail := hardcover.ExternalBook{
		Title:   "Old Title",
		Authors: []string{"Author"},
	}

	repo := &fakeBooksResync{ //nolint:exhaustruct //zero values fine
		books:     []models.Book{book},
		deleteErr: errors.New("proposal delete failed"),
	}
	svc := &BookService{ //nolint:exhaustruct // partial
		resyncSource: repo,
		hardcover: &fakeHCClient{ //nolint:exhaustruct //only relevant fields
			searchResults: []hardcover.ExternalBook{hcDetail},
		},
		objectStore: objectstore.NewFake(),
	}

	err := svc.SyncBookSource(
		context.Background(), logging.NewNopLogger(), bookID, "hardcover", 0, "", "",
	)
	require.NoError(t, err, "a failed pending-proposal cleanup must not fail the sync")
	require.Len(t, repo.refreshCalls, 1)
	assert.Equal(t, []uuid.UUID{bookID}, repo.deletedIDs)
}

func TestSyncBookSource_UnknownSource_ErrProposalNotFound(t *testing.T) {
	bookID := uuid.New()
	book := models.Book{ID: bookID} //nolint:exhaustruct // partial
	repo := &fakeBooksResync{       //nolint:exhaustruct //zero values fine
		books: []models.Book{book},
	}
	svc := &BookService{ //nolint:exhaustruct // partial
		resyncSource: repo,
		//nolint:exhaustruct // byISBN unused, err drives the not-found path
		hardcover:   &fakeHCClient{err: hardcover.ErrNotFound},
		objectStore: objectstore.NewFake(),
	}

	err := svc.SyncBookSource(
		context.Background(), logging.NewNopLogger(), bookID, "hardcover", 0, "", "",
	)
	require.ErrorIs(t, err, ErrProposalNotFound)
}

// TestWriteResyncResult_ClearCoverCacheErrors_NonFatal: a failed R2 delete
// doesn't fail the apply.
func TestWriteResyncResult_ClearCoverCacheErrors_NonFatal(t *testing.T) {
	bookID := uuid.New()
	oldCover := "https://example.com/old.jpg"
	book := models.Book{ID: bookID, CoverURL: &oldCover} //nolint:exhaustruct // partial

	repo := &fakeBooksResync{} //nolint:exhaustruct //zero values fine
	store := failDeleteObjectStore{inner: objectstore.NewFake()}
	svc := &BookService{ //nolint:exhaustruct // partial
		logger:       logging.NewNopLogger(),
		resyncSource: repo,
		objectStore:  store,
	}

	err := svc.writeResyncResult(
		context.Background(), logging.NewNopLogger(), book,
		"", "", 0, "", "New Title", nil, "hardcover",
	)
	require.NoError(t, err, "a cover-cache-clear failure must not fail the apply")
}

// TestWriteResyncResult_RefreshError_Propagates checks refresh errors abort the apply.
func TestWriteResyncResult_RefreshError_Propagates(t *testing.T) {
	refreshErr := errors.New("refresh failed")
	book := models.Book{ID: uuid.New()} //nolint:exhaustruct // partial
	repo := &fakeBooksResync{           //nolint:exhaustruct //zero values fine
		refreshErr: refreshErr,
	}
	svc := &BookService{ //nolint:exhaustruct // partial
		logger:       logging.NewNopLogger(),
		resyncSource: repo,
		objectStore:  objectstore.NewFake(),
	}

	err := svc.writeResyncResult(
		context.Background(), logging.NewNopLogger(), book,
		"", "", 0, "", "New Title", nil, "hardcover",
	)
	require.ErrorIs(t, err, refreshErr)
}
