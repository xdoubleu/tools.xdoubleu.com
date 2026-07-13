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
	"tools.xdoubleu.com/apps/books/pkg/objectstore"
	booksv1 "tools.xdoubleu.com/gen/books/v1"
	sharedmocks "tools.xdoubleu.com/internal/mocks"
	"tools.xdoubleu.com/internal/testhelper"
)

// isbnFromUUID derives a valid ISBN-13 from a UUID so each test run produces
// a unique ISBN that won't collide with ISBNs inserted by previous runs.
func isbnFromUUID(id uuid.UUID) string {
	// Use bytes 10-15 (48 bits) as a 9-digit number after the "978" prefix.
	b := id[10:]
	n := uint64(b[0])<<40 | uint64(b[1])<<32 | uint64(b[2])<<24 |
		uint64(b[3])<<16 | uint64(b[4])<<8 | uint64(b[5])
	prefix := fmt.Sprintf("978%09d", n%1_000_000_000)
	// Compute ISBN-13 check digit per the standard alternating-weight formula.
	sum := 0
	for i, r := range prefix {
		d := int(r - '0')
		if i%2 == 0 {
			sum += d
		} else {
			sum += 3 * d
		}
	}
	check := (10 - (sum % 10)) % 10
	return fmt.Sprintf("%s%d", prefix, check)
}

