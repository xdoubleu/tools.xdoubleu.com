package books_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/books/internal/models"
	"tools.xdoubleu.com/apps/books/internal/services"
	"tools.xdoubleu.com/apps/books/pkg/objectstore"
	booksv1 "tools.xdoubleu.com/gen/books/v1"
)

const mergeTestUser = "merge-books-test-user"

func cleanupMergeUser(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		ctx := context.Background()
		_, _ = testDB.Exec(ctx,
			`DELETE FROM books.user_books WHERE user_id = $1`, mergeTestUser)
		_, _ = testDB.Exec(ctx,
			`DELETE FROM books.book_files WHERE user_id = $1`, mergeTestUser)
		_, _ = testDB.Exec(ctx,
			`DELETE FROM books.book_reading_state WHERE user_id = $1`, mergeTestUser)
	})
}

func addMergeBook(
	t *testing.T,
	title, isbn string,
	status string,
	tags []string,
) *models.UserBook {
	t.Helper()
	ext := services.SourceProposal{ //nolint:exhaustruct //only required fields
		Source:   "manual",
		Title:    title,
		Authors:  []string{"Merge Author"},
		ISBN13:   isbn,
		CoverURL: "https://example.com/cover.jpg",
	}
	ub, err := testApp.Services.Books.AddToLibrary(
		context.Background(), mergeTestUser, ext, status, tags,
	)
	require.NoError(t, err)
	require.NotNil(t, ub)
	return ub
}

// insertBookFile seeds an "epub" book_files row for mergeTestUser.
func insertBookFile(
	t *testing.T,
	bookID uuid.UUID,
	storageKey, checksum string,
) {
	t.Helper()
	_, err := testDB.Exec(context.Background(), `
		INSERT INTO books.book_files
		    (book_id, user_id, format, storage_key, size_bytes, checksum,
		     original_filename, status)
		VALUES ($1, $2, 'epub', $3, 100, $4, 'test.epub', 'ready')
	`, bookID, mergeTestUser, storageKey, checksum)
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
		INSERT INTO books.book_reading_state (user_id, book_id, source, percent)
		VALUES ($1, $2, 'web', $3)
		ON CONFLICT (user_id, book_id) DO UPDATE SET percent = EXCLUDED.percent
	`, userID, bookID, percent)
	require.NoError(t, err)
}

func TestMergeBooks_UnionsTagsAndFinishedAt(t *testing.T) {
	cleanupMergeUser(t)

	isbn1 := "9780001111111"
	isbn2 := "9780001111112"
	finA := time.Date(2023, 3, 1, 0, 0, 0, 0, time.UTC)
	finB := time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC)

	winner := addMergeBook(t, "BookA", isbn1, models.StatusRead,
		[]string{"own-physical", "sci-fi"})
	_, err := testDB.Exec(context.Background(),
		`UPDATE books.user_books SET finished_at = $1
		 WHERE user_id = $2 AND book_id = $3`,
		[]time.Time{finA}, mergeTestUser, winner.BookID)
	require.NoError(t, err)

	loser := addMergeBook(t, "BookB", isbn2, models.StatusToRead,
		[]string{"own-digital", "fantasy"})
	_, err = testDB.Exec(context.Background(),
		`UPDATE books.user_books SET finished_at = $1
		 WHERE user_id = $2 AND book_id = $3`,
		[]time.Time{finB}, mergeTestUser, loser.BookID)
	require.NoError(t, err)

	_, _, err = testApp.Services.Books.MergeBooks(
		context.Background(), mergeTestUser, winner.BookID, []uuid.UUID{loser.BookID},
		nil, nil, nil,
	)
	require.NoError(t, err)

	var loserCount int
	err = testDB.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM books.user_books WHERE user_id = $1 AND book_id = $2`,
		mergeTestUser, loser.BookID,
	).Scan(&loserCount)
	require.NoError(t, err)
	assert.Equal(t, 0, loserCount, "loser user_book must be deleted")

	var winnerTags []string
	err = testDB.QueryRow(context.Background(),
		`SELECT tags FROM books.user_books WHERE user_id = $1 AND book_id = $2`,
		mergeTestUser, winner.BookID,
	).Scan(&winnerTags)
	require.NoError(t, err)
	assert.ElementsMatch(
		t,
		[]string{"own-physical", "sci-fi", "own-digital", "fantasy"},
		winnerTags,
	)

	var finishedAt []time.Time
	err = testDB.QueryRow(
		context.Background(),
		`SELECT finished_at FROM books.user_books WHERE user_id = $1 AND book_id = $2`,
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

	_, _, err := testApp.Services.Books.MergeBooks(
		context.Background(), mergeTestUser, winner.BookID, []uuid.UUID{loser.BookID},
		nil, nil, nil,
	)
	require.NoError(t, err)

	var status string
	err = testDB.QueryRow(context.Background(),
		`SELECT status FROM books.user_books WHERE user_id = $1 AND book_id = $2`,
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
		loser.BookID,
		"users/merge/loser.epub",
		"abc123",
	)

	_, _, err := testApp.Services.Books.MergeBooks(
		context.Background(), mergeTestUser, winner.BookID, []uuid.UUID{loser.BookID},
		nil, nil, nil,
	)
	require.NoError(t, err)

	var fileBookID uuid.UUID
	err = testDB.QueryRow(context.Background(),
		`SELECT book_id FROM books.book_files
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

	// Same format + checksum: the loser file is a duplicate.
	insertBookFile(
		t,
		winner.BookID,
		"users/merge/winner.epub",
		"dupchk",
	)
	insertBookFile(
		t,
		loser.BookID,
		"users/merge/loser-dup.epub",
		"dupchk",
	)

	deletedFiles, _, err := testApp.Services.Books.MergeBooks(
		context.Background(), mergeTestUser, winner.BookID, []uuid.UUID{loser.BookID},
		nil, nil, nil,
	)
	require.NoError(t, err)
	assert.Equal(t, uint32(1), deletedFiles, "duplicate file row must be deleted")

	var fileCount int
	err = testDB.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM books.book_files
		 WHERE user_id = $1 AND book_id = $2 AND format = 'epub'`,
		mergeTestUser, winner.BookID,
	).Scan(&fileCount)
	require.NoError(t, err)
	assert.Equal(t, 1, fileCount)
}

