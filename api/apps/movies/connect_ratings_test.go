package movies_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	moviesv1 "tools.xdoubleu.com/gen/movies/v1"
	"tools.xdoubleu.com/gen/movies/v1/moviesv1connect"
)

func rate(
	c moviesv1connect.MoviesServiceClient,
	id string,
	rating *int32,
) (*moviesv1.BacklogEntry, error) {
	resp, err := c.SetRating(context.Background(), connect.NewRequest(
		&moviesv1.SetRatingRequest{Id: id, Rating: rating},
	))
	if err != nil {
		return nil, err
	}
	return resp.Msg.Entry, nil
}

func stars(n int32) *int32 { return &n }

func TestSetRating_RatesAndClears(t *testing.T) {
	c := newClient(t, userID, fakeTMDB{})
	e := add(t, c, "movie", 603, "watched")
	assert.Nil(t, e.Rating, "new entries are unrated")

	for _, n := range []int32{1, 5} {
		got, err := rate(c, e.Id, stars(n))
		require.NoError(t, err)
		assert.Equal(t, n, got.GetRating())
	}
	assert.Equal(t, int32(5), getTitle(t, c, e.Id).Entry.GetRating())

	got, err := rate(c, e.Id, nil)
	require.NoError(t, err)
	assert.Nil(t, got.Rating)
}

func TestSetRating_KeptAcrossStatusChanges(t *testing.T) {
	c := newClient(t, userID, fakeTMDB{})
	e := add(t, c, "series", 1396, "watched")
	_, err := rate(c, e.Id, stars(4))
	require.NoError(t, err)

	resp, err := c.SetStatus(context.Background(), connect.NewRequest(
		&moviesv1.SetStatusRequest{Id: e.Id, Status: "watching", UnknownDate: false},
	))
	require.NoError(t, err)
	assert.Equal(t, int32(4), resp.Msg.Entry.GetRating())
}

func TestSetRating_Errors(t *testing.T) {
	c := newClient(t, userID, fakeTMDB{})
	e := add(t, c, "movie", 603, "watched")

	for _, n := range []int32{0, 6, -1} {
		_, err := rate(c, e.Id, stars(n))
		assert.Equal(t, connect.CodeInvalidArgument, code(err), n)
	}
	_, err := rate(c, "x", stars(3))
	assert.Equal(t, connect.CodeInvalidArgument, code(err))

	theirs := newClient(t, otherUserID, fakeTMDB{})
	_, err = rate(theirs, e.Id, stars(3))
	assert.Equal(t, connect.CodeNotFound, code(err))
	assert.Nil(t, getTitle(t, c, e.Id).Entry.Rating)
}

func TestListBacklog_SortsByRating(t *testing.T) {
	c := newClient(t, userID, fakeTMDB{})
	matrix := add(t, c, "movie", 603, "watched")
	add(t, c, "movie", 129, "want")
	bb := add(t, c, "series", 1396, "watched")
	add(t, c, "movie", 999, "want")
	_, err := rate(c, matrix.Id, stars(3))
	require.NoError(t, err)
	_, err = rate(c, bb.Id, stars(5))
	require.NoError(t, err)

	resp, err := c.ListBacklog(context.Background(), connect.NewRequest(
		&moviesv1.ListBacklogRequest{Sort: "rating"},
	))
	require.NoError(t, err)
	titles := make([]string, len(resp.Msg.Entries))
	for i, e := range resp.Msg.Entries {
		titles[i] = e.Title
	}
	assert.Equal(t,
		[]string{"Breaking Bad", "The Matrix", "Avatar 5", "Spirited Away"},
		titles,
		"highest first; unrated last, newest added first",
	)
	assert.Equal(t, int32(5), resp.Msg.Entries[0].GetRating())
	assert.Nil(t, resp.Msg.Entries[2].Rating)
}
