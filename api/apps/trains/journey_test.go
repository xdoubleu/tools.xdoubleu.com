package trains_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/trains/internal/jobs"
	"tools.xdoubleu.com/apps/trains/internal/models"
	"tools.xdoubleu.com/apps/trains/internal/services"
	"tools.xdoubleu.com/apps/trains/pkg/csa"
	"tools.xdoubleu.com/internal/logging"
)

// journeyFeed builds a hand-assembled Feed exercising the router's real
// traps directly against the DB path (repository -> Refresh -> Search),
// independent of app_test.go's GTFS-zip fixtures: a direct trip, a trip
// requiring a same-station transfer, an after-midnight trip, and a
// non-boarding technical pass-through.
func journeyFeed(windowStart time.Time) *models.Feed {
	day := func(n int) time.Time { return windowStart.AddDate(0, 0, n) }

	//nolint:exhaustruct //FeedInfo/Transfers filled below, rest zero-valued
	return &models.Feed{
		Stops: []models.Stop{
			{StopID: "SA", NameFR: "Alpha", LocationType: 1},
			{StopID: "A1", NameFR: "Alpha", ParentStation: "SA", PlatformCode: "1"},
			{StopID: "SB", NameFR: "Bravo", LocationType: 1},
			{StopID: "B1", NameFR: "Bravo", ParentStation: "SB", PlatformCode: "1"},
			{StopID: "B2", NameFR: "Bravo", ParentStation: "SB", PlatformCode: "2"},
			{StopID: "SC", NameFR: "Charlie", LocationType: 1},
			{StopID: "C1", NameFR: "Charlie", ParentStation: "SC", PlatformCode: "1"},
			{StopID: "M1", NameFR: "Midway"},
		},
		Routes: journeyRoutes(),
		Trips:  journeyTrips(),
		Transfers: []models.Transfer{
			{
				FromStopID: "B1", ToStopID: "B2",
				TransferType: 2, MinTransferTime: intPtr(120),
			},
		},
		StopTimes: journeyStopTimes(),
		CalendarDates: []models.CalendarDate{
			{ServiceID: "svc_direct", Date: day(0), ExceptionType: 1},
			{ServiceID: "svc_transfer", Date: day(0), ExceptionType: 1},
			{ServiceID: "svc_night", Date: day(0), ExceptionType: 1},
			// svc_direct/transfer/night intentionally do NOT run on day 3 —
			// used by the no-service-on-requested-date case below.
		},
	}
}

func intPtr(n int) *int { return &n }

func journeyRoutes() []models.Route {
	return []models.Route{
		{RouteID: "r1", ShortName: "IC", LongName: "", RouteType: 2},
		{RouteID: "r2", ShortName: "IC", LongName: "", RouteType: 2},
		{RouteID: "r3", ShortName: "IC", LongName: "", RouteType: 2},
	}
}

func journeyTrips() []models.Trip {
	return []models.Trip{
		{
			TripID: "t_direct", RouteID: "r1", ServiceID: "svc_direct",
			ShortName: "100", Headsign: "Bravo", DirectionID: nil,
		},
		{
			TripID: "t_leg1", RouteID: "r1", ServiceID: "svc_transfer",
			ShortName: "200", Headsign: "Bravo", DirectionID: nil,
		},
		{
			TripID: "t_leg2", RouteID: "r2", ServiceID: "svc_transfer",
			ShortName: "300", Headsign: "Charlie", DirectionID: nil,
		},
		{
			TripID: "t_night", RouteID: "r3", ServiceID: "svc_night",
			ShortName: "900", Headsign: "Bravo", DirectionID: nil,
		},
	}
}

// journeyDirectStopTimes is a direct A->B trip with a non-boarding
// technical pass-through at M1 in the middle.
func journeyDirectStopTimes() []models.StopTime {
	const h = 8 * 3600
	return []models.StopTime{
		{
			TripID: "t_direct", StopSequence: 1, StopID: "A1",
			ArrivalSeconds: h, DepartureSeconds: h,
			PickupType: 0, DropOffType: 0,
		},
		{
			TripID: "t_direct", StopSequence: 2, StopID: "M1",
			ArrivalSeconds: h + 600, DepartureSeconds: h + 600,
			PickupType: 1, DropOffType: 1,
		},
		{
			TripID: "t_direct", StopSequence: 3, StopID: "B1",
			ArrivalSeconds: h + 1200, DepartureSeconds: h + 1200,
			PickupType: 0, DropOffType: 0,
		},
	}
}

