package books_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"connectrpc.com/connect/v2"
	"connectrpc.com/connect/v2/connecthttp"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/books/internal/models"
	booksv1 "tools.xdoubleu.com/gen/books/v1"
	booksv1connect "tools.xdoubleu.com/gen/books/v1/booksv1connect"
	"tools.xdoubleu.com/internal/testhelper"
)

func newBooksTestClient(t *testing.T) booksTestClient {
	t.Helper()
	ts := httptest.NewServer(getRoutes())
	t.Cleanup(ts.Close)
	return newBooksClientFor(ts.URL, connecthttp.WithHTTPGet())
}

// booksTestClient bundles the four books service clients.
type booksTestClient struct {
	booksv1connect.LibraryServiceClient
	booksv1connect.BookFilesServiceClient
	booksv1connect.KoboServiceClient
	booksv1connect.CatalogServiceClient
}

func newBooksClientFor(url string, opts ...connecthttp.Option) booksTestClient {
	return booksTestClient{
		LibraryServiceClient: booksv1connect.NewLibraryServiceClient(
			connect.NewClient(connecthttp.NewTransport(
				http.DefaultClient,
				url,
				append([]connecthttp.Option{connecthttp.WithReadMaxBytes(0)}, opts...)...,
			)),
		),
		BookFilesServiceClient: booksv1connect.NewBookFilesServiceClient(
			connect.NewClient(connecthttp.NewTransport(
				http.DefaultClient,
				url,
				append([]connecthttp.Option{connecthttp.WithReadMaxBytes(0)}, opts...)...,
			)),
		),
		KoboServiceClient: booksv1connect.NewKoboServiceClient(
			connect.NewClient(connecthttp.NewTransport(
				http.DefaultClient,
				url,
				append([]connecthttp.Option{connecthttp.WithReadMaxBytes(0)}, opts...)...,
			)),
		),
		CatalogServiceClient: booksv1connect.NewCatalogServiceClient(
			connect.NewClient(connecthttp.NewTransport(
				http.DefaultClient,
				url,
				append([]connecthttp.Option{connecthttp.WithReadMaxBytes(0)}, opts...)...,
			)),
		),
	}
}

func TestConnectGetLibrary(t *testing.T) {
	client := newBooksTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := &booksv1.GetLibraryRequest{}
	ctx, info := connect.NewClientContext(ctx)
	info.RequestHeader().Set("Cookie", accessToken.String())

	resp, err := client.GetLibrary(ctx, req)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.NotNil(t, resp.Library)
}

func TestConnectGetLibrary_WithBooks(t *testing.T) {
	book := addTestBook(t, "Test Book")
	require.NotNil(t, book)

	client := newBooksTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := &booksv1.GetLibraryRequest{}
	ctx, info := connect.NewClientContext(ctx)
	info.RequestHeader().Set("Cookie", accessToken.String())

	resp, err := client.GetLibrary(ctx, req)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.NotNil(t, resp.Library)
	assert.NotEmpty(t, resp.Library.Wishlist, "book should be in wishlist")
}

func TestConnectGetBooksProgress_DefaultRange(t *testing.T) {
	client := newBooksTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := &booksv1.GetBooksProgressRequest{}
	ctx, info := connect.NewClientContext(ctx)
	info.RequestHeader().Set("Cookie", accessToken.String())

	resp, err := client.GetBooksProgress(ctx, req)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.NotNil(t, resp.Progress)
	assert.NotEmpty(t, resp.Progress.DateStart)
	assert.NotEmpty(t, resp.Progress.DateEnd)
}

func TestConnectGetBooksProgress_WithDateRange(t *testing.T) {
	client := newBooksTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := &booksv1.GetBooksProgressRequest{
		DateStart: "2024-01-01",
		DateEnd:   "2024-12-31",
	}
	ctx, info := connect.NewClientContext(ctx)
	info.RequestHeader().Set("Cookie", accessToken.String())

	resp, err := client.GetBooksProgress(ctx, req)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.NotNil(t, resp.Progress)
	assert.Equal(t, "2024-01-01", resp.Progress.DateStart)
	assert.Equal(t, "2024-12-31", resp.Progress.DateEnd)
}

func TestConnectSearchLibrary_Empty(t *testing.T) {
	client := newBooksTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := &booksv1.SearchLibraryRequest{
		Query: "",
	}
	ctx, info := connect.NewClientContext(ctx)
	info.RequestHeader().Set("Cookie", accessToken.String())

	resp, err := client.SearchLibrary(ctx, req)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Empty(t, resp.Books)
}

