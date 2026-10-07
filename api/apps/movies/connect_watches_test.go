package movies_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	moviesv1 "tools.xdoubleu.com/gen/movies/v1"
)

func season(n int32) *int32 { return &n }

func TestMovieWatchDates_AddEditRemove(t *testing.T) {
	c := newClient(t, userID, fakeTMDB{})
	ctx := context.Background()
	e := add(t, c, "movie", 129, "watched")
	require.Len(t, e.WatchedAt, 1)

	added, err := c.AddWatchDate(ctx, connect.NewRequest(
		&moviesv1.AddWatchDateRequest{Id: e.Id, SeasonNumber: nil, Date: "2020-05-01"},
	))
	require.NoError(t, err)
	assert.Equal(t, "2020-05-01T12:00:00Z", added.Msg.Entry.WatchedAt[1])

	edited, err := c.EditWatchDate(ctx, connect.NewRequest(
		&moviesv1.EditWatchDateRequest{Id: e.Id, SeasonNumber: nil, Index: 0, Date: ""},
	))
	require.NoError(t, err)
	assert.Equal(t, []string{"", "2020-05-01T12:00:00Z"}, edited.Msg.Entry.WatchedAt)

	removed, err := c.RemoveWatchDate(ctx, connect.NewRequest(
		&moviesv1.RemoveWatchDateRequest{Id: e.Id, SeasonNumber: nil, Index: 1},
	))
	require.NoError(t, err)
	assert.Equal(t, []string{""}, removed.Msg.Entry.WatchedAt)

	removed, err = c.RemoveWatchDate(ctx, connect.NewRequest(
		&moviesv1.RemoveWatchDateRequest{Id: e.Id, SeasonNumber: nil, Index: 0},
	))
	require.NoError(t, err)
	assert.Empty(t, removed.Msg.Entry.WatchedAt)
	assert.Equal(t, "watched", removed.Msg.Entry.Status, "status is left alone")
}

func TestMovieWatched_UnknownDate(t *testing.T) {
	c := newClient(t, userID, fakeTMDB{})
	ctx := context.Background()

	added, err := c.AddTitle(ctx, connect.NewRequest(&moviesv1.AddTitleRequest{
		MediaType: "movie", TmdbId: 603, Status: "watched", UnknownDate: true,
	}))
	require.NoError(t, err)
	assert.Equal(t, []string{""}, added.Msg.Entry.WatchedAt)

	other := add(t, c, "movie", 129, "want")
	set, err := c.SetStatus(ctx, connect.NewRequest(&moviesv1.SetStatusRequest{
		Id: other.Id, Status: "watched", UnknownDate: true,
	}))
	require.NoError(t, err)
	assert.Equal(t, []string{""}, set.Msg.Entry.WatchedAt)
}

func TestSeasonWatchDates_RewatchAndUntick(t *testing.T) {
	c := newClient(t, userID, fakeTMDB{})
	ctx := context.Background()
	e := add(t, c, "series", 1396, "want")
	setSeason(t, c, e.Id, 1, true)

	added, err := c.AddWatchDate(ctx, connect.NewRequest(
		&moviesv1.AddWatchDateRequest{Id: e.Id, SeasonNumber: season(1), Date: ""},
	))
	require.NoError(t, err)
	assert.Len(t, ticks(added.Msg.Seasons)[1], 2, "a rewatch")

	edited, err := c.EditWatchDate(ctx, connect.NewRequest(
		&moviesv1.EditWatchDateRequest{
			Id: e.Id, SeasonNumber: season(1), Index: 1, Date: "2021-02-03",
		},
	))
	require.NoError(t, err)
	assert.Equal(t, "2021-02-03T12:00:00Z", ticks(edited.Msg.Seasons)[1][1])

	for range 2 {
		_, err = c.RemoveWatchDate(ctx, connect.NewRequest(
			&moviesv1.RemoveWatchDateRequest{Id: e.Id, SeasonNumber: season(1), Index: 0},
		))
		require.NoError(t, err)
	}
	got := getTitle(t, c, e.Id)
	assert.Empty(t, ticks(got.Seasons)[1])
	assert.Equal(t, "want", got.Entry.Status, "removing the last watch unticks")

	added, err = c.AddWatchDate(ctx, connect.NewRequest(
		&moviesv1.AddWatchDateRequest{Id: e.Id, SeasonNumber: season(2), Date: ""},
	))
	require.NoError(t, err)
	assert.Equal(t, "watching", added.Msg.Entry.Status, "adding a watch ticks")
}

func TestWatchDates_Errors(t *testing.T) {
	c := newClient(t, userID, fakeTMDB{})
	ctx := context.Background()
	movie := add(t, c, "movie", 603, "want")
	series := add(t, c, "series", 1396, "want")

	cases := map[string]struct {
		call func() error
		want connect.Code
	}{
		"series watch without a season": {func() error {
			_, err := c.AddWatchDate(ctx, connect.NewRequest(
				&moviesv1.AddWatchDateRequest{Id: series.Id, SeasonNumber: nil, Date: ""},
			))
			return err
		}, connect.CodeInvalidArgument},
		"bad date": {func() error {
			_, err := c.AddWatchDate(ctx, connect.NewRequest(
				&moviesv1.AddWatchDateRequest{Id: movie.Id, SeasonNumber: nil, Date: "May"},
			))
			return err
		}, connect.CodeInvalidArgument},
		"edit past the end": {func() error {
			_, err := c.EditWatchDate(ctx, connect.NewRequest(
				&moviesv1.EditWatchDateRequest{
					Id: movie.Id, SeasonNumber: nil, Index: 0, Date: "",
				},
			))
			return err
		}, connect.CodeInvalidArgument},
		"edit with a bad date": {func() error {
			_, err := c.EditWatchDate(ctx, connect.NewRequest(
				&moviesv1.EditWatchDateRequest{
					Id: movie.Id, SeasonNumber: nil, Index: 0, Date: "x",
				},
			))
			return err
		}, connect.CodeInvalidArgument},
		"remove a negative index": {func() error {
			_, err := c.RemoveWatchDate(ctx, connect.NewRequest(
				&moviesv1.RemoveWatchDateRequest{Id: movie.Id, SeasonNumber: nil, Index: -1},
			))
			return err
		}, connect.CodeInvalidArgument},
		"unknown season": {func() error {
			_, err := c.AddWatchDate(ctx, connect.NewRequest(
				&moviesv1.AddWatchDateRequest{Id: series.Id, SeasonNumber: season(7), Date: ""},
			))
			return err
		}, connect.CodeNotFound},
		"edit a season past the end": {func() error {
			_, err := c.EditWatchDate(ctx, connect.NewRequest(
				&moviesv1.EditWatchDateRequest{
					Id: series.Id, SeasonNumber: season(1), Index: 0, Date: "",
				},
			))
			return err
		}, connect.CodeInvalidArgument},
		"bad id": {func() error {
			_, err := c.RemoveWatchDate(ctx, connect.NewRequest(
				&moviesv1.RemoveWatchDateRequest{Id: "x", SeasonNumber: nil, Index: 0},
			))
			return err
		}, connect.CodeInvalidArgument},
		"another user's entry": {func() error {
			other := newKeepingClient(t, otherUserID, fakeTMDB{})
			_, err := other.AddWatchDate(ctx, connect.NewRequest(
				&moviesv1.AddWatchDateRequest{Id: movie.Id, SeasonNumber: nil, Date: ""},
			))
			return err
		}, connect.CodeNotFound},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tc.want, code(tc.call()))
		})
	}
}
