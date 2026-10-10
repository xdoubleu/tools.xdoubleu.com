package books_test

import (
	"context"
	"math"
	"testing"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/books/internal/models"
	"tools.xdoubleu.com/apps/books/internal/services"
	booksv1 "tools.xdoubleu.com/gen/books/v1"
)

func f64(f float64) *float64 { return &f }

// addBookInSeries adds a full-metadata book (no Hardcover enrichment) in series.
func addBookInSeries(
	t *testing.T,
	title string,
	series string,
	position *float64,
) *models.UserBook {
	t.Helper()
	ext := services.SourceProposal{ //nolint:exhaustruct // Index/Differs unused
		Source:         "manual",
		Title:          title,
		Authors:        []string{"Series Author"},
		ISBN13:         testISBN(title),
		CoverURL:       "https://example.com/cover.jpg",
		Description:    "Desc.",
		PageCount:      100,
		SeriesName:     series,
		SeriesPosition: position,
	}
	ub, err := testApp.Services.Books.AddToLibrary(
		context.Background(), userID, ext, models.StatusToRead, []string{},
	)
	require.NoError(t, err)
	return ub
}

func getBook(t *testing.T, bookID uuid.UUID) *models.Book {
	t.Helper()
	book, err := testApp.Repositories.Books.GetBookByID(context.Background(), bookID)
	require.NoError(t, err)
	return book
}

func TestCreateBook_StoresSeries_FillOnly(t *testing.T) {
	uid := uuid.NewString()[:8]
	isbn := testISBN("create-series-" + uid)
	client := newBooksTestClient(t)

	create := func(series string, pos float64, total int32) {
		req := connect.NewRequest(&booksv1.CreateBookRequest{
			Provider:       "hardcover",
			Title:          "Series Create " + uid,
			Author:         "Series Author",
			Isbn13:         isbn,
			CoverUrl:       "https://example.com/c.jpg",
			Description:    "Desc.",
			SeriesName:     "  " + series + " ",
			SeriesPosition: &pos,
			SeriesTotal:    total,
		})
		req.Header().Set("Cookie", accessToken.String())
		_, err := client.CreateBook(context.Background(), req)
		require.NoError(t, err)
	}

	create("Saga-"+uid, 3, 12)
	create("Other-"+uid, 9, 0)

	ub, err := testApp.Repositories.Books.FindUserBookByISBN13(
		context.Background(), userID, isbn,
	)
	require.NoError(t, err)
	series := getBook(t, ub.BookID).Series
	require.NotNil(t, series)
	assert.Equal(t, "Saga-"+uid, series.Name, "an existing series is kept")
	assert.InDelta(t, 3.0, *series.Position, 0)
	require.NotNil(t, series.Total)
	assert.Equal(t, 12, *series.Total)
}

func TestCreateBook_DropsInvalidSeriesPosition(t *testing.T) {
	uid := uuid.NewString()[:8]
	isbn := testISBN("create-series-nan-" + uid)
	pos := math.NaN()
	req := connect.NewRequest(&booksv1.CreateBookRequest{
		Provider:       "hardcover",
		Title:          "Series NaN " + uid,
		Author:         "Series Author",
		Isbn13:         isbn,
		CoverUrl:       "https://example.com/c.jpg",
		Description:    "Desc.",
		SeriesName:     "Saga-" + uid,
		SeriesPosition: &pos,
	})
	req.Header().Set("Cookie", accessToken.String())
	_, err := newBooksTestClient(t).CreateBook(context.Background(), req)
	require.NoError(t, err)

	ub, err := testApp.Repositories.Books.FindUserBookByISBN13(
		context.Background(), userID, isbn,
	)
	require.NoError(t, err)
	series := getBook(t, ub.BookID).Series
	require.NotNil(t, series)
	assert.Nil(t, series.Position)
	assert.Nil(t, series.Total, "a zero total is unknown")
}

func TestGetSeries_MergesLibraryAndHardcover(t *testing.T) {
	uid := uuid.NewString()[:8]
	name := "Merge Saga " + uid
	owned := addBookInSeries(t, "Second Volume "+uid, name, f64(2))

	req := connect.NewRequest(&booksv1.GetSeriesRequest{Name: name})
	req.Header().Set("Cookie", accessToken.String())
	resp, err := newBooksTestClient(t).GetSeries(context.Background(), req)
	require.NoError(t, err)

	msg := resp.Msg
	assert.Equal(t, name, msg.Name)
	assert.Equal(t, int32(2), msg.Total)
	assert.False(t, msg.ExternalUnavailable)
	require.Len(t, msg.Entries, 2)

	missing := msg.Entries[0]
	require.NotNil(t, missing.External, "the mock's volume 1 isn't owned")
	assert.Nil(t, missing.UserBook)
	assert.InDelta(t, 1.0, missing.GetPosition(), 0)
	assert.Equal(t, name, missing.External.SeriesName)

	have := msg.Entries[1]
	require.NotNil(t, have.UserBook)
	assert.Equal(t, owned.BookID.String(), have.UserBook.BookId)
	assert.Equal(t, name, have.UserBook.Book.SeriesName)

	total := getBook(t, owned.BookID).Series.Total
	require.NotNil(t, total)
	assert.Equal(t, 2, *total, "Hardcover's count is cached on the book")
}

