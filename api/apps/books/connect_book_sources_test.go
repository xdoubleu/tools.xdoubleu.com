package books_test

import (
	"context"
	"net/http/httptest"
	"testing"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/books"
	"tools.xdoubleu.com/apps/books/internal/mocks"
	"tools.xdoubleu.com/apps/books/internal/models"
	"tools.xdoubleu.com/apps/books/internal/services"
	"tools.xdoubleu.com/apps/books/pkg/objectstore"
	booksv1 "tools.xdoubleu.com/gen/books/v1"
	sharedmocks "tools.xdoubleu.com/internal/mocks"
	"tools.xdoubleu.com/internal/testhelper"
)

// newAdminBooksTestClientWithMockSources wires the mocked Hardcover client
// (always "The Odyssey" by Homer) and an empty, non-nil UniCat mock: a nil
// client would leave its found flag NULL forever, which source stats never
// count as absent. Returns the app for driving scans.
func newAdminBooksTestClientWithMockSources(
	t *testing.T,
) (booksTestClient, *books.Books) {
	t.Helper()
	adminApp := books.NewInner(
		sharedmocks.NewMockedAdminAuthService(userID),
		testApp.Logger,
		testCfg,
		testDB,
		books.Clients{
			UniCat:           mocks.NewMockEmptyUniCatClient(),
			WebFetch:         nil,
			Hardcover:        mocks.NewMockHardcoverClient(),
			ObjectStore:      objectstore.NewFake(),
			PublicAPIBaseURL: "",
			KoboStoreBaseURL: "",
		},
	)
	ts := httptest.NewServer(testhelper.BuildMux(adminApp))
	t.Cleanup(ts.Close)
	return newBooksClientFor(ts.URL, connect.WithHTTPGet()), adminApp
}

// newAdminBooksTestClientWithTwoSources wires both mocks so an ISBN'd book is
// found by both (the overlap combo). Returns the app for driving scans.
func newAdminBooksTestClientWithTwoSources(
	t *testing.T,
) (booksTestClient, *books.Books) {
	t.Helper()
	adminApp := books.NewInner(
		sharedmocks.NewMockedAdminAuthService(userID),
		testApp.Logger,
		testCfg,
		testDB,
		books.Clients{
			UniCat:           mocks.NewMockUniCatClient(),
			WebFetch:         nil,
			Hardcover:        mocks.NewMockHardcoverClient(),
			ObjectStore:      objectstore.NewFake(),
			PublicAPIBaseURL: "",
			KoboStoreBaseURL: "",
		},
	)
	ts := httptest.NewServer(testhelper.BuildMux(adminApp))
	t.Cleanup(ts.Close)
	return newBooksClientFor(ts.URL, connect.WithHTTPGet()), adminApp
}

// GetBookSources / ApplyBookSource: requireAdmin + invalid input.

func TestGetBookSources_NonAdmin_PermissionDenied(t *testing.T) {
	client := newBooksTestClient(t)
	req := connect.NewRequest(&booksv1.GetBookSourcesRequest{
		BookId: "00000000-0000-0000-0000-000000000001",
	})
	req.Header().Set("Cookie", accessToken.String())

	_, err := client.GetBookSources(context.Background(), req)
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	assert.Equal(t, connect.CodePermissionDenied, connErr.Code())
}

func TestGetBookSources_Admin_InvalidUUID_InvalidArgument(t *testing.T) {
	client := newAdminBooksTestClient(t)
	req := connect.NewRequest(&booksv1.GetBookSourcesRequest{BookId: "not-a-uuid"})
	req.Header().Set("Cookie", accessToken.String())

	_, err := client.GetBookSources(context.Background(), req)
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	assert.Equal(t, connect.CodeInvalidArgument, connErr.Code())
}

func TestGetBookSources_Admin_UnknownBook_NotFound(t *testing.T) {
	client := newAdminBooksTestClient(t)
	req := connect.NewRequest(
		&booksv1.GetBookSourcesRequest{BookId: uuid.New().String()},
	)
	req.Header().Set("Cookie", accessToken.String())

	_, err := client.GetBookSources(context.Background(), req)
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	assert.Equal(t, connect.CodeNotFound, connErr.Code())
}

func TestApplyBookSource_NonAdmin_PermissionDenied(t *testing.T) {
	client := newBooksTestClient(t)
	req := connect.NewRequest(&booksv1.ApplyBookSourceRequest{
		BookId: "00000000-0000-0000-0000-000000000001",
		Source: "",
	})
	req.Header().Set("Cookie", accessToken.String())

	_, err := client.ApplyBookSource(context.Background(), req)
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	assert.Equal(t, connect.CodePermissionDenied, connErr.Code())
}

func TestApplyBookSource_Admin_InvalidUUID_InvalidArgument(t *testing.T) {
	client := newAdminBooksTestClient(t)
	req := connect.NewRequest(&booksv1.ApplyBookSourceRequest{
		BookId: "not-a-uuid",
		Source: "",
	})
	req.Header().Set("Cookie", accessToken.String())

	_, err := client.ApplyBookSource(context.Background(), req)
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	assert.Equal(t, connect.CodeInvalidArgument, connErr.Code())
}

// GetBookSources / ApplyBookSource: admin success, live fetch.

