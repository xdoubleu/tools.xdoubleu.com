package podcasts_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	podcastsv1 "tools.xdoubleu.com/gen/podcasts/v1"
	"tools.xdoubleu.com/gen/podcasts/v1/podcastsv1connect"
)

func code(err error) connect.Code {
	var ce *connect.Error
	if errors.As(err, &ce) {
		return ce.Code()
	}
	return connect.CodeUnknown
}

func add(
	t *testing.T,
	c podcastsv1connect.PodcastsServiceClient,
	id int64,
) *podcastsv1.Favourite {
	t.Helper()
	resp, err := c.AddFavourite(context.Background(), connect.NewRequest(
		&podcastsv1.AddFavouriteRequest{ItunesId: id},
	))
	require.NoError(t, err)
	return resp.Msg.Favourite
}

func favourites(
	t *testing.T,
	c podcastsv1connect.PodcastsServiceClient,
) []*podcastsv1.Favourite {
	t.Helper()
	resp, err := c.ListFavourites(context.Background(), connect.NewRequest(
		&podcastsv1.ListFavouritesRequest{},
	))
	require.NoError(t, err)
	return resp.Msg.Favourites
}

func search(
	t *testing.T,
	c podcastsv1connect.PodcastsServiceClient,
	query string,
) []*podcastsv1.SearchResult {
	t.Helper()
	resp, err := c.SearchShows(context.Background(), connect.NewRequest(
		&podcastsv1.SearchShowsRequest{Query: query},
	))
	require.NoError(t, err)
	return resp.Msg.Results
}

func TestAddFavourite(t *testing.T) {
	c := newClient(t, userID, fakeITunes{})

	f := add(t, c, 1)
	assert.Equal(t, "Hardcore History", f.Title)
	assert.Equal(t, "Dan Carlin", f.Author)
	assert.Equal(t, int64(1), f.ItunesId)
	assert.Equal(t, "https://img.example/1.jpg", f.ArtworkUrl)
	assert.Equal(t, "https://podcasts.apple.com/podcast/id1", f.AppleUrl)
	added, err := time.Parse(time.RFC3339, f.AddedAt)
	require.NoError(t, err)
	assert.WithinDuration(t, time.Now(), added, time.Minute)

	assert.Equal(t, []string{f.Id}, []string{favourites(t, c)[0].Id})
}

func TestAddFavourite_IsIdempotent(t *testing.T) {
	c := newClient(t, userID, fakeITunes{})

	first := add(t, c, 1)
	again := add(t, c, 1)
	assert.Equal(t, first.Id, again.Id)
	assert.Equal(t, first.AddedAt, again.AddedAt)
	assert.Len(t, favourites(t, c), 1)
}

func TestAddFavourite_UnknownShow(t *testing.T) {
	c := newClient(t, userID, fakeITunes{})

	_, err := c.AddFavourite(context.Background(), connect.NewRequest(
		&podcastsv1.AddFavouriteRequest{ItunesId: 999},
	))
	assert.Equal(t, connect.CodeNotFound, code(err))
}

func TestAddFavourite_InvalidID(t *testing.T) {
	c := newClient(t, userID, fakeITunes{})

	_, err := c.AddFavourite(context.Background(), connect.NewRequest(
		&podcastsv1.AddFavouriteRequest{ItunesId: 0},
	))
	assert.Equal(t, connect.CodeInvalidArgument, code(err))
}

func TestAddFavourite_UpstreamDown(t *testing.T) {
	c := newClient(t, userID, downITunes{})

	_, err := c.AddFavourite(context.Background(), connect.NewRequest(
		&podcastsv1.AddFavouriteRequest{ItunesId: 1},
	))
	assert.Equal(t, connect.CodeUnavailable, code(err))
}

func TestListFavourites_NewestFirst(t *testing.T) {
	c := newClient(t, userID, fakeITunes{})

	add(t, c, 1)
	add(t, c, 2)

	got := favourites(t, c)
	require.Len(t, got, 2)
	assert.Equal(t, "History of Rome", got[0].Title)
	assert.Equal(t, "Hardcore History", got[1].Title)
}

func TestListFavourites_Empty(t *testing.T) {
	c := newClient(t, userID, fakeITunes{})
	assert.Empty(t, favourites(t, c))
}

func TestRemoveFavourite(t *testing.T) {
	c := newClient(t, userID, fakeITunes{})
	f := add(t, c, 1)

	_, err := c.RemoveFavourite(context.Background(), connect.NewRequest(
		&podcastsv1.RemoveFavouriteRequest{Id: f.Id},
	))
	require.NoError(t, err)
	assert.Empty(t, favourites(t, c))

	_, err = c.RemoveFavourite(context.Background(), connect.NewRequest(
		&podcastsv1.RemoveFavouriteRequest{Id: f.Id},
	))
	assert.Equal(t, connect.CodeNotFound, code(err))
}

func TestRemoveFavourite_InvalidID(t *testing.T) {
	c := newClient(t, userID, fakeITunes{})

	_, err := c.RemoveFavourite(context.Background(), connect.NewRequest(
		&podcastsv1.RemoveFavouriteRequest{Id: "nope"},
	))
	assert.Equal(t, connect.CodeInvalidArgument, code(err))
}

func TestFavourites_ArePerUser(t *testing.T) {
	mine := newClient(t, userID, fakeITunes{})
	other := newClient(t, otherUserID, fakeITunes{})

	f := add(t, mine, 1)
	assert.Empty(t, favourites(t, other))
	assert.False(t, search(t, other, "hardcore")[0].Favourite)

	// Another user's favourite reads as not found and survives.
	_, err := other.RemoveFavourite(context.Background(), connect.NewRequest(
		&podcastsv1.RemoveFavouriteRequest{Id: f.Id},
	))
	assert.Equal(t, connect.CodeNotFound, code(err))
	assert.Len(t, favourites(t, mine), 1)

	_, err = other.RemoveFavourite(context.Background(), connect.NewRequest(
		&podcastsv1.RemoveFavouriteRequest{Id: uuid.NewString()},
	))
	assert.Equal(t, connect.CodeNotFound, code(err))
}

func TestSearchShows_MarksFavourites(t *testing.T) {
	c := newClient(t, userID, fakeITunes{})
	add(t, c, 1)

	got := search(t, c, "history")
	require.Len(t, got, 2)
	byID := map[int64]*podcastsv1.SearchResult{}
	for _, r := range got {
		byID[r.ItunesId] = r
	}
	assert.True(t, byID[1].Favourite)
	assert.False(t, byID[2].Favourite)
	assert.Equal(t, "Dan Carlin", byID[1].Author)
	assert.Equal(t, "https://img.example/1.jpg", byID[1].ArtworkUrl)
}

func TestSearchShows_ShortQueryFindsNothing(t *testing.T) {
	c := newClient(t, userID, downITunes{})
	// No iTunes call is made, so the down client is never hit.
	assert.Empty(t, search(t, c, " a "))
	assert.Empty(t, search(t, c, ""))
	// One multi-byte character is still one character.
	assert.Empty(t, search(t, c, "日"))
}

func TestSearchShows_UpstreamDown(t *testing.T) {
	c := newClient(t, userID, downITunes{})

	_, err := c.SearchShows(context.Background(), connect.NewRequest(
		&podcastsv1.SearchShowsRequest{Query: "history"},
	))
	assert.Equal(t, connect.CodeUnavailable, code(err))
}
