package movies_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/movies/pkg/tmdb"
	moviesv1 "tools.xdoubleu.com/gen/movies/v1"
)

// streamingMovedTMDB moves The Matrix to Disney Plus only.
type streamingMovedTMDB struct{ fakeTMDB }

func (f streamingMovedTMDB) GetMovie(
	ctx context.Context, id int64,
) (*tmdb.Title, error) {
	t, err := f.fakeTMDB.GetMovie(ctx, id)
	if t != nil && id == 603 {
		t.Providers = []tmdb.Provider{{
			ID: 337, Name: "Disney Plus", LogoPath: "/disney.jpg",
			OfferType: tmdb.OfferFlatrate, DisplayPriority: 3,
		}}
	}
	return t, err
}

func offerNames(resp *moviesv1.GetTitleResponse) map[string][]string {
	out := map[string][]string{}
	for _, o := range resp.Offers {
		for _, p := range o.Providers {
			out[o.OfferType] = append(out[o.OfferType], p.Name)
		}
	}
	return out
}

func TestGetTitle_ProvidersGroupedByOfferType(t *testing.T) {
	c := newClient(t, userID, fakeTMDB{})
	e := add(t, c, "movie", 603, "want")

	got := getTitle(t, c, e.Id)

	assert.Equal(t, map[string][]string{
		"flatrate": {"Netflix", "Prime Video"},
		"rent":     {"Apple TV"},
	}, offerNames(got), "ordered by display priority within an offer type")
	require.Len(t, got.Offers, 2)
	assert.Equal(t, "flatrate", got.Offers[0].OfferType, "streaming comes first")
	assert.Equal(t, "/netflix.jpg", got.Offers[0].Providers[0].LogoPath)
	assert.Equal(t, int64(8), got.Offers[0].Providers[0].Id)
	assert.Equal(t, "https://www.themoviedb.org/movie/603/watch?locale=BE", got.WatchLink)
}

func TestGetTitle_NoProviders(t *testing.T) {
	c := newClient(t, userID, fakeTMDB{})
	e := add(t, c, "movie", 129, "want")

	got := getTitle(t, c, e.Id)

	assert.Empty(t, got.Offers)
	assert.Empty(t, got.WatchLink)
}

func TestRefresh_ReplacesProvidersOfWantedTitles(t *testing.T) {
	c := newClient(t, userID, fakeTMDB{})
	e := add(t, c, "movie", 603, "want")
	age(t, 603, 21*time.Hour)

	refresh(t, streamingMovedTMDB{fakeTMDB{}})

	got := getTitle(t, c, e.Id)
	assert.Equal(t, map[string][]string{"flatrate": {"Disney Plus"}}, offerNames(got))
}

func TestRefresh_LeavesProvidersOfFinishedTitlesForThirtyDays(t *testing.T) {
	c := newClient(t, userID, fakeTMDB{})
	e := add(t, c, "movie", 603, "watched")
	age(t, 603, 2*24*time.Hour)

	refresh(t, streamingMovedTMDB{fakeTMDB{}})
	assert.Contains(t, offerNames(getTitle(t, c, e.Id)), "rent", "not yet due")

	age(t, 603, 31*24*time.Hour)
	refresh(t, streamingMovedTMDB{fakeTMDB{}})
	assert.Equal(t, map[string][]string{"flatrate": {"Disney Plus"}},
		offerNames(getTitle(t, c, e.Id)))
}
