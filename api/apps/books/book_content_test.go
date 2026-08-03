package books_test

import (
	"context"
	"fmt"
	"testing"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xdoubleu/essentia/v4/pkg/database"

	"tools.xdoubleu.com/apps/books/internal/models"
	booksv1 "tools.xdoubleu.com/gen/books/v1"
)

// articlePageHTML builds an HTML page with enough content for readability
// extraction to accept it.
func articlePageHTML(title string) string {
	const para = `<p>Lorem ipsum dolor sit amet, consectetur adipiscing elit,
sed do eiusmod tempor incididunt ut labore et dolore magna aliqua. Ut enim ad
minim veniam, quis nostrud exercitation ullamco laboris nisi ut aliquip ex ea
commodo consequat. Duis aute irure dolor in reprehenderit in voluptate velit
esse cillum dolore eu fugiat nulla pariatur.</p>`
	return `<html><head><title>` + title + `</title></head><body><article><h1>` +
		title + `</h1>` + para + para + para + `</article></body></html>`
}

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

// seedSourcedBookInLibrary adds a catalog book with the given source URL to
// ownerID's library, for tests exercising the GetBookContent lazy-backfill
// path.
func seedSourcedBookInLibrary(
	t *testing.T, ownerID, sourceURL string,
) *models.Book {
	t.Helper()
	book, err := testApp.Repositories.Books.UpsertBook(
		context.Background(),
		models.Book{ //nolint:exhaustruct //only required fields
			Title:     fmt.Sprintf("unique-sourced-book-%s", uuid.NewString()),
			Authors:   []string{"Test Author"},
			SourceURL: &sourceURL,
		},
	)
	require.NoError(t, err)
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

func TestBookContentRepo_Get_QueryError_ReturnsErr(t *testing.T) {
	book := seedContentBookInLibrary(t, userID)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := testApp.Repositories.Books.GetBookContentHTML(ctx, userID, book.ID)
	require.Error(t, err)
	assert.NotErrorIs(t, err, database.ErrResourceNotFound)
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

func TestBookService_GetContentHTML_PropagatesRepoError(t *testing.T) {
	book := seedContentBookInLibrary(t, userID)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := testApp.Services.Books.GetContentHTML(ctx, userID, book.ID)
	require.Error(t, err)
	assert.NotErrorIs(t, err, database.ErrResourceNotFound)
}

// --- handler-level tests ---

func TestConnectGetBookContent_OK(t *testing.T) {
	client := newBooksTestClient(t)
	book := seedContentBookInLibrary(t, userID)
	require.NoError(t, testApp.Repositories.Books.SetBookContentHTML(
		context.Background(), book.ID, "<p>Article body</p>",
	))

	req := connect.NewRequest(&booksv1.GetBookContentRequest{
		BookId: book.ID.String(),
	})
	req.Header().Set("Cookie", accessToken.String())

	resp, err := client.GetBookContent(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, "<p>Article body</p>", resp.Msg.Html)
}

func TestConnectGetBookContent_NotFound(t *testing.T) {
	client := newBooksTestClient(t)

	req := connect.NewRequest(&booksv1.GetBookContentRequest{
		BookId: uuid.NewString(),
	})
	req.Header().Set("Cookie", accessToken.String())

	_, err := client.GetBookContent(context.Background(), req)
	require.Error(t, err)
	var connectErr *connect.Error
	require.ErrorAs(t, err, &connectErr)
	assert.Equal(t, connect.CodeNotFound, connectErr.Code())
}

func TestConnectGetBookContent_BackfillsMissingContentForSourcedItem(t *testing.T) {
	client := newBooksTestClient(t)
	url := fmt.Sprintf("https://blog.example.com/backfill-%s", uuid.NewString())
	book := seedSourcedBookInLibrary(t, userID, url)
	mockWebFetch.SetHTML(url, articlePageHTML("Backfilled Post"))

	req := connect.NewRequest(&booksv1.GetBookContentRequest{
		BookId: book.ID.String(),
	})
	req.Header().Set("Cookie", accessToken.String())

	resp, err := client.GetBookContent(context.Background(), req)
	require.NoError(t, err)
	assert.Contains(t, resp.Msg.Html, "Lorem ipsum")

	stored, err := testApp.Repositories.Books.GetBookContentHTML(
		context.Background(), userID, book.ID,
	)
	require.NoError(t, err)
	require.NotNil(t, stored)
	assert.Contains(t, *stored, "Lorem ipsum")
}

func TestBackfillContentHTML_LogsWhenPersistFails(t *testing.T) {
	url := fmt.Sprintf("https://blog.example.com/persist-fail-%s", uuid.NewString())
	book := seedSourcedBookInLibrary(t, userID, url)
	mockWebFetch.SetHTML(url, articlePageHTML("Unpersisted Post"))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	html := testApp.Services.Ingest.BackfillContentHTML(ctx, book)
	assert.Contains(t, html, "Lorem ipsum")

	stored, err := testApp.Repositories.Books.GetBookContentHTML(
		context.Background(), userID, book.ID,
	)
	require.NoError(t, err)
	assert.Nil(t, stored)
}

func TestConnectGetBookContent_NoBackfillWhenFetchFails(t *testing.T) {
	client := newBooksTestClient(t)
	url := fmt.Sprintf("https://blog.example.com/missing-%s", uuid.NewString())
	book := seedSourcedBookInLibrary(t, userID, url)
	// No mockWebFetch entry for url: fetch fails, so the empty-state response
	// must be returned as-is rather than erroring.

	req := connect.NewRequest(&booksv1.GetBookContentRequest{
		BookId: book.ID.String(),
	})
	req.Header().Set("Cookie", accessToken.String())

	resp, err := client.GetBookContent(context.Background(), req)
	require.NoError(t, err)
	assert.Empty(t, resp.Msg.Html)
}

func TestConnectGetBookContent_NoBackfillWithoutSourceURL(t *testing.T) {
	client := newBooksTestClient(t)
	book := seedContentBookInLibrary(t, userID)

	req := connect.NewRequest(&booksv1.GetBookContentRequest{
		BookId: book.ID.String(),
	})
	req.Header().Set("Cookie", accessToken.String())

	resp, err := client.GetBookContent(context.Background(), req)
	require.NoError(t, err)
	assert.Empty(t, resp.Msg.Html)
}

func TestConnectGetBookContent_InvalidBookID(t *testing.T) {
	client := newBooksTestClient(t)

	req := connect.NewRequest(&booksv1.GetBookContentRequest{
		BookId: "not-a-uuid",
	})
	req.Header().Set("Cookie", accessToken.String())

	_, err := client.GetBookContent(context.Background(), req)
	require.Error(t, err)
	var connectErr *connect.Error
	require.ErrorAs(t, err, &connectErr)
	assert.Equal(t, connect.CodeInvalidArgument, connectErr.Code())
}
