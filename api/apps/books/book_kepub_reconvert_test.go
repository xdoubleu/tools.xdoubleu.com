package books_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect/v2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/books/internal/models"
	booksv1 "tools.xdoubleu.com/gen/books/v1"
)

// drainKEPUBQueue runs the workers job's drain until the shared queue is empty,
// since other tests may have left requests ahead of the one under test.
func drainKEPUBQueue(t *testing.T) {
	t.Helper()
	for {
		n, err := testApp.TestSeeds().Services.KEPUBQueue.Drain(context.Background())
		require.NoError(t, err)
		if n == 0 {
			return
		}
	}
}

func TestConnectGetKEPUBStatus_StaleKEPUBReportsConverting(t *testing.T) {
	client := newBooksTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	_, bookID := uploadFileForOwner(t, userID, models.FileFormatEPUB)
	insertStaleKEPUBRow(t, bookID, userID)

	req := &booksv1.GetKEPUBStatusRequest{BookId: bookID.String()}
	ctx, info := connect.NewClientContext(ctx)
	info.RequestHeader().Set("Cookie", accessToken.String())

	resp, err := client.GetKEPUBStatus(ctx, req)
	require.NoError(t, err)
	assert.True(t, resp.HasEpub)
	assert.Equal(t, models.FileStatusConverting, resp.KepubStatus)

	// The read queued the reconversion; the workers role's drain resolves it.
	drainKEPUBQueue(t)
	status, err := testApp.TestSeeds().Services.Books.GetKEPUBStatus(
		context.Background(), userID, bookID,
	)
	require.NoError(t, err)
	assert.False(t, status.KepubStale)
	assert.Equal(t, models.FileStatusReady, status.KepubStatus)
}

// TestKEPUBQueue_RequestsCollapseAndDrainClears: repeated requests for one book
// queue a single conversion, and a drain attempts it once and empties the queue.
func TestKEPUBQueue_RequestsCollapseAndDrainClears(t *testing.T) {
	ctx := context.Background()
	owner := "kepub-queue-" + uuid.NewString()
	_, bookID := uploadFileForOwner(t, owner, models.FileFormatEPUB)

	for range 2 {
		require.NoError(
			t,
			testApp.TestSeeds().Services.KEPUBQueue.Request(ctx, owner, bookID),
		)
	}
	var queued int
	require.NoError(t, testDB.QueryRow(ctx,
		`SELECT count(*) FROM books.kepub_conversion_requests WHERE user_id = $1`,
		owner,
	).Scan(&queued))
	assert.Equal(t, 1, queued)

	drainKEPUBQueue(t)
	require.NoError(t, testDB.QueryRow(ctx,
		`SELECT count(*) FROM books.kepub_conversion_requests WHERE user_id = $1`,
		owner,
	).Scan(&queued))
	assert.Zero(t, queued)

	kepub, err := testApp.TestSeeds().Repositories.BookFiles.GetByBookAndFormat(
		ctx, owner, bookID, models.FileFormatKEPUB,
	)
	require.NoError(t, err)
	assert.Equal(t, models.FileStatusReady, kepub.Status)
}

// lockRow holds FOR UPDATE on a book_files row in its own transaction, which
// blocks any INSERT referencing it (the FK check takes FOR KEY SHARE). It
// returns that transaction's backend pid and a release func.
func lockRow(t *testing.T, id uuid.UUID) (int32, func()) {
	t.Helper()
	ctx := context.Background()
	tx, err := testDB.Begin(ctx)
	require.NoError(t, err)

	var pid int32
	require.NoError(t, tx.QueryRow(ctx, `
		SELECT pg_backend_pid() FROM books.book_files WHERE id = $1 FOR UPDATE
	`, id).Scan(&pid))

	var once sync.Once
	release := func() { once.Do(func() { _ = tx.Rollback(ctx) }) }
	t.Cleanup(release)
	return pid, release
}

// waitForBlocked waits until n backends wait, directly or through another
// waiter, on the transaction of holderPID.
func waitForBlocked(t *testing.T, holderPID int32, n int) {
	t.Helper()
	require.Eventually(t, func() bool {
		var blocked int
		err := testDB.QueryRow(context.Background(), `
			WITH direct AS (
				SELECT pid FROM pg_stat_activity
				WHERE $1 = ANY(pg_blocking_pids(pid))
			)
			SELECT count(*) FROM pg_stat_activity a
			WHERE a.pid IN (SELECT pid FROM direct)
			   OR EXISTS (
				SELECT 1 FROM unnest(pg_blocking_pids(a.pid)) b
				WHERE b IN (SELECT pid FROM direct)
			   )
		`, holderPID).Scan(&blocked)
		return err == nil && blocked >= n
	}, 10*time.Second, 20*time.Millisecond)
}

func insertStaleKEPUBFrom(
	t *testing.T,
	bookID uuid.UUID,
	sourceID uuid.UUID,
) *models.BookFile {
	t.Helper()
	stale, err := testApp.TestSeeds().Repositories.BookFiles.Insert(
		context.Background(),
		models.BookFile{ //nolint:exhaustruct //optional nullable fields omitted
			BookID:       bookID,
			UserID:       userID,
			Format:       models.FileFormatKEPUB,
			StorageKey:   "users/" + userID + "/books/" + bookID.String() + "/old.kepub",
			SizeBytes:    8,
			Status:       models.FileStatusReady,
			SourceFileID: &sourceID,
		},
	)
	require.NoError(t, err)
	return stale
}