// TestGetBookSources_Admin_Success: the on-demand path always searches by
// title+author, so the request overrides to match the mock's canned result.
func TestGetBookSources_Admin_Success(t *testing.T) {
	id := uuid.New()
	ub := addTestBookWithISBN(t, "GetBookSourcesTestBook", isbnFromUUID(id))

	client, _ := newAdminBooksTestClientWithMockSources(t)
	title, author := "The Odyssey", "Homer"
	req := connect.NewRequest(&booksv1.GetBookSourcesRequest{
		BookId:        ub.BookID.String(),
		OverrideTitle: &title, OverrideAuthor: &author,
	})
	req.Header().Set("Cookie", accessToken.String())

	resp, err := client.GetBookSources(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, resp.Msg.Proposal)
	require.Len(t, resp.Msg.Proposal.Sources, 1)
	assert.Equal(t, "hardcover", resp.Msg.Proposal.Sources[0].Source)
	assert.Equal(t, "The Odyssey", resp.Msg.Proposal.Sources[0].Title)
	assert.Contains(t, resp.Msg.Proposal.Sources[0].Differs, "title")
}

// TestApplyBookSource_Admin_Success checks the live-fetched source is written
// without a prior scan.
func TestApplyBookSource_Admin_Success(t *testing.T) {
	id := uuid.New()
	ub := addTestBookWithISBN(t, "ApplyBookSourceTestBook", isbnFromUUID(id))

	client, _ := newAdminBooksTestClientWithMockSources(t)
	title, author := "The Odyssey", "Homer"
	req := connect.NewRequest(&booksv1.ApplyBookSourceRequest{
		BookId:        ub.BookID.String(),
		Source:        "hardcover",
		OverrideTitle: &title, OverrideAuthor: &author,
	})
	req.Header().Set("Cookie", accessToken.String())

	_, err := client.ApplyBookSource(context.Background(), req)
	require.NoError(t, err)

	book, err := testApp.Repositories.Books.GetBookByID(context.Background(), ub.BookID)
	require.NoError(t, err)
	assert.Equal(t, "The Odyssey", book.Title)
	assert.Equal(t, []string{"Homer"}, book.Authors)
	require.NotNil(t, book.MetadataSource,
		"applying a source must record provenance")
	assert.Equal(t, "hardcover", *book.MetadataSource)
}

// TestApplyBookSource_Admin_Override: a corrected title/author lets a
// guard-failing book be matched.
func TestApplyBookSource_Admin_Override(t *testing.T) {
	ub := addTestBookNoISBN(t, "Completely Unmatchable Stored Title")

	client, _ := newAdminBooksTestClientWithMockSources(t)

	noOverride := connect.NewRequest(&booksv1.ApplyBookSourceRequest{

		BookId: ub.BookID.String(),
		Source: "hardcover",
	})
	noOverride.Header().Set("Cookie", accessToken.String())
	_, err := client.ApplyBookSource(context.Background(), noOverride)
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	assert.Equal(t, connect.CodeNotFound, connErr.Code())

	title := "The Odyssey"
	author := "Homer"
	withOverride := connect.NewRequest(&booksv1.ApplyBookSourceRequest{
		BookId:         ub.BookID.String(),
		Source:         "hardcover",
		OverrideTitle:  &title,
		OverrideAuthor: &author,
	})
	withOverride.Header().Set("Cookie", accessToken.String())
	_, err = client.ApplyBookSource(context.Background(), withOverride)
	require.NoError(t, err)

	book, err := testApp.Repositories.Books.GetBookByID(context.Background(), ub.BookID)
	require.NoError(t, err)
	assert.Equal(t, "The Odyssey", book.Title)
	require.NotNil(t, book.MetadataSource)
	assert.Equal(t, "hardcover", *book.MetadataSource)
}

// TestApplyBookSource_Admin_SecondSyncStillSucceeds: a second sync must
// succeed even after the first filled in an ISBN, since the on-demand path
// always matches by title+author.
func TestApplyBookSource_Admin_SecondSyncStillSucceeds(t *testing.T) {
	ext := services.SourceProposal{ //nolint:exhaustruct // ISBN intentionally absent
		Source:  "manual",
		Title:   "The Odyssey",
		Authors: []string{"Homer"},
	}
	ub, err := testApp.Services.Books.AddToLibrary(
		context.Background(), userID, ext, models.StatusToRead, []string{},
	)
	require.NoError(t, err)

	client, _ := newAdminBooksTestClientWithMockSources(t)
	req := connect.NewRequest(&booksv1.ApplyBookSourceRequest{
		BookId: ub.BookID.String(),
		Source: "hardcover",
	})
	req.Header().Set("Cookie", accessToken.String())

	_, err = client.ApplyBookSource(context.Background(), req)
	require.NoError(t, err)

	_, err = client.ApplyBookSource(context.Background(), req)
	require.NoError(t, err, "a second sync must not fail")
}

func TestApplyBookSource_Admin_UnknownSource_NotFound(t *testing.T) {
	ub := addTestBookNoISBN(t, "ApplyBookSourceUnknownSourceBook")

	client, _ := newAdminBooksTestClientWithMockSources(t)
	req := connect.NewRequest(&booksv1.ApplyBookSourceRequest{
		BookId: ub.BookID.String(),
		Source: "unicat", // mock UniCat is configured but always confirmed-absent
	})
	req.Header().Set("Cookie", accessToken.String())

	_, err := client.ApplyBookSource(context.Background(), req)
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	assert.Equal(t, connect.CodeNotFound, connErr.Code())
}