// TestMergeBooks_R2DeleteFailsAfterRetries_LogsErrorButSucceeds: a failing R2
// delete must not fail the merge; the storage scan catches the object.
func TestMergeBooks_R2DeleteFailsAfterRetries_LogsErrorButSucceeds(t *testing.T) {
	cleanupMergeUser(t)
	objectstore.SetBackoffBase(time.Millisecond)
	t.Cleanup(func() { objectstore.SetBackoffBase(500 * time.Millisecond) })

	isbn1 := "9780005555551"
	isbn2 := "9780005555552"
	winner := addMergeBook(
		t,
		"MergeDeleteFailA",
		isbn1,
		models.StatusToRead,
		[]string{},
	)
	loser := addMergeBook(t, "MergeDeleteFailB", isbn2, models.StatusToRead, []string{})

	const loserKey = "users/merge/delete-fails-loser.epub"
	fakeStore.PutAt(loserKey, []byte("loser content"), time.Now())
	insertBookFile(t, loser.BookID, loserKey, "delfailchk")

	fakeStore.FailNextDeletes(4, errors.New("transient R2 failure"))

	_, _, err := testApp.Services.Books.MergeBooks(
		context.Background(), mergeTestUser, winner.BookID, []uuid.UUID{loser.BookID},
		nil, nil, nil,
	)
	require.NoError(t, err, "a leaked R2 object must not fail the merge")

	exists, err := fakeStore.Exists(context.Background(), loserKey)
	require.NoError(t, err)
	assert.True(t, exists, "object survives once every retry attempt fails")
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

	insertReadingState(t, mergeTestUser, loser.BookID, 42)

	_, _, err := testApp.Services.Books.MergeBooks(
		context.Background(), mergeTestUser, winner.BookID, []uuid.UUID{loser.BookID},
		nil, nil, nil,
	)
	require.NoError(t, err)

	var percent int
	err = testDB.QueryRow(context.Background(),
		`SELECT percent FROM books.book_reading_state
		 WHERE user_id = $1 AND book_id = $2`,
		mergeTestUser, winner.BookID,
	).Scan(&percent)
	require.NoError(t, err)
	assert.Equal(t, 42, percent)

	var loserStateCount int
	err = testDB.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM books.book_reading_state
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

	deleted, _, err := testApp.Services.Books.MergeBooks(
		context.Background(), mergeTestUser, winner.BookID, nil, nil, nil, nil,
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

	rating := int16(4)
	_, err := testDB.Exec(context.Background(),
		`UPDATE books.user_books SET rating = $1
		 WHERE user_id = $2 AND book_id = $3`,
		rating, mergeTestUser, loser.BookID)
	require.NoError(t, err)

	_, _, err = testApp.Services.Books.MergeBooks(
		context.Background(), mergeTestUser, winner.BookID, []uuid.UUID{loser.BookID},
		nil, nil, nil,
	)
	require.NoError(t, err)

	var gotRating *int16
	err = testDB.QueryRow(
		context.Background(),
		`SELECT rating FROM books.user_books WHERE user_id = $1 AND book_id = $2`,
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

	insertReadingState(t, mergeTestUser, winner.BookID, 75)
	insertReadingState(t, mergeTestUser, loser.BookID, 90)

	_, _, err := testApp.Services.Books.MergeBooks(
		context.Background(), mergeTestUser, winner.BookID, []uuid.UUID{loser.BookID},
		nil, nil, nil,
	)
	require.NoError(t, err)

	var percent int
	err = testDB.QueryRow(context.Background(),
		`SELECT percent FROM books.book_reading_state
		 WHERE user_id = $1 AND book_id = $2`,
		mergeTestUser, winner.BookID,
	).Scan(&percent)
	require.NoError(t, err)
	assert.Equal(t, 75, percent, "winner reading state must not be overridden by loser")
}

func TestMergeBooks_AppliesResolvedMetadata(t *testing.T) {
	cleanupMergeUser(t)

	isbn1 := "9780011111111"
	isbn2 := "9780011111112"
	winner := addMergeBook(t, "MetaWinnerA", isbn1, models.StatusToRead, []string{})
	loser := addMergeBook(t, "MetaLoserB", isbn2, models.StatusToRead, []string{})

	loserDesc := "A much better description from loser"
	resolvedTitle := "Resolved Final Title"
	//nolint:exhaustruct // catalog fields only; ID is set by the service
	resolved := &models.Book{
		Title:       resolvedTitle,
		Authors:     []string{"Resolved Author"},
		Description: &loserDesc,
	}

	_, _, err := testApp.Services.Books.MergeBooks(
		context.Background(), mergeTestUser, winner.BookID, []uuid.UUID{loser.BookID},
		resolved, nil, nil,
	)
	require.NoError(t, err)

	var gotTitle, gotDesc string
	err = testDB.QueryRow(context.Background(),
		`SELECT title, COALESCE(description, '') FROM books.books WHERE id = $1`,
		winner.BookID,
	).Scan(&gotTitle, &gotDesc)
	require.NoError(t, err)
	assert.Equal(t, resolvedTitle, gotTitle, "winner book title must be overwritten")
	assert.Equal(t, loserDesc, gotDesc, "winner book description must be overwritten")
}

func TestMergeBooks_NilResolvedMetadataPreservesBook(t *testing.T) {
	cleanupMergeUser(t)

	isbn1 := "9780012121211"
	isbn2 := "9780012121212"
	winner := addMergeBook(t, "PreservedTitle", isbn1, models.StatusToRead, []string{})
	loser := addMergeBook(t, "LoserTitle", isbn2, models.StatusToRead, []string{})

	var originalTitle string
	err := testDB.QueryRow(context.Background(),
		`SELECT title FROM books.books WHERE id = $1`, winner.BookID,
	).Scan(&originalTitle)
	require.NoError(t, err)

	_, _, err = testApp.Services.Books.MergeBooks(
		context.Background(), mergeTestUser, winner.BookID, []uuid.UUID{loser.BookID},
		nil, nil, nil,
	)
	require.NoError(t, err)

	var gotTitle string
	err = testDB.QueryRow(context.Background(),
		`SELECT title FROM books.books WHERE id = $1`, winner.BookID,
	).Scan(&gotTitle)
	require.NoError(t, err)
	assert.Equal(
		t,
		originalTitle,
		gotTitle,
		"winner book must not change when no resolved metadata",
	)
}

func TestMergeBooks_OrphanedLoserBookDeleted(t *testing.T) {
	cleanupMergeUser(t)

	isbn1 := "9780013131311"
	isbn2 := "9780013131312"
	winner := addMergeBook(t, "OrphanWinner", isbn1, models.StatusToRead, []string{})
	loser := addMergeBook(t, "OrphanLoser", isbn2, models.StatusToRead, []string{})
	loserBookID := loser.BookID

	_, _, err := testApp.Services.Books.MergeBooks(
		context.Background(), mergeTestUser, winner.BookID, []uuid.UUID{loserBookID},
		nil, nil, nil,
	)
	require.NoError(t, err)

	var count int
	err = testDB.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM books.books WHERE id = $1`, loserBookID,
	).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 0, count, "orphaned loser catalog book must be deleted")
}

func TestConnectMergeBooks_InvalidCoverSourceID(t *testing.T) {
	client := newAdminBooksTestClient(t)
	bad := "not-a-uuid"
	req := connect.NewRequest(&booksv1.MergeBooksRequest{
		WinnerBookId:              uuid.NewString(),
		LoserBookIds:              []string{},
		ResolvedCoverSourceBookId: &bad,
	})
	req.Header().Set("Cookie", accessToken.String())

	_, err := client.MergeBooks(context.Background(), req)
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	assert.Equal(t, connect.CodeInvalidArgument, connErr.Code())
}

func TestConnectFindDuplicates_OK(t *testing.T) {
	client := newAdminBooksTestClient(t)
	req := connect.NewRequest(&booksv1.FindDuplicatesRequest{})
	req.Header().Set("Cookie", accessToken.String())

	resp, err := client.FindDuplicates(context.Background(), req)
	require.NoError(t, err)
	assert.NotNil(t, resp.Msg)
}

func TestConnectMergeBooks_InvalidWinnerID(t *testing.T) {
	client := newAdminBooksTestClient(t)
	req := connect.NewRequest(&booksv1.MergeBooksRequest{
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
	client := newAdminBooksTestClient(t)
	req := connect.NewRequest(&booksv1.MergeBooksRequest{
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

func TestMergeBooks_CustomShelfBeatsBuiltInStatus(t *testing.T) {
	cleanupMergeUser(t)

	isbn1 := "9780020202021"
	isbn2 := "9780020202022"
	winner := addMergeBook(t, "ShelfWinA", isbn1, "sci-fi", []string{})
	loser := addMergeBook(t, "ShelfWinB", isbn2, models.StatusRead, []string{})

	_, _, err := testApp.Services.Books.MergeBooks(
		context.Background(), mergeTestUser, winner.BookID, []uuid.UUID{loser.BookID},
		nil, nil, nil,
	)
	require.NoError(t, err)

	var status string
	err = testDB.QueryRow(context.Background(),
		`SELECT status FROM books.user_books WHERE user_id = $1 AND book_id = $2`,
		mergeTestUser, winner.BookID,
	).Scan(&status)
	require.NoError(t, err)
	assert.Equal(t, "sci-fi", status,
		"custom shelf must win over built-in read status")
}

func TestMergeBooks_CustomShelfBeatsBuiltInStatus_LoserOnShelf(t *testing.T) {
	cleanupMergeUser(t)

	isbn1 := "9780020202031"
	isbn2 := "9780020202032"
	winner := addMergeBook(t, "ShelfLoserA", isbn1, models.StatusRead, []string{})
	loser := addMergeBook(t, "ShelfLoserB", isbn2, "favourites", []string{})

	_, _, err := testApp.Services.Books.MergeBooks(
		context.Background(), mergeTestUser, winner.BookID, []uuid.UUID{loser.BookID},
		nil, nil, nil,
	)
	require.NoError(t, err)

	var status string
	err = testDB.QueryRow(context.Background(),
		`SELECT status FROM books.user_books WHERE user_id = $1 AND book_id = $2`,
		mergeTestUser, winner.BookID,
	).Scan(&status)
	require.NoError(t, err)
	assert.Equal(t, "favourites", status,
		"loser's custom shelf must win over winner's built-in read status")
}

func TestMergeBooks_WinnerShelfKeptWhenBothOnCustomShelves(t *testing.T) {
	cleanupMergeUser(t)

	isbn1 := "9780020202041"
	isbn2 := "9780020202042"
	winner := addMergeBook(t, "TwoShelvesA", isbn1, "sci-fi", []string{})
	loser := addMergeBook(t, "TwoShelvesB", isbn2, "fantasy", []string{})

	_, _, err := testApp.Services.Books.MergeBooks(
		context.Background(), mergeTestUser, winner.BookID, []uuid.UUID{loser.BookID},
		nil, nil, nil,
	)
	require.NoError(t, err)

	var status string
	err = testDB.QueryRow(context.Background(),
		`SELECT status FROM books.user_books WHERE user_id = $1 AND book_id = $2`,
		mergeTestUser, winner.BookID,
	).Scan(&status)
	require.NoError(t, err)
	assert.Equal(t, "sci-fi", status,
		"when both entries are on custom shelves the winner's shelf must be kept")
}

func TestMergeBooks_ResolvedStatusOverridesAutoConsolidation(t *testing.T) {
	cleanupMergeUser(t)

	isbn1 := "9780020202051"
	isbn2 := "9780020202052"
	winner := addMergeBook(t, "ResolvedStatusA", isbn1, models.StatusToRead, []string{})
	loser := addMergeBook(t, "ResolvedStatusB", isbn2, models.StatusToRead, []string{})

	forced := "my-custom-shelf"
	_, _, err := testApp.Services.Books.MergeBooks(
		context.Background(), mergeTestUser, winner.BookID, []uuid.UUID{loser.BookID},
		nil, nil, &forced,
	)
	require.NoError(t, err)

	var status string
	err = testDB.QueryRow(context.Background(),
		`SELECT status FROM books.user_books WHERE user_id = $1 AND book_id = $2`,
		mergeTestUser, winner.BookID,
	).Scan(&status)
	require.NoError(t, err)
	assert.Equal(t, forced, status,
		"resolved_status must override auto-consolidated status")
}

func TestConnectMergeBooks_ResolvedStatusApplied(t *testing.T) {
	// Books created via the service are owned by the mocked auth identity.
	isbn1 := "9780020202061"
	isbn2 := "9780020202062"

	createBook := func(title string, isbn string) *models.UserBook {
		t.Helper()
		ext := services.SourceProposal{ //nolint:exhaustruct //only required fields
			Source:   "manual",
			Title:    title,
			Authors:  []string{"Shelf Author"},
			ISBN13:   isbn,
			CoverURL: "https://example.com/cover.jpg",
		}
		ub, addErr := testApp.Services.Books.AddToLibrary(
			context.Background(), userID, ext, models.StatusToRead, []string{},
		)
		require.NoError(t, addErr)
		return ub
	}
	t.Cleanup(func() {
		_, _ = testDB.Exec(
			context.Background(),
			`DELETE FROM books.user_books WHERE user_id = $1 AND status = 'connect-shelf'`,
			userID,
		)
	})

	winner := createBook("ConnectShelfA", isbn1)
	loser := createBook("ConnectShelfB", isbn2)

	client := newAdminBooksTestClient(t)
	forced := "connect-shelf"
	req := connect.NewRequest(&booksv1.MergeBooksRequest{
		WinnerBookId:   winner.BookID.String(),
		LoserBookIds:   []string{loser.BookID.String()},
		ResolvedStatus: &forced,
	})
	req.Header().Set("Cookie", accessToken.String())

	_, err := client.MergeBooks(context.Background(), req)
	require.NoError(t, err)

	var status string
	err = testDB.QueryRow(context.Background(),
		`SELECT status FROM books.user_books WHERE user_id = $1 AND book_id = $2`,
		userID, winner.BookID,
	).Scan(&status)
	require.NoError(t, err)
	assert.Equal(t, forced, status,
		"connect handler must forward resolved_status to the service")
}

const mergeTestUser2 = "merge-books-test-user-2"

func cleanupMergeUser2(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		ctx := context.Background()
		_, _ = testDB.Exec(ctx,
			`DELETE FROM books.user_books WHERE user_id = $1`, mergeTestUser2)
		_, _ = testDB.Exec(ctx,
			`DELETE FROM books.book_files WHERE user_id = $1`, mergeTestUser2)
		_, _ = testDB.Exec(ctx,
			`DELETE FROM books.book_reading_state WHERE user_id = $1`, mergeTestUser2)
	})
}

func addMergeBookForUser2(
	t *testing.T,
	title, isbn, status string,
	tags []string,
) *models.UserBook {
	t.Helper()
	ext := services.SourceProposal{ //nolint:exhaustruct //only required fields
		Source:   "manual",
		Title:    title,
		Authors:  []string{"Merge Author"},
		ISBN13:   isbn,
		CoverURL: "https://example.com/cover.jpg",
	}
	ub, err := testApp.Services.Books.AddToLibrary(
		context.Background(), mergeTestUser2, ext, status, tags,
	)
	require.NoError(t, err)
	require.NotNil(t, ub)
	return ub
}

// TestMergeBooks_UnownedLoser_GlobalCatalogDeleted: the admin owns the winner
// but the loser is only in another user's library.
func TestMergeBooks_UnownedLoser_GlobalCatalogDeleted(t *testing.T) {
	cleanupMergeUser(t)
	cleanupMergeUser2(t)

	isbn1 := "9780030303031"
	isbn2 := "9780030303032"

	winner := addMergeBook(t, "UnownedWinner", isbn1, models.StatusToRead, []string{})
	loser := addMergeBookForUser2(
		t, "UnownedLoser", isbn2, models.StatusToRead, []string{},
	)
	loserBookID := loser.BookID

	_, _, err := testApp.Services.Books.MergeBooks(
		context.Background(), mergeTestUser, winner.BookID, []uuid.UUID{loserBookID},
		nil, nil, nil,
	)
	require.NoError(t, err)

	var catCount int
	err = testDB.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM books.books WHERE id = $1`, loserBookID,
	).Scan(&catCount)
	require.NoError(t, err)
	assert.Equal(t, 0, catCount, "orphaned loser catalog book must be deleted globally")

	var winnerCount int
	err = testDB.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM books.user_books WHERE user_id = $1 AND book_id = $2`,
		mergeTestUser2, winner.BookID,
	).Scan(&winnerCount)
	require.NoError(t, err)
	assert.Equal(t, 1, winnerCount,
		"other user's loser entry must be repointed to winner")
}

// TestMergeBooks_CrossUserConsolidation: two users each own winner and loser.
func TestMergeBooks_CrossUserConsolidation(t *testing.T) {
	cleanupMergeUser(t)
	cleanupMergeUser2(t)

	isbn1 := "9780030303041"
	isbn2 := "9780030303042"

	winner := addMergeBook(
		t, "CrossWinA", isbn1, models.StatusToRead, []string{"admin-tag"},
	)
	loser := addMergeBook(
		t, "CrossLosA", isbn2, models.StatusRead, []string{"admin-loser-tag"},
	)

	addMergeBookForUser2(
		t,
		"CrossWinB",
		isbn1,
		models.StatusToRead,
		[]string{"user2-tag"},
	)
	addMergeBookForUser2(
		t,
		"CrossLosB",
		isbn2,
		models.StatusRead,
		[]string{"user2-loser-tag"},
	)

	_, _, err := testApp.Services.Books.MergeBooks(
		context.Background(), mergeTestUser, winner.BookID, []uuid.UUID{loser.BookID},
		nil, nil, nil,
	)
	require.NoError(t, err)

	for _, uid := range []string{mergeTestUser, mergeTestUser2} {
		var n int
		err = testDB.QueryRow(
			context.Background(),
			`SELECT COUNT(*) FROM books.user_books WHERE user_id = $1 AND book_id = $2`,
			uid,
			loser.BookID,
		).Scan(&n)
		require.NoError(t, err)
		assert.Equal(t, 0, n, "loser user_book must be deleted for user %s", uid)
	}

	var catCount int
	err = testDB.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM books.books WHERE id = $1`, loser.BookID,
	).Scan(&catCount)
	require.NoError(t, err)
	assert.Equal(t, 0, catCount, "orphaned loser catalog book must be deleted")

	var winnerTags []string
	err = testDB.QueryRow(context.Background(),
		`SELECT tags FROM books.user_books WHERE user_id = $1 AND book_id = $2`,
		mergeTestUser, winner.BookID,
	).Scan(&winnerTags)
	require.NoError(t, err)
	assert.ElementsMatch(t,
		[]string{"admin-tag", "admin-loser-tag"},
		winnerTags,
		"admin winner tags must include unioned loser tags",
	)
}

