package books_test

import (
	"context"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/books/internal/models"
	"tools.xdoubleu.com/apps/books/internal/services"
	booksv1 "tools.xdoubleu.com/gen/books/v1"
)

// addTestBookWithISBN adds a book with a unique ISBN so it gets its own DB row.
func addTestBookWithISBN(t *testing.T, title, isbn string) *models.UserBook {
	t.Helper()
	ext := services.SourceProposal{ //nolint:exhaustruct //optional fields not needed
		Source:   "manual",
		Title:    title,
		Authors:  []string{"Coverage Author"},
		ISBN13:   isbn,
		CoverURL: "https://example.com/cover.jpg",
	}
	ub, err := testApp.Services.Books.AddToLibrary(
		context.Background(), userID, ext, models.StatusToRead, []string{},
	)
	require.NoError(t, err)
	require.NotNil(t, ub)
	return ub
}

// TestConnectGetLibrary_WithVariousBooksAndShelves covers:
//   - StatusReading and StatusRead cases in buildLibraryData
//   - int32PtrFromInt16 nil path (fresh book, no rating set)
//   - int32PtrFromInt16 non-nil path (book with rating "4")
//   - protoBookshelves loop body via 3 custom-status shelves
//   - slices.SortFunc comparison body (return -1 and return 1) via 3+ shelves
//   - tags staying on books without leaking into Library.Shelves
func TestConnectGetLibrary_WithVariousBooksAndShelves(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	uid := uuid.New().String()[:8]

	// Three books with distinct ISBNs so each gets a separate DB row.
	bookA := addTestBookWithISBN(t, "CovReading-"+uid, "9780000000001")
	bookB := addTestBookWithISBN(t, "CovRead-"+uid, "9780000000002")
	bookC := addTestBookWithISBN(t, "CovNilRating-"+uid, "9780000000003")

	// Mark bookA as currently-reading (covers StatusReading in buildLibraryData)
	readingReq := connect.NewRequest(&booksv1.UpdateBookStatusRequest{
		BookId: bookA.BookID.String(), Status: models.StatusReading,
	})
	readingReq.Header().Set("Cookie", accessToken.String())
	_, err := newBooksTestClient(t).UpdateBookStatus(ctx, readingReq)
	require.NoError(t, err)

	// Mark bookB as read with rating (covers StatusRead + int32PtrFromInt16 non-nil)
	readReq := connect.NewRequest(&booksv1.UpdateBookStatusRequest{
		BookId: bookB.BookID.String(), Status: models.StatusRead, Rating: "4",
	})
	readReq.Header().Set("Cookie", accessToken.String())
	_, err = newBooksTestClient(t).UpdateBookStatus(ctx, readReq)
	require.NoError(t, err)

	// bookC stays as to-read with nil rating, covering int32PtrFromInt16 nil branch.

	// Add a user tag to bookA to verify it is NOT reflected in Library.Shelves.
	tagReq := connect.NewRequest(&booksv1.ToggleTagRequest{
		BookId: bookA.BookID.String(), Tag: "cov-user-tag",
	})
	tagReq.Header().Set("Cookie", accessToken.String())
	_, err = newBooksTestClient(t).ToggleTag(ctx, tagReq)
	require.NoError(t, err)

	// Add 3 custom-status shelves (one per extra book) to exercise
	// protoBookshelves loop body and SortFunc -1/1 comparison paths.
	bookD := addTestBookWithISBN(t, "CovCustomA-"+uid, "9780000000004")
	bookE := addTestBookWithISBN(t, "CovCustomB-"+uid, "9780000000005")
	for _, tc := range []struct {
		book   *models.UserBook
		status string
	}{
		{bookC, "alpha-shelf"},
		{bookD, "beta-shelf"},
		{bookE, "gamma-shelf"},
	} {
		statusReq := connect.NewRequest(&booksv1.UpdateBookStatusRequest{
			BookId: tc.book.BookID.String(), Status: tc.status,
		})
		statusReq.Header().Set("Cookie", accessToken.String())
		_, err = newBooksTestClient(t).UpdateBookStatus(ctx, statusReq)
		require.NoError(t, err)
	}

	// GetLibrary exercises all the paths above.
	libReq := connect.NewRequest(&booksv1.GetLibraryRequest{})
	libReq.Header().Set("Cookie", accessToken.String())
	resp, err := newBooksTestClient(t).GetLibrary(ctx, libReq)
	require.NoError(t, err)
	assert.NotNil(t, resp.Msg.Library)
	assert.NotEmpty(t, resp.Msg.Library.Finished)
	assert.NotEmpty(t, resp.Msg.Library.Shelves)
	// Tags must not bleed into shelves — the tag "cov-user-tag" is on bookA
	// but must not appear as a shelf name.
	for _, shelf := range resp.Msg.Library.Shelves {
		assert.NotEqual(t, "cov-user-tag", shelf.Name,
			"user tag must not appear as a shelf")
	}
}

