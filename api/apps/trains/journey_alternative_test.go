package trains_test

import (
	"context"
	"testing"
	"time"

	"github.com/MobilityData/gtfs-realtime-bindings/golang/gtfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"tools.xdoubleu.com/apps/trains/internal/models"
	"tools.xdoubleu.com/apps/trains/internal/services"
	"tools.xdoubleu.com/apps/trains/pkg/bmc"
)

// altDayStart anchors the fixture window: UTC midnight of the current
// Europe/Brussels calendar day. Keeping it a UTC instant preserves the
// consistent +offset csa.Build's epoch and the DATE-typed calendar_dates
// column both pick up (as journey_test.go's UTC-midnight anchor does), while
// pinning the day to Brussels means services.serviceDateOf and
// RealtimeService's Brussels-local fallback date agree on the service date
// even in the late-evening UTC window that flakes the sibling overlay tests
// (#1523).
func altDayStart() time.Time {
	loc, _ := time.LoadLocation("Europe/Brussels")
	now := time.Now().In(loc)
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
}

// altFeed is a two-leg itinerary with a same-station change and a later
// third train available as a fallback from the transfer platform:
//   - IC1: AX1 08:00 → MX1 08:30
//   - IC2: MX2 08:40 → BX1 09:20   (change MX1→MX2, default 180s)
//   - IC3: MX1 09:00 → BX1 09:45   (the re-plan target)
func altFeed(windowStart time.Time) *models.Feed {
	h := func(sec int) int { return 8*3600 + sec }
	//nolint:exhaustruct //only the fields the router/overlay read are set
	return &models.Feed{
		Stops: []models.Stop{
			{StopID: "AX", NameFR: "Ax", LocationType: 1},
			{StopID: "AX1", NameFR: "Ax", ParentStation: "AX", PlatformCode: "1"},
			{StopID: "MX", NameFR: "Mechelen", LocationType: 1},
			{StopID: "MX1", NameFR: "Mechelen", ParentStation: "MX", PlatformCode: "1"},
			{StopID: "MX2", NameFR: "Mechelen", ParentStation: "MX", PlatformCode: "2"},
			{StopID: "BX", NameFR: "Bx", LocationType: 1},
			{StopID: "BX1", NameFR: "Bx", ParentStation: "BX", PlatformCode: "1"},
		},
		Routes: []models.Route{{RouteID: "r", ShortName: "IC", RouteType: 2}},
		Trips: []models.Trip{
			{
				TripID:    "t1",
				RouteID:   "r",
				ServiceID: "s",
				ShortName: "IC1",
				Headsign:  "Mechelen",
			},
			{
				TripID:    "t2",
				RouteID:   "r",
				ServiceID: "s",
				ShortName: "IC2",
				Headsign:  "Bx",
			},
			{
				TripID:    "t3",
				RouteID:   "r",
				ServiceID: "s",
				ShortName: "IC3",
				Headsign:  "Bx",
			},
		},
		StopTimes: []models.StopTime{
			{
				TripID:           "t1",
				StopSequence:     1,
				StopID:           "AX1",
				ArrivalSeconds:   h(0),
				DepartureSeconds: h(0),
			},
			{
				TripID:           "t1",
				StopSequence:     2,
				StopID:           "MX1",
				ArrivalSeconds:   h(1800),
				DepartureSeconds: h(1800),
			},
			{
				TripID:           "t2",
				StopSequence:     1,
				StopID:           "MX2",
				ArrivalSeconds:   h(2400),
				DepartureSeconds: h(2400),
			},
			{
				TripID:           "t2",
				StopSequence:     2,
				StopID:           "BX1",
				ArrivalSeconds:   h(4800),
				DepartureSeconds: h(4800),
			},
			{
				TripID:           "t3",
				StopSequence:     1,
				StopID:           "MX1",
				ArrivalSeconds:   h(3600),
				DepartureSeconds: h(3600),
			},
			{
				TripID:           "t3",
				StopSequence:     2,
				StopID:           "BX1",
				ArrivalSeconds:   h(6300),
				DepartureSeconds: h(6300),
			},
		},
		CalendarDates: []models.CalendarDate{
			{ServiceID: "s", Date: windowStart, ExceptionType: 1},
		},
	}
}

func altLegRefs(windowStart time.Time) []services.LegRef {
	at := func(sec int) time.Time {
		return windowStart.Add(time.Duration(8*3600+sec) * time.Second)
	}
	return []services.LegRef{
		{
			TripShortName: "IC1",
			BoardStopID:   "AX1",
			BoardTime:     at(0),
			AlightStopID:  "MX1",
			AlightTime:    at(1800),
		},
		{
			TripShortName: "IC2",
			BoardStopID:   "MX2",
			BoardTime:     at(2400),
			AlightStopID:  "BX1",
			AlightTime:    at(4800),
		},
	}
}

