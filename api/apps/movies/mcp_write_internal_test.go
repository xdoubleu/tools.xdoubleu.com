//nolint:exhaustruct // sparse tool arguments
package movies

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"tools.xdoubleu.com/apps/movies/pkg/tmdb"
	moviesv1 "tools.xdoubleu.com/gen/movies/v1"
	"tools.xdoubleu.com/internal/kit/constants"
	"tools.xdoubleu.com/internal/kit/logging"
	"tools.xdoubleu.com/internal/kit/testhelper"
	sharedmocks "tools.xdoubleu.com/internal/mocks"
	sharedmodels "tools.xdoubleu.com/internal/models"
	"tools.xdoubleu.com/internal/platform/family"
)

// writeTMDB serves one movie and one two-season series.
type writeTMDB struct{}

func (writeTMDB) Search(context.Context, string) ([]tmdb.Title, error) {
	return nil, nil
}

func (writeTMDB) ListRegionProviders(context.Context) ([]tmdb.RegionProvider, error) {
	return nil, nil
}

func (writeTMDB) GetMovie(_ context.Context, id int64) (*tmdb.Title, error) {
	if id != 680 {
		return nil, tmdb.ErrNotFound
	}
	//nolint:exhaustruct // the fields the service reads
	return &tmdb.Title{
		MediaType: tmdb.MediaTypeMovie, TMDBID: 680, Title: "Pulp Fiction",
		Genres: []string{"Crime"},
	}, nil
}

func (writeTMDB) GetSeries(_ context.Context, id int64) (*tmdb.Title, error) {
	if id != 70523 {
		return nil, tmdb.ErrNotFound
	}
	aired := time.Date(2017, 12, 1, 0, 0, 0, 0, time.UTC)
	//nolint:exhaustruct // the fields the service reads
	return &tmdb.Title{
		MediaType: tmdb.MediaTypeSeries, TMDBID: 70523, Title: "Dark",
		Genres: []string{"Drama"},
		Seasons: []tmdb.Season{
			{Number: 1, Name: "Season 1", AirDate: &aired, EpisodeCount: 10},
			{Number: 2, Name: "Season 2", AirDate: &aired, EpisodeCount: 8},
		},
	}, nil
}

func newWriteHandler(
	t *testing.T, uid string,
) (*moviesConnectHandler, context.Context) {
	t.Helper()
	cfg := testhelper.NewTestConfig()
	pg := testhelper.ConnectTestDB(cfg.DBDsn)
	a := NewWithTMDB(
		context.Background(),
		sharedmocks.NewMockedAuthService(uid), logging.NewNopLogger(), cfg, pg,
		family.NewFamilyRepository(pg), writeTMDB{},
	)
	require.NoError(t, a.ApplyMigrations(context.Background(), pg))
	_, err := pg.Exec(context.Background(),
		"DELETE FROM movies.user_titles WHERE user_id = $1", uid)
	require.NoError(t, err)

	ctx := context.WithValue(
		context.Background(),
		constants.UserContextKey,
		//nolint:exhaustruct // only ID and AppAccess matter for the gate
		sharedmodels.User{ID: uid, AppAccess: []string{mcpAppName}},
	)
	return &moviesConnectHandler{app: a}, ctx
}

func entryOf(t *testing.T, msg proto.Message) *moviesv1.BacklogEntry {
	t.Helper()
	switch r := msg.(type) {
	case *moviesv1.AddTitleResponse:
		return r.Entry
	case *moviesv1.SetStatusResponse:
		return r.Entry
	}
	require.Failf(t, "unexpected response", "%T", msg)
	return nil
}

func backlog(
	ctx context.Context, t *testing.T, h *moviesConnectHandler, shared bool,
) []*moviesv1.BacklogEntry {
	t.Helper()
	msg, err := h.mcpListBacklog(ctx, mcpListBacklogArgs{Limit: 50, Shared: shared})
	require.NoError(t, err)
	resp, ok := msg.(*moviesv1.ListBacklogResponse)
	require.True(t, ok)
	return resp.Entries
}

