//nolint:testpackage // testing unexported service helpers
package services

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/books/internal/repositories"
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

// --- resolveOrCreateUserBook ---

// TestResolveOrCreateUserBook_UnknownBookID_PropagatesUpsertError exercises
// the create-new-row branch's error path: a book_id with no matching
// books.books row fails user_books' foreign key, and that error must
// propagate rather than being swallowed.
func TestResolveOrCreateUserBook_UnknownBookID_PropagatesUpsertError(t *testing.T) {
	db := testhelper.ConnectTestDB(testhelper.NewTestConfig().DBDsn)
	//nolint:exhaustruct // only the books repo is needed for this call
	s := &BookService{books: repositories.New(db).Books}

	_, _, err := s.resolveOrCreateUserBook(
		context.Background(), "resolve-or-create-fk-test-user", uuid.New(),
	)
	require.Error(t, err)
}