// journeyTransferStopTimes is A->B1 on one trip, then B2->C on a second
// trip — requires a same-station platform change at Bravo.
func journeyTransferStopTimes() []models.StopTime {
	const h = 9 * 3600
	return []models.StopTime{
		{
			TripID: "t_leg1", StopSequence: 1, StopID: "A1",
			ArrivalSeconds: h, DepartureSeconds: h,
			PickupType: 0, DropOffType: 0,
		},
		{
			TripID: "t_leg1", StopSequence: 2, StopID: "B1",
			ArrivalSeconds: h + 1200, DepartureSeconds: h + 1200,
			PickupType: 0, DropOffType: 0,
		},
		{
			TripID: "t_leg2", StopSequence: 1, StopID: "B2",
			ArrivalSeconds: h + 1500, DepartureSeconds: h + 1500,
			PickupType: 0, DropOffType: 0,
		},
		{
			TripID: "t_leg2", StopSequence: 2, StopID: "C1",
			ArrivalSeconds: h + 2700, DepartureSeconds: h + 2700,
			PickupType: 0, DropOffType: 0,
		},
	}
}

func journeyStopTimes() []models.StopTime {
	var out []models.StopTime
	out = append(out, journeyDirectStopTimes()...)
	out = append(out, journeyTransferStopTimes()...)
	out = append(out, journeyNightStopTimes()...)
	return out
}

// journeyNightStopTimes departs 23:50 day 0 and arrives 01:10 day 1.
func journeyNightStopTimes() []models.StopTime {
	return []models.StopTime{
		{
			TripID: "t_night", StopSequence: 1, StopID: "A1",
			ArrivalSeconds: 23*3600 + 3000, DepartureSeconds: 23*3600 + 3000,
			PickupType: 0, DropOffType: 0,
		},
		{
			TripID: "t_night", StopSequence: 2, StopID: "B1",
			ArrivalSeconds: 25*3600 + 600, DepartureSeconds: 25*3600 + 600,
			PickupType: 0, DropOffType: 0,
		},
	}
}

func TestSearchJourneys_EndToEnd(t *testing.T) {
	ctx := context.Background()
	windowStart := time.Now().UTC().Truncate(24 * time.Hour)

	require.NoError(
		t,
		testApp.Repositories.Feed.ImportFeed(ctx, journeyFeed(windowStart)),
	)
	_, refreshErr := testApp.Services.Journey.RefreshWindow(ctx, windowStart)
	require.NoError(t, refreshErr)

	t.Run("direct journey", func(t *testing.T) {
		when := windowStart.Add(7*time.Hour + 55*time.Minute)
		journeys, err := testApp.Services.Journey.SearchJourneys(
			ctx,
			"SA",
			"SB",
			when,
			false,
		)
		require.NoError(t, err)
		require.NotEmpty(t, journeys)
		assert.Len(t, journeys[0].Legs, 1)
		assert.Equal(t, "100", journeys[0].Legs[0].TripShortName)
	})

	t.Run("journey requiring a transfer", func(t *testing.T) {
		when := windowStart.Add(8*time.Hour + 55*time.Minute)
		journeys, err := testApp.Services.Journey.SearchJourneys(
			ctx,
			"SA",
			"SC",
			when,
			false,
		)
		require.NoError(t, err)
		require.NotEmpty(t, journeys)
		best := journeys[0]
		require.Len(t, best.Legs, 2)
		assert.Equal(t, 1, best.Transfers)
		assert.Equal(t, "B1", best.Legs[0].AlightStopID)
		assert.Equal(t, "B2", best.Legs[1].BoardStopID)
	})

	t.Run("after-midnight journey crosses the service day", func(t *testing.T) {
		when := windowStart.Add(23 * time.Hour)
		journeys, err := testApp.Services.Journey.SearchJourneys(
			ctx,
			"SA",
			"SB",
			when,
			false,
		)
		require.NoError(t, err)
		require.NotEmpty(t, journeys)
		require.Len(t, journeys[0].Legs, 1)
		arr := journeys[0].Legs[0].AlightTime
		assert.Equal(t, windowStart.AddDate(0, 0, 1).Day(), arr.Day())
	})

	t.Run("no service on the requested date", func(t *testing.T) {
		when := windowStart.AddDate(0, 0, 3).Add(8 * time.Hour)
		journeys, err := testApp.Services.Journey.SearchJourneys(
			ctx,
			"SA",
			"SB",
			when,
			false,
		)
		require.NoError(t, err)
		assert.Empty(t, journeys)
	})

	t.Run("non-boarding call cannot be a boarding point", func(t *testing.T) {
		when := windowStart.Add(8*time.Hour + 5*time.Minute)
		journeys, err := testApp.Services.Journey.SearchJourneys(
			ctx,
			"M1",
			"SB",
			when,
			false,
		)
		require.NoError(t, err)
		assert.Empty(t, journeys)
	})
}

