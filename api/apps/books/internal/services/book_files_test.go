//nolint:testpackage // testing unexported service helpers
package services

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/books/internal/models"
	"tools.xdoubleu.com/apps/books/internal/repositories"
	"tools.xdoubleu.com/internal/database"
	"tools.xdoubleu.com/internal/testhelper"
)

// --- titleFromFilename ---

func TestTitleFromFilename(t *testing.T) {
	cases := []struct {
		filename string
		want     string
	}{
		{"Black Hat Go.pdf", "Black Hat Go"},
		{"Attacking Network Protocols.pdf", "Attacking Network Protocols"},
		{
			"andrew-ng-machine-learning-yearning.pdf",
			"andrew ng machine learning yearning",
		},
		{"ebook-UndisturbedREST_v1.pdf", "ebook UndisturbedREST v1"},
		{"no-meta.pdf", "no meta"},
		{".pdf", ""},
	}
	for _, c := range cases {
		assert.Equal(t, c.want, titleFromFilename(c.filename), c.filename)
	}
}

// --- attachToCatalogBook ---

// TestAttachToCatalogBook_UnknownBook_PropagatesUpsertError exercises the
// create-new-row branch's error path: a book with no matching books.books
// row fails user_books' foreign key, and that error must propagate rather
// than being swallowed.
func TestAttachToCatalogBook_UnknownBook_PropagatesUpsertError(t *testing.T) {
	db := testhelper.ConnectTestDB(testhelper.NewTestConfig().DBDsn)
	//nolint:exhaustruct // only the books repo is needed for this call
	s := &BookService{books: repositories.New(db).Books}

	book := &models.Book{ //nolint:exhaustruct //only fields needed for this call
		ID:    uuid.New(),
		Title: "Unknown Book",
	}
	_, err := s.attachToCatalogBook(
		context.Background(), "attach-catalog-book-fk-test-user", book,
	)
	require.Error(t, err)
}

// TestAttachToCatalogBook_CanceledContext_PropagatesNonNotFoundError exercises
// the "an error other than not-found" branch of the initial GetUserBook
// lookup: a canceled context fails the query with context.Canceled, not
// database.ErrResourceNotFound, and that error must propagate rather than
// being mistaken for "book not yet in the library".
func TestAttachToCatalogBook_CanceledContext_PropagatesNonNotFoundError(t *testing.T) {
	db := testhelper.ConnectTestDB(testhelper.NewTestConfig().DBDsn)
	//nolint:exhaustruct // only the books repo is needed for this call
	s := &BookService{books: repositories.New(db).Books}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	book := &models.Book{ //nolint:exhaustruct //only fields needed for this call
		ID:    uuid.New(),
		Title: "Canceled Context Book",
	}
	_, err := s.attachToCatalogBook(ctx, "attach-catalog-book-canceled-user", book)
	require.Error(t, err)
	assert.False(t, errors.Is(err, database.ErrResourceNotFound))
}
