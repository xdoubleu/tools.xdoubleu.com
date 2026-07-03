package books_test

import (
	"bytes"
	"context"
	"fmt"
	"testing"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xdoubleu/essentia/v4/pkg/database"

	"tools.xdoubleu.com/apps/books/internal/models"
	booksv1 "tools.xdoubleu.com/gen/books/v1"
)

// uploadFileForOwner provides a (bookFile, bookID) pair for tests that need a
// stored file. For EPUB, the book is pre-seeded in the library so recognition
// succeeds. For PDF (and other formats), the book_files row is inserted
// directly — bare PDFs carry no parseable metadata so the upload service would
// reject them.
func uploadFileForOwner(
	t *testing.T,
	ownerID string,
	format string,
) (*models.BookFile, uuid.UUID) {
	t.Helper()
	switch format {
	case models.FileFormatEPUB:
		title := "GetFileBook-" + uuid.NewString()
		seedBookInLibrary(t, ownerID, title, "Author", "")
		data := buildEPUBBytes(title, "Author", "")
		result, err := uploadViaTestApp(
			t, ownerID, "test.epub", "application/epub+zip", data,
		)
		require.NoError(t, err)
		return result.BookFile, result.UserBook.BookID
	default:
		// Insert the PDF row directly so recognition is not required.
		book := addUniqueBook(t)
		// Ensure the user_books row exists (required by tag/status services).
		require.NoError(t, testApp.Repositories.Books.UpsertUserBook(
			context.Background(),
			models.UserBook{ //nolint:exhaustruct //optional fields
				UserID:         ownerID,
				BookID:         book.ID,
				Status:         models.StatusToRead,
				Tags:           []string{},
				ShelfPositions: map[string]int{},
			},
		))
		key := fmt.Sprintf("books/pdf-test-%s.pdf", uuid.NewString())
		require.NoError(t, fakeStore.Put(
			context.Background(), key,
			bytes.NewReader([]byte("%PDF-fake")), 9, "application/pdf",
		))
		f := models.BookFile{ //nolint:exhaustruct //optional fields omitted
			BookID:     book.ID,
			UserID:     ownerID,
			Format:     models.FileFormatPDF,
			StorageKey: key,
			SizeBytes:  9,
			Status:     models.FileStatusReady,
		}
		bf, err := testApp.Repositories.BookFiles.Insert(context.Background(), f)
		require.NoError(t, err)
		return bf, book.ID
	}
}

// insertKEPUBRow inserts a KEPUB book_file row directly (no objectstore entry).
func insertKEPUBRow(t *testing.T, bookID uuid.UUID, ownerID string) {
	t.Helper()
	key := "users/" + ownerID + "/books/" + bookID.String() + "/derived.kepub"
	f := models.BookFile{ //nolint:exhaustruct //optional fields omitted
		BookID:     bookID,
		UserID:     ownerID,
		Format:     models.FileFormatKEPUB,
		StorageKey: key,
		SizeBytes:  8,
		Status:     models.FileStatusReady,
	}
	_, err := testApp.Repositories.BookFiles.Insert(context.Background(), f)
	require.NoError(t, err)
}

// --- service-level tests ---

func TestGetBookFile_ByFormat_EPUB_Found(t *testing.T) {
	bf, bookID := uploadFileForOwner(t, userID, models.FileFormatEPUB)
	require.Equal(t, models.FileFormatEPUB, bf.Format)

	result, err := testApp.Services.Books.GetBookFile(
		context.Background(), userID, bookID, models.FileFormatEPUB,
	)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.NotEmpty(t, result.URL)
	assert.Equal(t, models.FileFormatEPUB, result.Format)
	assert.False(t, result.ExpiresAt.IsZero())
}

func TestGetBookFile_ByFormat_PDF_Found(t *testing.T) {
	bf, bookID := uploadFileForOwner(t, userID, models.FileFormatPDF)
	require.Equal(t, models.FileFormatPDF, bf.Format)

	result, err := testApp.Services.Books.GetBookFile(
		context.Background(), userID, bookID, models.FileFormatPDF,
	)
	require.NoError(t, err)
	assert.Equal(t, models.FileFormatPDF, result.Format)
	assert.NotEmpty(t, result.URL)
}

