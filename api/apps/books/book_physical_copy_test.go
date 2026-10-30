package books_test

import (
	"context"
	"net/http/httptest"
	"testing"
	"time"

	"connectrpc.com/connect/v2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/books/internal/mocks"
	"tools.xdoubleu.com/apps/books/internal/models"
	booksv1 "tools.xdoubleu.com/gen/books/v1"
	"tools.xdoubleu.com/internal/database"
	"tools.xdoubleu.com/internal/testhelper"
)

// setupPhysicalBook adds mocks.PhysicalEPUB to owner's library, tagged
// own-physical when physical.
func setupPhysicalBook(
	t *testing.T, owner string, printPages, physical bool,
) uuid.UUID {
	t.Helper()
	title := "Physical-" + uuid.NewString()
	seedBookInLibrary(t, owner, title, "Author", "")
	result, err := uploadViaTestApp(t, owner, "physical.epub",
		mocks.PhysicalEPUB(title, printPages))
	require.NoError(t, err)
	bookID := result.UserBook.BookID
	if physical {
		require.NoError(t, testApp.TestSeeds().Services.Books.ToggleTag(
			context.Background(), owner, bookID, models.TagOwnPhysical,
		))
	}
	return bookID
}

func savePage(t *testing.T, client booksTestClient, bookID uuid.UUID, page int32,
	readAt time.Time,
) {
	t.Helper()
	_, err := client.UpdateProgress(
		testhelper.WithCookie(context.Background(), accessToken.String()),
		&booksv1.UpdateProgressRequest{
			BookId: bookID.String(), ProgressMode: models.ProgressModePages,
			CurrentPage: page, ReadAt: readAt.Format(time.RFC3339Nano),
		},
	)
	require.NoError(t, err)
}

func TestPhysical_PageSaveWritesPhysicalReadingState(t *testing.T) {
	client := newBooksTestClient(t)
	bookID := setupPhysicalBook(t, userID, true, true)
	readAt := time.Now().Add(-time.Minute).UTC().Truncate(time.Second)

	savePage(t, client, bookID, 3, readAt)

	state := readState(t, bookID)
	assert.Equal(t, models.ReadingSourcePhysical, state.Source)
	assert.Equal(t, epubPos(mocks.PhysicalCh2Href, 0), state.Position)
	require.NotNil(t, state.ReadAt)
	assert.WithinDuration(t, readAt, *state.ReadAt, time.Second)

	resp, err := client.GetReadingState(
		testhelper.WithCookie(context.Background(), accessToken.String()),
		&booksv1.GetReadingStateRequest{BookId: bookID.String()},
	)
	require.NoError(t, err)
	assert.Equal(t, int32(3), resp.State.PhysicalPage)
	assert.Equal(t, "Epigraph. Two Chapter 2 sentence 01 is right here.",
		resp.State.PhysicalExcerpt, "blocks are spaced apart")

	// An older page entry (an offline replay) loses to the newer one.
	savePage(t, client, bookID, 5, readAt.Add(-time.Hour))
	assert.Equal(t, epubPos(mocks.PhysicalCh2Href, 0), readState(t, bookID).Position)
}

func TestPhysical_UnchangedPageKeepsTheNewerDigitalPosition(t *testing.T) {
	client := newBooksTestClient(t)
	bookID := setupPhysicalBook(t, userID, true, true)
	savePage(t, client, bookID, 3, time.Now().Add(-time.Hour))
	writeProgress(t, models.BookReadingState{ //nolint:exhaustruct //optional fields
		BookID: bookID, Source: models.ReadingSourceWeb, Percent: 40,
		ReadAt: at(1), Position: epubPos(mocks.PhysicalCh2Href, 5),
	})

	savePage(t, client, bookID, 3, time.Now())

	state := readState(t, bookID)
	assert.Equal(t, models.ReadingSourceWeb, state.Source)
	assert.Equal(t, epubPos(mocks.PhysicalCh2Href, 5), state.Position)
}

