package books_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/books/internal/services"
)

// books.books is shared: a second user adding the same ISBN with other
// metadata must not rewrite the catalog row or its cached cover.
func TestAddToLibrary_ExistingISBN_KeepsCatalogMetadata(t *testing.T) {
	isbn := testISBN(uuid.NewString())
	cover := func(body string) *httptest.Server {
		srv := httptest.NewServer(http.HandlerFunc(
			func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(body))
			},
		))
		t.Cleanup(srv.Close)
		return srv
	}
	original := cover("\xff\xd8\xfforiginal-cover")
	defaced := cover("\xff\xd8\xffdefaced-cover")

	first, err := testApp.Services.Books.AddToLibrary(
		context.Background(), userID,
		services.SourceProposal{ //nolint:exhaustruct //only required fields
			Source: "manual", Title: "Original Title",
			Authors: []string{"Original Author"}, ISBN13: isbn,
			CoverURL: original.URL,
		},
		"to-read", []string{},
	)
	require.NoError(t, err)

	second, err := testApp.Services.Books.AddToLibrary(
		context.Background(), mergeTestUser,
		services.SourceProposal{ //nolint:exhaustruct //only required fields
			Source: "manual", Title: "Defaced Title",
			Authors: []string{"Someone Else"}, ISBN13: isbn,
			CoverURL: defaced.URL,
		},
		"to-read", []string{},
	)
	require.NoError(t, err)
	require.Equal(t, first.BookID, second.BookID)

	book, err := testApp.Repositories.Books.GetBookByID(
		context.Background(), first.BookID,
	)
	require.NoError(t, err)
	assert.Equal(t, "Original Title", book.Title)
	assert.Equal(t, []string{"Original Author"}, book.Authors)
	require.NotNil(t, book.CoverURL)
	assert.Equal(t, original.URL, *book.CoverURL)

	data, cached := fakeStore.GetContent(
		"books/" + first.BookID.String() + "/cover.jpg",
	)
	require.True(t, cached)
	assert.Equal(t, "\xff\xd8\xfforiginal-cover", string(data))
}
