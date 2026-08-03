package feeds

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/xdoubleu/essentia/v4/pkg/database/postgres"
	"github.com/xdoubleu/essentia/v4/pkg/logging"

	"tools.xdoubleu.com/apps/feeds/internal/mocks"
	"tools.xdoubleu.com/internal/constants"
	"tools.xdoubleu.com/internal/mcptools"
	sharedmocks "tools.xdoubleu.com/internal/mocks"
	sharedmodels "tools.xdoubleu.com/internal/models"
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

	_, err = h.mcpListItems(ctx, mcptools.NoArgs{})
	require.NoError(t, err)
}
