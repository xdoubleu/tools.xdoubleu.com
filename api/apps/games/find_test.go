package games_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/games/pkg/steam"
)

type fakeStore struct {
	items []steam.StoreItem
	err   error
}

func (f fakeStore) SearchStore(context.Context, string) ([]steam.StoreItem, error) {
	return f.items, f.err
}

func TestFindGame(t *testing.T) {
	ctx := context.Background()
	const uid = "games-find-test-user"
	_, err := testDB.Exec(ctx, "DELETE FROM games.steam_games WHERE user_id = $1", uid)
	require.NoError(t, err)

	old := testApp.Store
	t.Cleanup(func() { testApp.Store = old })
	testApp.Store = fakeStore{items: []steam.StoreItem{
		{ID: 1, Name: "Outer Wilds - Echoes of the Eye", Type: "app", ImageURL: ""},
		{ID: 753640, Name: "Outer Wilds", Type: "app", ImageURL: "https://img/ow.jpg"},
	}, err: nil}

	m, err := testApp.FindGame(ctx, uid, "Outer Wilds")
	require.NoError(t, err)
	require.NotNil(t, m)
	assert.Equal(t, "753640", m.Ref)
	assert.Equal(t, "https://store.steampowered.com/app/753640", m.URL)
	assert.Equal(t, "https://img/ow.jpg", m.ImageURL)
	assert.False(t, m.Known)

	_, err = testDB.Exec(ctx, `INSERT INTO games.steam_games (id, user_id, name)
		VALUES (753640, $1, 'Outer Wilds')`, uid)
	require.NoError(t, err)
	m, err = testApp.FindGame(ctx, uid, "Outer Wilds")
	require.NoError(t, err)
	assert.True(t, m.Known)

	m, err = testApp.FindGame(ctx, uid, "Outer Wilds - Echoes of the Eye")
	require.NoError(t, err)
	require.NotNil(t, m, "the first hit")
	assert.Equal(t, "1", m.Ref)

	m, err = testApp.FindGame(ctx, uid, "Hades")
	require.NoError(t, err)
	assert.Nil(t, m)

	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	_, err = testApp.FindGame(cancelled, uid, "Outer Wilds")
	require.Error(t, err)

	testApp.Store = fakeStore{items: nil, err: errors.New("store down")}
	_, err = testApp.FindGame(ctx, uid, "Outer Wilds")
	require.ErrorContains(t, err, "store down")
}
