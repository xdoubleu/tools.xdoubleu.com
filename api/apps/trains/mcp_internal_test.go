package trains

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	"tools.xdoubleu.com/apps/trains/internal/mocks"
	"tools.xdoubleu.com/internal/constants"
	"tools.xdoubleu.com/internal/database/postgres"
	"tools.xdoubleu.com/internal/logging"
	"tools.xdoubleu.com/internal/mcptools"
	sharedmocks "tools.xdoubleu.com/internal/mocks"
	sharedmodels "tools.xdoubleu.com/internal/models"
	"tools.xdoubleu.com/internal/testhelper"
)

// TestMCPTools exercises the tool wrappers against a real app instance that
// has imported the sample feed. The station tool is the one issue #1453
// exists for: without it, "which names does the database actually hold" was
// unanswerable outside a psql session.
func TestMCPTools(t *testing.T) {
	cfg := testhelper.NewTestConfig()
	cfg.BMCPartnerKey = "test-key"
	var pg postgres.DB = testhelper.ConnectTestDB(cfg.DBDsn)

	const mcpUserID = "mcp-trains-user"
	app := NewInner(
		sharedmocks.NewMockedAuthService(mcpUserID),
		logging.NewNopLogger(),
		cfg,
		pg,
		mocks.NewMockBMCClient(mocks.BuildFeedZip(mocks.SampleFeedFiles())),
	)

	ctx := context.WithValue(
		context.Background(),
		constants.UserContextKey,
		//nolint:exhaustruct // only ID and AppAccess matter for the gate
		sharedmodels.User{ID: mcpUserID, AppAccess: []string{mcpAppName}},
	)
	// Migrations are applied once for the whole test binary by TestMain in
	// app_test.go, which shares this package's compiled tests.
	require.NoError(t, app.Services.StaticImport.Import(ctx))
	// SearchJourneys no longer builds the router in-request (issue #1484).
	require.NoError(t, app.Services.Journey.RefreshOnly(ctx))

	h := &trainsConnectHandler{app: app}

	msg, err := h.mcpSearchStations(ctx, mcpSearchStationsArgs{Query: "brussel"})
	require.NoError(t, err)
	assert.Contains(t, mcpJSON(t, msg), "Brussel-Zuid",
		"the tool reports every language's name, not just the feed's primary one")

	msg, err = h.mcpGetFeedInfo(ctx, mcptools.NoArgs{})
	require.NoError(t, err)
	assert.Contains(t, mcpJSON(t, msg), "importedAt",
		"a stale import is only visible through imported_at")

	//nolint:exhaustruct // an unknown origin exercises the empty-result path
	_, err = h.mcpSearchJourneys(ctx, mcpSearchJourneysArgs{
		OriginStopID:      "gs:nmbssncb:S8814001",
		DestinationStopID: "gs:nmbssncb:S8892007",
	})
	require.NoError(t, err)
}

// TestMCPTools_RequireAppAccess proves the gate rejects a caller without
// trains access, the same way every other app's tools do.
func TestMCPTools_RequireAppAccess(t *testing.T) {
	ctx := context.WithValue(
		context.Background(),
		constants.UserContextKey,
		//nolint:exhaustruct // a user with no app access at all
		sharedmodels.User{ID: "no-access"},
	)
	require.Error(t, mcptools.RequireAppAccess(ctx, mcpAppName))
}

func mcpJSON(t *testing.T, msg proto.Message) string {
	t.Helper()
	data, err := protojson.Marshal(msg)
	require.NoError(t, err)
	return string(data)
}
