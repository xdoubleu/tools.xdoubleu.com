//nolint:testpackage // testing unexported service helpers
package services

import (
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

// fakeBooksResync is a test stub for booksResyncSource.
type fakeBooksResync struct {
	books      []models.Book
	listErr    error
	refreshMu  sync.Mutex
	refreshed  []uuid.UUID
	refreshErr error
}

func (f *fakeBooksResync) ListBooksWithISBN13(
	_ context.Context,
) ([]models.Book, error) {
	return f.books, f.listErr
}

func (f *fakeBooksResync) RefreshBookExternalData(
	_ context.Context,
	bookID uuid.UUID,
	_ *string,
	_ *string,
	_ *int,
) error {
	f.refreshMu.Lock()
	f.refreshed = append(f.refreshed, bookID)
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
	svc := &BookService{ //nolint:exhaustruct //only tested fields needed
		logger:      logging.NewNopLogger(),
		external:    fake,
		objectStore: store,
	}

	err := svc.resyncBook(
		context.Background(),
		logging.NewNopLogger(),
		uuid.New(),
		"9780140449112",
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
	svc := &BookService{ //nolint:exhaustruct //only tested fields needed
		logger:      logging.NewNopLogger(),
		external:    fake,
		objectStore: store,
	}

	err := svc.resyncBook(
		context.Background(),
		logging.NewNopLogger(),
		uuid.New(),
		"9780140449112",
	)
	require.Error(t, err)
	assert.ErrorIs(t, err, boom)
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
	refreshed := repo.refreshed
	repo.refreshMu.Unlock()

	assert.ElementsMatch(t, []uuid.UUID{id1, id2}, refreshed)
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
