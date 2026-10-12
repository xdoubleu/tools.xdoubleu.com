package games_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/consumption"
)

func TestListConsumption(t *testing.T) {
	ctx := context.Background()
	const uid = "games-consumption-test-user"
	_, err := testDB.Exec(ctx, "DELETE FROM games.steam_games WHERE user_id = $1", uid)
	require.NoError(t, err)

	played := time.Date(2026, 4, 1, 20, 0, 0, 0, time.UTC)
	_, err = testDB.Exec(ctx, `
		INSERT INTO games.steam_games
		    (id, user_id, name, playtime_forever, completion_rate, favourite, last_played)
		VALUES
		    (1, $1, 'Favourite', 600, '10.00', true, $2),
		    (2, $1, 'Completed', 300, '100.00', false, NULL),
		    (3, $1, 'Bounced', 30, '0.00', false, NULL),
		    (4, $1, 'Marathon', 5000, '50.00', false, NULL),
		    (5, $1, 'Middling', 200, '20.00', false, NULL),
		    (6, $1, 'Unplayed', 0, '0.00', false, NULL)`, uid, played)
	require.NoError(t, err)

	events, err := testApp.ListConsumption(ctx, uid)
	require.NoError(t, err)

	got := map[string]consumption.Event{}
	for _, e := range events {
		got[e.Title] = e
	}
	require.Len(t, got, 5)
	assert.Equal(t, consumption.StrongPositive, got["Favourite"].Signal)
	assert.True(t, played.Equal(got["Favourite"].At))
	assert.Equal(t, "1", got["Favourite"].ItemID)
	assert.Equal(t, consumption.DomainGames, got["Favourite"].Domain)
	assert.Equal(t, consumption.StrongPositive, got["Completed"].Signal)
	assert.True(t, got["Completed"].At.IsZero())
	assert.Equal(t, consumption.Negative, got["Bounced"].Signal)
	assert.Equal(t, consumption.Positive, got["Marathon"].Signal)
	assert.Equal(t, consumption.Neutral, got["Middling"].Signal)
}
