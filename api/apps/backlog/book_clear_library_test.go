package backlog_test

import (
	"context"
	"fmt"
	"testing"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/backlog/internal/models"
	"tools.xdoubleu.com/apps/backlog/pkg/hardcover"
	backlogv1 "tools.xdoubleu.com/gen/backlog/v1"
)

// clearLibraryUser is an isolated user ID to avoid collisions with other tests.
const clearLibraryUser = "clear-library-test-user"

// addBooksForClear adds n distinct books to clearLibraryUser's library.
func addBooksForClear(t *testing.T, n int) {
	t.Helper()
	for i := range n {
		isbn := fmt.Sprintf("978%010d", i+1)
		cover := "https://example.com/cover.jpg"
		ext := hardcover.ExternalBook{ //nolint:exhaustruct //only required fields
			Provider:   "manual",
			ProviderID: fmt.Sprintf("clear-test-%d-%s", i, uuid.NewString()),
			Title:      fmt.Sprintf("ClearBook%d", i),
			Authors:    []string{"Clear Author"},
			ISBN13:     &isbn,
			CoverURL:   &cover,
		}
		_, err := testApp.Services.Books.AddToLibrary(
			context.Background(),
			clearLibraryUser,
			ext,
			models.StatusToRead,
			[]string{},
		)
		require.NoError(t, err)
	}
}

// cleanupClearLibraryUser removes all rows for clearLibraryUser after each test.
func cleanupClearLibraryUser(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		_, _ = testDB.Exec(context.Background(),
			`DELETE FROM backlog.user_books WHERE user_id = $1`, clearLibraryUser)
		_, _ = testDB.Exec(context.Background(),
			`DELETE FROM backlog.book_files WHERE user_id = $1`, clearLibraryUser)
		_, _ = testDB.Exec(
			context.Background(),
			`DELETE FROM backlog.book_reading_state WHERE user_id = $1`,
			clearLibraryUser,
		)
	})
}

// --- service-level tests ---

func TestClearLibrary_Service_CancelledContext_ReturnsError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately so the first DB call fails

	_, _, err := testApp.Services.Books.ClearLibrary(ctx, clearLibraryUser)
	require.Error(t, err)
}

func TestClearLibrary_Service_EmptyLibrary(t *testing.T) {
	cleanupClearLibraryUser(t)

	books, files, err := testApp.Services.Books.ClearLibrary(
		context.Background(), clearLibraryUser,
	)
	require.NoError(t, err)
	assert.Equal(t, uint32(0), books)
	assert.Equal(t, uint32(0), files)
}

