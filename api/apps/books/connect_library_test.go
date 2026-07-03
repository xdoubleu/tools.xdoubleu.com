package books_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/books/internal/models"
	booksv1 "tools.xdoubleu.com/gen/books/v1"
	booksv1connect "tools.xdoubleu.com/gen/books/v1/booksv1connect"
)

func newBooksTestClient(t *testing.T) booksTestClient {
	t.Helper()
	ts := httptest.NewServer(getRoutes())
	t.Cleanup(ts.Close)
	return newBooksClientFor(ts.URL, connect.WithHTTPGet())
}

// booksTestClient bundles all four books service clients so tests can call
// any RPC through one value, mirroring the pre-split single-service client.
type booksTestClient struct {
	booksv1connect.LibraryServiceClient
	booksv1connect.BookFilesServiceClient
	booksv1connect.KoboServiceClient
	booksv1connect.CatalogServiceClient
}

// newBooksClientFor builds a composite client against the given base URL.
func newBooksClientFor(url string, opts ...connect.ClientOption) booksTestClient {
	return booksTestClient{
		LibraryServiceClient: booksv1connect.NewLibraryServiceClient(
			http.DefaultClient, url, opts...,
		),
		BookFilesServiceClient: booksv1connect.NewBookFilesServiceClient(
			http.DefaultClient, url, opts...,
		),
		KoboServiceClient: booksv1connect.NewKoboServiceClient(
			http.DefaultClient, url, opts...,
		),
		CatalogServiceClient: booksv1connect.NewCatalogServiceClient(
			http.DefaultClient, url, opts...,
		),
	}
}

func TestConnectGetLibrary(t *testing.T) {
	client := newBooksTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := connect.NewRequest(&booksv1.GetLibraryRequest{})
	req.Header().Set("Cookie", accessToken.String())

	resp, err := client.GetLibrary(ctx, req)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.NotNil(t, resp.Msg.Library)
	// Library may have books from previous tests, just check it's not nil
}

func TestConnectGetLibrary_WithBooks(t *testing.T) {
	book := addTestBook(t, "Test Book")
	require.NotNil(t, book)

	client := newBooksTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := connect.NewRequest(&booksv1.GetLibraryRequest{})
	req.Header().Set("Cookie", accessToken.String())

	resp, err := client.GetLibrary(ctx, req)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.NotNil(t, resp.Msg.Library)
	assert.NotEmpty(t, resp.Msg.Library.Wishlist, "book should be in wishlist")
}

func TestConnectGetBooksProgress_DefaultRange(t *testing.T) {
	client := newBooksTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := connect.NewRequest(
		&booksv1.GetBooksProgressRequest{},
	)
	req.Header().Set("Cookie", accessToken.String())

	resp, err := client.GetBooksProgress(ctx, req)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.NotNil(t, resp.Msg.Progress)
	assert.NotEmpty(t, resp.Msg.Progress.DateStart)
	assert.NotEmpty(t, resp.Msg.Progress.DateEnd)
}

func TestConnectGetBooksProgress_WithDateRange(t *testing.T) {
	client := newBooksTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := connect.NewRequest(&booksv1.GetBooksProgressRequest{
		DateStart: "2024-01-01",
		DateEnd:   "2024-12-31",
	})
	req.Header().Set("Cookie", accessToken.String())

	resp, err := client.GetBooksProgress(ctx, req)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.NotNil(t, resp.Msg.Progress)
	assert.Equal(t, "2024-01-01", resp.Msg.Progress.DateStart)
	assert.Equal(t, "2024-12-31", resp.Msg.Progress.DateEnd)
}

func TestConnectSearchLibrary_Empty(t *testing.T) {
	client := newBooksTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := connect.NewRequest(&booksv1.SearchLibraryRequest{
		Query: "",
	})
	req.Header().Set("Cookie", accessToken.String())

	resp, err := client.SearchLibrary(ctx, req)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Empty(t, resp.Msg.Books)
}

func TestConnectSearchLibrary_WithResults(t *testing.T) {
	book := addTestBook(t, "SearchableBook")
	require.NotNil(t, book)

	client := newBooksTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := connect.NewRequest(&booksv1.SearchLibraryRequest{
		Query: "SearchableBook",
	})
	req.Header().Set("Cookie", accessToken.String())

	resp, err := client.SearchLibrary(ctx, req)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.NotEmpty(t, resp.Msg.Books)
}

func TestConnectSearchExternal_Empty(t *testing.T) {
	client := newBooksTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := connect.NewRequest(&booksv1.SearchExternalRequest{
		Query: "",
	})
	req.Header().Set("Cookie", accessToken.String())

	resp, err := client.SearchExternal(ctx, req)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Empty(t, resp.Msg.Results)
}

func TestConnectSearchExternal_WithQuery(t *testing.T) {
	client := newBooksTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := connect.NewRequest(&booksv1.SearchExternalRequest{
		Query: "Harry Potter",
	})
	req.Header().Set("Cookie", accessToken.String())

	resp, err := client.SearchExternal(ctx, req)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	// Results may be empty depending on mock data
}

