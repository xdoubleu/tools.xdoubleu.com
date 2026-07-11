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
	"tools.xdoubleu.com/apps/books/pkg/objectstore"
	booksv1 "tools.xdoubleu.com/gen/books/v1"
	sharedmocks "tools.xdoubleu.com/internal/mocks"
	"tools.xdoubleu.com/internal/testhelper"
)

// newAdminBooksTestClientWithMockSources is like newAdminBooksTestClient but
// wires the mocked Open Library client (always "The Odyssey" by Homer)
// instead of nil, so GetBookSources/ApplyBookSource's live fetch has
// something to find.
func newAdminBooksTestClientWithMockSources(t *testing.T) booksTestClient {
	t.Helper()
	adminApp := books.NewInner(
		sharedmocks.NewMockedAdminAuthService(userID),
		testApp.Logger,
		testCfg,
		testDB,
		books.Clients{
			OpenLibrary:      mocks.NewMockOpenLibraryClient(),
			GoogleBooks:      nil,
			UniCat:           nil,
			ObjectStore:      objectstore.NewFake(),
			PublicAPIBaseURL: "",
			KoboStoreBaseURL: "",
		},
	)
	ts := httptest.NewServer(testhelper.BuildMux(adminApp))
	t.Cleanup(ts.Close)
	return newBooksClientFor(ts.URL, connect.WithHTTPGet())
}

// ---------------------------------------------------------------------------
// GetBookSources / ApplyBookSource: requireAdmin + invalid input
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// GetBookSources / ApplyBookSource: admin success, live fetch (mocked OL client)
// ---------------------------------------------------------------------------

// TestGetBookSources_Admin_Success verifies the RPC live-fetches the mocked
// Open Library candidate (always "The Odyssey" by Homer) for any book on
// demand, without needing a prior wizard scan.
func TestGetBookSources_Admin_Success(t *testing.T) {
	id := uuid.New()
	ub := addTestBookWithISBN(t, "GetBookSourcesTestBook", isbnFromUUID(id))

	client := newAdminBooksTestClientWithMockSources(t)
	req := connect.NewRequest(
		&booksv1.GetBookSourcesRequest{BookId: ub.BookID.String()},
	)
	req.Header().Set("Cookie", accessToken.String())

	resp, err := client.GetBookSources(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, resp.Msg.Proposal)
	require.Len(t, resp.Msg.Proposal.Sources, 1)
	assert.Equal(t, "openlibrary", resp.Msg.Proposal.Sources[0].Source)
	assert.Equal(t, "The Odyssey", resp.Msg.Proposal.Sources[0].Title)
	assert.Contains(t, resp.Msg.Proposal.Sources[0].Differs, "title")
}

// TestApplyBookSource_Admin_Success verifies applying the live-fetched source
// writes its fields onto the book — usable on any book, unlike
// ApplyResyncChoice which requires a prior scan to have stored a proposal.
func TestApplyBookSource_Admin_Success(t *testing.T) {
	id := uuid.New()
	ub := addTestBookWithISBN(t, "ApplyBookSourceTestBook", isbnFromUUID(id))

	client := newAdminBooksTestClientWithMockSources(t)
	req := connect.NewRequest(&booksv1.ApplyBookSourceRequest{
		BookId: ub.BookID.String(),
		Source: "openlibrary",
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
	assert.Equal(t, "openlibrary", *book.MetadataSource)
}

// TestApplyBookSource_Admin_Override verifies the manual search override:
// a book whose stored title would never pass the match guards can still be
// matched and applied when the admin supplies a corrected title/author.
func TestApplyBookSource_Admin_Override(t *testing.T) {
	ub := addTestBookNoISBN(t, "Completely Unmatchable Stored Title")

	client := newAdminBooksTestClientWithMockSources(t)

	// Without an override the guard rejects the mock's "The Odyssey" result.
	noOverride := connect.NewRequest(&booksv1.ApplyBookSourceRequest{

		BookId: ub.BookID.String(),
		Source: "openlibrary",
	})
	noOverride.Header().Set("Cookie", accessToken.String())
	_, err := client.ApplyBookSource(context.Background(), noOverride)
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	assert.Equal(t, connect.CodeNotFound, connErr.Code())

	// With the override the top search result is taken unguarded.
	title := "The Odyssey"
	author := "Homer"
	withOverride := connect.NewRequest(&booksv1.ApplyBookSourceRequest{
		BookId:         ub.BookID.String(),
		Source:         "openlibrary",
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
	assert.Equal(t, "openlibrary", *book.MetadataSource)
}

func TestApplyBookSource_Admin_UnknownSource_NotFound(t *testing.T) {
	ub := addTestBookNoISBN(t, "ApplyBookSourceUnknownSourceBook")

	client := newAdminBooksTestClientWithMockSources(t)
	req := connect.NewRequest(&booksv1.ApplyBookSourceRequest{
		BookId: ub.BookID.String(),
		Source: "googlebooks", // not configured in this test app (nil client)
	})
	req.Header().Set("Cookie", accessToken.String())

	_, err := client.ApplyBookSource(context.Background(), req)
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	assert.Equal(t, connect.CodeNotFound, connErr.Code())
}
