//nolint:exhaustruct // sparse tool arguments
package movies_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	"tools.xdoubleu.com/apps/movies/pkg/tmdb"
	moviesv1 "tools.xdoubleu.com/gen/movies/v1"
	"tools.xdoubleu.com/internal/kit/constants"
	sharedmodels "tools.xdoubleu.com/internal/models"
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

type mcpUser struct {
	t       *testing.T
	session *mcp.ClientSession
}

// newMCPUser serves the app's tools as uid, with an empty solo backlog.
func newMCPUser(t *testing.T, uid string, client tmdb.Client) mcpUser {
	t.Helper()
	ctx := context.Background()
	_, err := testDB.Exec(ctx, "DELETE FROM movies.user_titles WHERE user_id = $1", uid)
	require.NoError(t, err)

	srv := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	newApp(uid, client).RegisterMCPTools(srv)
	srv.AddReceivingMiddleware(func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
			//nolint:exhaustruct // only ID and AppAccess matter for the gate
			user := sharedmodels.User{ID: uid, AppAccess: []string{"movies"}}
			return next(context.WithValue(ctx, constants.UserContextKey, user), method, req)
		}
	})
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	serverSession, err := srv.Connect(ctx, serverTransport, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = serverSession.Close() })
	c := mcp.NewClient(&mcp.Implementation{Name: "client", Version: "0"}, nil)
	session, err := c.Connect(ctx, clientTransport, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = session.Close() })
	return mcpUser{t: t, session: session}
}

// call runs the tool, decoding its result into out; a tool error is returned.
func (u mcpUser) call(name string, args map[string]any, out proto.Message) error {
	u.t.Helper()
	res, err := u.session.CallTool(
		u.t.Context(), &mcp.CallToolParams{Name: name, Arguments: args},
	)
	require.NoError(u.t, err, name)
	text, ok := res.Content[0].(*mcp.TextContent)
	require.True(u.t, ok)
	if res.IsError {
		return errors.New(text.Text)
	}
	return protojson.Unmarshal([]byte(text.Text), out)
}

func (u mcpUser) add(args map[string]any) (*moviesv1.BacklogEntry, error) {
	var resp moviesv1.AddTitleResponse
	err := u.call("movies_add_title", args, &resp)
	return resp.Entry, err
}

func (u mcpUser) update(args map[string]any) (*moviesv1.BacklogEntry, error) {
	var resp moviesv1.SetStatusResponse
	err := u.call("movies_update_title", args, &resp)
	return resp.Entry, err
}

func (u mcpUser) backlog(shared bool) []*moviesv1.BacklogEntry {
	u.t.Helper()
	var resp moviesv1.ListBacklogResponse
	require.NoError(u.t, u.call("movies_list_backlog",
		map[string]any{"limit": 50, "shared": shared}, &resp))
	return resp.Entries
}

func TestMCPReadTools(t *testing.T) {
	u := newMCPUser(t, "mcp-movies-user", nil)

	var list moviesv1.ListBacklogResponse
	require.NoError(t, u.call("movies_list_backlog",
		map[string]any{"status": "want", "sort": "title", "limit": 5}, &list))
	assert.Empty(t, list.Entries)

	var settings moviesv1.GetSettingsResponse
	require.NoError(t, u.call("movies_get_settings", map[string]any{}, &settings))
	assert.Empty(t, settings.ProviderIds)

	var search moviesv1.SearchTitlesResponse
	err := u.call("movies_search_titles", map[string]any{"query": "matrix"}, &search)
	require.ErrorContains(t, err, "TMDB is not configured")

	var title moviesv1.GetTitleResponse
	err = u.call("movies_get_title", map[string]any{"id": "not-a-uuid"}, &title)
	require.ErrorContains(t, err, "invalid id")
}

func TestMCPAddTitle_DefaultsToWantAndIsIdempotent(t *testing.T) {
	u := newMCPUser(t, "mcp-write-add", writeTMDB{})

	first, err := u.add(map[string]any{"tmdb_id": 680, "media_type": "movie"})
	require.NoError(t, err)
	assert.Equal(t, "Pulp Fiction", first.Title)
	assert.Equal(t, "want", first.Status)

	// Adding again returns the same entry and keeps its status.
	_, err = u.update(map[string]any{"id": first.Id, "status": "watching"})
	require.NoError(t, err)
	again, err := u.add(map[string]any{"tmdb_id": 680, "media_type": "movie"})
	require.NoError(t, err)
	assert.Equal(t, first.Id, again.Id)
	assert.Equal(t, "watching", again.Status)
	assert.Len(t, u.backlog(false), 1)

	// An explicit status changes the existing entry.
	dropped, err := u.add(map[string]any{
		"tmdb_id": 680, "media_type": "movie", "status": "dropped",
	})
	require.NoError(t, err)
	assert.Equal(t, "dropped", dropped.Status)
	assert.Len(t, u.backlog(false), 1)
}

func TestMCPAddTitle_WatchedIsDatedAndCountsInStats(t *testing.T) {
	u := newMCPUser(t, "mcp-write-watched", writeTMDB{})
	today := time.Now().Format(time.DateOnly)

	e, err := u.add(map[string]any{
		"tmdb_id": 680, "media_type": "movie", "status": "watched", "rating": 4,
		"watched_at": today,
	})
	require.NoError(t, err)
	assert.Equal(t, "watched", e.Status)
	require.NotNil(t, e.Rating)
	assert.Equal(t, int32(4), *e.Rating)
	require.Len(t, e.WatchedAt, 1)
	assert.Contains(t, e.WatchedAt[0], today)

	list := u.backlog(false)
	require.Len(t, list, 1)
	assert.Equal(t, "watched", list[0].Status)

	var stats moviesv1.GetStatsResponse
	require.NoError(t, u.call("movies_get_stats", map[string]any{}, &stats))
	assert.Equal(t, int32(1), stats.MoviesWatched)
}

