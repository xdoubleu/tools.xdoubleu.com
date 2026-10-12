package movies_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/consumption"
)

func TestListConsumption(t *testing.T) {
	const uid = "movies-consumption-test-user"
	c := newClient(t, uid, fakeTMDB{})
	matrix := add(t, c, "movie", 603, "watched")
	_, err := rate(c, matrix.Id, stars(5))
	require.NoError(t, err)
	add(t, c, "movie", 129, "want")
	dark := add(t, c, "series", 70523, "want")
	_, err = testDB.Exec(context.Background(),
		"UPDATE movies.user_titles SET status = 'dropped' WHERE id = $1", dark.Id)
	require.NoError(t, err)

	events, err := newApp(uid, fakeTMDB{}).ListConsumption(context.Background(), uid)
	require.NoError(t, err)
	require.Len(t, events, 2)

	got := map[string]consumption.Event{}
	for _, e := range events {
		got[e.Title] = e
	}
	m := got["The Matrix"]
	assert.Equal(t, consumption.DomainMovies, m.Domain)
	assert.Equal(t, "movie:603", m.ItemID)
	assert.Equal(t, []string{"Action", "Science Fiction"}, m.Tags)
	assert.Equal(t, consumption.StrongPositive, m.Signal)
	assert.Equal(t, "rated 5★", m.Reason)
	assert.False(t, m.At.IsZero())

	d := got["Dark"]
	assert.Equal(t, consumption.DomainSeries, d.Domain)
	assert.Equal(t, consumption.Negative, d.Signal)
	assert.Equal(t, "dropped", d.Reason)
}
