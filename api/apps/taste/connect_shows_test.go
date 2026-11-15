package taste_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"connectrpc.com/connect/v2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	tastev1 "tools.xdoubleu.com/gen/taste/v1"
	"tools.xdoubleu.com/gen/taste/v1/tastev1connect"
)

func showCode(err error) connect.Code {
	var ce *connect.Error
	if errors.As(err, &ce) {
		return ce.Code()
	}
	return connect.CodeUnknown
}

func addShow(
	t *testing.T,
	c tastev1connect.TasteServiceClient,
	id int64,
) *tastev1.FavouriteShow {
	t.Helper()
	resp, err := c.AddFavouriteShow(
		context.Background(),
		&tastev1.AddFavouriteShowRequest{ItunesId: id},
	)
	require.NoError(t, err)
	return resp.Favourite
}

func favouriteShows(
	t *testing.T,
	c tastev1connect.TasteServiceClient,
) []*tastev1.FavouriteShow {
	t.Helper()
	resp, err := c.ListFavouriteShows(
		context.Background(),
		&tastev1.ListFavouriteShowsRequest{},
	)
	require.NoError(t, err)
	return resp.Favourites
}

func searchShows(
	t *testing.T,
	c tastev1connect.TasteServiceClient,
	query string,
) []*tastev1.SearchResult {
	t.Helper()
	resp, err := c.SearchShows(
		context.Background(),
		&tastev1.SearchShowsRequest{Query: query},
	)
	require.NoError(t, err)
	return resp.Results
}

func TestAddFavourite(t *testing.T) {
	c := newShowsClient(t, showsUserID, fakeITunes{})

	f := addShow(t, c, 1)
	assert.Equal(t, "Hardcore History", f.Title)
	assert.Equal(t, "Dan Carlin", f.Author)
	assert.Equal(t, int64(1), f.ItunesId)
	assert.Equal(t, "https://img.example/1.jpg", f.ArtworkUrl)
	assert.Equal(t, "https://podcasts.apple.com/podcast/id1", f.AppleUrl)
	added, err := time.Parse(time.RFC3339, f.AddedAt)
	require.NoError(t, err)
	assert.WithinDuration(t, time.Now(), added, time.Minute)

	assert.Equal(t, []string{f.Id}, []string{favouriteShows(t, c)[0].Id})
}

func TestAddFavourite_IsIdempotent(t *testing.T) {
	c := newShowsClient(t, showsUserID, fakeITunes{})

	first := addShow(t, c, 1)
	again := addShow(t, c, 1)
	assert.Equal(t, first.Id, again.Id)
	assert.Equal(t, first.AddedAt, again.AddedAt)
	assert.Len(t, favouriteShows(t, c), 1)
}

func TestAddFavourite_UnknownShow(t *testing.T) {
	c := newShowsClient(t, showsUserID, fakeITunes{})

	_, err := c.AddFavouriteShow(
		context.Background(),
		&tastev1.AddFavouriteShowRequest{ItunesId: 999},
	)
	assert.Equal(t, connect.CodeNotFound, showCode(err))
}

func TestAddFavourite_InvalidID(t *testing.T) {
	c := newShowsClient(t, showsUserID, fakeITunes{})

	_, err := c.AddFavouriteShow(
		context.Background(),
		&tastev1.AddFavouriteShowRequest{ItunesId: 0},
	)
	assert.Equal(t, connect.CodeInvalidArgument, showCode(err))
}

func TestAddFavourite_UpstreamDown(t *testing.T) {
	c := newShowsClient(t, showsUserID, downITunes{})

	_, err := c.AddFavouriteShow(
		context.Background(),
		&tastev1.AddFavouriteShowRequest{ItunesId: 1},
	)
	assert.Equal(t, connect.CodeUnavailable, showCode(err))
}

func TestListFavourites_NewestFirst(t *testing.T) {
	c := newShowsClient(t, showsUserID, fakeITunes{})

	addShow(t, c, 1)
	addShow(t, c, 2)

	got := favouriteShows(t, c)
	require.Len(t, got, 2)
	assert.Equal(t, "History of Rome", got[0].Title)
	assert.Equal(t, "Hardcore History", got[1].Title)
}

func TestListFavourites_Empty(t *testing.T) {
	c := newShowsClient(t, showsUserID, fakeITunes{})
	assert.Empty(t, favouriteShows(t, c))
}

func TestRemoveFavourite(t *testing.T) {
	c := newShowsClient(t, showsUserID, fakeITunes{})
	f := addShow(t, c, 1)

	_, err := c.RemoveFavouriteShow(
		context.Background(),
		&tastev1.RemoveFavouriteShowRequest{Id: f.Id},
	)
	require.NoError(t, err)
	assert.Empty(t, favouriteShows(t, c))

	_, err = c.RemoveFavouriteShow(
		context.Background(),
		&tastev1.RemoveFavouriteShowRequest{Id: f.Id},
	)
	assert.Equal(t, connect.CodeNotFound, showCode(err))
}

func TestRemoveFavourite_InvalidID(t *testing.T) {
	c := newShowsClient(t, showsUserID, fakeITunes{})

	_, err := c.RemoveFavouriteShow(
		context.Background(),
		&tastev1.RemoveFavouriteShowRequest{Id: "nope"},
	)
	assert.Equal(t, connect.CodeInvalidArgument, showCode(err))
}

func TestFavourites_ArePerUser(t *testing.T) {
	mine := newShowsClient(t, showsUserID, fakeITunes{})
	other := newShowsClient(t, showsOtherUserID, fakeITunes{})

	f := addShow(t, mine, 1)
	assert.Empty(t, favouriteShows(t, other))
	assert.False(t, searchShows(t, other, "hardcore")[0].Favourite)

	// Another user's favourite reads as not found and survives.
	_, err := other.RemoveFavouriteShow(
		context.Background(),
		&tastev1.RemoveFavouriteShowRequest{Id: f.Id},
	)
	assert.Equal(t, connect.CodeNotFound, showCode(err))
	assert.Len(t, favouriteShows(t, mine), 1)

	_, err = other.RemoveFavouriteShow(
		context.Background(),
		&tastev1.RemoveFavouriteShowRequest{Id: uuid.NewString()},
	)
	assert.Equal(t, connect.CodeNotFound, showCode(err))
}

func TestSearchShows_MarksFavourites(t *testing.T) {
	c := newShowsClient(t, showsUserID, fakeITunes{})
	addShow(t, c, 1)

	got := searchShows(t, c, "history")
	require.Len(t, got, 2)
	byID := map[int64]*tastev1.SearchResult{}
	for _, r := range got {
		byID[r.ItunesId] = r
	}
	assert.True(t, byID[1].Favourite)
	assert.False(t, byID[2].Favourite)
	assert.Equal(t, "Dan Carlin", byID[1].Author)
	assert.Equal(t, "https://img.example/1.jpg", byID[1].ArtworkUrl)
}

func TestSearchShows_ShortQueryFindsNothing(t *testing.T) {
	c := newShowsClient(t, showsUserID, downITunes{})
	// No iTunes call is made, so the down client is never hit.
	assert.Empty(t, searchShows(t, c, " a "))
	assert.Empty(t, searchShows(t, c, ""))
	// One multi-byte character is still one character.
	assert.Empty(t, searchShows(t, c, "日"))
}

func TestSearchShows_UpstreamDown(t *testing.T) {
	c := newShowsClient(t, showsUserID, downITunes{})

	_, err := c.SearchShows(
		context.Background(),
		&tastev1.SearchShowsRequest{Query: "history"},
	)
	assert.Equal(t, connect.CodeUnavailable, showCode(err))
}
