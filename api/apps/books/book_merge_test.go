package books_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/books/internal/models"
	"tools.xdoubleu.com/apps/books/pkg/openlibrary"
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
		INSERT INTO books.book_files
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
		INSERT INTO books.book_reading_state (user_id, book_id, source, percent)
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

	// Loser row should be gone.
	var loserCount int
	err = testDB.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM books.user_books WHERE user_id = $1 AND book_id = $2`,
		mergeTestUser, loser.BookID,
	).Scan(&loserCount)
	require.NoError(t, err)
	assert.Equal(t, 0, loserCount, "loser user_book must be deleted")

	// Winner row should have union of tags.
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

	// Winner row should have both finished_at timestamps.
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
		mergeTestUser,
		loser.BookID,
		"epub",
		"users/merge/loser.epub",
		"abc123",
	)

	_, _, err := testApp.Services.Books.MergeBooks(
		context.Background(), mergeTestUser, winner.BookID, []uuid.UUID{loser.BookID},
		nil, nil, nil,
	)
	require.NoError(t, err)

	// The file should now point to winner.
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

	deletedFiles, _, err := testApp.Services.Books.MergeBooks(
		context.Background(), mergeTestUser, winner.BookID, []uuid.UUID{loser.BookID},
		nil, nil, nil,
	)
	require.NoError(t, err)
	assert.Equal(t, uint32(1), deletedFiles, "duplicate file row must be deleted")

	// Winner should still have exactly 1 epub file.
	var fileCount int
	err = testDB.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM books.book_files
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

	_, _, err := testApp.Services.Books.MergeBooks(
		context.Background(), mergeTestUser, winner.BookID, []uuid.UUID{loser.BookID},
		nil, nil, nil,
	)
	require.NoError(t, err)

	// Winner should now have reading state from loser.
	var percent int
	err = testDB.QueryRow(context.Background(),
		`SELECT percent FROM books.book_reading_state
		 WHERE user_id = $1 AND book_id = $2`,
		mergeTestUser, winner.BookID,
	).Scan(&percent)
	require.NoError(t, err)
	assert.Equal(t, 42, percent)

	// Loser reading state must be gone.
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

	// Give the loser a rating; winner has none.
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

	// Both have reading state; winner's should be preserved.
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

// --- resolved metadata tests ---

func TestMergeBooks_AppliesResolvedMetadata(t *testing.T) {
	cleanupMergeUser(t)

	isbn1 := "9780011111111"
	isbn2 := "9780011111112"
	winner := addMergeBook(t, "MetaWinnerA", isbn1, models.StatusToRead, []string{})
	loser := addMergeBook(t, "MetaLoserB", isbn2, models.StatusToRead, []string{})

	// Use the loser's title and description as the resolved values.
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

	// Loser catalog book row must be gone (no other user references it).
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

// --- connect handler tests ---

func TestConnectFindDuplicates_OK(t *testing.T) {
	client := newAdminBooksTestClient(t)
	req := connect.NewRequest(&booksv1.FindDuplicatesRequest{})
	req.Header().Set("Cookie", accessToken.String())

	resp, err := client.FindDuplicates(context.Background(), req)
	require.NoError(t, err)
	// Response is non-nil; groups may be empty for a fresh test user.
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

// --- shelf / status merge tests ---

func TestMergeBooks_CustomShelfBeatsBuiltInStatus(t *testing.T) {
	cleanupMergeUser(t)

	isbn1 := "9780020202021"
	isbn2 := "9780020202022"
	// winner is on a custom shelf; loser has a higher built-in reading status.
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
	// winner has a built-in reading status; loser is on a custom shelf.
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
	// Books created via the service are owned by userID (the mocked auth identity).
	isbn1 := "9780020202061"
	isbn2 := "9780020202062"
	cover := "https://example.com/cover.jpg"

	createBook := func(title string, isbn string) *models.UserBook {
		t.Helper()
		ext := openlibrary.ExternalBook{ //nolint:exhaustruct //only required fields
			Provider:   "manual",
			ProviderID: fmt.Sprintf("conn-shelf-%s", uuid.NewString()),
			Title:      title,
			Authors:    []string{"Shelf Author"},
			ISBN13:     &isbn,
			CoverURL:   &cover,
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

// --- global (cross-user) merge tests ---

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
		context.Background(), mergeTestUser2, ext, status, tags,
	)
	require.NoError(t, err)
	require.NotNil(t, ub)
	return ub
}

// TestMergeBooks_UnownedLoser_GlobalCatalogDeleted is the direct regression for
// the "load loser <uuid>: resource not found" 500. The admin owns the winner but
// the loser is only in another user's library — not the admin's.
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

	// Admin merges: loser is not in their library.
	// This used to fail with "load loser <uuid>: resource not found".
	_, _, err := testApp.Services.Books.MergeBooks(
		context.Background(), mergeTestUser, winner.BookID, []uuid.UUID{loserBookID},
		nil, nil, nil,
	)
	require.NoError(t, err)

	// Loser catalog row must be gone (no remaining references from any user).
	var catCount int
	err = testDB.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM books.books WHERE id = $1`, loserBookID,
	).Scan(&catCount)
	require.NoError(t, err)
	assert.Equal(t, 0, catCount, "orphaned loser catalog book must be deleted globally")

	// mergeTestUser2 must now own the winner (entry repointed from loser).
	var winnerCount int
	err = testDB.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM books.user_books WHERE user_id = $1 AND book_id = $2`,
		mergeTestUser2, winner.BookID,
	).Scan(&winnerCount)
	require.NoError(t, err)
	assert.Equal(t, 1, winnerCount,
		"other user's loser entry must be repointed to winner")
}

// TestMergeBooks_CrossUserConsolidation verifies that when two users each own
// both the winner and the loser, all four rows are handled: both loser rows are
// deleted, both winner rows are updated with the union of their loser's data, and
// the loser catalog row is deleted.
func TestMergeBooks_CrossUserConsolidation(t *testing.T) {
	cleanupMergeUser(t)
	cleanupMergeUser2(t)

	isbn1 := "9780030303041"
	isbn2 := "9780030303042"

	// Admin owns both winner and loser.
	winner := addMergeBook(
		t, "CrossWinA", isbn1, models.StatusToRead, []string{"admin-tag"},
	)
	loser := addMergeBook(
		t, "CrossLosA", isbn2, models.StatusRead, []string{"admin-loser-tag"},
	)

	// User2 owns both (same catalog book IDs — same ISBN13).
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

	// Both users' loser user_books rows must be gone.
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

	// Loser catalog row deleted (now orphaned).
	var catCount int
	err = testDB.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM books.books WHERE id = $1`, loser.BookID,
	).Scan(&catCount)
	require.NoError(t, err)
	assert.Equal(t, 0, catCount, "orphaned loser catalog book must be deleted")

	// Admin's winner must have the union of their own loser's tags.
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

