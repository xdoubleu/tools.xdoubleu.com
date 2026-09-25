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

// altDayStart is UTC midnight of the current Brussels calendar day: a UTC
// instant keeps csa.Build's epoch and DATE columns consistent, while the
// Brussels day keeps service-date lookups agreeing late in the UTC day.
func altDayStart() time.Time {
	loc, _ := time.LoadLocation("Europe/Brussels")
	now := time.Now().In(loc)
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
}

// altFeed: two legs with a same-station change, plus a fallback train:
//   - IC1: AX1 08:00 → MX1 08:30
//   - IC2: MX2 08:40 → BX1 09:20   (change MX1→MX2, default 180s)
//   - IC3: MX1 09:00 → BX1 09:45   (the re-plan target)
func altFeed(windowStart time.Time) *models.Feed {
	h := func(sec int) int { return 8*3600 + sec }
	//nolint:exhaustruct //only the fields the router/overlay read are set
	return &models.Feed{
		Stops: []models.Stop{
			{StopID: "AX", NameFR: "Ax", DisplayName: "Ax", LocationType: 1},
			{
				StopID:        "AX1",
				NameFR:        "Ax",
				DisplayName:   "Ax",
				ParentStation: "AX",
				PlatformCode:  "1",
			},
			{
				StopID:       "MX",
				NameFR:       "Mechelen",
				DisplayName:  "Mechelen",
				LocationType: 1,
			},
			{
				StopID: "MX1", NameFR: "Mechelen", DisplayName: "Mechelen",
				ParentStation: "MX", PlatformCode: "1",
			},
			{
				StopID: "MX2", NameFR: "Mechelen", DisplayName: "Mechelen",
				ParentStation: "MX", PlatformCode: "2",
			},
			{StopID: "BX", NameFR: "Bx", DisplayName: "Bx", LocationType: 1},
			{
				StopID:        "BX1",
				NameFR:        "Bx",
				DisplayName:   "Bx",
				ParentStation: "BX",
				PlatformCode:  "1",
			},
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

// altLegRefs hand-builds the LegRefs SearchJourneys would encode, in
// idx.fromAbs' frame: Brussels-local midnight of windowStart's Y/M/D.
func altLegRefs(windowStart time.Time) []services.LegRef {
	brussels, err := time.LoadLocation("Europe/Brussels")
	if err != nil {
		panic(err)
	}
	y, m, d := windowStart.Date()
	localMidnight := time.Date(y, m, d, 0, 0, 0, 0, brussels)
	at := func(sec int) time.Time {
		return localMidnight.Add(time.Duration(8*3600+sec) * time.Second)
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

// stopUpdateBody builds a one-StopTimeUpdate trip-update feed; arrivalDelay
// applies only when rel is SCHEDULED.
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
	// IC1 arrives 8 minutes late, missing the 180s change to IC2.
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
	// NO_DATA on IC1's arrival must not read as a broken connection.
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
