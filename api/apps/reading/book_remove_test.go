package reading_test

import (
	"context"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/reading/internal/models"
	"tools.xdoubleu.com/apps/reading/internal/services"
	readingv1 "tools.xdoubleu.com/gen/reading/v1"
)

// removeBookUser is an isolated user ID to avoid collisions with other tests.
const removeBookUser = "remove-book-test-user"

func cleanupRemoveBookUser(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		_, _ = testDB.Exec(context.Background(),
			`DELETE FROM reading.user_books WHERE user_id = $1`, removeBookUser)
		_, _ = testDB.Exec(context.Background(),
			`DELETE FROM reading.book_files WHERE user_id = $1`, removeBookUser)
		_, _ = testDB.Exec(
			context.Background(),
			`DELETE FROM reading.book_reading_state WHERE user_id = $1`,
			removeBookUser,
		)
	})
}

func TestRemoveFromLibrary_Service_CancelledContext_ReturnsError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately so the first DB call fails

	err := testApp.Services.Books.RemoveFromLibrary(
		ctx, removeBookUser, uuid.New(),
	)
	require.Error(t, err)
}

func TestRemoveFromLibrary_Service_DeletesUserBook(t *testing.T) {
	cleanupRemoveBookUser(t)
	book, err := testApp.Services.Books.AddToLibrary(
		context.Background(), removeBookUser,
		services.SourceProposal{ //nolint:exhaustruct //only required fields
			Source:   "manual",
			Title:    "RemoveBookOnly",
			Authors:  []string{"Remove Author"},
			ISBN13:   "9780316769488",
			CoverURL: "https://example.com/cover.jpg",
		},
		models.StatusToRead,
		[]string{},
	)
	require.NoError(t, err)

	err = testApp.Services.Books.RemoveFromLibrary(
		context.Background(), removeBookUser, book.BookID,
	)
	require.NoError(t, err)

	var count int
	err = testDB.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM reading.user_books WHERE user_id = $1 AND book_id = $2`,
		removeBookUser, book.BookID,
	).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestRemoveFromLibrary_Service_LastReference_DeletesOrphanCatalogRow(t *testing.T) {
	cleanupRemoveBookUser(t)
	book, err := testApp.Services.Books.AddToLibrary(
		context.Background(), removeBookUser,
		services.SourceProposal{ //nolint:exhaustruct //only required fields
			Source:   "manual",
			Title:    "RemoveBookOrphan",
			Authors:  []string{"Remove Author"},
			ISBN13:   "9780316769489",
			CoverURL: "https://example.com/cover.jpg",
		},
		models.StatusToRead,
		[]string{},
	)
	require.NoError(t, err)

	err = testApp.Services.Books.RemoveFromLibrary(
		context.Background(), removeBookUser, book.BookID,
	)
	require.NoError(t, err)

	var count int
	err = testDB.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM reading.books WHERE id = $1`, book.BookID,
	).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 0, count, "orphaned catalog row should be deleted")
}

func TestRemoveFromLibrary_Service_OtherUserStillHas_KeepsCatalogRowAndFile(
	t *testing.T,
) {
	cleanupRemoveBookUser(t)

	// Seed the shared book for the primary test user, then add removeBookUser
	// to the same book's library so it is shared.
	shared := addTestBookWithISBN(t, "RemoveBookShared", "9780316769490")
	err := testApp.Repositories.Books.UpsertUserBook(
		context.Background(),
		models.UserBook{ //nolint:exhaustruct //optional fields
			UserID: removeBookUser,
			BookID: shared.BookID,
			Status: models.StatusToRead,
			Tags:   []string{},
		},
	)
	require.NoError(t, err)

	err = testApp.Services.Books.RemoveFromLibrary(
		context.Background(), removeBookUser, shared.BookID,
	)
	require.NoError(t, err)

	// removeBookUser's entry is gone.
	var count int
	err = testDB.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM reading.user_books WHERE user_id = $1 AND book_id = $2`,
		removeBookUser, shared.BookID,
	).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 0, count)

	// The catalog row survives because the other user still references it.
	err = testDB.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM reading.books WHERE id = $1`, shared.BookID,
	).Scan(&count)
	require.NoError(t, err)
	assert.Equal(
		t,
		1,
		count,
		"catalog row must survive while another user references it",
	)
}

