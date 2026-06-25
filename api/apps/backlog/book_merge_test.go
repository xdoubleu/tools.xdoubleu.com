package backlog_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/backlog/internal/models"
	"tools.xdoubleu.com/apps/backlog/pkg/openlibrary"
	backlogv1 "tools.xdoubleu.com/gen/backlog/v1"
)

const mergeTestUser = "merge-books-test-user"

func cleanupMergeUser(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		ctx := context.Background()
		_, _ = testDB.Exec(ctx,
			`DELETE FROM backlog.user_books WHERE user_id = $1`, mergeTestUser)
		_, _ = testDB.Exec(ctx,
			`DELETE FROM backlog.book_files WHERE user_id = $1`, mergeTestUser)
		_, _ = testDB.Exec(ctx,
			`DELETE FROM backlog.book_reading_state WHERE user_id = $1`, mergeTestUser)
	})
}

func addMergeBook(
	t *testing.T,
	title, isbn string,
	status string,
	tags []string,
) *models.UserBook {
	t.Helper()
	cover := "https://example.com/cover.jpg"
	ext := openlibrary.ExternalBook{ //nolint:exhaustruct //only required fields
		Provider:   "manual",
		ProviderID: fmt.Sprintf("merge-%s-%s", title, uuid.NewString()),
		Title:      title,
		Authors:    []string{"Merge Author"},
		ISBN13:     &isbn,
		CoverURL:   &cover,
	}
	ub, err := testApp.Services.Books.AddToLibrary(
		context.Background(), mergeTestUser, ext, status, tags,
	)
	require.NoError(t, err)
	require.NotNil(t, ub)
	return ub
}

func insertBookFile(
	t *testing.T,
	userID string,
	bookID uuid.UUID,
	format, storageKey, checksum string,
) {
	t.Helper()
	_, err := testDB.Exec(context.Background(), `
		INSERT INTO backlog.book_files
		    (book_id, user_id, format, storage_key, size_bytes, checksum,
		     original_filename, status)
		VALUES ($1, $2, $3, $4, 100, $5, 'test.epub', 'ready')
	`, bookID, userID, format, storageKey, checksum)
	require.NoError(t, err)
}

func insertReadingState(
	t *testing.T,
	userID string,
	bookID uuid.UUID,
	percent int,
) {
	t.Helper()
	_, err := testDB.Exec(context.Background(), `
		INSERT INTO backlog.book_reading_state (user_id, book_id, source, percent)
		VALUES ($1, $2, 'web', $3)
		ON CONFLICT (user_id, book_id) DO UPDATE SET percent = EXCLUDED.percent
	`, userID, bookID, percent)
	require.NoError(t, err)
}

// --- service-level tests ---

