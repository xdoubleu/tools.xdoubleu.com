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

func getTitle(
	t *testing.T,
	c moviesv1connect.MoviesServiceClient,
	id string,
) *moviesv1.GetTitleResponse {
	t.Helper()
	resp, err := c.GetTitle(context.Background(), connect.NewRequest(
		&moviesv1.GetTitleRequest{Id: id},
	))
	require.NoError(t, err)
	return resp.Msg
}

// ticks maps season number to its watch dates.
func ticks(seasons []*moviesv1.Season) map[int32][]string {
	out := map[int32][]string{}
	for _, s := range seasons {
		out[s.Number] = s.WatchedAt
	}
	return out
}

func setSeason(
	t *testing.T,
	c moviesv1connect.MoviesServiceClient,
	id string,
	number int32,
	watched bool,
) *moviesv1.SetSeasonWatchedResponse {
	t.Helper()
	resp, err := c.SetSeasonWatched(context.Background(), connect.NewRequest(
		&moviesv1.SetSeasonWatchedRequest{
			Id: id, SeasonNumber: number, Watched: watched, UnknownDate: false,
		},
	))
	require.NoError(t, err)
	return resp.Msg
}

func TestGetTitle_ListsSeasons(t *testing.T) {
	c := newClient(t, userID, fakeTMDB{})
	e := add(t, c, "series", 1396, "want")

	got := getTitle(t, c, e.Id)
	require.Len(t, got.Seasons, 5)
	assert.Equal(t, int32(0), got.Seasons[0].Number)
	assert.False(t, got.Seasons[0].Aired, "undated specials")
	assert.Equal(t, "Season 1", got.Seasons[1].Name)
	assert.Equal(t, "2008-01-20", got.Seasons[1].AirDate)
	assert.Equal(t, int32(7), got.Seasons[1].EpisodeCount)
	assert.True(t, got.Seasons[1].Aired)
	assert.False(t, got.Seasons[3].Aired, "future air date")
	assert.Empty(t, got.Seasons[1].WatchedAt)
}

func TestAddTitle_WatchedSeriesTicksAiredSeasons(t *testing.T) {
	for name, unknown := range map[string]bool{"today": false, "a while ago": true} {
		t.Run(name, func(t *testing.T) {
			c := newClient(t, userID, fakeTMDB{})
			resp, err := c.AddTitle(context.Background(), connect.NewRequest(
				&moviesv1.AddTitleRequest{
					MediaType: "series", TmdbId: 1396, Status: "watched",
					UnknownDate: unknown,
				},
			))
			require.NoError(t, err)

			got := ticks(getTitle(t, c, resp.Msg.Entry.Id).Seasons)
			assert.Empty(t, got[0], "specials stay unticked")
			assert.Len(t, got[1], 1)
			assert.Len(t, got[2], 1)
			assert.Empty(t, got[3], "unaired")
			assert.Empty(t, got[4], "undated")
			assert.Equal(t, unknown, got[1][0] == "")
		})
	}
}

func TestSetStatus_WatchedSeriesTicksAiredSeasons(t *testing.T) {
	c := newClient(t, userID, fakeTMDB{})
	e := add(t, c, "series", 1396, "want")
	setSeason(t, c, e.Id, 1, true)

	resp, err := c.SetStatus(context.Background(), connect.NewRequest(
		&moviesv1.SetStatusRequest{Id: e.Id, Status: "watched", UnknownDate: true},
	))
	require.NoError(t, err)
	assert.Equal(t, "watched", resp.Msg.Entry.Status)

	got := ticks(getTitle(t, c, e.Id).Seasons)
	assert.Len(t, got[1], 1, "already ticked seasons keep their watch")
	assert.NotEqual(t, "", got[1][0])
	assert.Equal(t, []string{""}, got[2])
}

func TestSetSeasonWatched_DrivesSeriesStatus(t *testing.T) {
	c := newClient(t, userID, fakeTMDB{})
	e := add(t, c, "series", 1396, "want")

	assert.Equal(t, "want", setSeason(t, c, e.Id, 0, true).Entry.Status,
		"specials don't count")
	assert.Equal(t, "watching", setSeason(t, c, e.Id, 1, true).Entry.Status)
	assert.Equal(t, "watched", setSeason(t, c, e.Id, 2, true).Entry.Status,
		"every aired season ticked")
	assert.Equal(t, "watching", setSeason(t, c, e.Id, 2, false).Entry.Status)

	resp := setSeason(t, c, e.Id, 1, false)
	assert.Equal(t, "want", resp.Entry.Status)
	assert.Empty(t, ticks(resp.Seasons)[1])
}

func TestSetSeasonWatched_TickingTwiceKeepsOneWatch(t *testing.T) {
	c := newClient(t, userID, fakeTMDB{})
	e := add(t, c, "series", 1396, "want")
	setSeason(t, c, e.Id, 1, true)

	got := ticks(setSeason(t, c, e.Id, 1, true).Seasons)
	assert.Len(t, got[1], 1)
	watched, err := time.Parse(time.RFC3339, got[1][0])
	require.NoError(t, err)
	assert.WithinDuration(t, time.Now(), watched, time.Minute)
}

func TestSetSeasonWatched_DroppedStaysDropped(t *testing.T) {
	c := newClient(t, userID, fakeTMDB{})
	e := add(t, c, "series", 1396, "want")
	_, err := c.SetStatus(context.Background(), connect.NewRequest(
		&moviesv1.SetStatusRequest{Id: e.Id, Status: "dropped", UnknownDate: false},
	))
	require.NoError(t, err)

	assert.Equal(t, "dropped", setSeason(t, c, e.Id, 1, true).Entry.Status)
}

func TestSetSeasonWatched_UnknownSeasonOrEntry(t *testing.T) {
	c := newClient(t, userID, fakeTMDB{})
	e := add(t, c, "series", 1396, "want")
	ctx := context.Background()

	_, err := c.SetSeasonWatched(ctx, connect.NewRequest(
		&moviesv1.SetSeasonWatchedRequest{
			Id: e.Id, SeasonNumber: 9, Watched: true, UnknownDate: false,
		},
	))
	assert.Equal(t, connect.CodeNotFound, code(err))

	_, err = c.SetSeasonWatched(ctx, connect.NewRequest(
		&moviesv1.SetSeasonWatchedRequest{
			Id: "x", SeasonNumber: 1, Watched: true, UnknownDate: false,
		},
	))
	assert.Equal(t, connect.CodeInvalidArgument, code(err))
}

func TestGetTitle_LoadsMissingSeasonsOnce(t *testing.T) {
	old := newClient(t, userID, seasonlessTMDB{fakeTMDB{}})
	e := add(t, old, "series", 70523, "want")
	assert.Empty(t, getTitle(t, old, e.Id).Seasons)

	// No client, or a failing one, leaves the read working without seasons.
	assert.Empty(t, getTitle(t, newKeepingClient(t, userID, nil), e.Id).Seasons)
	down := newKeepingClient(t, userID, downTMDB{fakeTMDB{}})
	assert.Empty(t, getTitle(t, down, e.Id).Seasons)

	assert.Len(t, getTitle(t, newKeepingClient(t, userID, fakeTMDB{}), e.Id).Seasons, 2)
}
