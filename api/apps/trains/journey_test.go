package trains_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/trains/internal/models"
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
			{StopID: "SA", Name: "Alpha", LocationType: 1},
			{StopID: "A1", Name: "Alpha", ParentStation: "SA", PlatformCode: "1"},
			{StopID: "SB", Name: "Bravo", LocationType: 1},
			{StopID: "B1", Name: "Bravo", ParentStation: "SB", PlatformCode: "1"},
			{StopID: "B2", Name: "Bravo", ParentStation: "SB", PlatformCode: "2"},
			{StopID: "SC", Name: "Charlie", LocationType: 1},
			{StopID: "C1", Name: "Charlie", ParentStation: "SC", PlatformCode: "1"},
			{StopID: "M1", Name: "Midway"},
		},
		Routes:    journeyRoutes(),
		Trips:     journeyTrips(),
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
	_, err := testApp.Services.Journey.SearchJourneys(
		ctx, "does-not-exist", "also-not-real", time.Now(), false,
	)
	require.Error(t, err)
}
