package books_test

import (
	"context"
	"fmt"
	"net/http/httptest"
	"testing"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/books"
	"tools.xdoubleu.com/apps/books/internal/mocks"
	"tools.xdoubleu.com/apps/books/internal/models"
	"tools.xdoubleu.com/apps/books/pkg/objectstore"
	"tools.xdoubleu.com/apps/books/pkg/openlibrary"
	booksv1 "tools.xdoubleu.com/gen/books/v1"
	sharedmocks "tools.xdoubleu.com/internal/mocks"
	"tools.xdoubleu.com/internal/testhelper"
)

// newAdminBooksTestClientWithMockSources is like newAdminBooksTestClient but
// wires the mocked Open Library client (always "The Odyssey" by Homer)
// instead of nil, so GetBookSources/ApplyBookSource's live fetch has
// something to find. Google Books and UniCat are wired to empty (not nil)
// mocks — configured but confirmed-absent — matching production, where all
// three sources are always configured; a genuinely nil client would leave
// its found flag NULL (unresolved) forever, and GetSourceStats' IS TRUE/IS
// FALSE-aware uniqueness never counts an unresolved source as absent. Returns
// the app too so a test can drive a scan through its own service (with these
// mocked clients) rather than the shared testApp's (OL-only).
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
			OpenLibrary:      mocks.NewMockOpenLibraryClient(),
			GoogleBooks:      mocks.NewMockEmptyGoogleBooksClient(),
			UniCat:           mocks.NewMockEmptyUniCatClient(),
			Hardcover:        mocks.NewMockEmptyHardcoverClient(),
			ObjectStore:      objectstore.NewFake(),
			PublicAPIBaseURL: "",
			KoboStoreBaseURL: "",
		},
	)
	ts := httptest.NewServer(testhelper.BuildMux(adminApp))
	t.Cleanup(ts.Close)
	return newBooksClientFor(ts.URL, connect.WithHTTPGet()), adminApp
}

// newAdminBooksTestClientWithTwoSources wires the mocked Open Library and
// Google Books clients (both resolve any ISBN to their own canned book) plus
// an empty (confirmed-absent, not nil — see newAdminBooksTestClientWithMockSources)
// UniCat client, so a scanned ISBN'd book is found by exactly OL+GB — used to
// exercise the source-stats overlap combos. Returns the app too, for driving
// a scan through its own service.
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
			OpenLibrary:      mocks.NewMockOpenLibraryClient(),
			GoogleBooks:      mocks.NewMockGoogleBooksClient(),
			UniCat:           mocks.NewMockEmptyUniCatClient(),
			Hardcover:        mocks.NewMockEmptyHardcoverClient(),
			ObjectStore:      objectstore.NewFake(),
			PublicAPIBaseURL: "",
			KoboStoreBaseURL: "",
		},
	)
	ts := httptest.NewServer(testhelper.BuildMux(adminApp))
	t.Cleanup(ts.Close)
	return newBooksClientFor(ts.URL, connect.WithHTTPGet()), adminApp
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
// demand, without needing a prior wizard scan. The on-demand path always
// matches by title+author search (see fetchProposals), even for a book that
// has an ISBN, so the request carries an override matching the mock's canned
// result — a stored title/author that doesn't match would otherwise be
// rejected by the search guard.
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
	assert.Equal(t, "openlibrary", resp.Msg.Proposal.Sources[0].Source)
	assert.Equal(t, "The Odyssey", resp.Msg.Proposal.Sources[0].Title)
	assert.Contains(t, resp.Msg.Proposal.Sources[0].Differs, "title")
}

// TestApplyBookSource_Admin_Success verifies applying the live-fetched source
// writes its fields onto the book — usable on any book, unlike
// ApplyResyncChoice which requires a prior scan to have stored a proposal.
// See TestGetBookSources_Admin_Success for why the override is needed.
func TestApplyBookSource_Admin_Success(t *testing.T) {
	id := uuid.New()
	ub := addTestBookWithISBN(t, "ApplyBookSourceTestBook", isbnFromUUID(id))

	client, _ := newAdminBooksTestClientWithMockSources(t)
	title, author := "The Odyssey", "Homer"
	req := connect.NewRequest(&booksv1.ApplyBookSourceRequest{
		BookId:        ub.BookID.String(),
		Source:        "openlibrary",
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
	assert.Equal(t, "openlibrary", *book.MetadataSource)
}

// TestApplyBookSource_Admin_Override verifies the manual search override:
// a book whose stored title would never pass the match guards can still be
// matched and applied when the admin supplies a corrected title/author.
func TestApplyBookSource_Admin_Override(t *testing.T) {
	ub := addTestBookNoISBN(t, "Completely Unmatchable Stored Title")

	client, _ := newAdminBooksTestClientWithMockSources(t)

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

// TestApplyBookSource_Admin_SecondSyncStillSucceeds is the regression test for
// the reported "2nd sync always fails" bug: an ISBN-less book naturally
// matches the mocked source by title+author (no override needed). Applying
// once can fill in an ISBN the book previously lacked (subject to the
// repository's duplicate-ISBN guard); the bug was that a second sync then
// routed the follow-up fetch by that new ISBN instead of by title+author (see
// fetchSourceProposals), landing on a different candidate set and returning
// ErrProposalNotFound ("source not found"). The on-demand path now always
// matches by title+author (see fetchProposals), so a second sync on the same
// book/source must succeed exactly like the first, regardless of what
// happened to the ISBN in between.
func TestApplyBookSource_Admin_SecondSyncStillSucceeds(t *testing.T) {
	ext := openlibrary.ExternalBook{ //nolint:exhaustruct // ISBN intentionally absent
		Provider:   "manual",
		ProviderID: fmt.Sprintf("secondsync-%s", uuid.New()),
		Title:      "The Odyssey",
		Authors:    []string{"Homer"},
	}
	ub, err := testApp.Services.Books.AddToLibrary(
		context.Background(), userID, ext, models.StatusToRead, []string{},
	)
	require.NoError(t, err)

	client, _ := newAdminBooksTestClientWithMockSources(t)
	req := connect.NewRequest(&booksv1.ApplyBookSourceRequest{
		BookId: ub.BookID.String(),
		Source: "openlibrary",
	})
	req.Header().Set("Cookie", accessToken.String())

	// First apply: matches naturally.
	_, err = client.ApplyBookSource(context.Background(), req)
	require.NoError(t, err)

	// Second apply on the same book/source must still succeed.
	_, err = client.ApplyBookSource(context.Background(), req)
	require.NoError(t, err, "a second sync must not fail")
}

func TestApplyBookSource_Admin_UnknownSource_NotFound(t *testing.T) {
	ub := addTestBookNoISBN(t, "ApplyBookSourceUnknownSourceBook")

	client, _ := newAdminBooksTestClientWithMockSources(t)
	req := connect.NewRequest(&booksv1.ApplyBookSourceRequest{
		BookId: ub.BookID.String(),
		Source: "googlebooks", // mock GB is configured but always confirmed-absent
	})
	req.Header().Set("Cookie", accessToken.String())

	_, err := client.ApplyBookSource(context.Background(), req)
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	assert.Equal(t, connect.CodeNotFound, connErr.Code())
}
