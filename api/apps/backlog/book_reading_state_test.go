package backlog_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xdoubleu/essentia/v4/pkg/database"

	"tools.xdoubleu.com/apps/backlog/internal/models"
	backlogv1 "tools.xdoubleu.com/gen/backlog/v1"
)

// ---------------------------------------------------------------------------
// Repository tests
// ---------------------------------------------------------------------------

func TestReadingStateRepo_Upsert_Get(t *testing.T) {
	book := addUniqueBook(t)
	ctx := context.Background()

	loc := "epubcfi(/6/4!/4/2[chap01ref]!/4[body01]/10[para05]/3:10)"
	state := models.BookReadingState{ //nolint:exhaustruct //UpdatedAt set by DB
		UserID:   userID,
		BookID:   book.ID,
		Source:   models.ReadingSourceWeb,
		Percent:  42,
		Location: &loc,
	}

	err := testApp.Repositories.ReadingState.Upsert(ctx, state)
	require.NoError(t, err)

	got, err := testApp.Repositories.ReadingState.Get(ctx, userID, book.ID)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, models.ReadingSourceWeb, got.Source)
	assert.Equal(t, 42, got.Percent)
	require.NotNil(t, got.Location)
	assert.Equal(t, loc, *got.Location)
}

func TestReadingStateRepo_Upsert_Updates(t *testing.T) {
	book := addUniqueBook(t)
	ctx := context.Background()

	err := testApp.Repositories.ReadingState.Upsert(
		ctx,
		models.BookReadingState{ //nolint:exhaustruct //UpdatedAt set by DB
			UserID:  userID,
			BookID:  book.ID,
			Source:  models.ReadingSourceWeb,
			Percent: 10,
		},
	)
	require.NoError(t, err)

	// Upsert again with new values
	err = testApp.Repositories.ReadingState.Upsert(
		ctx,
		models.BookReadingState{ //nolint:exhaustruct //UpdatedAt set by DB
			UserID:  userID,
			BookID:  book.ID,
			Source:  models.ReadingSourceKobo,
			Percent: 75,
		},
	)
	require.NoError(t, err)

	got, err := testApp.Repositories.ReadingState.Get(ctx, userID, book.ID)
	require.NoError(t, err)
	assert.Equal(t, models.ReadingSourceKobo, got.Source)
	assert.Equal(t, 75, got.Percent)
}

func TestReadingStateRepo_Get_NotFound(t *testing.T) {
	book := addUniqueBook(t)
	ctx := context.Background()

	got, err := testApp.Repositories.ReadingState.Get(ctx, "nonexistent-user", book.ID)
	assert.ErrorIs(t, err, database.ErrResourceNotFound)
	assert.Nil(t, got)
}

func TestReadingStateRepo_Upsert_NilLocation(t *testing.T) {
	book := addUniqueBook(t)
	ctx := context.Background()

	err := testApp.Repositories.ReadingState.Upsert(
		ctx,
		models.BookReadingState{ //nolint:exhaustruct //UpdatedAt set by DB
			UserID:   userID,
			BookID:   book.ID,
			Source:   models.ReadingSourceManual,
			Percent:  50,
			Location: nil,
		},
	)
	require.NoError(t, err)

	got, err := testApp.Repositories.ReadingState.Get(ctx, userID, book.ID)
	require.NoError(t, err)
	assert.Nil(t, got.Location)
	assert.Equal(t, models.ReadingSourceManual, got.Source)
}

// ---------------------------------------------------------------------------
// Connect handler tests
// ---------------------------------------------------------------------------

func TestConnectUpdateReadingProgress_Valid(t *testing.T) {
	book := addUniqueBook(t)
	client := newBooksTestClient(t)
	ctx := context.Background()

	req := connect.NewRequest(&backlogv1.UpdateReadingProgressRequest{
		BookId:   book.ID.String(),
		Source:   models.ReadingSourceWeb,
		Percent:  55,
		Location: "epubcfi(/6/2!/4/2/6:0)",
	})
	req.Header().Set("Cookie", accessToken.String())

	resp, err := client.UpdateReadingProgress(ctx, req)
	require.NoError(t, err)
	assert.NotNil(t, resp.Msg)
}