func countKEPUBRows(t *testing.T, bookID uuid.UUID) int {
	t.Helper()
	files, err := testApp.TestSeeds().Repositories.BookFiles.ListByBook(
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

// TestEnsureKEPUB_StaleReplacementHasNoGap: a status read while EnsureKEPUB
// swaps a stale row for its placeholder still finds a KEPUB row.
func TestEnsureKEPUB_StaleReplacementHasNoGap(t *testing.T) {
	book := seedContentBookInLibrary(t, userID)
	conv, store := newTestConversionService(
		&fakeEPUBConverter{out: []byte("fresh kepub"), err: nil}, nil,
	)
	source := seedEPUBFile(t, store, book.ID)
	insertStaleKEPUBFrom(t, book.ID, source.ID)

	holder, release := lockRow(t, source.ID)
	done := make(chan error, 1)
	go func() {
		_, err := conv.EnsureKEPUB(context.Background(), userID, book.ID)
		done <- err
	}()
	waitForBlocked(t, holder, 1)

	status, err := testApp.TestSeeds().Services.Books.GetKEPUBStatus(
		context.Background(), userID, book.ID,
	)
	require.NoError(t, err)
	assert.NotEmpty(t, status.KepubStatus,
		"a status read mid-replacement must still see a KEPUB row")

	release()
	require.NoError(t, <-done)
	assert.Equal(t, 1, countKEPUBRows(t, book.ID))
}

// TestBookFilesReplace_FailedInsertKeepsStaleRow: the delete rolls back with
// a failed insert.
func TestBookFilesReplace_FailedInsertKeepsStaleRow(t *testing.T) {
	book := addUniqueBook(t)
	_, store := newTestConversionService(nil, nil)
	source := seedEPUBFile(t, store, book.ID)
	stale := insertStaleKEPUBFrom(t, book.ID, source.ID)

	_, err := testApp.TestSeeds().Repositories.BookFiles.Replace(
		context.Background(), stale.ID,
		models.BookFile{ //nolint:exhaustruct //optional nullable fields omitted
			BookID: book.ID,
			UserID: userID,
			Format: "not-a-format",
			Status: models.FileStatusConverting,
		},
	)
	require.Error(t, err)

	kept, err := testApp.TestSeeds().Repositories.BookFiles.GetByID(
		context.Background(),
		stale.ID,
	)
	require.NoError(t, err)
	assert.Equal(t, models.FileStatusReady, kept.Status)
}

// TestEnsureKEPUB_InProgressPlaceholderIsKept: a converting placeholder
// (version 0) belongs to a running conversion and is not replaced.
func TestEnsureKEPUB_InProgressPlaceholderIsKept(t *testing.T) {
	book := addUniqueBook(t)
	conv, store := newTestConversionService(
		&fakeEPUBConverter{out: []byte("fresh kepub"), err: nil}, nil,
	)
	source := seedEPUBFile(t, store, book.ID)
	sourceID := source.ID
	placeholder, err := testApp.TestSeeds().Repositories.BookFiles.Insert(
		context.Background(),
		models.BookFile{ //nolint:exhaustruct //optional nullable fields omitted
			BookID:       book.ID,
			UserID:       userID,
			Format:       models.FileFormatKEPUB,
			Status:       models.FileStatusConverting,
			SourceFileID: &sourceID,
		},
	)
	require.NoError(t, err)

	result, err := conv.EnsureKEPUB(context.Background(), userID, book.ID)
	require.NoError(t, err)
	assert.Equal(t, placeholder.ID, result.ID)
	assert.Equal(t, models.FileStatusConverting, result.Status)
	assert.Equal(t, 1, countKEPUBRows(t, book.ID))
}

// TestEnsureKEPUB_ConcurrentStaleReplacement: two reconversions of one stale
// row leave a single KEPUB row.
func TestEnsureKEPUB_ConcurrentStaleReplacement(t *testing.T) {
	book := addUniqueBook(t)
	conv, store := newTestConversionService(
		&fakeEPUBConverter{out: []byte("fresh kepub"), err: nil}, nil,
	)
	source := seedEPUBFile(t, store, book.ID)
	insertStaleKEPUBFrom(t, book.ID, source.ID)

	holder, release := lockRow(t, source.ID)
	results := make(chan *models.BookFile, 2)
	errs := make(chan error, 2)
	run := func() {
		f, err := conv.EnsureKEPUB(context.Background(), userID, book.ID)
		results <- f
		errs <- err
	}
	go run()
	waitForBlocked(t, holder, 1)
	go run()
	waitForBlocked(t, holder, 2)

	release()
	for range 2 {
		require.NoError(t, <-errs)
		require.NotNil(t, <-results)
	}
	assert.Equal(t, 1, countKEPUBRows(t, book.ID))
}

// TestKEPUBQueue_FailedConversionIsDropped: a request for a book with no
// source file fails to convert, and is still removed so it can't clog the queue.
func TestKEPUBQueue_FailedConversionIsDropped(t *testing.T) {
	ctx := context.Background()
	owner := "kepub-queue-fail-" + uuid.NewString()
	book := addUniqueBook(t)
	require.NoError(
		t,
		testApp.TestSeeds().Services.KEPUBQueue.Request(ctx, owner, book.ID),
	)

	drainKEPUBQueue(t)

	var queued int
	require.NoError(t, testDB.QueryRow(ctx,
		`SELECT count(*) FROM books.kepub_conversion_requests WHERE user_id = $1`,
		owner,
	).Scan(&queued))
	assert.Zero(t, queued)
}

func TestKEPUBQueue_CancelledContextErrors(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	require.Error(t, testApp.TestSeeds().Services.KEPUBQueue.Request(ctx, "u", uuid.New()))
	_, err := testApp.TestSeeds().Services.KEPUBQueue.Drain(ctx)
	require.Error(t, err)
}