func TestGetBookFile_NoFormat_FindsPrimary(t *testing.T) {
	_, bookID := uploadFileForOwner(t, userID, models.FileFormatPDF)

	result, err := testApp.Services.Books.GetBookFile(
		context.Background(), userID, bookID, "",
	)
	require.NoError(t, err)
	assert.Equal(t, models.FileFormatPDF, result.Format)
	assert.NotEmpty(t, result.URL)
}

func TestGetBookFile_NoFormat_SkipsKEPUB(t *testing.T) {
	// Only a KEPUB row in the DB — no pdf/epub — must return NotFound.
	book := addUniqueBook(t)
	insertKEPUBRow(t, book.ID, userID)

	_, err := testApp.Services.Books.GetBookFile(
		context.Background(), userID, book.ID, "",
	)
	assert.ErrorIs(t, err, database.ErrResourceNotFound)
}

func TestGetBookFile_NoFile_ReturnsNotFound(t *testing.T) {
	book := addUniqueBook(t)

	_, err := testApp.Services.Books.GetBookFile(
		context.Background(), userID, book.ID, models.FileFormatEPUB,
	)
	assert.ErrorIs(t, err, database.ErrResourceNotFound)
}

// TestGetBookFile_OtherUserBook_ReturnsNotFound verifies IDOR protection:
// user B must receive NotFound (not PermissionDenied) for user A's file.
func TestGetBookFile_OtherUserBook_ReturnsNotFound(t *testing.T) {
	_, bookID := uploadFileForOwner(t, userID, models.FileFormatEPUB)

	const otherUser = "other-user-idor-check-getfile"
	_, err := testApp.Services.Books.GetBookFile(
		context.Background(), otherUser, bookID, models.FileFormatEPUB,
	)
	assert.ErrorIs(t, err, database.ErrResourceNotFound)
}

func TestGetBookFile_UnknownBookID_ReturnsNotFound(t *testing.T) {
	_, err := testApp.Services.Books.GetBookFile(
		context.Background(), userID, uuid.New(), "",
	)
	assert.ErrorIs(t, err, database.ErrResourceNotFound)
}

// --- handler-level tests ---

func TestConnectGetBookFile_OK(t *testing.T) {
	client := newBooksTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	_, bookID := uploadFileForOwner(t, userID, models.FileFormatEPUB)

	req := connect.NewRequest(&booksv1.GetBookFileRequest{
		BookId: bookID.String(),
		Format: models.FileFormatEPUB,
	})
	req.Header().Set("Cookie", accessToken.String())

	resp, err := client.GetBookFile(ctx, req)
	require.NoError(t, err)
	assert.NotEmpty(t, resp.Msg.Url)
	assert.NotEmpty(t, resp.Msg.ExpiresAt)
	assert.Equal(t, models.FileFormatEPUB, resp.Msg.Format)
}

func TestConnectGetBookFile_NoFormat_OK(t *testing.T) {
	client := newBooksTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	_, bookID := uploadFileForOwner(t, userID, models.FileFormatPDF)

	req := connect.NewRequest(&booksv1.GetBookFileRequest{
		BookId: bookID.String(),
	})
	req.Header().Set("Cookie", accessToken.String())

	resp, err := client.GetBookFile(ctx, req)
	require.NoError(t, err)
	assert.Equal(t, models.FileFormatPDF, resp.Msg.Format)
}

func TestConnectGetBookFile_NotFound(t *testing.T) {
	client := newBooksTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := connect.NewRequest(&booksv1.GetBookFileRequest{
		BookId: uuid.NewString(),
	})
	req.Header().Set("Cookie", accessToken.String())

	_, err := client.GetBookFile(ctx, req)
	require.Error(t, err)
	var connectErr *connect.Error
	require.ErrorAs(t, err, &connectErr)
	assert.Equal(t, connect.CodeNotFound, connectErr.Code())
}

func TestConnectGetBookFile_InvalidBookID(t *testing.T) {
	client := newBooksTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := connect.NewRequest(&booksv1.GetBookFileRequest{
		BookId: "not-a-uuid",
	})
	req.Header().Set("Cookie", accessToken.String())

	_, err := client.GetBookFile(ctx, req)
	require.Error(t, err)
	var connectErr *connect.Error
	require.ErrorAs(t, err, &connectErr)
	assert.Equal(t, connect.CodeInvalidArgument, connectErr.Code())
}
