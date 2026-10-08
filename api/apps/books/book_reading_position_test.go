package books_test

import (
	"context"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/books/internal/models"
	booksv1 "tools.xdoubleu.com/gen/books/v1"
)

// writeProgress applies a reading-progress write through the service.
func writeProgress(t *testing.T, state models.BookReadingState) {
	t.Helper()
	state.UserID = userID
	require.NoError(t, testApp.Services.Books.UpdateReadingProgress(
		context.Background(), state,
	))
}

func readState(t *testing.T, bookID uuid.UUID) *models.BookReadingState {
	t.Helper()
	got, err := testApp.Services.Books.GetReadingState(
		context.Background(), userID, bookID,
	)
	require.NoError(t, err)
	return got
}

func at(minutesAgo int) *time.Time {
	t := time.Now().Add(-time.Duration(minutesAgo) * time.Minute).UTC()
	return &t
}

func epubPos(href string, offset int) *models.ReadingPosition {
	return &models.ReadingPosition{Href: href, Offset: offset, Page: 0}
}

func pdfPage(page int) *models.ReadingPosition {
	return &models.ReadingPosition{Href: "", Offset: 0, Page: page}
}

func koboLoc(value string) *models.KoboLocation {
	return &models.KoboLocation{
		Source: "OEBPS/ch1.xhtml", Type: "KoboSpan", Value: value,
	}
}

func TestReadingProgress_NewerReadAtWins(t *testing.T) {
	book := addUniqueBook(t)
	writeProgress(t, models.BookReadingState{ //nolint:exhaustruct //optional fields
		BookID: book.ID, Source: models.ReadingSourceWeb, Percent: 20, ReadAt: at(10),
	})
	writeProgress(t, models.BookReadingState{ //nolint:exhaustruct //optional fields
		BookID: book.ID, Source: models.ReadingSourceWeb, Percent: 30, ReadAt: at(5),
		Position: epubPos("OEBPS/ch2.xhtml", 42),
	})

	got := readState(t, book.ID)
	assert.Equal(t, 30, got.Percent)
	require.NotNil(t, got.Position)
	assert.Equal(t, "OEBPS/ch2.xhtml", got.Position.Href)
	assert.Equal(t, 42, got.Position.Offset)
	require.NotNil(t, got.ReadAt)
	assert.WithinDuration(t, *at(5), *got.ReadAt, time.Second)
}

func TestReadingProgress_OlderReadAtIgnored(t *testing.T) {
	book := addUniqueBook(t)
	writeProgress(t, models.BookReadingState{ //nolint:exhaustruct //optional fields
		BookID: book.ID, Source: models.ReadingSourceWeb, Percent: 50, ReadAt: at(5),
		Position: epubPos("a.xhtml", 7),
	})
	// An offline device replaying an older read is a no-op, not an error.
	writeProgress(t, models.BookReadingState{ //nolint:exhaustruct //optional fields
		BookID: book.ID, Source: models.ReadingSourceWeb, Percent: 80, ReadAt: at(10),
		Position: epubPos("b.xhtml", 1),
	})

	got := readState(t, book.ID)
	assert.Equal(t, 50, got.Percent)
	assert.Equal(t, "a.xhtml", got.Position.Href)
}

func TestReadingProgress_NewerWebWriteMovesBackwards(t *testing.T) {
	book := addUniqueBook(t)
	seedUserBook(t, book.ID, models.StatusReading)
	writeProgress(t, models.BookReadingState{ //nolint:exhaustruct //optional fields
		BookID: book.ID, Source: models.ReadingSourceKobo, Percent: 70, ReadAt: at(10),
		Location: &koboLoc("kobo.9.1").Value, KoboLocation: koboLoc("kobo.9.1"),
	})
	writeProgress(t, models.BookReadingState{ //nolint:exhaustruct //optional fields
		BookID: book.ID, Source: models.ReadingSourceWeb, Percent: 40, ReadAt: at(1),
		Position: pdfPage(12),
	})

	got := readState(t, book.ID)
	assert.Equal(t, 40, got.Percent)
	assert.Equal(t, models.ReadingSourceWeb, got.Source)
	assert.Nil(t, got.KoboLocation)
	require.NotNil(t, got.Position)
	assert.Equal(t, 12, got.Position.Page)

	ub, err := testApp.Repositories.Books.GetUserBook(
		context.Background(), userID, book.ID,
	)
	require.NoError(t, err)
	assert.Equal(t, 40, ub.ProgressPercent)
}

func TestReadingProgress_KoboWithLocationMovesBackwardsWhenNewer(t *testing.T) {
	book := addUniqueBook(t)
	writeProgress(t, models.BookReadingState{ //nolint:exhaustruct //optional fields
		BookID: book.ID, Source: models.ReadingSourceWeb, Percent: 60, ReadAt: at(10),
	})
	writeProgress(t, models.BookReadingState{ //nolint:exhaustruct //optional fields
		BookID: book.ID, Source: models.ReadingSourceKobo, Percent: 35, ReadAt: at(1),
		Location: &koboLoc("kobo.3.2").Value, KoboLocation: koboLoc("kobo.3.2"),
	})

	got := readState(t, book.ID)
	assert.Equal(t, 35, got.Percent)
	assert.Nil(t, got.Position)
	require.NotNil(t, got.KoboLocation)
	assert.Equal(t, "kobo.3.2", got.KoboLocation.Value)
}

