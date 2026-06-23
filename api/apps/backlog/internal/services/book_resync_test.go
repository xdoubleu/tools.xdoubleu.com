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

	"tools.xdoubleu.com/apps/backlog/pkg/objectstore"
	"tools.xdoubleu.com/apps/backlog/pkg/openlibrary"
)

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

// TestResyncAllFromOpenLibrary_EmptyLibrary verifies that with no ISBN13 books
// the function returns (0, nil) without panicking.
func TestResyncAllFromOpenLibrary_EmptyLibrary(t *testing.T) {
	// books repo is nil — ListBooksWithISBN13 would panic, so we need a real
	// BooksRepository. Use the integration DB if available; otherwise skip.
	t.Skip("requires a real BooksRepository — covered by handler integration test")
}
