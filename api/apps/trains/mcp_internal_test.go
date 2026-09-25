package trains

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	"tools.xdoubleu.com/apps/trains/internal/mocks"
	"tools.xdoubleu.com/apps/trains/internal/services"
	"tools.xdoubleu.com/internal/constants"
	"tools.xdoubleu.com/internal/database/postgres"
	"tools.xdoubleu.com/internal/logging"
	"tools.xdoubleu.com/internal/mcptools"
	sharedmocks "tools.xdoubleu.com/internal/mocks"
	sharedmodels "tools.xdoubleu.com/internal/models"
	"tools.xdoubleu.com/internal/testhelper"
)

// TestMCPTools exercises the tool wrappers against an app with the sample
// feed imported.
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
	require.NoError(t, app.Services.StaticImport.Import(ctx))
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

	// No realtime poll: detail renders with every stop DelayUnknown, not an error.
	journeyID := services.EncodeJourneyID([]services.LegRef{{
		TripShortName: "IC1234",
		BoardStopID:   "gs:nmbssncb:S8814001",
		BoardTime:     time.Now().UTC().Add(time.Hour),
		AlightStopID:  "gs:nmbssncb:S8892007",
		AlightTime:    time.Now().UTC().Add(2 * time.Hour),
	}})
	msg, err = h.mcpGetJourneyDetail(ctx, mcpGetJourneyDetailArgs{JourneyID: journeyID})
	require.NoError(t, err)
	assert.Contains(t, mcpJSON(t, msg), journeyID)

	_, err = h.mcpGetJourneyDetail(
		ctx,
		mcpGetJourneyDetailArgs{JourneyID: "!!not-valid!!"},
	)
	require.Error(t, err)
}

// TestMCPTools_RequireAppAccess rejects a caller without trains access.
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
