package books_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/books/internal/models"
	booksv1 "tools.xdoubleu.com/gen/books/v1"
)

func setBookTag(
	t *testing.T,
	bookID, tag string,
	enabled bool,
) error {
	t.Helper()
	req := connect.NewRequest(&booksv1.SetBookTagRequest{
		BookId: bookID, Tag: tag, Enabled: enabled,
	})
	req.Header().Set("Cookie", accessToken.String())
	_, err := newBooksTestClient(t).SetBookTag(context.Background(), req)
	return err
}

func bookTags(t *testing.T, book *models.UserBook) []string {
	t.Helper()
	got, err := testApp.Services.Books.GetUserBook(
		context.Background(), book.UserID, book.BookID,
	)
	require.NoError(t, err)
	return got.Tags
}

func TestConnectSetBookTag_RepeatIsIdempotent(t *testing.T) {
	book := addTestBook(t, "SetTagBook1")
	id := book.BookID.String()

	for range 2 {
		require.NoError(t, setBookTag(t, id, "poetry", true))
	}
	assert.Equal(t, 1, countTag(bookTags(t, book), "poetry"))

	for range 2 {
		require.NoError(t, setBookTag(t, id, "poetry", false))
	}
	assert.NotContains(t, bookTags(t, book), "poetry")
}

func TestConnectSetBookTag_KoboSync(t *testing.T) {
	book := addTestBook(t, "SetTagBook2")
	id := book.BookID.String()

	require.NoError(t, setBookTag(t, id, models.TagKoboSync, true))
	assert.Contains(t, bookTags(t, book), models.TagKoboSync)
	require.NoError(t, setBookTag(t, id, models.TagKoboSync, false))
	assert.NotContains(t, bookTags(t, book), models.TagKoboSync)
}

func TestConnectSetBookTag_InvalidRequests(t *testing.T) {
	book := addTestBook(t, "SetTagBook3")

	err := setBookTag(t, book.BookID.String(), "", true)
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
	err = setBookTag(t, "not-a-uuid", "poetry", true)
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
	err = setBookTag(t, "00000000-0000-0000-0000-000000000001", "poetry", true)
	assert.Equal(t, connect.CodeInternal, connect.CodeOf(err))
}

func countTag(tags []string, tag string) int {
	n := 0
	for _, t := range tags {
		if t == tag {
			n++
		}
	}
	return n
}