func TestRemoveFromLibrary_Service_DeletesR2ObjectWhenUnreferenced(t *testing.T) {
	cleanupRemoveBookUser(t)
	localFake := fakeStore

	seedBookInLibrary(
		t,
		removeBookUser,
		"RemoveFileBook",
		"Remove Author",
		"9780316769491",
	)

	data := buildEPUBBytes("RemoveFileBook", "Remove Author", "9780316769491")
	result, err := simulateUpload(
		context.Background(), t, removeBookUser,
		"remove-test.epub", "application/epub+zip", data, localFake,
	)
	require.NoError(t, err)
	require.NotNil(t, result)
	storageKey := result.BookFile.StorageKey

	exists, err := localFake.Exists(context.Background(), storageKey)
	require.NoError(t, err)
	assert.True(t, exists, "uploaded object should exist before removal")

	err = testApp.Services.Books.RemoveFromLibrary(
		context.Background(), removeBookUser, result.BookFile.BookID,
	)
	require.NoError(t, err)

	exists, err = localFake.Exists(context.Background(), storageKey)
	require.NoError(t, err)
	assert.False(t, exists, "object should be gone after the last reference is removed")
}

func TestRemoveFromLibrary_Service_SharedStorageKey_KeepsR2Object(t *testing.T) {
	cleanupRemoveBookUser(t)
	book := addUniqueBook(t)
	const sharedKey = "books/shared/remove-test-shared.epub"
	fakeStore.PutAt(sharedKey, []byte("shared content"), time.Now())

	// Two users' book_files rows point at the same content-addressed storage
	// key (as happens with cross-user dedup on upload).
	_, err := testApp.Repositories.BookFiles.Insert(
		context.Background(),
		models.BookFile{ //nolint:exhaustruct //optional fields omitted
			BookID:     book.ID,
			UserID:     removeBookUser,
			Format:     models.FileFormatEPUB,
			StorageKey: sharedKey,
			SizeBytes:  512,
			Status:     models.FileStatusReady,
		},
	)
	require.NoError(t, err)
	_, err = testApp.Repositories.BookFiles.Insert(
		context.Background(),
		models.BookFile{ //nolint:exhaustruct //optional fields omitted
			BookID:     book.ID,
			UserID:     userID,
			Format:     models.FileFormatEPUB,
			StorageKey: sharedKey,
			SizeBytes:  512,
			Status:     models.FileStatusReady,
		},
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = testDB.Exec(context.Background(),
			`DELETE FROM reading.book_files WHERE user_id = $1 AND storage_key = $2`,
			userID, sharedKey)
	})
	err = testApp.Repositories.Books.UpsertUserBook(
		context.Background(),
		models.UserBook{ //nolint:exhaustruct //optional fields
			UserID: removeBookUser,
			BookID: book.ID,
			Status: models.StatusToRead,
			Tags:   []string{},
		},
	)
	require.NoError(t, err)
	err = testApp.Repositories.Books.UpsertUserBook(
		context.Background(),
		models.UserBook{ //nolint:exhaustruct //optional fields
			UserID: userID,
			BookID: book.ID,
			Status: models.StatusToRead,
			Tags:   []string{},
		},
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = testDB.Exec(context.Background(),
			`DELETE FROM reading.user_books WHERE user_id = $1 AND book_id = $2`,
			userID, book.ID)
	})

	err = testApp.Services.Books.RemoveFromLibrary(
		context.Background(), removeBookUser, book.ID,
	)
	require.NoError(t, err)

	exists, err := fakeStore.Exists(context.Background(), sharedKey)
	require.NoError(t, err)
	assert.True(t, exists, "object referenced by another user's file must survive")

	var count int
	err = testDB.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM reading.user_books WHERE user_id = $1 AND book_id = $2`,
		userID, book.ID,
	).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count, "other user's library entry must survive")
}

func TestConnectRemoveBook_OK(t *testing.T) {
	book, err := testApp.Services.Books.AddToLibrary(
		context.Background(), userID,
		services.SourceProposal{ //nolint:exhaustruct //only required fields
			Source:   "manual",
			Title:    "RemoveHandlerBook",
			Authors:  []string{"H Author"},
			ISBN13:   "9780316769492",
			CoverURL: "https://example.com/cover.jpg",
		},
		models.StatusToRead,
		[]string{},
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = testDB.Exec(context.Background(),
			`DELETE FROM reading.user_books WHERE user_id = $1 AND book_id = $2`,
			userID, book.BookID)
	})

	client := newBooksTestClient(t)
	req := connect.NewRequest(
		&readingv1.RemoveBookRequest{BookId: book.BookID.String()},
	)
	req.Header().Set("Cookie", accessToken.String())
	_, err = client.RemoveBook(context.Background(), req)
	require.NoError(t, err)

	var count int
	err = testDB.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM reading.user_books WHERE user_id = $1 AND book_id = $2`,
		userID, book.BookID,
	).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestConnectRemoveBook_InvalidBookID(t *testing.T) {
	client := newBooksTestClient(t)
	req := connect.NewRequest(&readingv1.RemoveBookRequest{BookId: "not-a-uuid"})
	req.Header().Set("Cookie", accessToken.String())
	_, err := client.RemoveBook(context.Background(), req)
	require.Error(t, err)
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}
