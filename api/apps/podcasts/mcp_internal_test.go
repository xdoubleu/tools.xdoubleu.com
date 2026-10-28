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
		context.Background(),
		sharedmocks.NewMockedAuthService(uid), logging.NewNopLogger(), cfg, pg,
		nil,
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
	_, err := pg.Exec(ctx, `
		INSERT INTO podcasts.shows (user_id, itunes_id, title)
		VALUES ($1, 42, 'Mcp Show')`, uid,
	)
	require.NoError(t, err)

	msg, err := h.mcpListFavourites(userCtx(uid), mcptools.NoArgs{})
	require.NoError(t, err)
	favs, ok := msg.(*podcastsv1.ListFavouritesResponse)
	require.True(t, ok)
	require.Len(t, favs.Favourites, 1)
	assert.Equal(t, "Mcp Show", favs.Favourites[0].Title)

	// Another user sees none of it.
	msg, err = h.mcpListFavourites(userCtx(other), mcptools.NoArgs{})
	require.NoError(t, err)
	favs, ok = msg.(*podcastsv1.ListFavouritesResponse)
	require.True(t, ok)
	assert.Empty(t, favs.Favourites)
}
