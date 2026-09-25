package games_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSteamAchievementsGameUserIndexExists guards the (game_id, user_id)
// index: the primary key leads with name, so without it every game page
// load full-scans the table.
func TestSteamAchievementsGameUserIndexExists(t *testing.T) {
	ctx := context.Background()

	var exists bool
	err := testDB.QueryRow(
		ctx,
		`SELECT EXISTS (
			SELECT 1 FROM pg_indexes
			WHERE schemaname = 'games'
			  AND tablename = 'steam_achievements'
			  AND indexdef ILIKE '%(game_id, user_id)%'
		)`,
	).Scan(&exists)
	require.NoError(t, err)
	assert.True(
		t,
		exists,
		"games.steam_achievements should have an index covering (game_id, user_id) "+
			"— see api/apps/games/migrations/00006_achievements_game_user_index.sql",
	)
}

// TestGetAchievementsForGames_Repo: the lookup is scoped to (game_id,
// user_id).
func TestGetAchievementsForGames_Repo(t *testing.T) {
	seedSteamData(t)
	ctx := context.Background()

	achievements, err := testApp.Repositories.Steam.GetAchievementsForGames(
		ctx,
		[]int{1},
		userID,
	)
	require.NoError(t, err)
	require.NotEmpty(
		t,
		achievements[1],
		"seeded game 1 should have achievements for its owning user",
	)
	for _, achievement := range achievements[1] {
		assert.Equal(t, 1, achievement.GameID)
	}

	assert.Empty(t, achievements[9999])
}
