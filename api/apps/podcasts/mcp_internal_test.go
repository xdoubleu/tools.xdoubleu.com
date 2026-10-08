package podcasts

import (
	"context"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	podcastsv1 "tools.xdoubleu.com/gen/podcasts/v1"
	"tools.xdoubleu.com/internal/constants"
	"tools.xdoubleu.com/internal/logging"
	"tools.xdoubleu.com/internal/mcptools"
	sharedmocks "tools.xdoubleu.com/internal/mocks"
	sharedmodels "tools.xdoubleu.com/internal/models"
	"tools.xdoubleu.com/internal/testhelper"
)

func userCtx(uid string) context.Context {
	return context.WithValue(
		context.Background(),
		constants.UserContextKey,
		//nolint:exhaustruct // only ID and AppAccess matter for the gate
		sharedmodels.User{ID: uid, AppAccess: []string{mcpAppName}},
	)
}

// TestMCPTools: the read tools go through the Connect handlers, scoped to the
// caller in context.
func TestMCPTools(t *testing.T) {
	cfg := testhelper.NewTestConfig()
	pg := testhelper.ConnectTestDB(cfg.DBDsn)
	const uid, other = "mcp-podcasts-user", "mcp-podcasts-other"

	a := NewInner(
		sharedmocks.NewMockedAuthService(uid), logging.NewNopLogger(), cfg, pg,
		nil, nil,
	)
	ctx := context.Background()
	require.NoError(t, a.ApplyMigrations(ctx, pg))
	//nolint:exhaustruct // a name is all the server needs
	a.RegisterMCPTools(mcp.NewServer(&mcp.Implementation{Name: "test"}, nil))
	h := &podcastsConnectHandler{app: a}

	clean := func() {
		_, err := pg.Exec(ctx,
			"DELETE FROM podcasts.shows WHERE user_id IN ($1, $2)", uid, other)
		require.NoError(t, err)
	}
	clean()
	t.Cleanup(clean)
	var showID string
	require.NoError(t, pg.QueryRow(ctx, `
		INSERT INTO podcasts.shows (user_id, itunes_id, title, feed_url)
		VALUES ($1, 42, 'Mcp Show', 'https://feeds.example/42') RETURNING id`, uid,
	).Scan(&showID))
	_, err := pg.Exec(ctx, `
		INSERT INTO podcasts.episodes (show_id, guid, title, published_at)
		VALUES ($1, 'a', 'Older', '2026-01-01'), ($1, 'b', 'Newer', '2026-02-01')`, showID)
	require.NoError(t, err)

	msg, err := h.mcpListFavourites(userCtx(uid), mcptools.NoArgs{})
	require.NoError(t, err)
	favs, ok := msg.(*podcastsv1.ListFavouritesResponse)
	require.True(t, ok)
	require.Len(t, favs.Favourites, 1)
	assert.Equal(t, "Mcp Show", favs.Favourites[0].Title)

	msg, err = h.mcpListEpisodes(userCtx(uid), mcpListEpisodesArgs{
		ShowID: showID, Limit: 1, Offset: 0,
	})
	require.NoError(t, err)
	eps, ok := msg.(*podcastsv1.ListEpisodesResponse)
	require.True(t, ok)
	require.Len(t, eps.Episodes, 1)
	assert.Equal(t, "Newer", eps.Episodes[0].Title)
	assert.True(t, eps.HasMore)

	msg, err = h.mcpListEpisodes(userCtx(uid), mcpListEpisodesArgs{
		ShowID: "", Limit: 1, Offset: 1,
	})
	require.NoError(t, err)
	eps, ok = msg.(*podcastsv1.ListEpisodesResponse)
	require.True(t, ok)
	require.Len(t, eps.Episodes, 1)
	assert.Equal(t, "Older", eps.Episodes[0].Title)
	assert.False(t, eps.HasMore)

	// Another user sees none of it.
	msg, err = h.mcpListFavourites(userCtx(other), mcptools.NoArgs{})
	require.NoError(t, err)
	favs, ok = msg.(*podcastsv1.ListFavouritesResponse)
	require.True(t, ok)
	assert.Empty(t, favs.Favourites)
	msg, err = h.mcpListEpisodes(userCtx(other), mcpListEpisodesArgs{
		ShowID: showID, Limit: 0, Offset: 0,
	})
	require.NoError(t, err)
	eps, ok = msg.(*podcastsv1.ListEpisodesResponse)
	require.True(t, ok)
	assert.Empty(t, eps.Episodes)

	_, err = h.mcpListEpisodes(userCtx(uid), mcpListEpisodesArgs{
		ShowID: "nope", Limit: 0, Offset: 0,
	})
	require.ErrorContains(t, err, "invalid show id")
}