// stopUpdateBody builds a trip-update feed with one StopTimeUpdate for
// tripID at stopSequence. arrivalDelay is applied only when rel is
// SCHEDULED; pass NO_DATA to assert the overlay stays absent.
func stopUpdateBody(
	t *testing.T,
	tripID string,
	stopSequence uint32,
	arrivalDelay int32,
	rel gtfs.TripUpdate_StopTimeUpdate_ScheduleRelationship,
) []byte {
	t.Helper()
	//nolint:exhaustruct //only the fields the decoder reads are set
	stu := &gtfs.TripUpdate_StopTimeUpdate{
		StopSequence:         proto.Uint32(stopSequence),
		ScheduleRelationship: rel.Enum(),
	}
	if rel == gtfs.TripUpdate_StopTimeUpdate_SCHEDULED {
		//nolint:exhaustruct //only Delay is set
		stu.Arrival = &gtfs.TripUpdate_StopTimeEvent{Delay: proto.Int32(arrivalDelay)}
	}
	//nolint:exhaustruct //only the fields under test are set
	msg := &gtfs.FeedMessage{
		Header: &gtfs.FeedHeader{GtfsRealtimeVersion: proto.String("2.0")},
		Entity: []*gtfs.FeedEntity{{
			Id: proto.String("e1"),
			TripUpdate: &gtfs.TripUpdate{
				Trip:           &gtfs.TripDescriptor{TripId: proto.String(tripID)},
				StopTimeUpdate: []*gtfs.TripUpdate_StopTimeUpdate{stu},
			},
		}},
	}
	body, err := proto.Marshal(msg)
	require.NoError(t, err)
	return body
}

func seedAltFeed(ctx context.Context, t *testing.T) time.Time {
	t.Helper()
	windowStart := altDayStart()
	require.NoError(t, testApp.Repositories.Feed.ImportFeed(ctx, altFeed(windowStart)))
	_, err := testApp.Services.Journey.RefreshWindow(ctx, windowStart)
	require.NoError(t, err)
	return windowStart
}

func TestGetJourneyDetail_MissedConnectionSurfacesAlternative(t *testing.T) {
	ctx := context.Background()
	windowStart := seedAltFeed(ctx, t)

	t.Cleanup(func() { testBMC.RealtimeResults = nil })
	// IC1 arrives at MX1 (seq 2) 8 minutes late — the 180s change to IC2's
	// 08:40 departure is no longer makeable.
	testBMC.RealtimeResults = map[string]*bmc.RealtimeResult{
		bmc.FeedTripUpdate: {Body: stopUpdateBody(
			t, "t1", 2, 480, gtfs.TripUpdate_StopTimeUpdate_SCHEDULED,
		)},
		bmc.FeedAlert: {Body: emptyAlertFeedBody(t)},
	}
	require.NoError(t, testApp.Services.Realtime.Poll(ctx))

	journeyID := services.EncodeJourneyID(altLegRefs(windowStart))
	detail, err := testApp.Services.JourneyDetail.GetJourneyDetail(ctx, journeyID)
	require.NoError(t, err)

	require.NotNil(t, detail.Alternative)
	assert.Contains(t, detail.Alternative.Reason, "Mechelen")
	assert.Equal(t, "Mechelen", detail.Alternative.FromStopName)
	require.NotNil(t, detail.Alternative.Journey)
	require.Len(t, detail.Alternative.Journey.Legs, 1)
	assert.Equal(t, "IC3", detail.Alternative.Journey.Legs[0].TripShortName)
	assert.NotEmpty(t, detail.Alternative.Journey.JourneyID)
}

func TestGetJourneyDetail_NoDataNeverSurfacesAlternative(t *testing.T) {
	ctx := context.Background()
	windowStart := seedAltFeed(ctx, t)

	t.Cleanup(func() { testBMC.RealtimeResults = nil })
	// A trip update exists for IC1, but its arrival at MX1 carries NO_DATA —
	// the ~2/3 case (#1393). It must not be read as a broken connection.
	testBMC.RealtimeResults = map[string]*bmc.RealtimeResult{
		bmc.FeedTripUpdate: {Body: stopUpdateBody(
			t, "t1", 2, 0, gtfs.TripUpdate_StopTimeUpdate_NO_DATA,
		)},
		bmc.FeedAlert: {Body: emptyAlertFeedBody(t)},
	}
	require.NoError(t, testApp.Services.Realtime.Poll(ctx))

	journeyID := services.EncodeJourneyID(altLegRefs(windowStart))
	detail, err := testApp.Services.JourneyDetail.GetJourneyDetail(ctx, journeyID)
	require.NoError(t, err)
	assert.Nil(t, detail.Alternative)
}

func TestGetJourneyDetail_HealthyJourneyHasNoAlternative(t *testing.T) {
	ctx := context.Background()
	windowStart := seedAltFeed(ctx, t)

	journeyID := services.EncodeJourneyID(altLegRefs(windowStart))
	detail, err := testApp.Services.JourneyDetail.GetJourneyDetail(ctx, journeyID)
	require.NoError(t, err)
	assert.Nil(t, detail.Alternative, "no realtime signal at all → nothing to re-plan")
}