func TestConnectAddBook(t *testing.T) {
	client := newBooksTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := connect.NewRequest(
		&booksv1.CreateBookRequest{
			Provider:    "manual",
			ProviderId:  "test-add-book",
			Title:       "New Book to Add",
			Author:      "Test Author",
			Status:      models.StatusToRead,
			Isbn13:      "9780140449112",
			CoverUrl:    "https://example.com/cover.jpg",
			OwnPhysical: true,
		},
	)
	req.Header().Set("Cookie", accessToken.String())

	resp, err := client.CreateBook(ctx, req)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.NotNil(t, resp.Msg)
}

func TestConnectUpdateBookStatus(t *testing.T) {
	book := addTestBook(t, "StatusChangeBook")
	require.NotNil(t, book)

	client := newBooksTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := connect.NewRequest(
		&booksv1.UpdateBookStatusRequest{
			BookId:    book.BookID.String(),
			Status:    models.StatusReading,
			Favourite: false,
			Rating:    "4",
		},
	)
	req.Header().Set("Cookie", accessToken.String())

	resp, err := client.UpdateBookStatus(ctx, req)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.NotNil(t, resp.Msg)
}

func TestConnectUpdateBookStatus_MarkRead(t *testing.T) {
	book := addTestBook(t, "ReadBook")
	require.NotNil(t, book)

	client := newBooksTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := connect.NewRequest(
		&booksv1.UpdateBookStatusRequest{
			BookId:    book.BookID.String(),
			Status:    models.StatusRead,
			Favourite: false,
			Rating:    "5",
		},
	)
	req.Header().Set("Cookie", accessToken.String())

	resp, err := client.UpdateBookStatus(ctx, req)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.NotNil(t, resp.Msg)
}

func TestConnectUpdateProgress_Pages(t *testing.T) {
	book := addTestBook(t, "ProgressPagesBook")
	require.NotNil(t, book)

	client := newBooksTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := connect.NewRequest(
		&booksv1.UpdateProgressRequest{
			BookId:       book.BookID.String(),
			ProgressMode: models.ProgressModePages,
			CurrentPage:  120,
		},
	)
	req.Header().Set("Cookie", accessToken.String())

	resp, err := client.UpdateProgress(ctx, req)
	require.NoError(t, err)
	assert.NotNil(t, resp.Msg)

	saved, err := testApp.Repositories.Books.GetUserBook(ctx, userID, book.BookID)
	require.NoError(t, err)
	assert.Equal(t, models.ProgressModePages, saved.ProgressMode)
	assert.Equal(t, 120, saved.CurrentPage)
}

func TestConnectUpdateProgress_PercentClamped(t *testing.T) {
	book := addTestBook(t, "ProgressPercentBook")
	require.NotNil(t, book)

	client := newBooksTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := connect.NewRequest(
		&booksv1.UpdateProgressRequest{
			BookId:          book.BookID.String(),
			ProgressMode:    models.ProgressModePercent,
			ProgressPercent: 150,
		},
	)
	req.Header().Set("Cookie", accessToken.String())

	_, err := client.UpdateProgress(ctx, req)
	require.NoError(t, err)

	saved, err := testApp.Repositories.Books.GetUserBook(ctx, userID, book.BookID)
	require.NoError(t, err)
	assert.Equal(t, models.ProgressModePercent, saved.ProgressMode)
	assert.Equal(t, 100, saved.ProgressPercent)
}

func TestConnectUpdateProgress_NegativeValuesClampedToZero(t *testing.T) {
	book := addTestBook(t, "ProgressNegativeBook")
	require.NotNil(t, book)

	client := newBooksTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := connect.NewRequest(
		&booksv1.UpdateProgressRequest{
			BookId:          book.BookID.String(),
			ProgressMode:    models.ProgressModePercent,
			CurrentPage:     -5,
			ProgressPercent: -10,
		},
	)
	req.Header().Set("Cookie", accessToken.String())

	_, err := client.UpdateProgress(ctx, req)
	require.NoError(t, err)

	saved, err := testApp.Repositories.Books.GetUserBook(ctx, userID, book.BookID)
	require.NoError(t, err)
	assert.Equal(t, 0, saved.CurrentPage)
	assert.Equal(t, 0, saved.ProgressPercent)
}

func TestConnectUpdateProgress_InvalidBookID(t *testing.T) {
	client := newBooksTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := connect.NewRequest(
		&booksv1.UpdateProgressRequest{
			BookId:       "not-a-uuid",
			ProgressMode: models.ProgressModePages,
		},
	)
	req.Header().Set("Cookie", accessToken.String())

	_, err := client.UpdateProgress(ctx, req)
	require.Error(t, err)
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestConnectUpdateProgress_InvalidMode(t *testing.T) {
	book := addTestBook(t, "ProgressInvalidModeBook")
	require.NotNil(t, book)

	client := newBooksTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := connect.NewRequest(
		&booksv1.UpdateProgressRequest{
			BookId:       book.BookID.String(),
			ProgressMode: "chapters",
		},
	)
	req.Header().Set("Cookie", accessToken.String())

	_, err := client.UpdateProgress(ctx, req)
	require.Error(t, err)
}
