package movies_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/movies"
	"tools.xdoubleu.com/apps/movies/pkg/tmdb"
	moviesv1 "tools.xdoubleu.com/gen/movies/v1"
	sharedmocks "tools.xdoubleu.com/internal/mocks"
)

func TestFindTitle(t *testing.T) {
	const uid = "movies-find-test-user"
	c := newClient(t, uid, fakeTMDB{})
	a := newApp(uid, fakeTMDB{})
	ctx := context.Background()

	m, err := a.FindTitle(ctx, uid, "movie", "Matrix", 1999)
	require.NoError(t, err)
	require.NotNil(t, m)
	assert.Equal(t, "movie:603", m.Ref)
	assert.Equal(t, "The Matrix", m.Title)
	assert.Equal(t, 1999, m.Year)
	assert.Equal(t, "https://image.tmdb.org/t/p/w185/matrix.jpg", m.ImageURL)
	assert.Equal(t, "https://www.themoviedb.org/movie/603", m.URL)
	assert.False(t, m.Known)
	require.NotNil(t, m.Audience)
	assert.Equal(t, 25000, *m.Audience)

	// A series.
	m, err = a.FindTitle(ctx, uid, "series", "Dark", 0)
	require.NoError(t, err)
	require.NotNil(t, m)
	assert.Equal(t, "https://www.themoviedb.org/tv/70523", m.URL)
	assert.Empty(t, m.ImageURL)

	add(t, c, "movie", 129, "want")
	m, err = a.FindTitle(ctx, uid, "movie", "Spirited Away", 2001)
	require.NoError(t, err)
	require.NotNil(t, m)
	assert.Equal(t, "Spirited Away", m.Title)
	assert.True(t, m.Known)

	m, err = a.FindTitle(ctx, uid, "series", "The Matrix", 0)
	require.NoError(t, err)
	assert.Nil(t, m, "media type must match")

	m, err = a.FindTitle(ctx, uid, "movie", "Avatar 5", 0)
	require.NoError(t, err)
	require.NotNil(t, m)
	assert.Zero(t, m.Year)

	_, err = newApp(uid, nil).FindTitle(ctx, uid, "movie", "Matrix", 0)
	assert.Error(t, err)
}

// noDetailsTMDB searches but fails every detail lookup.
type noDetailsTMDB struct{ fakeTMDB }

func (noDetailsTMDB) GetMovie(context.Context, int64) (*tmdb.Title, error) {
	return nil, errors.New("tmdb down")
}

func TestFindTitle_OnMyServices(t *testing.T) {
	const uid = "movies-services-test-user"
	c := newClient(t, uid, fakeTMDB{})
	a := newApp(uid, fakeTMDB{})
	ctx := context.Background()
	onServices := func(app *movies.Movies, mediaType, title string) bool {
		t.Helper()
		m, err := app.FindTitle(ctx, uid, mediaType, title, 0)
		require.NoError(t, err)
		require.NotNil(t, m)
		return m.OnMyServices
	}

	var logs bytes.Buffer
	logged := func(client tmdb.Client) *movies.Movies {
		return movies.NewWithTMDB(sharedmocks.NewMockedAuthService(uid),
			slog.New(slog.NewTextHandler(&logs, nil)), testCfg, testDB, client)
	}

	setServices(t, c)
	assert.False(t, onServices(a, "movie", "Matrix"), "no services")
	setServices(t, c, 119)
	assert.True(t, onServices(logged(fakeTMDB{}), "movie", "Matrix"), "streams on Prime")
	assert.NotContains(t, logs.String(), "lookup failed")
	assert.False(t, onServices(logged(noDetailsTMDB{fakeTMDB{}}), "movie", "Matrix"),
		"a failed lookup reads as not")
	assert.Contains(t, logs.String(), "streaming providers lookup failed")
	setServices(t, c, 350)
	assert.False(t, onServices(a, "movie", "Matrix"), "rent only")
	setServices(t, c, 99)
	assert.True(t, onServices(a, "series", "Dark"), "free with ads")
}

func TestWantTitle(t *testing.T) {
	const uid = "movies-want-test-user"
	c := newClient(t, uid, fakeTMDB{})
	a := newApp(uid, fakeTMDB{})
	ctx := context.Background()

	require.NoError(t, a.WantTitle(ctx, uid, "movie:129"))
	add(t, c, "movie", 603, "watched")
	require.NoError(t, a.WantTitle(ctx, uid, "movie:603"))
	resp, err := c.ListBacklog(ctx, &moviesv1.ListBacklogRequest{})
	require.NoError(t, err)
	statuses := map[string]string{}
	for _, e := range resp.Entries {
		statuses[e.Title] = e.Status
	}
	assert.Equal(t, map[string]string{
		"Spirited Away": "want", "The Matrix": "watched",
	}, statuses, "an entry already there keeps its status")

	assert.Error(t, a.WantTitle(ctx, uid, "movie:abc"))
	assert.Error(t, a.WantTitle(ctx, uid, "movie:5"), "not on TMDB")
}