func TestSearchJourneys_UnknownStopIsAnError(t *testing.T) {
	ctx := context.Background()
	// SearchJourneys no longer builds the index in-request (issue #1484), so
	// warm it first — otherwise this asserts the warming-up path, not the
	// unknown-stop one.
	_, err := testApp.Services.Journey.RefreshWindow(
		ctx, time.Now().UTC().Truncate(24*time.Hour),
	)
	require.NoError(t, err)
	_, err = testApp.Services.Journey.SearchJourneys(
		ctx, "does-not-exist", "also-not-real", time.Now(), false,
	)
	require.Error(t, err)
}

// TestSearchJourneys_ColdRouterIsUnavailable pins issue #1484's C3: a
// JourneyService whose index has never been built rejects the query with
// ErrRouterWarmingUp instead of running the whole-window scan in the caller's
// goroutine.
func TestSearchJourneys_ColdRouterIsUnavailable(t *testing.T) {
	js := services.NewJourneyService(logging.NewNopLogger(), testApp.Repositories)
	_, err := js.SearchJourneys(
		context.Background(), "SA", "SB", time.Now(), false,
	)
	require.ErrorIs(t, err, services.ErrRouterWarmingUp)
}

// TestJourneyService_Refresh_CollapsesConcurrentRebuilds covers the
// singleflight guard added with #1484: the startup warm-up racing the first
// scheduled refresh must not run two window scans.
func TestJourneyService_Refresh_CollapsesConcurrentRebuilds(t *testing.T) {
	ctx := context.Background()
	require.NoError(
		t,
		testApp.Repositories.Feed.ImportFeed(
			ctx, journeyFeed(time.Now().UTC().Truncate(24*time.Hour)),
		),
	)
	js := services.NewJourneyService(logging.NewNopLogger(), testApp.Repositories)

	const n = 8
	idxs := make([]*csa.Index, n)
	var wg sync.WaitGroup
	for i := range n {
		wg.Go(func() {
			idx, err := js.Refresh(ctx)
			assert.NoError(t, err)
			idxs[i] = idx
		})
	}
	wg.Wait()

	for i := 1; i < n; i++ {
		assert.Same(
			t,
			idxs[0],
			idxs[i],
			"concurrent Refresh calls should share one index",
		)
	}
}

// TestJourneyService_Refresh exercises Refresh (as opposed to
// RefreshWindow, used by the fixture-aligned tests above), which anchors
// the rolling window to "today" in Europe/Brussels — the production path
// jobs.RouterRefreshJob calls on a schedule.
func TestJourneyService_Refresh(t *testing.T) {
	ctx := context.Background()
	idx, err := testApp.Services.Journey.Refresh(ctx)
	require.NoError(t, err)
	assert.NotNil(t, idx)
}

// TestJourneyService_RefreshOnly covers the func(context.Context) error
// adapter jobs.RouterRefreshJob is constructed with.
func TestJourneyService_RefreshOnly(t *testing.T) {
	require.NoError(t, testApp.Services.Journey.RefreshOnly(context.Background()))
}

// TestShortNamesByTripIDs covers the trip_id→trip_short_name resolution
// RealtimeService.Poll re-keys the GTFS-RT snapshot through (issue #1484).
func TestShortNamesByTripIDs(t *testing.T) {
	ctx := context.Background()
	windowStart := time.Now().UTC().Truncate(24 * time.Hour)
	require.NoError(
		t,
		testApp.Repositories.Feed.ImportFeed(ctx, detailFeed(windowStart)),
	)

	got, err := testApp.Repositories.Feed.ShortNamesByTripIDs(
		ctx, []string{"t_detail", "not-a-trip"},
	)
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"t_detail": "IC900"}, got)

	empty, err := testApp.Repositories.Feed.ShortNamesByTripIDs(ctx, nil)
	require.NoError(t, err)
	assert.Empty(t, empty)
}

// TestRouterRefreshJob_Metadata mirrors app_test.go's
// TestStaticImportJob_Metadata for the router-refresh job added in #1391.
func TestRouterRefreshJob_Metadata(t *testing.T) {
	j := jobs.NewRouterRefreshJob(testApp.Services.Journey.RefreshOnly)
	assert.Equal(t, "trains-router-refresh", j.ID())
	assert.Equal(t, 6*time.Hour, j.RunEvery())
	assert.NoError(t, j.Run(context.Background(), logging.NewNopLogger()))
}