// TestConnectUpdateBookStatus_ZeroRating covers parseRating's "0" early-return branch.
func TestConnectUpdateBookStatus_ZeroRating(t *testing.T) {
	book := addTestBook(t, "ZeroRatingBook")
	require.NotNil(t, book)

	client := newBooksTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := connect.NewRequest(&booksv1.UpdateBookStatusRequest{
		BookId:    book.BookID.String(),
		Status:    models.StatusReading,
		Favourite: false,
		Rating:    "0",
	})
	req.Header().Set("Cookie", accessToken.String())
	_, err := client.UpdateBookStatus(ctx, req)
	require.NoError(t, err)
}

// TestConnectUpdateBookStatus_NegativeRating covers parseRating's error/n<=0 branch.
func TestConnectUpdateBookStatus_NegativeRating(t *testing.T) {
	book := addTestBook(t, "NegativeRatingBook")
	require.NotNil(t, book)

	client := newBooksTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := connect.NewRequest(&booksv1.UpdateBookStatusRequest{
		BookId:    book.BookID.String(),
		Status:    models.StatusReading,
		Favourite: false,
		Rating:    "-1",
	})
	req.Header().Set("Cookie", accessToken.String())
	_, err := client.UpdateBookStatus(ctx, req)
	require.NoError(t, err)
}

// TestConnectUpdateBookStatus_OutOfRangeRating covers parseRating's n>5 branch.
// A rating above the DB's chk_user_books_rating bound (1-5) must not reach the
// database as a non-nil value, or the CHECK violation surfaces as a 500.
func TestConnectUpdateBookStatus_OutOfRangeRating(t *testing.T) {
	book := addTestBook(t, "OutOfRangeRatingBook")
	require.NotNil(t, book)

	client := newBooksTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := connect.NewRequest(&booksv1.UpdateBookStatusRequest{
		BookId:    book.BookID.String(),
		Status:    models.StatusReading,
		Favourite: false,
		Rating:    "6",
	})
	req.Header().Set("Cookie", accessToken.String())
	_, err := client.UpdateBookStatus(ctx, req)
	require.NoError(t, err)
}

// assertFinishedAtDates compares finished_at RFC3339 timestamps by calendar
// date only, since the DB session timezone (not the test's) determines the
// UTC offset the driver returns.
func assertFinishedAtDates(t *testing.T, got []string, wantDates ...string) {
	t.Helper()
	require.Len(t, got, len(wantDates))
	for i, raw := range got {
		parsed, err := time.Parse(time.RFC3339, raw)
		require.NoError(t, err)
		assert.Equal(t, wantDates[i], parsed.Format(time.DateOnly))
	}
}