func TestPhysical_PageSaveWithoutPhysicalCopyWritesNoState(t *testing.T) {
	client := newBooksTestClient(t)
	bookID := setupPhysicalBook(t, userID, true, false)

	savePage(t, client, bookID, 3, time.Now())

	_, err := testApp.TestSeeds().Services.Books.GetReadingState(
		context.Background(), userID, bookID,
	)
	require.ErrorIs(t, err, database.ErrResourceNotFound)

	_, err = client.UpdateProgress(
		testhelper.WithCookie(context.Background(), accessToken.String()),
		&booksv1.UpdateProgressRequest{
			BookId: bookID.String(), ProgressMode: models.ProgressModePages,
			CurrentPage: 3, ReadAt: "yesterday",
		},
	)
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestPhysical_DigitalPositionFollowsCurrentPage(t *testing.T) {
	client := newBooksTestClient(t)
	bookID := setupPhysicalBook(t, userID, true, true)
	half := len("Epigraph.Two") +
		len(mocks.PhysicalSentence(2, 1))*mocks.PhysicalSentences/2

	writeProgress(t, models.BookReadingState{ //nolint:exhaustruct //optional fields
		BookID: bookID, Source: models.ReadingSourceWeb, Percent: 60,
		ReadAt: at(1), Position: epubPos(mocks.PhysicalCh2Href, half),
	})

	require.Eventually(t, func() bool {
		ub, err := testApp.TestSeeds().Services.Books.GetUserBook(
			context.Background(),
			userID,
			bookID,
		)
		return err == nil && ub.CurrentPage == 4
	}, 5*time.Second, 20*time.Millisecond)

	resp, err := client.TranslateReadingPosition(
		testhelper.WithCookie(context.Background(), accessToken.String()),
		&booksv1.TranslateReadingPositionRequest{
			BookId:   bookID.String(),
			Position: &booksv1.ReadingPosition{Href: mocks.PhysicalCh3Href, Offset: 0},
		},
	)
	require.NoError(t, err)
	assert.Equal(t, int32(5), resp.PhysicalPage)
}

func TestPhysical_SetCurrentPageOnlyForTheLatestDigitalState(t *testing.T) {
	ctx := context.Background()
	bookID := setupPhysicalBook(t, userID, false, true)
	writeProgress(t, models.BookReadingState{ //nolint:exhaustruct //optional fields
		BookID: bookID, Source: models.ReadingSourceWeb, Percent: 10, ReadAt: at(1),
	})
	state := readState(t, bookID)
	books := testApp.TestSeeds().Repositories.Books

	require.NoError(t, books.SetCurrentPageIfLatest(
		ctx, userID, bookID, 77, state.UpdatedAt.Add(-time.Second),
	))
	ub, err := testApp.TestSeeds().Services.Books.GetUserBook(ctx, userID, bookID)
	require.NoError(t, err)
	assert.NotEqual(t, 77, ub.CurrentPage, "a stale digital write")

	require.NoError(
		t,
		books.SetCurrentPageIfLatest(ctx, userID, bookID, 77, state.UpdatedAt),
	)
	ub, err = testApp.TestSeeds().Services.Books.GetUserBook(ctx, userID, bookID)
	require.NoError(t, err)
	assert.Equal(t, 77, ub.CurrentPage)
}

func TestPhysical_CopyDetailsRoundTrip(t *testing.T) {
	client := newBooksTestClient(t)
	ctx := context.Background()
	bookID := setupPhysicalBook(t, userID, false, true)

	got, err := client.GetPhysicalCopy(testhelper.WithCookie(ctx, accessToken.String()),
		&booksv1.GetPhysicalCopyRequest{BookId: bookID.String()})
	require.NoError(t, err)
	assert.True(t, got.Syncable)
	assert.False(t, got.HasPrintPages)
	assert.Equal(t, int32(1), got.Copy.TextStartPage)
	require.Len(t, got.Chapters, 3)
	chapterTwo := got.Chapters[1]
	assert.Equal(t, "Chapter Two", chapterTwo.Title)

	_, err = client.UpdatePhysicalCopy(
		testhelper.WithCookie(ctx, accessToken.String()),
		&booksv1.UpdatePhysicalCopyRequest{
			BookId: bookID.String(),
			Copy: &booksv1.PhysicalCopy{
				PageCount: 320, TextStartPage: 9,
				Anchors: []*booksv1.PhysicalAnchor{{Page: 87, Position: chapterTwo.Position}},
			},
		},
	)
	require.NoError(t, err)

	got, err = client.GetPhysicalCopy(testhelper.WithCookie(ctx, accessToken.String()),
		&booksv1.GetPhysicalCopyRequest{BookId: bookID.String()})
	require.NoError(t, err)
	assert.Equal(t, int32(320), got.Copy.PageCount)
	assert.Equal(t, int32(9), got.Copy.TextStartPage)
	require.Len(t, got.Copy.Anchors, 1)
	assert.Equal(t, int32(87), got.Copy.Anchors[0].Page)
	assert.Equal(t, chapterTwo.Position.Href, got.Copy.Anchors[0].Position.Href)

	ub, err := testApp.TestSeeds().Services.Books.GetUserBook(ctx, userID, bookID)
	require.NoError(t, err)
	require.NotNil(t, ub.PhysicalPageCount)
	assert.Equal(t, 320, *ub.PhysicalPageCount)

	savePage(t, client, bookID, 87, time.Now())
	assert.Equal(t, epubPos(chapterTwo.Position.Href, int(chapterTwo.Position.Offset)),
		readState(t, bookID).Position, "the anchor pins page 87")
}

func TestPhysical_CopyErrors(t *testing.T) {
	client := newBooksTestClient(t)
	ctx := context.Background()
	bookID := setupPhysicalBook(t, userID, false, true)

	_, err := client.UpdatePhysicalCopy(
		testhelper.WithCookie(ctx, accessToken.String()),
		&booksv1.UpdatePhysicalCopyRequest{
			BookId: bookID.String(), Copy: &booksv1.PhysicalCopy{PageCount: -3},
		},
	)
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))

	_, err = client.UpdatePhysicalCopy(
		testhelper.WithCookie(ctx, accessToken.String()),
		&booksv1.UpdatePhysicalCopyRequest{
			BookId: uuid.NewString(), Copy: &booksv1.PhysicalCopy{PageCount: 3},
		},
	)
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))

	_, err = client.GetPhysicalCopy(testhelper.WithCookie(ctx, accessToken.String()),
		&booksv1.GetPhysicalCopyRequest{BookId: uuid.NewString()})
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))

	_, err = client.GetPhysicalCopy(
		testhelper.WithCookie(ctx, accessToken.String()),
		&booksv1.GetPhysicalCopyRequest{BookId: "x"},
	)
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestPhysical_KoboFollowsPageEntry(t *testing.T) {
	ts := httptest.NewServer(getRoutes())
	t.Cleanup(ts.Close)
	ctx := context.Background()
	owner := "physical-kobo-" + uuid.NewString()
	bookID := setupPhysicalBook(t, owner, true, true)
	_, err := testApp.TestSeeds().Services.Conversion.EnsureKEPUB(ctx, owner, bookID)
	require.NoError(t, err)
	require.NoError(
		t,
		testApp.TestSeeds().Services.Books.EnableKoboSync(ctx, owner, bookID),
	)
	rawToken := registerTestDevice(t, owner)

	require.NoError(t, testApp.TestSeeds().Services.Physical.SyncPage(
		ctx, owner, bookID, 5, time.Now().Add(-time.Second),
	))

	rs := koboGetState(t, ts, rawToken, bookID)
	loc, ok := bookmarkLocation(t, rs).(map[string]any)
	require.True(t, ok, "the page entry reaches the Kobo as a span")
	assert.Equal(t, mocks.PhysicalCh3Href, loc["Source"])
	assert.Equal(t, "KoboSpan", loc["Type"])
}