func TestMCPAddTitle_Validation(t *testing.T) {
	u := newMCPUser(t, "mcp-write-validation", writeTMDB{})

	cases := map[string]struct {
		args map[string]any
		want string
	}{
		"bad status": {
			map[string]any{"tmdb_id": 680, "media_type": "movie", "status": "seen"},
			"status must be",
		},
		"bad rating": {
			map[string]any{"tmdb_id": 680, "media_type": "movie", "rating": 6},
			"rating must be",
		},
		"bad date": {
			map[string]any{
				"tmdb_id": 680, "media_type": "movie", "status": "watched",
				"watched_at": "yesterday",
			},
			"YYYY-MM-DD",
		},
		"date without watched": {
			map[string]any{
				"tmdb_id": 680, "media_type": "movie", "status": "want",
				"watched_at": "2024-01-02",
			},
			"needs status watched",
		},
		"date on series": {
			map[string]any{
				"tmdb_id": 70523, "media_type": "series", "status": "watched",
				"watched_at": "2024-01-02",
			},
			"watched_at is for movies",
		},
		"bad media type": {
			map[string]any{"tmdb_id": 680, "media_type": "book"},
			"media_type must be",
		},
		"unknown tmdb id": {
			map[string]any{"tmdb_id": 1, "media_type": "movie"},
			"not found",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := u.add(tc.args)
			require.ErrorContains(t, err, tc.want)
		})
	}
}

func TestMCPUpdateAndRemoveTitle(t *testing.T) {
	u := newMCPUser(t, "mcp-write-update", writeTMDB{})
	added, err := u.add(map[string]any{"tmdb_id": 680, "media_type": "movie"})
	require.NoError(t, err)
	id := added.Id

	e, err := u.update(map[string]any{
		"id": id, "status": "watched", "rating": 5, "watched_at": "2024-03-02",
	})
	require.NoError(t, err)
	assert.Equal(t, "watched", e.Status)
	require.Len(t, e.WatchedAt, 1)
	assert.Contains(t, e.WatchedAt[0], "2024-03-02")

	// Redating edits the latest watch; 0 clears the rating.
	e, err = u.update(map[string]any{"id": id, "rating": 0, "watched_at": "2024-04-05"})
	require.NoError(t, err)
	assert.Nil(t, e.Rating)
	require.Len(t, e.WatchedAt, 1)
	assert.Contains(t, e.WatchedAt[0], "2024-04-05")

	_, err = u.update(map[string]any{"id": id, "status": "nope"})
	require.ErrorContains(t, err, "status must be")
	_, err = u.update(map[string]any{"id": "x"})
	require.ErrorContains(t, err, "invalid id")
	_, err = u.update(map[string]any{"id": id, "season": 1})
	require.ErrorContains(t, err, "go together")
	_, err = u.update(map[string]any{"id": id, "season": 1, "season_watched": true})
	require.ErrorContains(t, err, "only applies to series")

	var removed moviesv1.RemoveTitleResponse
	require.NoError(t, u.call("movies_remove_title", map[string]any{"id": id}, &removed))
	assert.Empty(t, u.backlog(false))
	err = u.call("movies_remove_title", map[string]any{"id": id}, &removed)
	require.ErrorContains(t, err, "not found")
}

func TestMCPUpdateTitle_SeriesSeasons(t *testing.T) {
	u := newMCPUser(t, "mcp-write-series", writeTMDB{})
	added, err := u.add(map[string]any{"tmdb_id": 70523, "media_type": "series"})
	require.NoError(t, err)
	id := added.Id

	e, err := u.update(map[string]any{
		"id": id, "season": 1, "season_watched": true, "watched_at": "2024-05-06",
	})
	require.NoError(t, err)
	assert.Equal(t, "watching", e.Status)

	var title moviesv1.GetTitleResponse
	require.NoError(t, u.call("movies_get_title", map[string]any{"id": id}, &title))
	require.Len(t, title.Seasons, 2)
	assert.True(t, title.Seasons[0].Aired)
	require.Len(t, title.Seasons[0].WatchedAt, 1)
	assert.Contains(t, title.Seasons[0].WatchedAt[0], "2024-05-06")

	// Marking the series watched ticks the remaining aired season.
	e, err = u.update(map[string]any{"id": id, "status": "watched"})
	require.NoError(t, err)
	assert.Equal(t, "watched", e.Status)

	_, err = u.update(map[string]any{
		"id": id, "season": 1, "season_watched": false, "watched_at": "2024-05-06",
	})
	require.ErrorContains(t, err, "needs season_watched true")
}

func TestMCPAddTitle_SharedListIsSeparate(t *testing.T) {
	u := newMCPUser(t, "mcp-write-shared", writeTMDB{})
	for range 2 {
		_, err := u.add(map[string]any{
			"tmdb_id": 680, "media_type": "movie", "shared": true,
		})
		require.NoError(t, err)
	}

	assert.Len(t, u.backlog(true), 1)
	assert.Empty(t, u.backlog(false))
}
