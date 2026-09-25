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

// isbnFromUUID derives a valid, per-run-unique ISBN-13 from a UUID.
func isbnFromUUID(id uuid.UUID) string {
	b := id[10:]
	n := uint64(b[0])<<40 | uint64(b[1])<<32 | uint64(b[2])<<24 |
		uint64(b[3])<<16 | uint64(b[4])<<8 | uint64(b[5])
	prefix := fmt.Sprintf("978%09d", n%1_000_000_000)
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

// newAdminBooksTestClient returns a client authenticated as RoleAdmin.
func newAdminBooksTestClient(t *testing.T) booksTestClient {
	t.Helper()
	adminApp := books.NewInner(
		sharedmocks.NewMockedAdminAuthService(userID),
		testApp.Logger,
		testCfg,
		testDB,
		books.Clients{
			UniCat:           nil,
			WebFetch:         nil,
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

// requireAdmin: non-admin gets PermissionDenied.

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

// TestListResyncProposals_Admin_Success only checks the round trip: other
// tests share the proposals table.
func TestListResyncProposals_Admin_Success(t *testing.T) {
	client := newAdminBooksTestClient(t)
	req := connect.NewRequest(&booksv1.ListResyncProposalsRequest{})
	req.Header().Set("Cookie", accessToken.String())

	resp, err := client.ListResyncProposals(context.Background(), req)
	require.NoError(t, err)
	assert.NotNil(t, resp)
}

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

func TestFindDuplicates_Admin_Success(t *testing.T) {
	client := newAdminBooksTestClient(t)
	req := connect.NewRequest(&booksv1.FindDuplicatesRequest{})
	req.Header().Set("Cookie", accessToken.String())

	resp, err := client.FindDuplicates(context.Background(), req)
	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.NotNil(t, resp.Msg)
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
	newISBN := isbnFromUUID(ub.BookID)

	client := newAdminBooksTestClient(t)
	req := connect.NewRequest(&booksv1.SetBookISBNRequest{
		BookId: ub.BookID.String(),
		Isbn13: newISBN,
	})
	req.Header().Set("Cookie", accessToken.String())

	_, err := client.SetBookISBN(context.Background(), req)
	require.NoError(t, err)

	book, err := testApp.Repositories.Books.GetBookByID(context.Background(), ub.BookID)
	require.NoError(t, err)
	require.NotNil(t, book.ISBN13)
	assert.Equal(t, newISBN, *book.ISBN13)
}

func TestSetBookISBN_DuplicateISBN_AlreadyExists(t *testing.T) {
	ubA := addTestBook(t, "SetISBNDuplicateBookA")
	ubB := addTestBookNoISBN(t, "SetISBNDuplicateBookB")

	client := newAdminBooksTestClient(t)
	req := connect.NewRequest(&booksv1.SetBookISBNRequest{
		BookId: ubB.BookID.String(),
		Isbn13: "9780140449112", // same ISBN addTestBook uses
	})
	req.Header().Set("Cookie", accessToken.String())

	// addTestBook's ISBN is hard-coded and may already exist.
	require.NotNil(t, ubA)

	_, err := client.SetBookISBN(context.Background(), req)
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	assert.Equal(t, connect.CodeAlreadyExists, connErr.Code())
}

func TestSetBookISBN_WithHyphens_NormalisedAndAccepted(t *testing.T) {
	ub := addTestBookNoISBN(t, "SetISBNHyphenBook")
	rawISBN := isbnFromUUID(ub.BookID)
	// Arbitrary grouping; the handler strips hyphens.
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

func TestUpdateBook_NonAdmin_PermissionDenied(t *testing.T) {
	client := newBooksTestClient(t)
	req := connect.NewRequest(&booksv1.UpdateBookRequest{
		BookId: "00000000-0000-0000-0000-000000000001",
		Metadata: &booksv1.Book{
			Title: "New Title",
		},
	})
	req.Header().Set("Cookie", accessToken.String())

	_, err := client.UpdateBook(context.Background(), req)
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	assert.Equal(t, connect.CodePermissionDenied, connErr.Code())
}

func TestUpdateBook_InvalidUUID_InvalidArgument(t *testing.T) {
	client := newAdminBooksTestClient(t)
	req := connect.NewRequest(&booksv1.UpdateBookRequest{
		BookId: "not-a-uuid",
		Metadata: &booksv1.Book{
			Title: "New Title",
		},
	})
	req.Header().Set("Cookie", accessToken.String())

	_, err := client.UpdateBook(context.Background(), req)
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	assert.Equal(t, connect.CodeInvalidArgument, connErr.Code())
}

func TestUpdateBook_InvalidISBN_InvalidArgument(t *testing.T) {
	ub := addTestBookNoISBN(t, "UpdateBookInvalidISBNTest")

	client := newAdminBooksTestClient(t)
	req := connect.NewRequest(&booksv1.UpdateBookRequest{
		BookId: ub.BookID.String(),
		Metadata: &booksv1.Book{
			Title:  "New Title",
			Isbn13: "123",
		},
	})
	req.Header().Set("Cookie", accessToken.String())

	_, err := client.UpdateBook(context.Background(), req)
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	assert.Equal(t, connect.CodeInvalidArgument, connErr.Code())
}

func TestUpdateBook_UnknownBook_NotFound(t *testing.T) {
	client := newAdminBooksTestClient(t)
	req := connect.NewRequest(&booksv1.UpdateBookRequest{
		BookId: uuid.New().String(),
		Metadata: &booksv1.Book{
			Title: "New Title",
		},
	})
	req.Header().Set("Cookie", accessToken.String())

	_, err := client.UpdateBook(context.Background(), req)
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	assert.Equal(t, connect.CodeNotFound, connErr.Code())
}

func TestUpdateBook_Success_UpdatesAllFields(t *testing.T) {
	ub := addTestBookNoISBN(t, "UpdateBookSuccessTest")
	newISBN := isbnFromUUID(ub.BookID)

	client := newAdminBooksTestClient(t)
	req := connect.NewRequest(&booksv1.UpdateBookRequest{
		BookId: ub.BookID.String(),
		Metadata: &booksv1.Book{
			Title:       "Edited Title",
			Authors:     []string{"Edited Author"},
			Isbn13:      newISBN,
			Description: "Edited description.",
			PageCount:   321,
			CoverUrl:    "https://example.com/edited-cover.jpg",
		},
	})
	req.Header().Set("Cookie", accessToken.String())

	resp, err := client.UpdateBook(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, resp.Msg.Book)
	assert.Equal(t, "Edited Title", resp.Msg.Book.Title)
	assert.Equal(t, []string{"Edited Author"}, resp.Msg.Book.Authors)
	assert.Equal(t, newISBN, resp.Msg.Book.Isbn13)
	assert.Equal(t, "Edited description.", resp.Msg.Book.Description)
	assert.Equal(t, int32(321), resp.Msg.Book.PageCount)

	book, err := testApp.Repositories.Books.GetBookByID(context.Background(), ub.BookID)
	require.NoError(t, err)
	assert.Equal(t, "Edited Title", book.Title)
	require.NotNil(t, book.ISBN13)
	assert.Equal(t, newISBN, *book.ISBN13)
	require.NotNil(t, book.CoverURL)
	assert.Equal(t, "https://example.com/edited-cover.jpg", *book.CoverURL)
}

func TestUpdateBook_EmptyCoverURL_ClearsCover(t *testing.T) {
	ub := addTestBook(t, "UpdateBookClearCoverTest")

	client := newAdminBooksTestClient(t)
	req := connect.NewRequest(&booksv1.UpdateBookRequest{
		BookId: ub.BookID.String(),
		Metadata: &booksv1.Book{
			Title:   "UpdateBookClearCoverTest",
			Authors: []string{"Test Author"},
		},
	})
	req.Header().Set("Cookie", accessToken.String())

	_, err := client.UpdateBook(context.Background(), req)
	require.NoError(t, err)

	book, err := testApp.Repositories.Books.GetBookByID(context.Background(), ub.BookID)
	require.NoError(t, err)
	assert.Nil(t, book.CoverURL)
}

func TestUpdateBook_DuplicateISBN_AlreadyExists(t *testing.T) {
	ubA := addTestBook(t, "UpdateBookDuplicateISBNBookA")
	ubB := addTestBookNoISBN(t, "UpdateBookDuplicateISBNBookB")

	client := newAdminBooksTestClient(t)
	req := connect.NewRequest(&booksv1.UpdateBookRequest{
		BookId: ubB.BookID.String(),
		Metadata: &booksv1.Book{
			Title:  "UpdateBookDuplicateISBNBookB",
			Isbn13: "9780140449112", // same ISBN addTestBook uses
		},
	})
	req.Header().Set("Cookie", accessToken.String())

	require.NotNil(t, ubA)

	_, err := client.UpdateBook(context.Background(), req)
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	assert.Equal(t, connect.CodeAlreadyExists, connErr.Code())
}

// TestGetBooksByIDs_ReturnsMatchingBooks: pgx has no encoder for []uuid.UUID,
// so IDs must be passed as strings and cast.
func TestGetBooksByIDs_ReturnsMatchingBooks(t *testing.T) {
	// ISBN-less books get distinct catalog entries.
	ub1 := addTestBookNoISBN(t, "GetBooksByIDs_Book1")
	ub2 := addTestBookNoISBN(t, "GetBooksByIDs_Book2")

	ctx := context.Background()

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

	none, err := testApp.Repositories.Books.GetBooksByIDs(ctx, nil)
	require.NoError(t, err)
	assert.Nil(t, none)
}
