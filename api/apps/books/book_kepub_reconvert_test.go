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
)

func TestConnectGetKEPUBStatus_StaleKEPUBReportsConverting(t *testing.T) {
	client := newBooksTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	_, bookID := uploadFileForOwner(t, userID, models.FileFormatEPUB)
	insertStaleKEPUBRow(t, bookID, userID)

	req := connect.NewRequest(&booksv1.GetKEPUBStatusRequest{BookId: bookID.String()})
	req.Header().Set("Cookie", accessToken.String())

	resp, err := client.GetKEPUBStatus(ctx, req)
	require.NoError(t, err)
	assert.True(t, resp.Msg.HasEpub)
	assert.Equal(t, models.FileStatusConverting, resp.Msg.KepubStatus)

	// The read started the reconversion, so "converting" resolves.
	require.Eventually(t, func() bool {
		status, statusErr := testApp.Services.Books.GetKEPUBStatus(
			context.Background(), userID, bookID,
		)
		return statusErr == nil && !status.KepubStale &&
			status.KepubStatus == models.FileStatusReady
	}, 10*time.Second, 20*time.Millisecond)
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
	stale, err := testApp.Repositories.BookFiles.Insert(
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

// TestEnsureKEPUB_StaleReplacementHasNoGap: a status read while EnsureKEPUB
// swaps a stale row for its placeholder still finds a KEPUB row.
func TestEnsureKEPUB_StaleReplacementHasNoGap(t *testing.T) {
	book := addUniqueBook(t)
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

	status, err := testApp.Services.Books.GetKEPUBStatus(
		context.Background(), userID, book.ID,
	)
	require.NoError(t, err)
	assert.NotEmpty(t, status.KepubStatus,
		"a status read mid-replacement must still see a KEPUB row")

	release()
	require.NoError(t, <-done)
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