// TestMergeBooks_RepointsOtherUsersLoserEntry: user2 owns only the loser and
// is repointed to the winner with its data.
func TestMergeBooks_RepointsOtherUsersLoserEntry(t *testing.T) {
	cleanupMergeUser(t)
	cleanupMergeUser2(t)

	isbn1 := "9780030303051"
	isbn2 := "9780030303052"

	winner := addMergeBook(t, "RepointWin", isbn1, models.StatusToRead, []string{})
	loser := addMergeBookForUser2(
		t,
		"RepointLos",
		isbn2,
		models.StatusRead,
		[]string{"carried-tag"},
	)

	_, _, err := testApp.Services.Books.MergeBooks(
		context.Background(), mergeTestUser, winner.BookID, []uuid.UUID{loser.BookID},
		nil, nil, nil,
	)
	require.NoError(t, err)

	var loserCount int
	err = testDB.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM books.user_books WHERE user_id = $1 AND book_id = $2`,
		mergeTestUser2, loser.BookID,
	).Scan(&loserCount)
	require.NoError(t, err)
	assert.Equal(t, 0, loserCount, "user2's loser user_book must be deleted")

	var winnerCount int
	err = testDB.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM books.user_books WHERE user_id = $1 AND book_id = $2`,
		mergeTestUser2, winner.BookID,
	).Scan(&winnerCount)
	require.NoError(t, err)
	assert.Equal(t, 1, winnerCount, "user2 must own the winner after repoint")

	var tags []string
	err = testDB.QueryRow(context.Background(),
		`SELECT tags FROM books.user_books WHERE user_id = $1 AND book_id = $2`,
		mergeTestUser2, winner.BookID,
	).Scan(&tags)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"carried-tag"}, tags,
		"loser's tags must be carried to the new winner entry for user2")

	var catCount int
	err = testDB.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM books.books WHERE id = $1`, loser.BookID,
	).Scan(&catCount)
	require.NoError(t, err)
	assert.Equal(t, 0, catCount, "orphaned loser catalog book must be deleted")
}

// TestMergeBooks_CallerOwnsNeither covers the caller having no ownership stake.
func TestMergeBooks_CallerOwnsNeither(t *testing.T) {
	cleanupMergeUser(t)
	cleanupMergeUser2(t)

	isbn1 := "9780030303061"
	isbn2 := "9780030303062"

	winner := addMergeBookForUser2(
		t, "NeitherWin", isbn1, models.StatusToRead, []string{},
	)
	loser := addMergeBookForUser2(
		t, "NeitherLos", isbn2, models.StatusRead, []string{"carried"},
	)
	loserBookID := loser.BookID

	_, _, err := testApp.Services.Books.MergeBooks(
		context.Background(), mergeTestUser, winner.BookID, []uuid.UUID{loserBookID},
		nil, nil, nil,
	)
	require.NoError(t, err)

	var loserCount int
	err = testDB.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM books.user_books WHERE user_id = $1 AND book_id = $2`,
		mergeTestUser2, loserBookID,
	).Scan(&loserCount)
	require.NoError(t, err)
	assert.Equal(t, 0, loserCount, "user2 loser entry must be deleted")

	var winnerCount int
	err = testDB.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM books.user_books WHERE user_id = $1 AND book_id = $2`,
		mergeTestUser2, winner.BookID,
	).Scan(&winnerCount)
	require.NoError(t, err)
	assert.Equal(t, 1, winnerCount, "user2 must own the winner after merge")

	var catCount int
	err = testDB.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM books.books WHERE id = $1`, loserBookID,
	).Scan(&catCount)
	require.NoError(t, err)
	assert.Equal(t, 0, catCount, "orphaned loser catalog book must be deleted")
}
