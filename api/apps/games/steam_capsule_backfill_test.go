package games_test

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/games/pkg/steam"
)

const capsuleBackfillUser = "capsule-backfill-user"

// TestCapsuleBackfillMigration: stored Steam icon and header URLs become the
// capsule; anything else is left alone.
func TestCapsuleBackfillMigration(t *testing.T) {
	ctx := t.Context()
	clean := func() {
		// t.Context is already cancelled when cleanups run.
		bg := context.Background()
		_, _ = testDB.Exec(bg, "DELETE FROM games.steam_games WHERE user_id = $1",
			capsuleBackfillUser)
		_, _ = testDB.Exec(bg, "DELETE FROM games.steam_wishlist WHERE user_id = $1",
			capsuleBackfillUser)
	}
	clean()
	t.Cleanup(clean)

	const custom = "https://example.com/own.jpg"
	for id, url := range map[int]string{
		9001: "http://media.steampowered.com/steamcommunity/public/images/a.jpg",
		9002: "https://cdn.cloudflare.steamstatic.com/steam/apps/9002/header.jpg",
		9003: custom,
	} {
		_, err := testDB.Exec(ctx,
			`INSERT INTO games.steam_games (id, user_id, name, image_url)
			 VALUES ($1, $2, 'g', $3)`, id, capsuleBackfillUser, url)
		require.NoError(t, err)
		_, err = testDB.Exec(ctx,
			`INSERT INTO games.steam_wishlist (game_id, user_id, name, image_url)
			 VALUES ($1, $2, 'g', $3)`, id, capsuleBackfillUser, url)
		require.NoError(t, err)
	}

	sqlBytes, err := os.ReadFile("migrations/00008_capsule_images.sql")
	require.NoError(t, err)
	up, _, _ := strings.Cut(string(sqlBytes), "-- +goose Down")
	_, err = testDB.Exec(ctx, up)
	require.NoError(t, err)

	want := map[int]string{
		9001: steam.CapsuleURL(9001),
		9002: steam.CapsuleURL(9002),
		9003: custom,
	}
	for _, q := range []string{
		"SELECT image_url FROM games.steam_games WHERE user_id = $1 AND id = $2",
		"SELECT image_url FROM games.steam_wishlist WHERE user_id = $1 AND game_id = $2",
	} {
		for id, url := range want {
			var got string
			require.NoError(t, testDB.QueryRow(ctx, q, capsuleBackfillUser, id).Scan(&got))
			assert.Equal(t, url, got)
		}
	}
}
