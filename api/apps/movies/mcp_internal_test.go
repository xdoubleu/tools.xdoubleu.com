package movies

import (
	"context"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	moviesv1 "tools.xdoubleu.com/gen/movies/v1"
	"tools.xdoubleu.com/internal/constants"
	"tools.xdoubleu.com/internal/logging"
	sharedmocks "tools.xdoubleu.com/internal/mocks"
	sharedmodels "tools.xdoubleu.com/internal/models"
	"tools.xdoubleu.com/internal/testhelper"
)

// TestMCPTools: the read tools go through the Connect handlers, scoped to the
// caller in context.
func TestMCPTools(t *testing.T) {
	cfg := testhelper.NewTestConfig()
	pg := testhelper.ConnectTestDB(cfg.DBDsn)
	const uid = "mcp-movies-user"

	a := NewWithTMDB(
		sharedmocks.NewMockedAuthService(uid), logging.NewNopLogger(), cfg, pg, nil,
	)
	require.NoError(t, a.ApplyMigrations(context.Background(), pg))
	//nolint:exhaustruct // a name is all the server needs
	a.RegisterMCPTools(mcp.NewServer(&mcp.Implementation{Name: "test"}, nil))
	h := &moviesConnectHandler{app: a}

	ctx := context.WithValue(
		context.Background(),
		constants.UserContextKey,
		//nolint:exhaustruct // only ID and AppAccess matter for the gate
		sharedmodels.User{ID: uid, AppAccess: []string{mcpAppName}},
	)

	msg, err := h.mcpListBacklog(ctx, mcpListBacklogArgs{
		Status: "want", MediaType: "", Sort: "title", NewSeason: false, Limit: 5,
		Offset: 0,
	})
	require.NoError(t, err)
	resp, ok := msg.(*moviesv1.ListBacklogResponse)
	require.True(t, ok)
	assert.Empty(t, resp.Entries)

	_, err = h.mcpSearchTitles(ctx, mcpSearchArgs{Query: "matrix"})
	require.ErrorContains(t, err, "TMDB is not configured")

	_, err = h.mcpGetTitle(ctx, mcpGetTitleArgs{ID: "not-a-uuid"})
	require.ErrorContains(t, err, "invalid id")
}
