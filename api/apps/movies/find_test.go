package movies_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
