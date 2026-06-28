package backlog_test

import (
	"context"
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
