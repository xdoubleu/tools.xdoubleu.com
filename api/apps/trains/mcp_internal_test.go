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

// TestMCPTools exercises the tool wrappers over the recorded iRail responses.
func TestMCPTools(t *testing.T) {
	cfg := testhelper.NewTestConfig()
	var pg postgres.DB = testhelper.ConnectTestDB(cfg.DBDsn)

	const mcpUserID = "mcp-trains-user"
	app := NewInner(
		context.Background(),
		sharedmocks.NewMockedAuthService(mcpUserID),
		logging.NewNopLogger(), cfg, pg, mocks.NewFixtureClient(t),
	)

	ctx := context.WithValue(
		context.Background(),
		constants.UserContextKey,
		//nolint:exhaustruct // only ID and AppAccess matter for the gate
		sharedmodels.User{ID: mcpUserID, AppAccess: []string{mcpAppName}},
	)
	h := &trainsConnectHandler{app: app}

	msg, err := h.mcpSearchStations(ctx, mcpSearchStationsArgs{Query: "brussel-noord"})
	require.NoError(t, err)
	assert.Contains(t, mcpJSON(t, msg), "Bruxelles-Nord",
		"the tool reports every language's name")

	//nolint:exhaustruct // time defaults to now
	msg, err = h.mcpSearchJourneys(ctx, mcpSearchJourneysArgs{
		OriginStopID:      "008812005",
		DestinationStopID: "008841004",
	})
	require.NoError(t, err)
	assert.Contains(t, mcpJSON(t, msg), "journeyId")

	t0 := time.Now().UTC()
	journeyID := services.EncodeJourneyID([]services.LegRef{{
		VehicleID: "BE.NMBS.IC530", BoardStopID: "008841004", BoardTime: t0,
		AlightStopID: "008812005", AlightTime: t0.Add(time.Hour),
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

func TestMCPIrailDiagnostic(t *testing.T) {
	cfg := testhelper.NewTestConfig()
	app := NewInner(
		context.Background(),
		sharedmocks.NewMockedAuthService("diag"), logging.NewNopLogger(), cfg,
		testhelper.ConnectTestDB(cfg.DBDsn), mocks.NewFixtureClient(t),
	)

	//nolint:exhaustruct //no vehicle: summary only
	msg, err := app.mcpIrailDiagnostic(context.Background(), mcpIrailDiagnosticArgs{})
	require.NoError(t, err)
	out := mcpJSON(t, msg)
	assert.Contains(t, out, "rate_limited")
	assert.NotContains(t, out, "vehicle")

	msg, err = app.mcpIrailDiagnostic(context.Background(), mcpIrailDiagnosticArgs{
		VehicleID: "IC530", Date: "2026-10-23",
	})
	require.NoError(t, err)
	out = mcpJSON(t, msg)
	assert.Contains(t, out, "BE.NMBS.IC530")
	assert.Contains(t, out, "Welkenraedt", "the raw iRail response is included")

	msg, err = app.mcpIrailDiagnostic(context.Background(), mcpIrailDiagnosticArgs{
		VehicleID: "IC1", Date: "2026-10-23",
	})
	require.NoError(t, err)
	assert.Contains(t, mcpJSON(t, msg), "not found")

	_, err = app.mcpIrailDiagnostic(context.Background(), mcpIrailDiagnosticArgs{
		VehicleID: "IC530", Date: "yesterday",
	})
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