func TestReadingProgress_KoboWithoutLocationNeverRegresses(t *testing.T) {
	book := addUniqueBook(t)
	writeProgress(t, models.BookReadingState{ //nolint:exhaustruct //optional fields
		BookID: book.ID, Source: models.ReadingSourceWeb, Percent: 60, ReadAt: at(10),
	})
	writeProgress(t, models.BookReadingState{ //nolint:exhaustruct //optional fields
		BookID: book.ID, Source: models.ReadingSourceKobo, Percent: 35, ReadAt: at(1),
	})

	assert.Equal(t, 60, readState(t, book.ID).Percent)
}

func TestReadingProgress_KoboAtZeroNeverRegresses(t *testing.T) {
	book := addUniqueBook(t)
	writeProgress(t, models.BookReadingState{ //nolint:exhaustruct //optional fields
		BookID: book.ID, Source: models.ReadingSourceWeb, Percent: 60, ReadAt: at(10),
	})
	writeProgress(t, models.BookReadingState{ //nolint:exhaustruct //optional fields
		BookID: book.ID, Source: models.ReadingSourceKobo, Percent: 0, ReadAt: at(1),
		Location: &koboLoc("kobo.1.1").Value, KoboLocation: koboLoc("kobo.1.1"),
	})

	assert.Equal(t, 60, readState(t, book.ID).Percent)
}

// A newer 0% write from the web reader (an open-time settle) must not clobber
// the stored position, or the list's percent and the resume would diverge.
func TestReadingProgress_WebAtZeroNeverRegresses(t *testing.T) {
	book := addUniqueBook(t)
	writeProgress(t, models.BookReadingState{ //nolint:exhaustruct //optional fields
		BookID: book.ID, Source: models.ReadingSourceWeb, Percent: 36, ReadAt: at(10),
	})
	writeProgress(t, models.BookReadingState{ //nolint:exhaustruct //optional fields
		BookID: book.ID, Source: models.ReadingSourceWeb, Percent: 0, ReadAt: at(1),
	})

	state := readState(t, book.ID)
	assert.Equal(t, 36, state.Percent)
	ub, err := testApp.Repositories.Books.GetUserBook(
		context.Background(), userID, book.ID,
	)
	require.NoError(t, err)
	assert.Equal(t, 36, ub.ProgressPercent)
}

func TestReadingProgress_NoReadAtUsesNow(t *testing.T) {
	book := addUniqueBook(t)
	writeProgress(t, models.BookReadingState{ //nolint:exhaustruct //optional fields
		BookID: book.ID, Source: models.ReadingSourceWeb, Percent: 60, ReadAt: at(1),
	})
	writeProgress(t, models.BookReadingState{ //nolint:exhaustruct //optional fields
		BookID: book.ID, Source: models.ReadingSourceManual, Percent: 10,
	})

	got := readState(t, book.ID)
	assert.Equal(t, 10, got.Percent)
	require.NotNil(t, got.ReadAt)
	assert.WithinDuration(t, time.Now(), *got.ReadAt, time.Minute)
}

func TestReadingProgress_FutureReadAtClampedToNow(t *testing.T) {
	book := addUniqueBook(t)
	future := time.Now().Add(24 * time.Hour)
	writeProgress(t, models.BookReadingState{ //nolint:exhaustruct //optional fields
		BookID: book.ID, Source: models.ReadingSourceWeb, Percent: 60, ReadAt: &future,
	})
	got := readState(t, book.ID)
	require.NotNil(t, got.ReadAt)
	assert.WithinDuration(t, time.Now(), *got.ReadAt, time.Minute)

	// A skewed clock can't pin the position: the next real read still wins.
	writeProgress(t, models.BookReadingState{ //nolint:exhaustruct //optional fields
		BookID: book.ID, Source: models.ReadingSourceWeb, Percent: 61,
		ReadAt: new(time.Now().Add(time.Second)),
	})
	assert.Equal(t, 61, readState(t, book.ID).Percent)
}

func TestReadingProgress_LegacyRowComparesAgainstUpdatedAt(t *testing.T) {
	book := addUniqueBook(t)
	// A row from before read_at existed.
	_, err := testDB.Exec(context.Background(), `
		INSERT INTO books.book_reading_state
		    (user_id, book_id, source, percent, updated_at)
		VALUES ($1, $2, 'kobo', 50, now() - interval '5 minutes')
	`, userID, book.ID)
	require.NoError(t, err)

	writeProgress(t, models.BookReadingState{ //nolint:exhaustruct //optional fields
		BookID: book.ID, Source: models.ReadingSourceWeb, Percent: 20, ReadAt: at(10),
	})
	assert.Equal(t, 50, readState(t, book.ID).Percent)

	writeProgress(t, models.BookReadingState{ //nolint:exhaustruct //optional fields
		BookID: book.ID, Source: models.ReadingSourceWeb, Percent: 20, ReadAt: at(1),
	})
	assert.Equal(t, 20, readState(t, book.ID).Percent)
}

