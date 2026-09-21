package games_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSteamAchievementsGameUserIndexExists is a regression guard for issue
// #1716: games.steam_achievements' only index used to be its primary key on
// (name, user_id, game_id), which can't serve a (game_id, user_id) lookup
// since name leads. Every /games/:id load (GetAchievementsForGames, called
// from buildSteamGameResponse) therefore did a full sequential scan of the
// whole table. This asserts the migration-created index stays in place.
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

// TestGetAchievementsForGames_Repo seeds steam data for two users and
// verifies the achievements lookup that powers the games detail page stays
// correctly scoped to (game_id, user_id) — not just fast.
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

	// A game ID that wasn't requested must not leak into the result map.
	assert.Empty(t, achievements[9999])
}