func TestConnectSearchLibrary_WithResults(t *testing.T) {
	book := addTestBook(t, "SearchableBook")
	require.NotNil(t, book)

	client := newBooksTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := &booksv1.SearchLibraryRequest{
		Query: "SearchableBook",
	}
	ctx, info := connect.NewClientContext(ctx)
	info.RequestHeader().Set("Cookie", accessToken.String())

	resp, err := client.SearchLibrary(ctx, req)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.NotEmpty(t, resp.Books)
}

// TestConnectSearchLibrary_TitleAndAuthorNarrows: each word may match title
// or author.
func TestConnectSearchLibrary_TitleAndAuthorNarrows(t *testing.T) {
	addTestBookNoISBN(t, "NarrowSearchTitle")

	client := newBooksTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := &booksv1.SearchLibraryRequest{
		Query: "NarrowSearchTitle Test",
	}

	resp, err := client.SearchLibrary(
		testhelper.WithCookie(ctx, accessToken.String()),
		req,
	)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.NotEmpty(t, resp.Books)

	unmatched := &booksv1.SearchLibraryRequest{
		Query: "NarrowSearchTitle NoSuchAuthor",
	}
	resp, err = client.SearchLibrary(
		testhelper.WithCookie(ctx, accessToken.String()),
		unmatched,
	)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Empty(t, resp.Books)
}

// TestConnectSearchLibrary_Pagination uses a per-run token in titles so other
// tests' books can't skew the exact counts.
func TestConnectSearchLibrary_Pagination(t *testing.T) {
	prefix := "PaginatedBook-" + uuid.NewString()
	addTestBookNoISBN(t, prefix+"-One")
	addTestBookNoISBN(t, prefix+"-Two")
	addTestBookNoISBN(t, prefix+"-Three")

	client := newBooksTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	firstPage := &booksv1.SearchLibraryRequest{
		Query: prefix,
		Limit: 2,
	}
	resp, err := client.SearchLibrary(
		testhelper.WithCookie(ctx, accessToken.String()),
		firstPage,
	)
	assert.NoError(t, err)
	assert.Len(t, resp.Books, 2)
	assert.True(t, resp.HasMore)

	secondPage := &booksv1.SearchLibraryRequest{
		Query:  prefix,
		Limit:  2,
		Offset: 2,
	}
	resp, err = client.SearchLibrary(
		testhelper.WithCookie(ctx, accessToken.String()),
		secondPage,
	)
	assert.NoError(t, err)
	assert.Len(t, resp.Books, 1)
	assert.False(t, resp.HasMore)
}

func TestConnectSearchExternal_Empty(t *testing.T) {
	client := newBooksTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := &booksv1.SearchExternalRequest{
		Query: "",
	}
	ctx, info := connect.NewClientContext(ctx)
	info.RequestHeader().Set("Cookie", accessToken.String())

	resp, err := client.SearchExternal(ctx, req)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Empty(t, resp.Results)
}

func TestConnectSearchExternal_WithQuery(t *testing.T) {
	client := newBooksTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := &booksv1.SearchExternalRequest{
		Query: "Harry Potter",
	}
	ctx, info := connect.NewClientContext(ctx)
	info.RequestHeader().Set("Cookie", accessToken.String())

	resp, err := client.SearchExternal(ctx, req)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
}

// TestConnectSearchExternal_TitleAndAuthorNarrows: a wrong author drops the
// mock's result; the right one keeps it.
func TestConnectSearchExternal_TitleAndAuthorNarrows(t *testing.T) {
	client := newBooksTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	matching := &booksv1.SearchExternalRequest{
		Query: "Odyssey Homer",
	}
	resp, err := client.SearchExternal(
		testhelper.WithCookie(ctx, accessToken.String()),
		matching,
	)
	assert.NoError(t, err)
	assert.NotEmpty(t, resp.Results)

	mismatched := &booksv1.SearchExternalRequest{
		Query: "Odyssey NoSuchAuthor",
	}
	resp, err = client.SearchExternal(
		testhelper.WithCookie(ctx, accessToken.String()),
		mismatched,
	)
	assert.NoError(t, err)
	assert.Empty(t, resp.Results)
}