func TestPhysical_KoboBackfillFollowsCurrentPage(t *testing.T) {
	ctx := context.Background()
	owner := "physical-backfill-" + uuid.NewString()
	bookID := setupPhysicalBook(t, owner, true, true)
	loc := models.KoboLocation{
		Source: mocks.PhysicalCh3Href, Type: "KoboSpan", Value: "kobo.1.1",
	}
	svc := testApp.TestSeeds().Services.Books
	require.NoError(t, svc.UpdateReadingProgress(ctx,
		models.BookReadingState{ //nolint:exhaustruct //optional fields
			UserID: owner, BookID: bookID, Source: models.ReadingSourceKobo,
			Percent: 70, Location: &loc.Value, KoboLocation: &loc,
		}))

	require.NoError(t, svc.SetKoboPosition(ctx, owner, bookID, loc,
		models.ReadingPosition{Href: mocks.PhysicalCh3Href, Offset: 0, Page: 0}))

	require.Eventually(t, func() bool {
		ub, err := svc.GetUserBook(ctx, owner, bookID)
		return err == nil && ub.CurrentPage == 5
	}, 5*time.Second, 20*time.Millisecond)
}

func TestPhysical_SetPhysicalPositionOnlyFillsThatWrite(t *testing.T) {
	ctx := context.Background()
	bookID := setupPhysicalBook(t, userID, false, true)
	states := testApp.TestSeeds().Repositories.ReadingState
	updatedAt, err := states.UpsertIfNewer(ctx,
		models.BookReadingState{ //nolint:exhaustruct //optional fields
			UserID: userID, BookID: bookID, Source: models.ReadingSourcePhysical,
			Percent: 30, ReadAt: at(1),
		})
	require.NoError(t, err)
	require.NotNil(t, updatedAt)
	pos := models.ReadingPosition{Href: mocks.PhysicalCh2Href, Offset: 4, Page: 0}

	require.NoError(t, states.SetPhysicalPosition(
		ctx, userID, bookID, updatedAt.Add(-time.Second), pos, 40,
	))
	assert.Nil(t, readState(t, bookID).Position, "another write's")

	require.NoError(
		t,
		states.SetPhysicalPosition(ctx, userID, bookID, *updatedAt, pos, 40),
	)
	got := readState(t, bookID)
	assert.Equal(t, &pos, got.Position)
	assert.Equal(t, 40, got.Percent)
	assert.True(t, got.UpdatedAt.After(*updatedAt), "devices get it again")
}
