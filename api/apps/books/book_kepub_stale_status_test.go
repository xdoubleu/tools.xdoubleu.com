package books_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/books/internal/models"
	booksv1 "tools.xdoubleu.com/gen/books/v1"
	"tools.xdoubleu.com/internal/database"
)

func countKEPUBRows(t *testing.T, bookID uuid.UUID) int {
	t.Helper()
	files, err := testApp.Repositories.BookFiles.ListByBook(
		context.Background(), userID, bookID,
	)
	require.NoError(t, err)
	n := 0
	for _, f := range files {
		if f.Format == models.FileFormatKEPUB {
			n++
		}
	}
	return n
}

// A stale KEPUB must never be reported ready, matching what
// RequestKEPUBConversion does with it.
func TestConnectGetKEPUBStatus_StaleKEPUB_ReportsConverting(t *testing.T) {
	client := newBooksTestClient(t)
	_, bookID := uploadFileForOwner(t, userID, models.FileFormatEPUB)
	insertStaleKEPUBRow(t, bookID, userID)

	req := connect.NewRequest(&booksv1.GetKEPUBStatusRequest{BookId: bookID.String()})
	req.Header().Set("Cookie", accessToken.String())

	resp, err := client.GetKEPUBStatus(context.Background(), req)
	require.NoError(t, err)
	assert.True(t, resp.Msg.HasEpub)
	assert.Equal(t, models.FileStatusConverting, resp.Msg.KepubStatus)
}

// A missing KEPUB stays a side-effect-free read.
func TestConnectGetKEPUBStatus_MissingKEPUB_StaysEmpty(t *testing.T) {
	client := newBooksTestClient(t)
	_, bookID := uploadFileForOwner(t, userID, models.FileFormatEPUB)

	req := connect.NewRequest(&booksv1.GetKEPUBStatusRequest{BookId: bookID.String()})
	req.Header().Set("Cookie", accessToken.String())

	resp, err := client.GetKEPUBStatus(context.Background(), req)
	require.NoError(t, err)
	assert.Empty(t, resp.Msg.KepubStatus)
	assert.Equal(t, 0, countKEPUBRows(t, bookID))
}

// While EnsureKEPUB swaps the stale row for the placeholder, a concurrent
// status read still sees a KEPUB row (never "").
func TestEnsureKEPUB_StaleReplacement_ReadsNeverSeeNoRow(t *testing.T) {
	book := addUniqueBook(t)
	conv, store := newTestConversionService(
		&fakeEPUBConverter{out: []byte("fresh kepub content"), err: nil},
		nil,
	)
	source := seedEPUBFile(t, store, book.ID)
	sourceID := source.ID
	_, err := testApp.Repositories.BookFiles.Insert(
		context.Background(),
		models.BookFile{ //nolint:exhaustruct //optional nullable fields omitted
			BookID:       book.ID,
			UserID:       userID,
			Format:       models.FileFormatKEPUB,
			StorageKey:   "users/" + userID + "/stale.kepub",
			SizeBytes:    20,
			Status:       models.FileStatusReady,
			SourceFileID: &sourceID,
		},
	)
	require.NoError(t, err)

	// Holding a row lock on the source blocks the placeholder insert (FK key
	// share) after the stale row's delete has run inside the same transaction.
	ctx := context.Background()
	lockTx, err := testDB.Begin(ctx)
	require.NoError(t, err)
	_, err = lockTx.Exec(
		ctx, `SELECT 1 FROM books.book_files WHERE id = $1 FOR UPDATE`, source.ID,
	)
	require.NoError(t, err)

	done := make(chan error, 1)
	go func() {
		_, ensureErr := conv.EnsureKEPUB(ctx, userID, book.ID)
		done <- ensureErr
	}()
	time.Sleep(500 * time.Millisecond)

	got, readErr := testApp.Repositories.BookFiles.GetByBookAndFormat(
		ctx, userID, book.ID, models.FileFormatKEPUB,
	)
	require.NoError(t, readErr, "a status read mid-replacement must see a KEPUB row")
	assert.NotNil(t, got)

	require.NoError(t, lockTx.Rollback(ctx))
	require.NoError(t, <-done)
	assert.Equal(t, 1, countKEPUBRows(t, book.ID))
}

// Two concurrent EnsureKEPUB calls on one stale row leave exactly one row.
func TestEnsureKEPUB_ConcurrentStale_LeavesOneRow(t *testing.T) {
	book := addUniqueBook(t)
	conv, store := newTestConversionService(
		&fakeEPUBConverter{out: []byte("fresh kepub content"), err: nil},
		nil,
	)
	source := seedEPUBFile(t, store, book.ID)
	sourceID := source.ID
	_, err := testApp.Repositories.BookFiles.Insert(
		context.Background(),
		models.BookFile{ //nolint:exhaustruct //optional nullable fields omitted
			BookID:       book.ID,
			UserID:       userID,
			Format:       models.FileFormatKEPUB,
			StorageKey:   "users/" + userID + "/stale2.kepub",
			SizeBytes:    20,
			Status:       models.FileStatusReady,
			SourceFileID: &sourceID,
		},
	)
	require.NoError(t, err)

	const callers = 4
	var wg sync.WaitGroup
	start := make(chan struct{})
	errs := make([]error, callers)
	for i := range callers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, errs[i] = conv.EnsureKEPUB(context.Background(), userID, book.ID)
		}()
	}
	close(start)
	wg.Wait()

	for _, e := range errs {
		require.NoError(t, e)
	}
	assert.Equal(t, 1, countKEPUBRows(t, book.ID))
}

// Replace reports a lost race instead of inserting a duplicate.
func TestBookFilesReplace_StaleRowAlreadyGone_ReturnsFalse(t *testing.T) {
	book := addUniqueBook(t)
	ok, row, err := testApp.Repositories.BookFiles.Replace(
		context.Background(), uuid.New(),
		models.BookFile{ //nolint:exhaustruct //optional nullable fields omitted
			BookID: book.ID,
			UserID: userID,
			Format: models.FileFormatKEPUB,
			Status: models.FileStatusConverting,
		},
	)
	require.NoError(t, err)
	assert.False(t, ok)
	assert.Nil(t, row)
	_, getErr := testApp.Repositories.BookFiles.GetByBookAndFormat(
		context.Background(), userID, book.ID, models.FileFormatKEPUB,
	)
	assert.ErrorIs(t, getErr, database.ErrResourceNotFound)
}

// A failed insert rolls the delete back, leaving the stale row in place.
func TestBookFilesReplace_InsertFails_KeepsStaleRow(t *testing.T) {
	_, bookID := uploadFileForOwner(t, userID, models.FileFormatEPUB)
	insertStaleKEPUBRow(t, bookID, userID)
	stale, err := testApp.Repositories.BookFiles.GetByBookAndFormat(
		context.Background(), userID, bookID, models.FileFormatKEPUB,
	)
	require.NoError(t, err)

	_, _, err = testApp.Repositories.BookFiles.Replace(
		context.Background(), stale.ID,
		models.BookFile{ //nolint:exhaustruct //optional nullable fields omitted
			BookID: uuid.New(), // violates the book FK
			UserID: userID,
			Format: models.FileFormatKEPUB,
			Status: models.FileStatusConverting,
		},
	)
	require.Error(t, err)

	got, getErr := testApp.Repositories.BookFiles.GetByID(
		context.Background(), stale.ID,
	)
	require.NoError(t, getErr)
	assert.Equal(t, stale.ID, got.ID)
}
