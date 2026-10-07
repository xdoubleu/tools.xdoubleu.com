package movies_test

import (
	"context"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	moviesv1 "tools.xdoubleu.com/gen/movies/v1"
	"tools.xdoubleu.com/gen/movies/v1/moviesv1connect"
)

// setWatches replaces an entry's own watches; nil is an unknown date.
func setWatches(t *testing.T, entryID string, watches ...*time.Time) {
	t.Helper()
	_, err := testDB.Exec(context.Background(),
		"UPDATE movies.user_titles SET watched_at = $2 WHERE id = $1",
		entryID, watches)
	require.NoError(t, err)
}

// setSeasonWatches replaces a ticked season's watches.
func setSeasonWatches(t *testing.T, entryID string, number int, watches ...*time.Time) {
	t.Helper()
	_, err := testDB.Exec(context.Background(), `
		UPDATE movies.user_seasons SET watched_at = $3
		WHERE user_title_id = $1 AND season_number = $2`,
		entryID, number, watches)
	require.NoError(t, err)
}

func stats(
	t *testing.T,
	c moviesv1connect.MoviesServiceClient,
) *moviesv1.GetStatsResponse {
	t.Helper()
	resp, err := c.GetStats(context.Background(), connect.NewRequest(
		&moviesv1.GetStatsRequest{},
	))
	require.NoError(t, err)
	return resp.Msg
}

func TestGetStats_Empty(t *testing.T) {
	got := stats(t, newClient(t, userID, fakeTMDB{}))
	assert.Len(t, got.Months, 12)
	assert.Empty(t, got.Genres)
	assert.Equal(t, []int32{0, 0, 0, 0, 0}, got.Ratings)
	assert.Zero(t, got.MoviesWatched+got.SeriesWatched+got.SeasonsWatched)
}

func TestGetStats_CountsSeededWatches(t *testing.T) {
	brussels, err := time.LoadLocation("Europe/Brussels")
	require.NoError(t, err)
	now := time.Now().In(brussels)
	thisMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, brussels)
	mid := func(monthsAgo int) *time.Time {
		d := thisMonth.AddDate(0, -monthsAgo, 14).Add(12 * time.Hour)
		return &d
	}
	// 00:30 on the 1st in Brussels is still last month in UTC.
	boundary := thisMonth.Add(30 * time.Minute)

	theirs := newClient(t, otherUserID, fakeTMDB{})
	setWatches(t, add(t, theirs, "movie", 129, "watched").Id, mid(0))

	c := newClient(t, userID, fakeTMDB{})
	matrix := add(t, c, "movie", 603, "watched")
	setWatches(t, matrix.Id, mid(0), mid(2), nil, mid(13))
	_, err = rate(c, matrix.Id, stars(5))
	require.NoError(t, err)

	spirited := add(t, c, "movie", 129, "want")
	_, err = rate(c, spirited.Id, stars(3))
	require.NoError(t, err)

	setWatches(t, add(t, c, "movie", 999, "watched").Id, &boundary)

	bb := add(t, c, "series", 1396, "want")
	for _, n := range []int32{0, 1, 2} {
		setSeason(t, c, bb.Id, n, true)
	}
	setSeasonWatches(t, bb.Id, 0, mid(0))
	setSeasonWatches(t, bb.Id, 1, mid(0), mid(0))
	setSeasonWatches(t, bb.Id, 2, nil)
	// Ticking every aired season made it watched; it counts as watching.
	watching(t, c, bb.Id)

	_, err = c.AddTitle(context.Background(), connect.NewRequest(&moviesv1.AddTitleRequest{
		MediaType: "series", TmdbId: 95396, Status: "watched", UnknownDate: true,
	}))
	require.NoError(t, err)

	got := stats(t, c)
	require.Len(t, got.Months, 12)
	assert.Equal(t, thisMonth.Format("2006-01"), got.Months[11].Month)
	assert.Equal(t, thisMonth.AddDate(0, -11, 0).Format("2006-01"), got.Months[0].Month)
	assert.Equal(t, int32(2), got.Months[11].Movies, "a dated watch and the boundary one")
	assert.Equal(t, int32(2), got.Months[11].Seasons, "a rewatch counts; specials don't")
	assert.Equal(t, int32(1), got.Months[9].Movies)
	var movies, seasons int32
	for _, m := range got.Months {
		movies += m.Movies
		seasons += m.Seasons
	}
	assert.Equal(t, int32(3), movies, "undated and 13-month-old watches are left out")
	assert.Equal(t, int32(2), seasons)

	assert.Equal(t, int32(2), got.MoviesWatched)
	assert.Equal(t, int32(1), got.SeriesWatched, "only series whose status is watched")
	assert.Equal(t, int32(3), got.SeasonsWatched, "ticked seasons, undated included")
	assert.Equal(t, []int32{0, 0, 1, 0, 1}, got.Ratings)

	genres := make([]string, len(got.Genres))
	for i, g := range got.Genres {
		genres[i] = g.Genre
		assert.Equal(t, int32(1), g.Count, g.Genre)
	}
	assert.Equal(t, []string{"Action", "Drama", "Science Fiction"}, genres,
		"genres of titles with a watch; the unwatched Animation is left out")
}
