package trains

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/trains/internal/mocks"
	"tools.xdoubleu.com/apps/trains/internal/models"
	"tools.xdoubleu.com/apps/trains/internal/services"
	"tools.xdoubleu.com/internal/logging"
	sharedmocks "tools.xdoubleu.com/internal/mocks"
	"tools.xdoubleu.com/internal/testhelper"
)

// TestTrains_warmRouter covers the startup warm-up: after it runs the router
// answers queries instead of reporting ErrRouterWarmingUp (issue #1484).
// Migrations are applied once for the whole test binary by app_test.go's
// TestMain, which this internal test shares.
func TestTrains_warmRouter(t *testing.T) {
	ctx := context.Background()
	cfg := testhelper.NewTestConfig()
	cfg.BMCPartnerKey = "test-key"
	pool := testhelper.ConnectTestDB(cfg.DBDsn)

	a := NewInner(
		sharedmocks.NewMockedAuthService("warm-router-user"),
		logging.NewNopLogger(),
		cfg,
		pool,
		mocks.NewMockBMCClient(mocks.BuildFeedZip(mocks.SampleFeedFiles())),
	)

	windowStart := time.Now().UTC().Truncate(24 * time.Hour)
	//nolint:exhaustruct //a one-stop-pair feed is enough to build an index
	require.NoError(t, a.Repositories.Feed.ImportFeed(ctx, &models.Feed{
		Stops: []models.Stop{
			{StopID: "WA", NameFR: "West", LocationType: 1},
			{StopID: "WA1", NameFR: "West", ParentStation: "WA", PlatformCode: "1"},
			{StopID: "WB", NameFR: "East", LocationType: 1},
			{StopID: "WB1", NameFR: "East", ParentStation: "WB", PlatformCode: "1"},
		},
		Routes: []models.Route{
			{RouteID: "wr", ShortName: "IC", LongName: "", RouteType: 2},
		},
		Trips: []models.Trip{{
			TripID: "wt", RouteID: "wr", ServiceID: "wsvc", ShortName: "IC1",
			Headsign: "East", DirectionID: nil,
		}},
		StopTimes: []models.StopTime{
			{
				TripID: "wt", StopSequence: 1, StopID: "WA1",
				ArrivalSeconds: 28800, DepartureSeconds: 28800,
				PickupType: 0, DropOffType: 0,
			},
			{
				TripID: "wt", StopSequence: 2, StopID: "WB1",
				ArrivalSeconds: 30000, DepartureSeconds: 30000,
				PickupType: 0, DropOffType: 0,
			},
		},
		CalendarDates: []models.CalendarDate{
			{ServiceID: "wsvc", Date: windowStart, ExceptionType: 1},
		},
	}))

	_, err := a.Services.Journey.SearchJourneys(ctx, "WA", "WB", windowStart, false)
	require.ErrorIs(t, err, services.ErrRouterWarmingUp)

	a.warmRouter()

	_, err = a.Services.Journey.SearchJourneys(
		ctx, "WA", "WB", windowStart.Add(7*time.Hour), false,
	)
	assert.NoError(t, err)
}