func TestConnectUpdateReadingProgress_PercentClamped(t *testing.T) {
	book := addUniqueBook(t)
	client := newBooksTestClient(t)
	ctx := context.Background()

	req := connect.NewRequest(&backlogv1.UpdateReadingProgressRequest{
		BookId:  book.ID.String(),
		Source:  models.ReadingSourceWeb,
		Percent: 150,
	})
	req.Header().Set("Cookie", accessToken.String())

	_, err := client.UpdateReadingProgress(ctx, req)
	require.NoError(t, err)

	got, err := testApp.Repositories.ReadingState.Get(ctx, userID, book.ID)
	require.NoError(t, err)
	assert.Equal(t, 100, got.Percent)
}

func TestConnectUpdateReadingProgress_InvalidSource(t *testing.T) {
	book := addUniqueBook(t)
	client := newBooksTestClient(t)
	ctx := context.Background()

	req := connect.NewRequest(&backlogv1.UpdateReadingProgressRequest{
		BookId:  book.ID.String(),
		Source:  "invalid-source",
		Percent: 10,
	})
	req.Header().Set("Cookie", accessToken.String())

	_, err := client.UpdateReadingProgress(ctx, req)
	require.Error(t, err)
	assert.Equal(t, connect.CodeInternal, connect.CodeOf(err))
}

