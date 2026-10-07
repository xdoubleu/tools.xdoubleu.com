package movies_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/movies/pkg/tmdb"
	moviesv1 "tools.xdoubleu.com/gen/movies/v1"
	"tools.xdoubleu.com/gen/movies/v1/moviesv1connect"
)

// grownTMDB is the catalog after Severance's second season aired.
type grownTMDB struct{ fakeTMDB }

func (f grownTMDB) GetSeries(ctx context.Context, id int64) (*tmdb.Title, error) {
	t, err := f.fakeTMDB.GetSeries(ctx, id)
	if t != nil && id == 95396 {
		t.Seasons = append(t.Seasons, tmdb.Season{
			Number: 2, Name: "Season 2", AirDate: date("2025-01-17"), EpisodeCount: 10,
		})
	}
	return t, err
}

// recordingTMDB records each lookup; broken IDs fail and gone IDs are not
// found. Movie overviews read "refreshed".
type recordingTMDB struct {
	fakeTMDB
	mu     *sync.Mutex
	asked  *[]int64
	broken int64
	gone   int64
}

func newRecordingTMDB(broken, gone int64) recordingTMDB {
	return recordingTMDB{
		fakeTMDB: fakeTMDB{}, mu: &sync.Mutex{}, asked: &[]int64{},
		broken: broken, gone: gone,
	}
}

func (f recordingTMDB) record(id int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	*f.asked = append(*f.asked, id)
	switch id {
	case f.broken:
		return errors.New("tmdb down")
	case f.gone:
		return tmdb.ErrNotFound
	}
	return nil
}

func (f recordingTMDB) GetMovie(ctx context.Context, id int64) (*tmdb.Title, error) {
	if err := f.record(id); err != nil {
		return nil, err
	}
	t, err := f.fakeTMDB.GetMovie(ctx, id)
	if t != nil {
		t.Overview = "refreshed"
	}
	return t, err
}

func (f recordingTMDB) GetSeries(ctx context.Context, id int64) (*tmdb.Title, error) {
	if err := f.record(id); err != nil {
		return nil, err
	}
	return f.fakeTMDB.GetSeries(ctx, id)
}

// age backdates a catalog title's last fetch.
func age(t *testing.T, tmdbID int64, by time.Duration) {
	t.Helper()
	_, err := testDB.Exec(context.Background(),
		"UPDATE movies.titles SET fetched_at = now() - $2::interval WHERE tmdb_id = $1",
		tmdbID, by.String())
	require.NoError(t, err)
}

func watching(t *testing.T, c moviesv1connect.MoviesServiceClient, id string) {
	t.Helper()
	_, err := c.SetStatus(context.Background(), connect.NewRequest(
		&moviesv1.SetStatusRequest{Id: id, Status: "watching", UnknownDate: false},
	))
	require.NoError(t, err)
}

func refresh(t *testing.T, client tmdb.Client) {
	t.Helper()
	require.NoError(t, newApp(userID, client).RefreshNow(context.Background()))
}

func titleExists(t *testing.T, tmdbID int64) bool {
	t.Helper()
	var n int
	err := testDB.QueryRow(context.Background(),
		"SELECT count(*) FROM movies.titles WHERE tmdb_id = $1", tmdbID).Scan(&n)
	require.NoError(t, err)
	return n > 0
}

func backlog(
	t *testing.T,
	c moviesv1connect.MoviesServiceClient,
	req *moviesv1.ListBacklogRequest,
) []*moviesv1.BacklogEntry {
	t.Helper()
	resp, err := c.ListBacklog(context.Background(), connect.NewRequest(req))
	require.NoError(t, err)
	return resp.Msg.Entries
}

func TestRefresh_NewSeasonOfAWatchedSeries(t *testing.T) {
	c := newClient(t, userID, fakeTMDB{})
	e := add(t, c, "series", 95396, "watched")
	add(t, c, "movie", 603, "watched")
	for _, entry := range backlog(t, c, &moviesv1.ListBacklogRequest{}) {
		assert.False(t, entry.HasNewSeason, entry.Title)
	}

	age(t, 95396, 2*24*time.Hour)
	refresh(t, grownTMDB{fakeTMDB{}})

	got := getTitle(t, c, e.Id)
	assert.Equal(t, "watched", got.Entry.Status, "a new season keeps watched")
	assert.True(t, got.Entry.HasNewSeason)
	require.Len(t, got.Seasons, 2)
	assert.Empty(t, got.Seasons[1].WatchedAt)

	newOnes := backlog(t, c, &moviesv1.ListBacklogRequest{NewSeason: true})
	require.Len(t, newOnes, 1)
	assert.Equal(t, "Severance", newOnes[0].Title)
	assert.True(t, newOnes[0].HasNewSeason)

	setSeason(t, c, e.Id, 2, true)
	assert.Empty(t, backlog(t, c, &moviesv1.ListBacklogRequest{NewSeason: true}))
}