// newAdminBooksTestClient returns a Connect client whose app authenticates
// all requests as an admin user (RoleAdmin).
func newAdminBooksTestClient(t *testing.T) booksTestClient {
	t.Helper()
	adminApp := books.NewInner(
		sharedmocks.NewMockedAdminAuthService(userID),
		testApp.Logger,
		testCfg,
		testDB,
		books.Clients{
			OpenLibrary:      nil,
			UniCat:           nil,
			Hardcover:        nil,
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
// requireAdmin: non-admin gets PermissionDenied
// ---------------------------------------------------------------------------

func TestFindDuplicates_NonAdmin_PermissionDenied(t *testing.T) {
	client := newBooksTestClient(t)
	req := connect.NewRequest(&booksv1.FindDuplicatesRequest{})
	req.Header().Set("Cookie", accessToken.String())

	_, err := client.FindDuplicates(context.Background(), req)
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	assert.Equal(t, connect.CodePermissionDenied, connErr.Code())
}

func TestMergeBooks_NonAdmin_PermissionDenied(t *testing.T) {
	client := newBooksTestClient(t)
	req := connect.NewRequest(&booksv1.MergeBooksRequest{
		WinnerBookId: "00000000-0000-0000-0000-000000000001",
		LoserBookIds: []string{"00000000-0000-0000-0000-000000000002"},
	})
	req.Header().Set("Cookie", accessToken.String())

	_, err := client.MergeBooks(context.Background(), req)
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	assert.Equal(t, connect.CodePermissionDenied, connErr.Code())
}

func TestStartResync_NonAdmin_PermissionDenied(t *testing.T) {
	client := newBooksTestClient(t)
	req := connect.NewRequest(&booksv1.StartResyncRequest{})
	req.Header().Set("Cookie", accessToken.String())

	_, err := client.StartResync(context.Background(), req)
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	assert.Equal(t, connect.CodePermissionDenied, connErr.Code())
}

func TestListResyncProposals_NonAdmin_PermissionDenied(t *testing.T) {
	client := newBooksTestClient(t)
	req := connect.NewRequest(&booksv1.ListResyncProposalsRequest{})
	req.Header().Set("Cookie", accessToken.String())

	_, err := client.ListResyncProposals(context.Background(), req)
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	assert.Equal(t, connect.CodePermissionDenied, connErr.Code())
}

func TestApplyResyncChoice_NonAdmin_PermissionDenied(t *testing.T) {
	client := newBooksTestClient(t)
	req := connect.NewRequest(&booksv1.ApplyResyncChoiceRequest{
		BookId: "00000000-0000-0000-0000-000000000001",
		Source: "",
	})
	req.Header().Set("Cookie", accessToken.String())

	_, err := client.ApplyResyncChoice(context.Background(), req)
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	assert.Equal(t, connect.CodePermissionDenied, connErr.Code())
}

// ---------------------------------------------------------------------------
// ListResyncProposals: admin success (empty when nothing was scanned)
// ---------------------------------------------------------------------------

// TestListResyncProposals_Admin_Success verifies the RPC round-trips
// successfully. It cannot assert on the exact proposal set: the DB and
// resync_proposals table are shared across this package's tests, so other
// tests may have already run a scan (see TestBuildResyncProposals_Service).
func TestListResyncProposals_Admin_Success(t *testing.T) {
	client := newAdminBooksTestClient(t)
	req := connect.NewRequest(&booksv1.ListResyncProposalsRequest{})
	req.Header().Set("Cookie", accessToken.String())

	resp, err := client.ListResyncProposals(context.Background(), req)
	require.NoError(t, err)
	assert.NotNil(t, resp)
}

// ---------------------------------------------------------------------------
// ApplyResyncChoice: admin, invalid input
// ---------------------------------------------------------------------------

func TestApplyResyncChoice_Admin_InvalidUUID_InvalidArgument(t *testing.T) {
	client := newAdminBooksTestClient(t)
	req := connect.NewRequest(&booksv1.ApplyResyncChoiceRequest{
		BookId: "not-a-uuid",
		Source: "",
	})
	req.Header().Set("Cookie", accessToken.String())

	_, err := client.ApplyResyncChoice(context.Background(), req)
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	assert.Equal(t, connect.CodeInvalidArgument, connErr.Code())
}

func TestApplyResyncChoice_Admin_UnknownBook_NotFound(t *testing.T) {
	client := newAdminBooksTestClient(t)
	req := connect.NewRequest(&booksv1.ApplyResyncChoiceRequest{
		BookId: uuid.New().String(),
		Source: "",
	})
	req.Header().Set("Cookie", accessToken.String())

	_, err := client.ApplyResyncChoice(context.Background(), req)
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	assert.Equal(t, connect.CodeNotFound, connErr.Code())
}

// ---------------------------------------------------------------------------
// FindDuplicates: admin success
// ---------------------------------------------------------------------------

func TestFindDuplicates_Admin_Success(t *testing.T) {
	client := newAdminBooksTestClient(t)
	req := connect.NewRequest(&booksv1.FindDuplicatesRequest{})
	req.Header().Set("Cookie", accessToken.String())

	resp, err := client.FindDuplicates(context.Background(), req)
	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.NotNil(t, resp.Msg)
}

// ---------------------------------------------------------------------------
// Repo: ListCatalogBooks
// ---------------------------------------------------------------------------

func TestListCatalogBooks_ReturnsAllBooks(t *testing.T) {
	ub := addTestBook(t, "CatalogListTestBook")
	require.NotNil(t, ub)

	books, err := testApp.Repositories.Books.ListCatalogBooks(context.Background())
	require.NoError(t, err)

	var found bool
	for _, b := range books {
		if b.ID == ub.BookID {
			found = true
			break
		}
	}
	assert.True(t, found, "newly added book must appear in ListCatalogBooks")
}

// ---------------------------------------------------------------------------
// SetBookISBN
// ---------------------------------------------------------------------------

func TestSetBookISBN_NonAdmin_PermissionDenied(t *testing.T) {
	client := newBooksTestClient(t)
	req := connect.NewRequest(&booksv1.SetBookISBNRequest{
		BookId: "00000000-0000-0000-0000-000000000001",
		Isbn13: "9780140449112",
	})
	req.Header().Set("Cookie", accessToken.String())

	_, err := client.SetBookISBN(context.Background(), req)
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	assert.Equal(t, connect.CodePermissionDenied, connErr.Code())
}

func TestSetBookISBN_InvalidUUID_InvalidArgument(t *testing.T) {
	client := newAdminBooksTestClient(t)
	req := connect.NewRequest(&booksv1.SetBookISBNRequest{
		BookId: "not-a-uuid",
		Isbn13: "9780140449112",
	})
	req.Header().Set("Cookie", accessToken.String())

	_, err := client.SetBookISBN(context.Background(), req)
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	assert.Equal(t, connect.CodeInvalidArgument, connErr.Code())
}

func TestSetBookISBN_InvalidISBN_InvalidArgument(t *testing.T) {
	client := newAdminBooksTestClient(t)
	for _, bad := range []string{"", "123", "12345678901234", "978014044911X"} {
		req := connect.NewRequest(&booksv1.SetBookISBNRequest{
			BookId: "00000000-0000-0000-0000-000000000001",
			Isbn13: bad,
		})
		req.Header().Set("Cookie", accessToken.String())

		_, err := client.SetBookISBN(context.Background(), req)
		require.Error(t, err, "expected error for ISBN %q", bad)
		var connErr *connect.Error
		require.ErrorAs(t, err, &connErr)
		assert.Equal(
			t,
			connect.CodeInvalidArgument,
			connErr.Code(),
			"bad ISBN: %q",
			bad,
		)
	}
}

func TestSetBookISBN_UnknownBook_NotFound(t *testing.T) {
	client := newAdminBooksTestClient(t)
	req := connect.NewRequest(&booksv1.SetBookISBNRequest{
		BookId: uuid.New().String(),
		Isbn13: "9780140449113",
	})
	req.Header().Set("Cookie", accessToken.String())

	_, err := client.SetBookISBN(context.Background(), req)
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	assert.Equal(t, connect.CodeNotFound, connErr.Code())
}

func TestSetBookISBN_Success_UpdatesISBN(t *testing.T) {
	ub := addTestBookNoISBN(t, "SetISBNSuccessBook")
	// Derive a unique ISBN from the book's own UUID so re-runs never collide.
	newISBN := isbnFromUUID(ub.BookID)

	client := newAdminBooksTestClient(t)
	req := connect.NewRequest(&booksv1.SetBookISBNRequest{
		BookId: ub.BookID.String(),
		Isbn13: newISBN,
	})
	req.Header().Set("Cookie", accessToken.String())

	_, err := client.SetBookISBN(context.Background(), req)
	require.NoError(t, err)

	// Verify via the repo that the ISBN was written.
	book, err := testApp.Repositories.Books.GetBookByID(context.Background(), ub.BookID)
	require.NoError(t, err)
	require.NotNil(t, book.ISBN13)
	assert.Equal(t, newISBN, *book.ISBN13)
}

func TestSetBookISBN_DuplicateISBN_AlreadyExists(t *testing.T) {
	// Book A already has an ISBN.
	ubA := addTestBook(t, "SetISBNDuplicateBookA")
	// Book B has no ISBN.
	ubB := addTestBookNoISBN(t, "SetISBNDuplicateBookB")

	// Attempt to assign book A's ISBN to book B — must be rejected.
	client := newAdminBooksTestClient(t)
	req := connect.NewRequest(&booksv1.SetBookISBNRequest{
		BookId: ubB.BookID.String(),
		Isbn13: "9780140449112", // same ISBN addTestBook uses
	})
	req.Header().Set("Cookie", accessToken.String())

	// addTestBook uses a hard-coded ISBN; if another test already inserted it
	// this test is only valid when book A exists and has that ISBN.
	require.NotNil(t, ubA)

	_, err := client.SetBookISBN(context.Background(), req)
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	assert.Equal(t, connect.CodeAlreadyExists, connErr.Code())
}

func TestSetBookISBN_WithHyphens_NormalisedAndAccepted(t *testing.T) {
	ub := addTestBookNoISBN(t, "SetISBNHyphenBook")
	// Derive a unique base ISBN from the UUID and insert hyphens into it.
	rawISBN := isbnFromUUID(ub.BookID)
	// Format as hyphenated ISBN-13: 978-X-XX-XXXXXX-X (arbitrary grouping,
	// the handler strips all hyphens before validating).
	hyphenated := fmt.Sprintf("%s-%s-%s-%s-%s",
		rawISBN[0:3],
		rawISBN[3:4],
		rawISBN[4:6],
		rawISBN[6:12],
		rawISBN[12:13],
	)

	client := newAdminBooksTestClient(t)
	req := connect.NewRequest(&booksv1.SetBookISBNRequest{
		BookId: ub.BookID.String(),
		Isbn13: hyphenated,
	})
	req.Header().Set("Cookie", accessToken.String())

	_, err := client.SetBookISBN(context.Background(), req)
	require.NoError(t, err)

	book, err := testApp.Repositories.Books.GetBookByID(context.Background(), ub.BookID)
	require.NoError(t, err)
	require.NotNil(t, book.ISBN13)
	assert.Equal(t, rawISBN, *book.ISBN13, "hyphens must be stripped")
}

// ---------------------------------------------------------------------------
// TestGetBooksByIDs_ReturnsMatchingBooks is a regression test for the pgx
// UUID-array encoding bug: passing []uuid.UUID directly to ANY($1) produced
// "cannot find encode plan" because pgx has no registered encoder for that type.
// The fix converts IDs to []string and casts with ANY($1::uuid[]).
func TestGetBooksByIDs_ReturnsMatchingBooks(t *testing.T) {
	// Use ISBN-less books so each call creates a distinct catalog entry.
	ub1 := addTestBookNoISBN(t, "GetBooksByIDs_Book1")
	ub2 := addTestBookNoISBN(t, "GetBooksByIDs_Book2")

	ctx := context.Background()

	// Requesting both IDs must return exactly those two books without an encode error.
	books, err := testApp.Repositories.Books.GetBooksByIDs(
		ctx,
		[]uuid.UUID{ub1.BookID, ub2.BookID},
	)
	require.NoError(t, err)

	ids := make([]uuid.UUID, len(books))
	for i, b := range books {
		ids[i] = b.ID
	}
	assert.ElementsMatch(
		t,
		[]uuid.UUID{ub1.BookID, ub2.BookID},
		ids,
		"GetBooksByIDs must return exactly the requested books",
	)

	// Empty slice must return nil without error.
	none, err := testApp.Repositories.Books.GetBooksByIDs(ctx, nil)
	require.NoError(t, err)
	assert.Nil(t, none)
}