func TestConnectUpdateReadingProgress_InvalidBookID(t *testing.T) {
	client := newBooksTestClient(t)
	ctx := context.Background()

	req := connect.NewRequest(&backlogv1.UpdateReadingProgressRequest{
		BookId:  "not-a-uuid",
		Source:  models.ReadingSourceWeb,
		Percent: 10,
	})
	req.Header().Set("Cookie", accessToken.String())

	_, err := client.UpdateReadingProgress(ctx, req)
	require.Error(t, err)
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestConnectGetReadingState_Found(t *testing.T) {
	book := addUniqueBook(t)
	client := newBooksTestClient(t)
	ctx := context.Background()

	// Set state first
	setReq := connect.NewRequest(&backlogv1.UpdateReadingProgressRequest{
		BookId:  book.ID.String(),
		Source:  models.ReadingSourceManual,
		Percent: 33,
	})
	setReq.Header().Set("Cookie", accessToken.String())
	_, err := client.UpdateReadingProgress(ctx, setReq)
	require.NoError(t, err)

	// Get it back
	getReq := connect.NewRequest(&backlogv1.GetReadingStateRequest{
		BookId: book.ID.String(),
	})
	getReq.Header().Set("Cookie", accessToken.String())

	resp, err := client.GetReadingState(ctx, getReq)
	require.NoError(t, err)
	require.NotNil(t, resp.Msg.State)
	assert.Equal(t, models.ReadingSourceManual, resp.Msg.State.Source)
	assert.Equal(t, int32(33), resp.Msg.State.Percent)
	assert.NotEmpty(t, resp.Msg.State.UpdatedAt)
}

func TestConnectGetReadingState_NotFound(t *testing.T) {
	book := addUniqueBook(t)
	client := newBooksTestClient(t)
	ctx := context.Background()

	req := connect.NewRequest(&backlogv1.GetReadingStateRequest{
		BookId: book.ID.String(),
	})
	req.Header().Set("Cookie", accessToken.String())

	// No state set — service returns ErrResourceNotFound, handler wraps as Internal
	_, err := client.GetReadingState(ctx, req)
	require.Error(t, err)
	assert.Equal(t, connect.CodeInternal, connect.CodeOf(err))
}

func TestConnectGetReadingState_InvalidBookID(t *testing.T) {
	client := newBooksTestClient(t)
	ctx := context.Background()

	req := connect.NewRequest(&backlogv1.GetReadingStateRequest{
		BookId: "bad-uuid",
	})
	req.Header().Set("Cookie", accessToken.String())

	_, err := client.GetReadingState(ctx, req)
	require.Error(t, err)
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

// ---------------------------------------------------------------------------
// Auto-promote tests (status promotion when progress > 0)
// ---------------------------------------------------------------------------

// seedUserBook creates a user_book row for the global userID with the given status.
func seedUserBook(t *testing.T, bookID uuid.UUID, status string) {
	t.Helper()
	require.NoError(t, testApp.Repositories.Books.UpsertUserBook(
		context.Background(),
		models.UserBook{ //nolint:exhaustruct //optional fields
			UserID:         userID,
			BookID:         bookID,
			Status:         status,
			Tags:           []string{},
			ShelfPositions: map[string]int{},
		},
	))
}

func getUserBookStatus(t *testing.T, bookID uuid.UUID) string {
	t.Helper()
	ub, err := testApp.Repositories.Books.GetUserBook(
		context.Background(),
		userID,
		bookID,
	)
	require.NoError(t, err)
	return ub.Status
}

// TestUpdateReadingProgress_PromotesToReading_FromToRead verifies that a book
// with status "to-read" is promoted to "currently-reading" when progress > 0.
func TestUpdateReadingProgress_PromotesToReading_FromToRead(t *testing.T) {
	book := addUniqueBook(t)
	seedUserBook(t, book.ID, models.StatusToRead)

	err := testApp.Services.Books.UpdateReadingProgress(
		context.Background(), userID, book.ID, models.ReadingSourceKobo, 10, nil,
	)
	require.NoError(t, err)

	assert.Equal(t, models.StatusReading, getUserBookStatus(t, book.ID))
}

// TestUpdateReadingProgress_PromotesToReading_FromDropped verifies that a
// dropped book is revived to "currently-reading" when progress > 0.
func TestUpdateReadingProgress_PromotesToReading_FromDropped(t *testing.T) {
	book := addUniqueBook(t)
	seedUserBook(t, book.ID, models.StatusDropped)

	err := testApp.Services.Books.UpdateReadingProgress(
		context.Background(), userID, book.ID, models.ReadingSourceKobo, 5, nil,
	)
	require.NoError(t, err)

	assert.Equal(t, models.StatusReading, getUserBookStatus(t, book.ID))
}

// TestUpdateReadingProgress_NoPromote_AlreadyReading confirms a book already
// "currently-reading" keeps its status unchanged.
func TestUpdateReadingProgress_NoPromote_AlreadyReading(t *testing.T) {
	book := addUniqueBook(t)
	seedUserBook(t, book.ID, models.StatusReading)

	err := testApp.Services.Books.UpdateReadingProgress(
		context.Background(), userID, book.ID, models.ReadingSourceWeb, 50, nil,
	)
	require.NoError(t, err)

	assert.Equal(t, models.StatusReading, getUserBookStatus(t, book.ID))
}

// TestUpdateReadingProgress_NoPromote_AlreadyRead confirms a finished book is
// not demoted back to "currently-reading".
func TestUpdateReadingProgress_NoPromote_AlreadyRead(t *testing.T) {
	book := addUniqueBook(t)
	seedUserBook(t, book.ID, models.StatusRead)

	err := testApp.Services.Books.UpdateReadingProgress(
		context.Background(), userID, book.ID, models.ReadingSourceWeb, 80, nil,
	)
	require.NoError(t, err)

	assert.Equal(t, models.StatusRead, getUserBookStatus(t, book.ID))
}

// TestUpdateReadingProgress_NoPromote_ZeroPercent confirms that a 0% progress
// update does not promote the book (Kobo sends 0 on initial open/sync).
func TestUpdateReadingProgress_NoPromote_ZeroPercent(t *testing.T) {
	book := addUniqueBook(t)
	seedUserBook(t, book.ID, models.StatusToRead)

	err := testApp.Services.Books.UpdateReadingProgress(
		context.Background(), userID, book.ID, models.ReadingSourceKobo, 0, nil,
	)
	require.NoError(t, err)

	assert.Equal(t, models.StatusToRead, getUserBookStatus(t, book.ID))
}