func TestClearLibrary_Service_DeletesUserBooks(t *testing.T) {
	cleanupClearLibraryUser(t)
	addBooksForClear(t, 3)

	books, files, err := testApp.Services.Books.ClearLibrary(
		context.Background(), clearLibraryUser,
	)
	require.NoError(t, err)
	assert.Equal(t, uint32(3), books)
	assert.Equal(t, uint32(0), files)

	// Verify user_books are gone.
	var count int
	err = testDB.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM backlog.user_books WHERE user_id = $1`, clearLibraryUser,
	).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestClearLibrary_Service_DeletesFilesAndR2Objects(t *testing.T) {
	cleanupClearLibraryUser(t)
	localFake := fakeStore

	// Pre-seed the book so recognition succeeds during upload.
	seedBookInLibrary(
		t,
		clearLibraryUser,
		"ClearFileBook",
		"Clear Author",
		"9789999999999",
	)

	// Upload an ebook as clearLibraryUser.
	data := buildEPUBBytes("ClearFileBook", "Clear Author", "9789999999999")
	result, err := simulateUpload(
		context.Background(), t, clearLibraryUser,
		"clear-test.epub", "application/epub+zip", data, localFake,
	)
	require.NoError(t, err)
	require.NotNil(t, result)
	storageKey := result.BookFile.StorageKey

	// Confirm the object is in the store.
	exists, err := localFake.Exists(context.Background(), storageKey)
	require.NoError(t, err)
	assert.True(t, exists, "uploaded object should exist before clear")

	_, files, err := testApp.Services.Books.ClearLibrary(
		context.Background(), clearLibraryUser,
	)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, files, uint32(1))

	// Confirm the object was deleted from R2.
	exists, err = localFake.Exists(context.Background(), storageKey)
	require.NoError(t, err)
	assert.False(t, exists, "object should be gone after clear")
}

func TestClearLibrary_Service_DoesNotTouchSharedCatalog(t *testing.T) {
	cleanupClearLibraryUser(t)
	addBooksForClear(t, 2)

	// Count shared catalog rows before.
	var before int
	err := testDB.QueryRow(
		context.Background(), `SELECT COUNT(*) FROM backlog.books`,
	).Scan(&before)
	require.NoError(t, err)
	require.Positive(t, before)

	_, _, err = testApp.Services.Books.ClearLibrary(
		context.Background(), clearLibraryUser,
	)
	require.NoError(t, err)

	// Shared catalog must be unchanged.
	var after int
	err = testDB.QueryRow(
		context.Background(), `SELECT COUNT(*) FROM backlog.books`,
	).Scan(&after)
	require.NoError(t, err)
	assert.Equal(t, before, after, "shared book catalog must not be modified")
}

func TestClearLibrary_Service_DoesNotTouchOtherUser(t *testing.T) {
	cleanupClearLibraryUser(t)

	// Add a book for clearLibraryUser AND one for the main testApp userID.
	addBooksForClear(t, 1)
	otherBook := addTestBookWithISBN(t, "OtherUserShouldSurvive", "9781234509876")

	_, _, err := testApp.Services.Books.ClearLibrary(
		context.Background(), clearLibraryUser,
	)
	require.NoError(t, err)

	// The other user's book must still exist.
	got, err := testApp.Repositories.Books.GetUserBook(
		context.Background(), userID, otherBook.BookID,
	)
	require.NoError(t, err)
	assert.Equal(t, otherBook.BookID, got.BookID)
}

// --- repo-level tests ---

func TestBookFilesRepo_StorageKeysByUser_Empty(t *testing.T) {
	keys, err := testApp.Repositories.BookFiles.StorageKeysByUser(
		context.Background(), "no-such-user-abc",
	)
	require.NoError(t, err)
	assert.Empty(t, keys)
}

func TestBookFilesRepo_DeleteByUser(t *testing.T) {
	cleanupClearLibraryUser(t)
	book := addUniqueBook(t)
	f := models.BookFile{ //nolint:exhaustruct //optional fields omitted
		BookID:     book.ID,
		UserID:     clearLibraryUser,
		Format:     models.FileFormatEPUB,
		StorageKey: "users/clear/books/file.epub",
		SizeBytes:  512,
		Status:     models.FileStatusReady,
	}
	_, err := testApp.Repositories.BookFiles.Insert(context.Background(), f)
	require.NoError(t, err)

	n, err := testApp.Repositories.BookFiles.DeleteByUser(
		context.Background(), clearLibraryUser,
	)
	require.NoError(t, err)
	assert.Equal(t, int64(1), n)

	keys, err := testApp.Repositories.BookFiles.StorageKeysByUser(
		context.Background(), clearLibraryUser,
	)
	require.NoError(t, err)
	assert.Empty(t, keys)
}

func TestBooksRepo_DeleteUserBooks(t *testing.T) {
	cleanupClearLibraryUser(t)
	addBooksForClear(t, 2)

	n, err := testApp.Repositories.Books.DeleteUserBooks(
		context.Background(), clearLibraryUser,
	)
	require.NoError(t, err)
	assert.Equal(t, int64(2), n)
}

// --- connect handler tests ---

func TestConnectClearLibrary_OK(t *testing.T) {
	// newBooksTestClient resolves auth to the global userID; add books for it
	// and clean up afterwards so other tests are not affected.
	isbn1 := "9780099512432"
	isbn2 := "9780007458424"
	cover := "https://example.com/cover.jpg"
	for _, ext := range []hardcover.ExternalBook{
		{ //nolint:exhaustruct //only required fields
			Provider:   "manual",
			ProviderID: "clear-handler-1",
			Title:      "ClearHandlerBook1",
			Authors:    []string{"H Author"},
			ISBN13:     &isbn1,
			CoverURL:   &cover,
		},
		{ //nolint:exhaustruct //only required fields
			Provider:   "manual",
			ProviderID: "clear-handler-2",
			Title:      "ClearHandlerBook2",
			Authors:    []string{"H Author"},
			ISBN13:     &isbn2,
			CoverURL:   &cover,
		},
	} {
		_, err := testApp.Services.Books.AddToLibrary(
			context.Background(), userID, ext, models.StatusToRead, []string{},
		)
		require.NoError(t, err)
	}
	t.Cleanup(func() {
		_, _ = testDB.Exec(context.Background(),
			`DELETE FROM backlog.user_books WHERE user_id = $1`, userID)
	})

	client := newBooksTestClient(t)
	req := connect.NewRequest(&backlogv1.ClearLibraryRequest{})
	req.Header().Set("Cookie", accessToken.String())
	resp, err := client.ClearLibrary(context.Background(), req)
	require.NoError(t, err)
	// At least the 2 books added above (may be more from other tests in the suite).
	assert.GreaterOrEqual(t, resp.Msg.DeletedBooks, uint32(2))
}