func TestMergeBooks_UnionsTagsAndFinishedAt(t *testing.T) {
	cleanupMergeUser(t)

	isbn1 := "9780001111111"
	isbn2 := "9780001111112"
	finA := time.Date(2023, 3, 1, 0, 0, 0, 0, time.UTC)
	finB := time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC)

	winner := addMergeBook(t, "BookA", isbn1, models.StatusRead,
		[]string{"own-physical", "sci-fi"})
	_, err := testDB.Exec(context.Background(),
		`UPDATE backlog.user_books SET finished_at = $1
		 WHERE user_id = $2 AND book_id = $3`,
		[]time.Time{finA}, mergeTestUser, winner.BookID)
	require.NoError(t, err)

	loser := addMergeBook(t, "BookB", isbn2, models.StatusToRead,
		[]string{"own-digital", "fantasy"})
	_, err = testDB.Exec(context.Background(),
		`UPDATE backlog.user_books SET finished_at = $1
		 WHERE user_id = $2 AND book_id = $3`,
		[]time.Time{finB}, mergeTestUser, loser.BookID)
	require.NoError(t, err)

	_, err = testApp.Services.Books.MergeBooks(
		context.Background(), mergeTestUser, winner.BookID, []uuid.UUID{loser.BookID},
	)
	require.NoError(t, err)

	// Loser row should be gone.
	var loserCount int
	err = testDB.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM backlog.user_books WHERE user_id = $1 AND book_id = $2`,
		mergeTestUser, loser.BookID,
	).Scan(&loserCount)
	require.NoError(t, err)
	assert.Equal(t, 0, loserCount, "loser user_book must be deleted")

	// Winner row should have union of tags.
	var winnerTags []string
	err = testDB.QueryRow(context.Background(),
		`SELECT tags FROM backlog.user_books WHERE user_id = $1 AND book_id = $2`,
		mergeTestUser, winner.BookID,
	).Scan(&winnerTags)
	require.NoError(t, err)
	assert.ElementsMatch(
		t,
		[]string{"own-physical", "sci-fi", "own-digital", "fantasy"},
		winnerTags,
	)

	// Winner row should have both finished_at timestamps.
	var finishedAt []time.Time
	err = testDB.QueryRow(
		context.Background(),
		`SELECT finished_at FROM backlog.user_books WHERE user_id = $1 AND book_id = $2`,
		mergeTestUser,
		winner.BookID,
	).Scan(&finishedAt)
	require.NoError(t, err)
	assert.Len(t, finishedAt, 2, "both finished_at timestamps must be kept")
}

func TestMergeBooks_PicksMostProgressedStatus(t *testing.T) {
	cleanupMergeUser(t)

	isbn1 := "9780002222221"
	isbn2 := "9780002222222"
	winner := addMergeBook(
		t,
		"StatusA",
		isbn1,
		models.StatusToRead,
		[]string{},
	)
	loser := addMergeBook(
		t,
		"StatusB",
		isbn2,
		models.StatusReading,
		[]string{},
	)

	_, err := testApp.Services.Books.MergeBooks(
		context.Background(), mergeTestUser, winner.BookID, []uuid.UUID{loser.BookID},
	)
	require.NoError(t, err)

	var status string
	err = testDB.QueryRow(context.Background(),
		`SELECT status FROM backlog.user_books WHERE user_id = $1 AND book_id = $2`,
		mergeTestUser, winner.BookID,
	).Scan(&status)
	require.NoError(t, err)
	assert.Equal(t, models.StatusReading, status,
		"winner should take the more-progressed loser status")
}

func TestMergeBooks_RepointsBookFiles(t *testing.T) {
	cleanupMergeUser(t)

	isbn1 := "9780003333331"
	isbn2 := "9780003333332"
	winner := addMergeBook(
		t,
		"FileA",
		isbn1,
		models.StatusToRead,
		[]string{},
	)
	loser := addMergeBook(
		t,
		"FileB",
		isbn2,
		models.StatusToRead,
		[]string{},
	)

	insertBookFile(
		t,
		mergeTestUser,
		loser.BookID,
		"epub",
		"users/merge/loser.epub",
		"abc123",
	)

	_, err := testApp.Services.Books.MergeBooks(
		context.Background(), mergeTestUser, winner.BookID, []uuid.UUID{loser.BookID},
	)
	require.NoError(t, err)

	// The file should now point to winner.
	var fileBookID uuid.UUID
	err = testDB.QueryRow(context.Background(),
		`SELECT book_id FROM backlog.book_files
		 WHERE user_id = $1 AND storage_key = $2`,
		mergeTestUser, "users/merge/loser.epub",
	).Scan(&fileBookID)
	require.NoError(t, err)
	assert.Equal(t, winner.BookID, fileBookID, "file must be repointed to winner")
}

func TestMergeBooks_DeduplicatesIdenticalFiles(t *testing.T) {
	cleanupMergeUser(t)

	isbn1 := "9780004444441"
	isbn2 := "9780004444442"
	winner := addMergeBook(
		t,
		"DedupA",
		isbn1,
		models.StatusToRead,
		[]string{},
	)
	loser := addMergeBook(
		t,
		"DedupB",
		isbn2,
		models.StatusToRead,
		[]string{},
	)

	// Same format + checksum on both — the loser file is a duplicate.
	insertBookFile(
		t,
		mergeTestUser,
		winner.BookID,
		"epub",
		"users/merge/winner.epub",
		"dupchk",
	)
	insertBookFile(
		t,
		mergeTestUser,
		loser.BookID,
		"epub",
		"users/merge/loser-dup.epub",
		"dupchk",
	)

	deletedFiles, err := testApp.Services.Books.MergeBooks(
		context.Background(), mergeTestUser, winner.BookID, []uuid.UUID{loser.BookID},
	)
	require.NoError(t, err)
	assert.Equal(t, uint32(1), deletedFiles, "duplicate file row must be deleted")

	// Winner should still have exactly 1 epub file.
	var fileCount int
	err = testDB.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM backlog.book_files
		 WHERE user_id = $1 AND book_id = $2 AND format = 'epub'`,
		mergeTestUser, winner.BookID,
	).Scan(&fileCount)
	require.NoError(t, err)
	assert.Equal(t, 1, fileCount)
}

