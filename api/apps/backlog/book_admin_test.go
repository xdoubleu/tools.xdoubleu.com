package backlog_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/backlog"
	"tools.xdoubleu.com/apps/backlog/pkg/objectstore"
	"tools.xdoubleu.com/apps/backlog/pkg/steam"
	backlogv1 "tools.xdoubleu.com/gen/backlog/v1"
	backlogv1connect "tools.xdoubleu.com/gen/backlog/v1/backlogv1connect"
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
func newAdminBooksTestClient(t *testing.T) backlogv1connect.BooksServiceClient {
	t.Helper()
	adminApp := backlog.NewInner(
		context.Background(),
		sharedmocks.NewMockedAdminAuthService(userID),
		testApp.Logger,
		testCfg,
		testDB,
		backlog.Clients{
			SteamFactory:     func(_ string) steam.Client { return nil },
			OpenLibrary:      nil,
			GoogleBooks:      nil,
			UniCat:           nil,
			ObjectStore:      objectstore.NewFake(),
			PublicAPIBaseURL: "",
			KoboStoreBaseURL: "",
		},
	)
	ts := httptest.NewServer(testhelper.BuildMux(adminApp))
	t.Cleanup(ts.Close)
	return backlogv1connect.NewBooksServiceClient(
		http.DefaultClient,
		ts.URL,
		connect.WithHTTPGet(),
	)
}

// ---------------------------------------------------------------------------
// requireAdmin: non-admin gets PermissionDenied
// ---------------------------------------------------------------------------

func TestFindDuplicates_NonAdmin_PermissionDenied(t *testing.T) {
	client := newBooksTestClient(t)
	req := connect.NewRequest(&backlogv1.FindDuplicatesRequest{})
	req.Header().Set("Cookie", accessToken.String())

	_, err := client.FindDuplicates(context.Background(), req)
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	assert.Equal(t, connect.CodePermissionDenied, connErr.Code())
}

func TestMergeBooks_NonAdmin_PermissionDenied(t *testing.T) {
	client := newBooksTestClient(t)
	req := connect.NewRequest(&backlogv1.MergeBooksRequest{
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

func TestResyncOpenLibrary_NonAdmin_PermissionDenied(t *testing.T) {
	client := newBooksTestClient(t)
	req := connect.NewRequest(&backlogv1.ResyncOpenLibraryRequest{})
	req.Header().Set("Cookie", accessToken.String())

	_, err := client.ResyncOpenLibrary(context.Background(), req)
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	assert.Equal(t, connect.CodePermissionDenied, connErr.Code())
}

func TestListCatalogBooks_NonAdmin_PermissionDenied(t *testing.T) {
	client := newBooksTestClient(t)
	req := connect.NewRequest(&backlogv1.ListCatalogBooksRequest{})
	req.Header().Set("Cookie", accessToken.String())

	_, err := client.ListCatalogBooks(context.Background(), req)
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	assert.Equal(t, connect.CodePermissionDenied, connErr.Code())
}

func TestResyncBooks_NonAdmin_PermissionDenied(t *testing.T) {
	client := newBooksTestClient(t)
	req := connect.NewRequest(&backlogv1.ResyncBooksRequest{
		BookIds: []string{"00000000-0000-0000-0000-000000000001"},
		Force:   false,
	})
	req.Header().Set("Cookie", accessToken.String())

	_, err := client.ResyncBooks(context.Background(), req)
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	assert.Equal(t, connect.CodePermissionDenied, connErr.Code())
}

// ---------------------------------------------------------------------------
// ListCatalogBooks: admin success
// ---------------------------------------------------------------------------

func TestListCatalogBooks_Admin_ReturnsBooks(t *testing.T) {
	addTestBook(t, "Catalog Test Book")

	client := newAdminBooksTestClient(t)
	req := connect.NewRequest(&backlogv1.ListCatalogBooksRequest{})
	req.Header().Set("Cookie", accessToken.String())

	resp, err := client.ListCatalogBooks(context.Background(), req)
	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.NotEmpty(t, resp.Msg.Books)
}

// ---------------------------------------------------------------------------
// ResyncBooks: admin success (empty IDs rejected)
// ---------------------------------------------------------------------------

func TestResyncBooks_Admin_EmptyIDs_InvalidArgument(t *testing.T) {
	client := newAdminBooksTestClient(t)
	req := connect.NewRequest(&backlogv1.ResyncBooksRequest{
		BookIds: []string{},
		Force:   false,
	})
	req.Header().Set("Cookie", accessToken.String())

	_, err := client.ResyncBooks(context.Background(), req)
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	assert.Equal(t, connect.CodeInvalidArgument, connErr.Code())
}

func TestResyncBooks_Admin_InvalidUUID_InvalidArgument(t *testing.T) {
	client := newAdminBooksTestClient(t)
	req := connect.NewRequest(&backlogv1.ResyncBooksRequest{
		BookIds: []string{"not-a-uuid"},
		Force:   false,
	})
	req.Header().Set("Cookie", accessToken.String())

	_, err := client.ResyncBooks(context.Background(), req)
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	assert.Equal(t, connect.CodeInvalidArgument, connErr.Code())
}

// ---------------------------------------------------------------------------
// FindDuplicates: admin success
// ---------------------------------------------------------------------------

func TestFindDuplicates_Admin_Success(t *testing.T) {
	client := newAdminBooksTestClient(t)
	req := connect.NewRequest(&backlogv1.FindDuplicatesRequest{})
	req.Header().Set("Cookie", accessToken.String())

	resp, err := client.FindDuplicates(context.Background(), req)
	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.NotNil(t, resp.Msg)
}

// ---------------------------------------------------------------------------
// Repo: SetResyncStatus and ListCatalogBooks
// ---------------------------------------------------------------------------

func TestSetResyncStatus_UpdatesColumns(t *testing.T) {
	ub := addTestBook(t, "ResyncStatusTestBook")
	require.NotNil(t, ub)

	ctx := context.Background()
	err := testApp.Repositories.Books.SetResyncStatus(ctx, ub.BookID, true, false)
	require.NoError(t, err)

	// Verify by reading back via the repo.
	books, err := testApp.Repositories.Books.ListCatalogBooks(ctx)
	require.NoError(t, err)

	var found *struct {
		olFound *bool
		gbFound *bool
		resync  bool
	}
	for _, b := range books {
		if b.ID == ub.BookID {
			found = &struct {
				olFound *bool
				gbFound *bool
				resync  bool
			}{
				olFound: b.OpenLibraryFound,
				gbFound: b.GoogleBooksFound,
				resync:  b.LastResyncAt != nil,
			}
			break
		}
	}

	require.NotNil(t, found, "book must appear in catalog")
	require.NotNil(t, found.olFound)
	assert.True(t, *found.olFound, "openlibrary_found must be true")
	require.NotNil(t, found.gbFound)
	assert.False(t, *found.gbFound, "googlebooks_found must be false")
	assert.True(t, found.resync, "last_resync_at must be set")
}

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
	req := connect.NewRequest(&backlogv1.SetBookISBNRequest{
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
	req := connect.NewRequest(&backlogv1.SetBookISBNRequest{
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
		req := connect.NewRequest(&backlogv1.SetBookISBNRequest{
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
	req := connect.NewRequest(&backlogv1.SetBookISBNRequest{
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
	req := connect.NewRequest(&backlogv1.SetBookISBNRequest{
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
	req := connect.NewRequest(&backlogv1.SetBookISBNRequest{
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
	req := connect.NewRequest(&backlogv1.SetBookISBNRequest{
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