// TestConnectUpdateFinishedAt_OverwritesDates covers manually editing a
// book's read-date history: setting an initial date, then replacing it with
// a different set (add + remove in one call), and finally clearing it.
func TestConnectUpdateFinishedAt_OverwritesDates(t *testing.T) {
	book := addTestBook(t, "FinishedAtEditBook")
	require.NotNil(t, book)

	client := newBooksTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	setReq := connect.NewRequest(&booksv1.UpdateFinishedAtRequest{
		BookId:     book.BookID.String(),
		FinishedAt: []string{"2024-01-15", "2024-06-01"},
	})
	setReq.Header().Set("Cookie", accessToken.String())
	_, err := client.UpdateFinishedAt(ctx, setReq)
	require.NoError(t, err)

	getReq := connect.NewRequest(
		&booksv1.SearchLibraryRequest{Query: "FinishedAtEditBook"},
	)
	getReq.Header().Set("Cookie", accessToken.String())
	searchResp, err := client.SearchLibrary(ctx, getReq)
	require.NoError(t, err)
	require.Len(t, searchResp.Msg.Books, 1)
	assertFinishedAtDates(
		t,
		searchResp.Msg.Books[0].FinishedAt,
		"2024-01-15",
		"2024-06-01",
	)

	// Replace with a single, different date.
	replaceReq := connect.NewRequest(&booksv1.UpdateFinishedAtRequest{
		BookId:     book.BookID.String(),
		FinishedAt: []string{"2024-12-25"},
	})
	replaceReq.Header().Set("Cookie", accessToken.String())
	_, err = client.UpdateFinishedAt(ctx, replaceReq)
	require.NoError(t, err)

	searchResp, err = client.SearchLibrary(ctx, getReq)
	require.NoError(t, err)
	require.Len(t, searchResp.Msg.Books, 1)
	assertFinishedAtDates(t, searchResp.Msg.Books[0].FinishedAt, "2024-12-25")

	// Clear entirely.
	clearReq := connect.NewRequest(&booksv1.UpdateFinishedAtRequest{
		BookId: book.BookID.String(),
	})
	clearReq.Header().Set("Cookie", accessToken.String())
	_, err = client.UpdateFinishedAt(ctx, clearReq)
	require.NoError(t, err)

	searchResp, err = client.SearchLibrary(ctx, getReq)
	require.NoError(t, err)
	require.Len(t, searchResp.Msg.Books, 1)
	assert.Empty(t, searchResp.Msg.Books[0].FinishedAt)
}

// TestConnectUpdateFinishedAt_InvalidDate covers the date-parse error path.
func TestConnectUpdateFinishedAt_InvalidDate(t *testing.T) {
	book := addTestBook(t, "FinishedAtInvalidDateBook")
	require.NotNil(t, book)

	client := newBooksTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := connect.NewRequest(&booksv1.UpdateFinishedAtRequest{
		BookId:     book.BookID.String(),
		FinishedAt: []string{"not-a-date"},
	})
	req.Header().Set("Cookie", accessToken.String())
	_, err := client.UpdateFinishedAt(ctx, req)
	require.Error(t, err)
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

// TestConnectGetLibrary_FormatsPopulated asserts that a book with an uploaded
// PDF file has its Formats field populated on the GetLibrary response.
func TestConnectGetLibrary_FormatsPopulated(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	uid := uuid.New().String()[:8]
	book := addTestBookWithISBN(t, "FormatsBook-"+uid, "9780000099001")

	// Insert a ready PDF book_file row directly via the repository so we don't
	// need a real object store upload.
	pdfFile := models.BookFile{ //nolint:exhaustruct //optional nullable fields omitted
		BookID:     book.BookID,
		UserID:     userID,
		Format:     models.FileFormatPDF,
		StorageKey: "users/test/books/pdf/formats-lib.pdf",
		SizeBytes:  512,
		Status:     models.FileStatusReady,
	}
	_, err := testApp.Repositories.BookFiles.Insert(ctx, pdfFile)
	require.NoError(t, err)

	libReq := connect.NewRequest(&booksv1.GetLibraryRequest{})
	libReq.Header().Set("Cookie", accessToken.String())
	resp, err := newBooksTestClient(t).GetLibrary(ctx, libReq)
	require.NoError(t, err)
	require.NotNil(t, resp.Msg.Library)

	// Find our book in the wishlist (default status is to-read).
	var found bool
	for _, ub := range resp.Msg.Library.Wishlist {
		if ub.BookId == book.BookID.String() {
			assert.Contains(t, ub.Formats, models.FileFormatPDF)
			assert.NotContains(t, ub.Formats, models.FileFormatEPUB)
			found = true
			break
		}
	}
	assert.True(t, found, "expected book in wishlist")
}