func TestMergeBooks_ConsolidatesReadingState(t *testing.T) {
	cleanupMergeUser(t)

	isbn1 := "9780005555551"
	isbn2 := "9780005555552"
	winner := addMergeBook(
		t,
		"StateA",
		isbn1,
		models.StatusReading,
		[]string{},
	)
	loser := addMergeBook(
		t,
		"StateB",
		isbn2,
		models.StatusReading,
		[]string{},
	)

	// Only loser has reading state.
	insertReadingState(t, mergeTestUser, loser.BookID, 42)

	_, err := testApp.Services.Books.MergeBooks(
		context.Background(), mergeTestUser, winner.BookID, []uuid.UUID{loser.BookID},
	)
	require.NoError(t, err)

	// Winner should now have reading state from loser.
	var percent int
	err = testDB.QueryRow(context.Background(),
		`SELECT percent FROM backlog.book_reading_state
		 WHERE user_id = $1 AND book_id = $2`,
		mergeTestUser, winner.BookID,
	).Scan(&percent)
	require.NoError(t, err)
	assert.Equal(t, 42, percent)

	// Loser reading state must be gone.
	var loserStateCount int
	err = testDB.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM backlog.book_reading_state
		 WHERE user_id = $1 AND book_id = $2`,
		mergeTestUser, loser.BookID,
	).Scan(&loserStateCount)
	require.NoError(t, err)
	assert.Equal(t, 0, loserStateCount)
}

func TestMergeBooks_NoLosers_IsNoop(t *testing.T) {
	cleanupMergeUser(t)

	isbn1 := "9780006666661"
	winner := addMergeBook(
		t,
		"NoopBook",
		isbn1,
		models.StatusToRead,
		[]string{},
	)

	deleted, err := testApp.Services.Books.MergeBooks(
		context.Background(), mergeTestUser, winner.BookID, nil,
	)
	require.NoError(t, err)
	assert.Equal(t, uint32(0), deleted)
}

func TestMergeBooks_FallsBackToLoserRating(t *testing.T) {
	cleanupMergeUser(t)

	isbn1 := "9780007777771"
	isbn2 := "9780007777772"
	winner := addMergeBook(
		t,
		"RatingA",
		isbn1,
		models.StatusToRead,
		[]string{},
	)
	loser := addMergeBook(
		t,
		"RatingB",
		isbn2,
		models.StatusToRead,
		[]string{},
	)

	// Give the loser a rating; winner has none.
	rating := int16(4)
	_, err := testDB.Exec(context.Background(),
		`UPDATE backlog.user_books SET rating = $1
		 WHERE user_id = $2 AND book_id = $3`,
		rating, mergeTestUser, loser.BookID)
	require.NoError(t, err)

	_, err = testApp.Services.Books.MergeBooks(
		context.Background(), mergeTestUser, winner.BookID, []uuid.UUID{loser.BookID},
	)
	require.NoError(t, err)

	var gotRating *int16
	err = testDB.QueryRow(
		context.Background(),
		`SELECT rating FROM backlog.user_books WHERE user_id = $1 AND book_id = $2`,
		mergeTestUser,
		winner.BookID,
	).Scan(&gotRating)
	require.NoError(t, err)
	require.NotNil(t, gotRating)
	assert.Equal(t, rating, *gotRating, "winner should inherit loser rating")
}

func TestMergeBooks_WinnerReadingStateNotOverridden(t *testing.T) {
	cleanupMergeUser(t)

	isbn1 := "9780008888881"
	isbn2 := "9780008888882"
	winner := addMergeBook(
		t,
		"StateWinA",
		isbn1,
		models.StatusReading,
		[]string{},
	)
	loser := addMergeBook(
		t,
		"StateWinB",
		isbn2,
		models.StatusReading,
		[]string{},
	)

	// Both have reading state; winner's should be preserved.
	insertReadingState(t, mergeTestUser, winner.BookID, 75)
	insertReadingState(t, mergeTestUser, loser.BookID, 90)

	_, err := testApp.Services.Books.MergeBooks(
		context.Background(), mergeTestUser, winner.BookID, []uuid.UUID{loser.BookID},
	)
	require.NoError(t, err)

	var percent int
	err = testDB.QueryRow(context.Background(),
		`SELECT percent FROM backlog.book_reading_state
		 WHERE user_id = $1 AND book_id = $2`,
		mergeTestUser, winner.BookID,
	).Scan(&percent)
	require.NoError(t, err)
	assert.Equal(t, 75, percent, "winner reading state must not be overridden by loser")
}

// --- connect handler tests ---

func TestConnectFindDuplicates_OK(t *testing.T) {
	client := newBooksTestClient(t)
	req := connect.NewRequest(&backlogv1.FindDuplicatesRequest{})
	req.Header().Set("Cookie", accessToken.String())

	resp, err := client.FindDuplicates(context.Background(), req)
	require.NoError(t, err)
	// Response is non-nil; groups may be empty for a fresh test user.
	assert.NotNil(t, resp.Msg)
}

func TestConnectMergeBooks_InvalidWinnerID(t *testing.T) {
	client := newBooksTestClient(t)
	req := connect.NewRequest(&backlogv1.MergeBooksRequest{
		WinnerBookId: "not-a-uuid",
		LoserBookIds: []string{},
	})
	req.Header().Set("Cookie", accessToken.String())

	_, err := client.MergeBooks(context.Background(), req)
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	assert.Equal(t, connect.CodeInvalidArgument, connErr.Code())
}

func TestConnectMergeBooks_InvalidLoserID(t *testing.T) {
	client := newBooksTestClient(t)
	req := connect.NewRequest(&backlogv1.MergeBooksRequest{
		WinnerBookId: uuid.NewString(),
		LoserBookIds: []string{"not-a-uuid"},
	})
	req.Header().Set("Cookie", accessToken.String())

	_, err := client.MergeBooks(context.Background(), req)
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	assert.Equal(t, connect.CodeInvalidArgument, connErr.Code())
}