func TestReadingProgress_PositionStoredNeutrally(t *testing.T) {
	book := addUniqueBook(t)
	other := addUniqueBook(t)
	writeProgress(t, models.BookReadingState{ //nolint:exhaustruct //optional fields
		BookID: book.ID, Source: models.ReadingSourceWeb, Percent: 5,
		Position: epubPos("OEBPS/Text/ch1.xhtml", 0),
	})
	writeProgress(t, models.BookReadingState{ //nolint:exhaustruct //optional fields
		BookID: other.ID, Source: models.ReadingSourceWeb, Percent: 5,
		Position: pdfPage(3),
	})

	var epub, pdf string
	require.NoError(t, testDB.QueryRow(context.Background(), `
		SELECT position::text FROM books.book_reading_state
		WHERE user_id = $1 AND book_id = $2
	`, userID, book.ID).Scan(&epub))
	require.NoError(t, testDB.QueryRow(context.Background(), `
		SELECT position::text FROM books.book_reading_state
		WHERE user_id = $1 AND book_id = $2
	`, userID, other.ID).Scan(&pdf))
	assert.JSONEq(t, `{"href":"OEBPS/Text/ch1.xhtml","offset":0}`, epub)
	assert.JSONEq(t, `{"page":3}`, pdf)
}

func TestConnectReadingPosition_RoundTrip(t *testing.T) {
	book := addUniqueBook(t)
	client := newBooksTestClient(t)
	ctx := context.Background()
	readAt := time.Now().Add(-time.Minute).UTC().Truncate(time.Second)

	setReq := connect.NewRequest(&booksv1.UpdateReadingProgressRequest{
		BookId:   book.ID.String(),
		Source:   models.ReadingSourceWeb,
		Percent:  12,
		Location: "epubcfi(/6/4!/4/2/1:0)",
		Position: &booksv1.ReadingPosition{Href: "text/part1.xhtml", Offset: 812},
		ReadAt:   readAt.Format(time.RFC3339),
	})
	setReq.Header().Set("Cookie", accessToken.String())
	_, err := client.UpdateReadingProgress(ctx, setReq)
	require.NoError(t, err)

	getReq := connect.NewRequest(&booksv1.GetReadingStateRequest{
		BookId: book.ID.String(),
	})
	getReq.Header().Set("Cookie", accessToken.String())
	resp, err := client.GetReadingState(ctx, getReq)
	require.NoError(t, err)
	state := resp.Msg.State
	require.NotNil(t, state.Position)
	assert.Equal(t, "text/part1.xhtml", state.Position.Href)
	assert.Equal(t, int32(812), state.Position.Offset)
	assert.Equal(t, int32(0), state.Position.Page)
	assert.Equal(t, readAt.Format(time.RFC3339), state.ReadAt)
	assert.Equal(t, "epubcfi(/6/4!/4/2/1:0)", state.Location)
}

func TestConnectReadingPosition_PercentOnlyHasNoPosition(t *testing.T) {
	book := addUniqueBook(t)
	client := newBooksTestClient(t)
	ctx := context.Background()
	writeProgress(t, models.BookReadingState{ //nolint:exhaustruct //optional fields
		BookID: book.ID, Source: models.ReadingSourceKobo, Percent: 30,
	})

	req := connect.NewRequest(&booksv1.GetReadingStateRequest{
		BookId: book.ID.String(),
	})
	req.Header().Set("Cookie", accessToken.String())
	resp, err := client.GetReadingState(ctx, req)
	require.NoError(t, err)
	assert.Nil(t, resp.Msg.State.Position)
	assert.Equal(t, int32(30), resp.Msg.State.Percent)
}

func TestConnectReadingPosition_InvalidInput(t *testing.T) {
	book := addUniqueBook(t)
	client := newBooksTestClient(t)
	ctx := context.Background()

	cases := map[string]*booksv1.UpdateReadingProgressRequest{
		"bad read_at": {
			BookId: book.ID.String(), Source: models.ReadingSourceWeb,
			ReadAt: "yesterday",
		},
		"negative offset": {
			BookId: book.ID.String(), Source: models.ReadingSourceWeb,
			Position: &booksv1.ReadingPosition{Href: "a.xhtml", Offset: -1},
		},
		"negative page": {
			BookId: book.ID.String(), Source: models.ReadingSourceWeb,
			Position: &booksv1.ReadingPosition{Page: -2},
		},
		"href and page": {
			BookId: book.ID.String(), Source: models.ReadingSourceWeb,
			Position: &booksv1.ReadingPosition{Href: "a.xhtml", Page: 2},
		},
		"empty": {
			BookId: book.ID.String(), Source: models.ReadingSourceWeb,
			Position: &booksv1.ReadingPosition{},
		},
	}
	for name, msg := range cases {
		t.Run(name, func(t *testing.T) {
			req := connect.NewRequest(msg)
			req.Header().Set("Cookie", accessToken.String())
			_, err := client.UpdateReadingProgress(ctx, req)
			require.Error(t, err)
			assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
		})
	}
}