// TestMergeBooks_RepointsOtherUsersLoserEntry verifies that when user2 owns only
// the loser (not the winner), after the merge user2's entry is repointed to the
// winner with the loser's data carried over.
func TestMergeBooks_RepointsOtherUsersLoserEntry(t *testing.T) {
	cleanupMergeUser(t)
	cleanupMergeUser2(t)

	isbn1 := "9780030303051"
	isbn2 := "9780030303052"

	// Admin owns only the winner.
	winner := addMergeBook(t, "RepointWin", isbn1, models.StatusToRead, []string{})
	// User2 owns only the loser (not the winner).
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

	// User2's loser row must be gone.
	var loserCount int
	err = testDB.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM books.user_books WHERE user_id = $1 AND book_id = $2`,
		mergeTestUser2, loser.BookID,
	).Scan(&loserCount)
	require.NoError(t, err)
	assert.Equal(t, 0, loserCount, "user2's loser user_book must be deleted")

	// User2 must now own the winner.
	var winnerCount int
	err = testDB.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM books.user_books WHERE user_id = $1 AND book_id = $2`,
		mergeTestUser2, winner.BookID,
	).Scan(&winnerCount)
	require.NoError(t, err)
	assert.Equal(t, 1, winnerCount, "user2 must own the winner after repoint")

	// User2's winner entry must carry the loser's tags.
	var tags []string
	err = testDB.QueryRow(context.Background(),
		`SELECT tags FROM books.user_books WHERE user_id = $1 AND book_id = $2`,
		mergeTestUser2, winner.BookID,
	).Scan(&tags)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"carried-tag"}, tags,
		"loser's tags must be carried to the new winner entry for user2")

	// Loser catalog row must be deleted.
	var catCount int
	err = testDB.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM books.books WHERE id = $1`, loser.BookID,
	).Scan(&catCount)
	require.NoError(t, err)
	assert.Equal(t, 0, catCount, "orphaned loser catalog book must be deleted")
}

// TestMergeBooks_CallerOwnsNeither exercises the !callerIncluded path in
// MergeBooks and the early-return path in consolidateUserBookData (caller has
// no ownership stake at all).
func TestMergeBooks_CallerOwnsNeither(t *testing.T) {
	cleanupMergeUser(t)
	cleanupMergeUser2(t)

	isbn1 := "9780030303061"
	isbn2 := "9780030303062"

	// Only user2 owns these books — the admin (mergeTestUser) owns neither.
	winner := addMergeBookForUser2(
		t, "NeitherWin", isbn1, models.StatusToRead, []string{},
	)
	loser := addMergeBookForUser2(
		t, "NeitherLos", isbn2, models.StatusRead, []string{"carried"},
	)
	loserBookID := loser.BookID

	// Admin merges without owning either book.
	_, _, err := testApp.Services.Books.MergeBooks(
		context.Background(), mergeTestUser, winner.BookID, []uuid.UUID{loserBookID},
		nil, nil, nil,
	)
	require.NoError(t, err)

	// User2's loser row must be gone.
	var loserCount int
	err = testDB.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM books.user_books WHERE user_id = $1 AND book_id = $2`,
		mergeTestUser2, loserBookID,
	).Scan(&loserCount)
	require.NoError(t, err)
	assert.Equal(t, 0, loserCount, "user2 loser entry must be deleted")

	// User2 must now own the winner.
	var winnerCount int
	err = testDB.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM books.user_books WHERE user_id = $1 AND book_id = $2`,
		mergeTestUser2, winner.BookID,
	).Scan(&winnerCount)
	require.NoError(t, err)
	assert.Equal(t, 1, winnerCount, "user2 must own the winner after merge")

	// Loser catalog row must be deleted.
	var catCount int
	err = testDB.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM books.books WHERE id = $1`, loserBookID,
	).Scan(&catCount)
	require.NoError(t, err)
	assert.Equal(t, 0, catCount, "orphaned loser catalog book must be deleted")
}
