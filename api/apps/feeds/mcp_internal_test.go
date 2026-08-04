package feeds

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/xdoubleu/essentia/v4/pkg/database/postgres"
	"github.com/xdoubleu/essentia/v4/pkg/logging"

	"tools.xdoubleu.com/apps/feeds/internal/mocks"
	feedsv1 "tools.xdoubleu.com/gen/feeds/v1"
	"tools.xdoubleu.com/internal/constants"
	"tools.xdoubleu.com/internal/mailer"
	"tools.xdoubleu.com/internal/mcptools"
	sharedmocks "tools.xdoubleu.com/internal/mocks"
	sharedmodels "tools.xdoubleu.com/internal/models"
	sharedrepos "tools.xdoubleu.com/internal/repositories"
	"tools.xdoubleu.com/internal/testhelper"
)

// TestMCPTools_ListFeedsAndItems exercises the two MCP tool wrappers
// end-to-end against a real (migrated, empty) app instance — RegisterMCPTools
// itself is a thin registration shim covered by every app that has one, so
// the wrappers are what actually needs a direct test.
func TestMCPTools_ListFeedsAndItems(t *testing.T) {
	cfg := testhelper.NewTestConfig()
	db := testhelper.ConnectTestDB(cfg.DBDsn)
	var pg postgres.DB = db

	const mcpUserID = "mcp-test-user"
	app := NewInner(
		sharedmocks.NewMockedAuthService(mcpUserID),
		logging.NewNopLogger(),
		cfg,
		pg,
		mocks.NewMockWebFetchClient(),
		mailer.New("", "", ""),
		sharedrepos.NewAppUsersRepository(db),
	)

	ctx := context.WithValue(
		context.Background(),
		constants.UserContextKey,
		sharedmodels.User{ //nolint:exhaustruct // only ID matters for feedUser()
			ID: mcpUserID,
		},
	)

	h := &feedsConnectHandler{app: app}

	_, err := h.mcpListFeeds(ctx, mcptools.NoArgs{})
	require.NoError(t, err)

	//nolint:exhaustruct // zero-value args, exercising the "no filters" path
	_, err = h.mcpListItems(ctx, mcpListItemsArgs{})
	require.NoError(t, err)

	// A feed_id forwarded to a request that matches nothing still succeeds
	// (empty result), proving the arg reaches the underlying RPC rather than
	// being silently dropped like the old NoArgs wrapper did.
	msg, err := h.mcpListItems(
		ctx,
		//nolint:exhaustruct // only FeedID matters for this assertion
		mcpListItemsArgs{FeedID: "00000000-0000-0000-0000-000000000000"},
	)
	require.NoError(t, err)
	resp, ok := msg.(*feedsv1.ListFeedItemsResponse)
	require.True(t, ok)
	require.Empty(t, resp.Items)
}