func TestRefresh_OnlyWatchedSeriesHaveNewSeasons(t *testing.T) {
	c := newClient(t, userID, fakeTMDB{})
	// Breaking Bad's aired season 2 is unticked either way.
	e := add(t, c, "series", 1396, "want")
	setSeason(t, c, e.Id, 1, true)
	watching(t, c, e.Id)
	assert.False(t, getTitle(t, c, e.Id).Entry.HasNewSeason)
	assert.Empty(t, backlog(t, c, &moviesv1.ListBacklogRequest{NewSeason: true}))
}

func TestRefresh_DropsSeasonsTMDBNoLongerLists(t *testing.T) {
	c := newClient(t, userID, grownTMDB{fakeTMDB{}})
	e := add(t, c, "series", 95396, "watched")
	require.Len(t, getTitle(t, c, e.Id).Seasons, 2)

	age(t, 95396, 2*24*time.Hour)
	refresh(t, fakeTMDB{})

	got := getTitle(t, c, e.Id)
	require.Len(t, got.Seasons, 1)
	assert.False(t, got.Entry.HasNewSeason)

	// The dropped season's watch is kept for when TMDB lists it again.
	age(t, 95396, 2*24*time.Hour)
	refresh(t, grownTMDB{fakeTMDB{}})
	assert.Len(t, ticks(getTitle(t, c, e.Id).Seasons)[2], 1)
}

func TestRefresh_FetchesDueTitlesAndIsolatesFailures(t *testing.T) {
	c := newClient(t, userID, fakeTMDB{})
	matrix := add(t, c, "movie", 603, "want")
	spirited := add(t, c, "movie", 129, "want")
	add(t, c, "movie", 999, "want")
	add(t, c, "series", 1396, "want")
	watching(t, c, add(t, c, "series", 95396, "want").Id)
	add(t, c, "series", 83867, "watched")
	for _, id := range []int64{603, 129, 999} {
		age(t, id, 31*24*time.Hour)
	}
	age(t, 1396, 2*24*time.Hour)
	age(t, 95396, 2*24*time.Hour)

	client := newRecordingTMDB(603, 999)
	refresh(t, client)

	assert.ElementsMatch(t, []int64{603, 129, 999, 95396}, *client.asked,
		"stale titles, and series being watched a day after their last fetch")
	assert.Equal(t, "refreshed", getTitle(t, c, spirited.Id).Overview,
		"one title failing doesn't stop the rest")
	assert.NotEqual(t, "refreshed", getTitle(t, c, matrix.Id).Overview)
	assert.True(t, titleExists(t, 999), "a title gone from TMDB is kept")
}

func TestRefresh_WithoutTMDBOnlyPrunes(t *testing.T) {
	newClient(t, otherUserID, fakeTMDB{})
	c := newClient(t, userID, fakeTMDB{})
	gone := add(t, c, "movie", 603, "want")
	justGone := add(t, c, "movie", 999, "want")
	add(t, c, "movie", 129, "want")
	for _, e := range []*moviesv1.BacklogEntry{gone, justGone} {
		_, err := c.RemoveTitle(context.Background(), connect.NewRequest(
			&moviesv1.RemoveTitleRequest{Id: e.Id},
		))
		require.NoError(t, err)
	}
	age(t, 603, 2*24*time.Hour)
	age(t, 129, 31*24*time.Hour)

	refresh(t, nil)

	assert.False(t, titleExists(t, 603), "an orphan title is deleted")
	assert.True(t, titleExists(t, 999), "an orphan fetched today may be mid-add")
	assert.True(t, titleExists(t, 129), "a held title is kept")
}

func TestRefresh_SeriesDueAfterTwentyHours(t *testing.T) {
	c := newClient(t, userID, fakeTMDB{})
	add(t, c, "series", 95396, "watched")
	add(t, c, "series", 83867, "watched")
	age(t, 95396, 21*time.Hour)
	age(t, 83867, 19*time.Hour)

	client := newRecordingTMDB(0, 0)
	refresh(t, client)
	assert.Equal(t, []int64{95396}, *client.asked)
}

func TestRefresh_FailsOnlyWhenEveryTitleFails(t *testing.T) {
	c := newClient(t, userID, fakeTMDB{})
	add(t, c, "movie", 603, "want")
	age(t, 603, 31*24*time.Hour)

	err := newApp(userID, newRecordingTMDB(603, 0)).RefreshNow(context.Background())
	assert.ErrorContains(t, err, "refreshing all 1 due titles failed: tmdb down")
}

// cancellingTMDB cancels the run on its first lookup, as a shutdown would.
type cancellingTMDB struct {
	recordingTMDB
	cancel context.CancelFunc
}

func (f cancellingTMDB) GetMovie(ctx context.Context, id int64) (*tmdb.Title, error) {
	f.cancel()
	return f.recordingTMDB.GetMovie(ctx, id)
}

func TestRefresh_StopsWhenCancelled(t *testing.T) {
	c := newClient(t, userID, fakeTMDB{})
	add(t, c, "movie", 603, "want")
	add(t, c, "movie", 129, "want")
	age(t, 603, 31*24*time.Hour)
	age(t, 129, 31*24*time.Hour)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	client := cancellingTMDB{recordingTMDB: newRecordingTMDB(0, 0), cancel: cancel}
	err := newApp(userID, client).RefreshNow(ctx)
	assert.ErrorIs(t, err, context.Canceled)
	assert.Len(t, *client.asked, 1)
}