func TestMCPAddTitle_DefaultsToWantAndIsIdempotent(t *testing.T) {
	h, ctx := newWriteHandler(t, "mcp-write-add")

	msg, err := h.mcpAddTitle(ctx, mcpAddTitleArgs{TMDBID: 680, MediaType: "movie"})
	require.NoError(t, err)
	first := entryOf(t, msg)
	assert.Equal(t, "Pulp Fiction", first.Title)
	assert.Equal(t, "want", first.Status)

	// Adding again returns the same entry and keeps its status.
	_, err = h.mcpUpdateTitle(ctx, mcpUpdateTitleArgs{ID: first.Id, Status: "watching"})
	require.NoError(t, err)
	msg, err = h.mcpAddTitle(ctx, mcpAddTitleArgs{TMDBID: 680, MediaType: "movie"})
	require.NoError(t, err)
	again := entryOf(t, msg)
	assert.Equal(t, first.Id, again.Id)
	assert.Equal(t, "watching", again.Status)
	assert.Len(t, backlog(ctx, t, h, false), 1)

	// An explicit status changes the existing entry.
	msg, err = h.mcpAddTitle(ctx, mcpAddTitleArgs{
		TMDBID: 680, MediaType: "movie", Status: "dropped",
	})
	require.NoError(t, err)
	assert.Equal(t, "dropped", entryOf(t, msg).Status)
	assert.Len(t, backlog(ctx, t, h, false), 1)
}

func TestMCPAddTitle_WatchedIsDatedAndCountsInStats(t *testing.T) {
	h, ctx := newWriteHandler(t, "mcp-write-watched")
	today := time.Now().Format(time.DateOnly)

	msg, err := h.mcpAddTitle(ctx, mcpAddTitleArgs{
		TMDBID: 680, MediaType: "movie", Status: "watched", Rating: new(4),
		WatchedAt: today,
	})
	require.NoError(t, err)
	e := entryOf(t, msg)
	assert.Equal(t, "watched", e.Status)
	require.NotNil(t, e.Rating)
	assert.Equal(t, int32(4), *e.Rating)
	require.Len(t, e.WatchedAt, 1)
	assert.Contains(t, e.WatchedAt[0], today)

	list := backlog(ctx, t, h, false)
	require.Len(t, list, 1)
	assert.Equal(t, "watched", list[0].Status)

	stats, err := h.mcpGetStats(ctx, mcpStatsArgs{})
	require.NoError(t, err)
	resp, ok := stats.(*moviesv1.GetStatsResponse)
	require.True(t, ok)
	assert.Equal(t, int32(1), resp.MoviesWatched)
}

func TestMCPAddTitle_Validation(t *testing.T) {
	h, ctx := newWriteHandler(t, "mcp-write-validation")

	cases := map[string]struct {
		args mcpAddTitleArgs
		want string
	}{
		"bad status": {
			mcpAddTitleArgs{TMDBID: 680, MediaType: "movie", Status: "seen"},
			"status must be",
		},
		"bad rating": {
			mcpAddTitleArgs{TMDBID: 680, MediaType: "movie", Rating: new(6)},
			"rating must be",
		},
		"bad date": {
			mcpAddTitleArgs{
				TMDBID: 680, MediaType: "movie", Status: "watched", WatchedAt: "yesterday",
			},
			"YYYY-MM-DD",
		},
		"date without watched": {
			mcpAddTitleArgs{
				TMDBID: 680, MediaType: "movie", Status: "want", WatchedAt: "2024-01-02",
			},
			"needs status watched",
		},
		"date on series": {
			mcpAddTitleArgs{
				TMDBID: 70523, MediaType: "series", Status: "watched",
				WatchedAt: "2024-01-02",
			},
			"watched_at is for movies",
		},
		"bad media type": {
			mcpAddTitleArgs{TMDBID: 680, MediaType: "book"},
			"media_type must be",
		},
		"unknown tmdb id": {
			mcpAddTitleArgs{TMDBID: 1, MediaType: "movie"},
			"not found",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := h.mcpAddTitle(ctx, tc.args)
			require.ErrorContains(t, err, tc.want)
		})
	}
}