func TestGetSeries_EmptyName_InvalidArgument(t *testing.T) {
	req := connect.NewRequest(&booksv1.GetSeriesRequest{Name: "  "})
	req.Header().Set("Cookie", accessToken.String())
	_, err := newBooksTestClient(t).GetSeries(context.Background(), req)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	assert.Equal(t, connect.CodeInvalidArgument, connErr.Code())
}

func TestUpdateBook_SetsAndClearsSeries(t *testing.T) {
	uid := uuid.NewString()[:8]
	ub := addBookInSeries(t, "Edit Series "+uid, "Edit Saga "+uid, f64(1))
	require.NoError(t, testApp.Repositories.Books.SetSeriesTotal(
		context.Background(), "Edit Saga "+uid, 7,
	))
	client := newAdminBooksTestClient(t)

	update := func(series string, pos *float64) *booksv1.Book {
		req := connect.NewRequest(&booksv1.UpdateBookRequest{
			BookId: ub.BookID.String(),
			Metadata: &booksv1.Book{
				Title:          "Edit Series " + uid,
				Authors:        []string{"Series Author"},
				SeriesName:     series,
				SeriesPosition: pos,
			},
		})
		req.Header().Set("Cookie", accessToken.String())
		resp, err := client.UpdateBook(context.Background(), req)
		require.NoError(t, err)
		return resp.Msg.Book
	}

	got := update("Edit Saga "+uid, f64(1.5))
	assert.Equal(t, "Edit Saga "+uid, got.SeriesName)
	assert.InDelta(t, 1.5, got.GetSeriesPosition(), 0)
	assert.Equal(t, int32(7), got.SeriesTotal, "same series keeps its total")

	got = update("Renamed "+uid, nil)
	assert.Equal(t, "Renamed "+uid, got.SeriesName)
	assert.Nil(t, got.SeriesPosition)
	assert.Zero(t, got.SeriesTotal)

	got = update("", nil)
	assert.Empty(t, got.SeriesName)
	assert.Nil(t, getBook(t, ub.BookID).Series)
}

func TestMergeBooks_KeepsLoserSeries(t *testing.T) {
	uid := uuid.NewString()[:8]
	winner := addTestBookNoISBN(t, "Merge Winner "+uid)
	loser := addBookInSeries(t, "Merge Loser "+uid, "Loser Saga "+uid, f64(4))

	_, _, err := testApp.Services.Books.MergeBooks(
		context.Background(), userID, winner.BookID,
		[]uuid.UUID{loser.BookID}, nil, nil, nil,
	)
	require.NoError(t, err)

	series := getBook(t, winner.BookID).Series
	require.NotNil(t, series)
	assert.Equal(t, "Loser Saga "+uid, series.Name)
	assert.InDelta(t, 4.0, *series.Position, 0)
}

func TestMergeBooks_WinnerKeepsOwnSeries(t *testing.T) {
	uid := uuid.NewString()[:8]
	winner := addBookInSeries(t, "Own Winner "+uid, "Winner Saga "+uid, f64(1))
	loser := addBookInSeries(t, "Own Loser "+uid, "Loser Saga "+uid, f64(2))

	_, _, err := testApp.Services.Books.MergeBooks(
		context.Background(), userID, winner.BookID,
		[]uuid.UUID{loser.BookID}, nil, nil, nil,
	)
	require.NoError(t, err)
	assert.Equal(t, "Winner Saga "+uid, getBook(t, winner.BookID).Series.Name)
}

func TestMergeBooks_ResolvedMetadataKeepsSeries(t *testing.T) {
	uid := uuid.NewString()[:8]
	winner := addBookInSeries(t, "Resolved Winner "+uid, "Winner Saga "+uid, f64(1))
	loser := addTestBookNoISBN(t, "Resolved Loser "+uid)

	_, _, err := testApp.Services.Books.MergeBooks(
		context.Background(), userID, winner.BookID,
		[]uuid.UUID{loser.BookID},
		&models.Book{ //nolint:exhaustruct // partial
			Title: "Resolved " + uid, Authors: []string{"Series Author"},
		},
		nil, nil,
	)
	require.NoError(t, err)

	book := getBook(t, winner.BookID)
	assert.Equal(t, "Resolved "+uid, book.Title)
	require.NotNil(t, book.Series)
	assert.Equal(t, "Winner Saga "+uid, book.Series.Name)
}
