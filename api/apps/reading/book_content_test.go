package reading_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xdoubleu/essentia/v4/pkg/database"

	"tools.xdoubleu.com/apps/reading/internal/models"
	readingv1 "tools.xdoubleu.com/gen/reading/v1"
)

// seedContentBookInLibrary adds a fresh catalog book to userID's library, for
// tests exercising SetBookContentHTML/GetBookContentHTML/GetBookContent.
func seedContentBookInLibrary(t *testing.T, ownerID string) *models.Book {
	t.Helper()
	book := addUniqueBook(t)
	require.NoError(t, testApp.Repositories.Books.UpsertUserBook(
		context.Background(),
		models.UserBook{ //nolint:exhaustruct //optional fields
			UserID:         ownerID,
			BookID:         book.ID,
			Status:         models.StatusToRead,
			Tags:           []string{},
			ShelfPositions: map[string]int{},
		},
	))
	return book
}

// --- repository-level tests ---

func TestBookContentRepo_SetAndGet_RoundTrip(t *testing.T) {
	book := seedContentBookInLibrary(t, userID)

	require.NoError(t, testApp.Repositories.Books.SetBookContentHTML(
		context.Background(), book.ID, "<p>Hello world</p>",
	))

	html, err := testApp.Repositories.Books.GetBookContentHTML(
		context.Background(), userID, book.ID,
	)
	require.NoError(t, err)
	require.NotNil(t, html)
	assert.Equal(t, "<p>Hello world</p>", *html)
}

func TestBookContentRepo_Get_NilWhenNeverSet(t *testing.T) {
	book := seedContentBookInLibrary(t, userID)

	html, err := testApp.Repositories.Books.GetBookContentHTML(
		context.Background(), userID, book.ID,
	)
	require.NoError(t, err)
	assert.Nil(t, html)
}

func TestBookContentRepo_Get_OtherUserBook_ReturnsNotFound(t *testing.T) {
	book := seedContentBookInLibrary(t, userID)

	_, err := testApp.Repositories.Books.GetBookContentHTML(
		context.Background(), "other-user-idor-check-getcontent", book.ID,
	)
	assert.ErrorIs(t, err, database.ErrResourceNotFound)
}

func TestBookContentRepo_Get_UnknownBookID_ReturnsNotFound(t *testing.T) {
	_, err := testApp.Repositories.Books.GetBookContentHTML(
		context.Background(), userID, uuid.New(),
	)
	assert.ErrorIs(t, err, database.ErrResourceNotFound)
}

// --- service-level tests ---

func TestBookService_GetContentHTML_EmptyWhenNeverSet(t *testing.T) {
	book := seedContentBookInLibrary(t, userID)

	html, err := testApp.Services.Books.GetContentHTML(
		context.Background(), userID, book.ID,
	)
	require.NoError(t, err)
	assert.Empty(t, html)
}

// --- handler-level tests ---

func TestConnectGetBookContent_OK(t *testing.T) {
	client := newBooksTestClient(t)
	book := seedContentBookInLibrary(t, userID)
	require.NoError(t, testApp.Repositories.Books.SetBookContentHTML(
		context.Background(), book.ID, "<p>Article body</p>",
	))

	req := connect.NewRequest(&readingv1.GetBookContentRequest{
		BookId: book.ID.String(),
	})
	req.Header().Set("Cookie", accessToken.String())

	resp, err := client.GetBookContent(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, "<p>Article body</p>", resp.Msg.Html)
}

func TestConnectGetBookContent_NotFound(t *testing.T) {
	client := newBooksTestClient(t)

	req := connect.NewRequest(&readingv1.GetBookContentRequest{
		BookId: uuid.NewString(),
	})
	req.Header().Set("Cookie", accessToken.String())

	_, err := client.GetBookContent(context.Background(), req)
	require.Error(t, err)
	var connectErr *connect.Error
	require.ErrorAs(t, err, &connectErr)
	assert.Equal(t, connect.CodeNotFound, connectErr.Code())
}

func TestConnectGetBookContent_InvalidBookID(t *testing.T) {
	client := newBooksTestClient(t)

	req := connect.NewRequest(&readingv1.GetBookContentRequest{
		BookId: "not-a-uuid",
	})
	req.Header().Set("Cookie", accessToken.String())

	_, err := client.GetBookContent(context.Background(), req)
	require.Error(t, err)
	var connectErr *connect.Error
	require.ErrorAs(t, err, &connectErr)
	assert.Equal(t, connect.CodeInvalidArgument, connectErr.Code())
}