func TestMCPUpdateAndRemoveTitle(t *testing.T) {
	h, ctx := newWriteHandler(t, "mcp-write-update")
	msg, err := h.mcpAddTitle(ctx, mcpAddTitleArgs{TMDBID: 680, MediaType: "movie"})
	require.NoError(t, err)
	id := entryOf(t, msg).Id

	msg, err = h.mcpUpdateTitle(ctx, mcpUpdateTitleArgs{
		ID: id, Status: "watched", Rating: new(5), WatchedAt: "2024-03-02",
	})
	require.NoError(t, err)
	e := entryOf(t, msg)
	assert.Equal(t, "watched", e.Status)
	require.Len(t, e.WatchedAt, 1)
	assert.Contains(t, e.WatchedAt[0], "2024-03-02")

	// Redating edits the latest watch; 0 clears the rating.
	msg, err = h.mcpUpdateTitle(ctx, mcpUpdateTitleArgs{
		ID: id, Rating: new(0), WatchedAt: "2024-04-05",
	})
	require.NoError(t, err)
	e = entryOf(t, msg)
	assert.Nil(t, e.Rating)
	require.Len(t, e.WatchedAt, 1)
	assert.Contains(t, e.WatchedAt[0], "2024-04-05")

	_, err = h.mcpUpdateTitle(ctx, mcpUpdateTitleArgs{ID: id, Status: "nope"})
	require.ErrorContains(t, err, "status must be")
	_, err = h.mcpUpdateTitle(ctx, mcpUpdateTitleArgs{ID: "x"})
	require.ErrorContains(t, err, "invalid id")
	_, err = h.mcpUpdateTitle(ctx, mcpUpdateTitleArgs{ID: id, Season: new(1)})
	require.ErrorContains(t, err, "go together")
	_, err = h.mcpUpdateTitle(ctx, mcpUpdateTitleArgs{
		ID: id, Season: new(1), SeasonWatched: new(true),
	})
	require.ErrorContains(t, err, "only applies to series")

	_, err = h.mcpRemoveTitle(ctx, mcpRemoveTitleArgs{ID: id})
	require.NoError(t, err)
	assert.Empty(t, backlog(ctx, t, h, false))
	_, err = h.mcpRemoveTitle(ctx, mcpRemoveTitleArgs{ID: id})
	require.ErrorContains(t, err, "not found")
}

func TestMCPUpdateTitle_SeriesSeasons(t *testing.T) {
	h, ctx := newWriteHandler(t, "mcp-write-series")
	msg, err := h.mcpAddTitle(ctx, mcpAddTitleArgs{TMDBID: 70523, MediaType: "series"})
	require.NoError(t, err)
	id := entryOf(t, msg).Id

	msg, err = h.mcpUpdateTitle(ctx, mcpUpdateTitleArgs{
		ID: id, Season: new(1), SeasonWatched: new(true), WatchedAt: "2024-05-06",
	})
	require.NoError(t, err)
	assert.Equal(t, "watching", entryOf(t, msg).Status)

	got, err := h.mcpGetTitle(ctx, mcpGetTitleArgs{ID: id})
	require.NoError(t, err)
	title, ok := got.(*moviesv1.GetTitleResponse)
	require.True(t, ok)
	require.Len(t, title.Seasons, 2)
	require.Len(t, title.Seasons[0].WatchedAt, 1)
	assert.Contains(t, title.Seasons[0].WatchedAt[0], "2024-05-06")

	// Marking the series watched ticks the remaining aired season.
	msg, err = h.mcpUpdateTitle(ctx, mcpUpdateTitleArgs{ID: id, Status: "watched"})
	require.NoError(t, err)
	assert.Equal(t, "watched", entryOf(t, msg).Status)

	_, err = h.mcpUpdateTitle(ctx, mcpUpdateTitleArgs{
		ID: id, Season: new(1), SeasonWatched: new(false), WatchedAt: "2024-05-06",
	})
	require.ErrorContains(t, err, "needs season_watched true")
}

func TestMCPAddTitle_SharedListIsSeparate(t *testing.T) {
	h, ctx := newWriteHandler(t, "mcp-write-shared")
	for range 2 {
		_, err := h.mcpAddTitle(ctx, mcpAddTitleArgs{
			TMDBID: 680, MediaType: "movie", Shared: true,
		})
		require.NoError(t, err)
	}

	assert.Len(t, backlog(ctx, t, h, true), 1)
	assert.Empty(t, backlog(ctx, t, h, false))
}