func TestConnectGetExternalBook_Found(t *testing.T) {
	client := newBooksTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// The Hardcover mock resolves any ISBN; provider_id is the ISBN13.
	req := &booksv1.GetExternalBookRequest{
		Provider:   "hardcover",
		ProviderId: "9780140447934",
	}
	ctx, info := connect.NewClientContext(ctx)
	info.RequestHeader().Set("Cookie", accessToken.String())

	resp, err := client.GetExternalBook(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, resp.Result)
	assert.Equal(t, "hardcover", resp.Result.Provider)
	assert.NotEmpty(t, resp.Result.Title)
}

func TestConnectGetExternalBook_UnknownProvider(t *testing.T) {
	client := newBooksTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := &booksv1.GetExternalBookRequest{
		Provider:   "unknownprovider",
		ProviderId: "anything",
	}
	ctx, info := connect.NewClientContext(ctx)
	info.RequestHeader().Set("Cookie", accessToken.String())

	_, err := client.GetExternalBook(ctx, req)
	require.Error(t, err)
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

func TestConnectAddBook(t *testing.T) {
	client := newBooksTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := &booksv1.CreateBookRequest{
		Provider:    "manual",
		ProviderId:  "test-add-book",
		Title:       "New Book to Add",
		Author:      "Test Author",
		Status:      models.StatusToRead,
		Isbn13:      "9780140449112",
		CoverUrl:    "https://example.com/cover.jpg",
		OwnPhysical: true,
	}
	ctx, info := connect.NewClientContext(ctx)
	info.RequestHeader().Set("Cookie", accessToken.String())

	resp, err := client.CreateBook(ctx, req)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.NotNil(t, resp)
}

func TestConnectUpdateBookStatus(t *testing.T) {
	book := addTestBook(t, "StatusChangeBook")
	require.NotNil(t, book)

	client := newBooksTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := &booksv1.UpdateBookStatusRequest{
		BookId:    book.BookID.String(),
		Status:    models.StatusReading,
		Favourite: false,
		Rating:    "4",
	}
	ctx, info := connect.NewClientContext(ctx)
	info.RequestHeader().Set("Cookie", accessToken.String())

	resp, err := client.UpdateBookStatus(ctx, req)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.NotNil(t, resp)
}

func TestConnectUpdateBookStatus_MarkRead(t *testing.T) {
	book := addTestBook(t, "ReadBook")
	require.NotNil(t, book)

	client := newBooksTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := &booksv1.UpdateBookStatusRequest{
		BookId:    book.BookID.String(),
		Status:    models.StatusRead,
		Favourite: false,
		Rating:    "5",
	}
	ctx, info := connect.NewClientContext(ctx)
	info.RequestHeader().Set("Cookie", accessToken.String())

	resp, err := client.UpdateBookStatus(ctx, req)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.NotNil(t, resp)
}

func TestConnectUpdateProgress_Pages(t *testing.T) {
	book := addTestBook(t, "ProgressPagesBook")
	require.NotNil(t, book)

	client := newBooksTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := &booksv1.UpdateProgressRequest{
		BookId:       book.BookID.String(),
		ProgressMode: models.ProgressModePages,
		CurrentPage:  120,
	}
	ctx, info := connect.NewClientContext(ctx)
	info.RequestHeader().Set("Cookie", accessToken.String())

	resp, err := client.UpdateProgress(ctx, req)
	require.NoError(t, err)
	assert.NotNil(t, resp)

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

	req := &booksv1.UpdateProgressRequest{
		BookId:          book.BookID.String(),
		ProgressMode:    models.ProgressModePercent,
		ProgressPercent: 150,
	}
	ctx, info := connect.NewClientContext(ctx)
	info.RequestHeader().Set("Cookie", accessToken.String())

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

	req := &booksv1.UpdateProgressRequest{
		BookId:          book.BookID.String(),
		ProgressMode:    models.ProgressModePercent,
		CurrentPage:     -5,
		ProgressPercent: -10,
	}
	ctx, info := connect.NewClientContext(ctx)
	info.RequestHeader().Set("Cookie", accessToken.String())

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

	req := &booksv1.UpdateProgressRequest{
		BookId:       "not-a-uuid",
		ProgressMode: models.ProgressModePages,
	}
	ctx, info := connect.NewClientContext(ctx)
	info.RequestHeader().Set("Cookie", accessToken.String())

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

	req := &booksv1.UpdateProgressRequest{
		BookId:       book.BookID.String(),
		ProgressMode: "chapters",
	}
	ctx, info := connect.NewClientContext(ctx)
	info.RequestHeader().Set("Cookie", accessToken.String())

	_, err := client.UpdateProgress(ctx, req)
	require.Error(t, err)
}
