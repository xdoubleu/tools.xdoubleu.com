package trains_test

import (
	"context"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/MobilityData/gtfs-realtime-bindings/golang/gtfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"tools.xdoubleu.com/apps/trains/internal/models"
	"tools.xdoubleu.com/apps/trains/internal/services"
	"tools.xdoubleu.com/apps/trains/pkg/bmc"
	trainsv1 "tools.xdoubleu.com/gen/trains/v1"
)

// delayedTripUpdateBody builds a minimal GTFS-RT trip-update FeedMessage
// carrying one StopTimeUpdate at stopSequence with the given arrival delay
// (seconds) — enough to exercise JourneyDetailService's realtime overlay
// without needing decodeTripUpdates' own package-internal test helpers.
func delayedTripUpdateBody(
	t *testing.T, tripID string, stopSequence uint32, arrivalDelaySeconds int32,
) []byte {
	t.Helper()
	//nolint:exhaustruct //only the fields under test are set
	msg := &gtfs.FeedMessage{
		Header: &gtfs.FeedHeader{
			GtfsRealtimeVersion: proto.String("2.0"),
		},
		Entity: []*gtfs.FeedEntity{
			{
				Id: proto.String("e1"),
				TripUpdate: &gtfs.TripUpdate{
					Trip: &gtfs.TripDescriptor{TripId: proto.String(tripID)},
					StopTimeUpdate: []*gtfs.TripUpdate_StopTimeUpdate{
						{
							StopSequence: proto.Uint32(stopSequence),
							Arrival: &gtfs.TripUpdate_StopTimeEvent{
								Delay: proto.Int32(arrivalDelaySeconds),
							},
						},
					},
				},
			},
		},
	}
	body, err := proto.Marshal(msg)
	require.NoError(t, err)
	return body
}

// emptyAlertFeedBody builds a valid, entity-less GTFS-RT alert FeedMessage —
// decodeAlerts requires the header field to be set even for an empty feed.
func emptyAlertFeedBody(t *testing.T) []byte {
	t.Helper()
	//nolint:exhaustruct //only Header is required for a valid empty feed
	msg := &gtfs.FeedMessage{
		Header: &gtfs.FeedHeader{GtfsRealtimeVersion: proto.String("2.0")},
	}
	body, err := proto.Marshal(msg)
	require.NoError(t, err)
	return body
}

// detailFeed is a small, self-contained feed for exercising
// JourneyDetailService/GetJourneyDetail end to end (issue #1394),
// independent of journey_test.go's own fixture — one trip IC900 stopping at
// A1 (origin), M1 (technical pass-through, non-boarding), and B1
// (destination).
func detailFeed(windowStart time.Time) *models.Feed {
	const h = 8 * 3600
	//nolint:exhaustruct //Transfers/CalendarDates filled below, rest zero-valued
	return &models.Feed{
		Stops: []models.Stop{
			{StopID: "DA", NameFR: "Delta", LocationType: 1},
			{StopID: "DA1", NameFR: "Delta", ParentStation: "DA", PlatformCode: "1"},
			{StopID: "DM1", NameFR: "Midway"},
			{StopID: "DB", NameFR: "Echo", LocationType: 1},
			{StopID: "DB1", NameFR: "Echo", ParentStation: "DB", PlatformCode: "2"},
		},
		Routes: []models.Route{
			{RouteID: "dr1", ShortName: "IC", LongName: "", RouteType: 2},
		},
		Trips: []models.Trip{
			{
				TripID: "t_detail", RouteID: "dr1", ServiceID: "svc_detail",
				ShortName: "IC900", Headsign: "Echo", DirectionID: nil,
			},
		},
		StopTimes: []models.StopTime{
			{
				TripID: "t_detail", StopSequence: 1, StopID: "DA1",
				ArrivalSeconds: h, DepartureSeconds: h,
				PickupType: 0, DropOffType: 0,
			},
			{
				TripID: "t_detail", StopSequence: 2, StopID: "DM1",
				ArrivalSeconds: h + 300, DepartureSeconds: h + 300,
				PickupType: 1, DropOffType: 1,
			},
			{
				TripID: "t_detail", StopSequence: 3, StopID: "DB1",
				ArrivalSeconds: h + 900, DepartureSeconds: h + 900,
				PickupType: 0, DropOffType: 0,
			},
		},
		CalendarDates: []models.CalendarDate{
			{ServiceID: "svc_detail", Date: windowStart, ExceptionType: 1},
		},
	}
}

// detailFeedWithRTVariant is detailFeed plus a second trip carrying the same
// trip_short_name (IC900) on the same service day but a different trip_id —
// the shape issue #1484 is about, where the GTFS-RT feed labels a train with
// a trip_id the static feed assigns to a different stopping-pattern variant.
// TripByShortNameOnDate resolves the leg to "t_detail" (ordered by trip_id);
// the realtime feed publishes under "t_detail_rt".
func detailFeedWithRTVariant(windowStart time.Time) *models.Feed {
	feed := detailFeed(windowStart)
	feed.Trips = append(feed.Trips, models.Trip{
		TripID: "t_detail_rt", RouteID: "dr1", ServiceID: "svc_detail",
		ShortName: "IC900", Headsign: "Echo", DirectionID: nil,
	})
	variantStopTimes := make([]models.StopTime, len(feed.StopTimes))
	for i, st := range feed.StopTimes {
		st.TripID = "t_detail_rt"
		variantStopTimes[i] = st
	}
	feed.StopTimes = append(feed.StopTimes, variantStopTimes...)
	return feed
}

func detailLegRef(windowStart time.Time) services.LegRef {
	const h = 8 * 3600
	return services.LegRef{
		TripShortName: "IC900",
		BoardStopID:   "DA1",
		BoardTime:     windowStart.Add(h * time.Second),
		AlightStopID:  "DB1",
		AlightTime:    windowStart.Add((h + 900) * time.Second),
	}
}

func TestJourneyDetailService_GetJourneyDetail_NoLiveDataIsNeverOnTime(t *testing.T) {
	ctx := context.Background()
	windowStart := time.Now().UTC().Truncate(24 * time.Hour)
	require.NoError(
		t,
		testApp.Repositories.Feed.ImportFeed(ctx, detailFeed(windowStart)),
	)

	journeyID := services.EncodeJourneyID([]services.LegRef{detailLegRef(windowStart)})
	detail, err := testApp.Services.JourneyDetail.GetJourneyDetail(ctx, journeyID)
	require.NoError(t, err)
	require.Len(t, detail.Legs, 1)

	leg := detail.Legs[0]
	assert.Equal(t, "IC900", leg.TripShortName)
	assert.False(t, leg.Cancelled)
	require.Len(t, leg.Stops, 3)
	assert.Equal(t, "DA1", leg.Stops[0].StopID)
	assert.True(t, leg.Stops[0].IsBoardStop)
	assert.Equal(t, "DB1", leg.Stops[2].StopID)
	assert.True(t, leg.Stops[2].IsAlightStop)
	for _, s := range leg.Stops {
		// No realtime poll has run yet — every stop must render as
		// DelayUnknown, never DelayOnTime (issue #1394's core requirement).
		assert.Equal(t, models.DelayUnknown, s.State)
	}
}

func TestJourneyDetailService_GetJourneyDetail_OverlaysRealtimeState(t *testing.T) {
	ctx := context.Background()
	windowStart := time.Now().UTC().Truncate(24 * time.Hour)
	require.NoError(
		t,
		testApp.Repositories.Feed.ImportFeed(ctx, detailFeed(windowStart)),
	)

	t.Cleanup(func() { testBMC.RealtimeResults = nil })
	testBMC.RealtimeResults = map[string]*bmc.RealtimeResult{
		bmc.FeedTripUpdate: {Body: delayedTripUpdateBody(t, "t_detail", 3, 400)},
		bmc.FeedAlert:      {Body: emptyAlertFeedBody(t)},
	}
	require.NoError(t, testApp.Services.Realtime.Poll(ctx))

	journeyID := services.EncodeJourneyID([]services.LegRef{detailLegRef(windowStart)})
	detail, err := testApp.Services.JourneyDetail.GetJourneyDetail(ctx, journeyID)
	require.NoError(t, err)
	require.Len(t, detail.Legs, 1)

	last := detail.Legs[0].Stops[2]
	assert.Equal(t, models.DelayDelayed, last.State)
	require.NotNil(t, last.ArrivalDelay)
	assert.Equal(t, 400, *last.ArrivalDelay)
}

// TestJourneyDetailService_GetJourneyDetail_OverlaysAcrossTripIDNamespaces is
// issue #1484's C1: the realtime overlay must land even when the GTFS-RT feed
// identifies the train by a trip_id the static feed never resolves the leg
// to, because correlation now runs through (trip_short_name, service date).
func TestJourneyDetailService_GetJourneyDetail_OverlaysAcrossTripIDNamespaces(
	t *testing.T,
) {
	ctx := context.Background()
	windowStart := time.Now().UTC().Truncate(24 * time.Hour)
	require.NoError(
		t,
		testApp.Repositories.Feed.ImportFeed(ctx, detailFeedWithRTVariant(windowStart)),
	)

	t.Cleanup(func() { testBMC.RealtimeResults = nil })
	testBMC.RealtimeResults = map[string]*bmc.RealtimeResult{
		bmc.FeedTripUpdate: {Body: delayedTripUpdateBody(t, "t_detail_rt", 3, 300)},
		bmc.FeedAlert:      {Body: emptyAlertFeedBody(t)},
	}
	require.NoError(t, testApp.Services.Realtime.Poll(ctx))
	assert.Zero(t, testApp.Services.Realtime.Snapshot().UnresolvedTripCount)

	journeyID := services.EncodeJourneyID([]services.LegRef{detailLegRef(windowStart)})
	detail, err := testApp.Services.JourneyDetail.GetJourneyDetail(ctx, journeyID)
	require.NoError(t, err)
	require.Len(t, detail.Legs, 1)

	last := detail.Legs[0].Stops[2]
	assert.Equal(t, models.DelayDelayed, last.State)
	require.NotNil(t, last.ArrivalDelay)
	assert.Equal(t, 300, *last.ArrivalDelay)
}

// TestJourneyDetailService_GetJourneyDetail_UncorrelatedRealtimeTripIsCounted
// covers the other half of #1484's C1: a realtime trip with no matching
// static trip is dropped from the snapshot and counted, and the leg falls
// back to DelayUnknown rather than silently swallowing the gap.
func TestJourneyDetailService_GetJourneyDetail_UncorrelatedRealtimeTripIsCounted(
	t *testing.T,
) {
	ctx := context.Background()
	windowStart := time.Now().UTC().Truncate(24 * time.Hour)
	require.NoError(
		t,
		testApp.Repositories.Feed.ImportFeed(ctx, detailFeed(windowStart)),
	)

	t.Cleanup(func() { testBMC.RealtimeResults = nil })
	testBMC.RealtimeResults = map[string]*bmc.RealtimeResult{
		bmc.FeedTripUpdate: {Body: delayedTripUpdateBody(t, "ghost-trip", 3, 600)},
		bmc.FeedAlert:      {Body: emptyAlertFeedBody(t)},
	}
	require.NoError(t, testApp.Services.Realtime.Poll(ctx))
	assert.Equal(t, 1, testApp.Services.Realtime.Snapshot().UnresolvedTripCount)

	journeyID := services.EncodeJourneyID([]services.LegRef{detailLegRef(windowStart)})
	detail, err := testApp.Services.JourneyDetail.GetJourneyDetail(ctx, journeyID)
	require.NoError(t, err)
	require.Len(t, detail.Legs, 1)
	for _, s := range detail.Legs[0].Stops {
		assert.Equal(t, models.DelayUnknown, s.State)
	}
}

func TestJourneyDetailService_GetJourneyDetail_TripOutsideWindowRendersEmptyLeg(
	t *testing.T,
) {
	ctx := context.Background()
	ref := services.LegRef{
		TripShortName: "does-not-exist",
		BoardStopID:   "X",
		BoardTime:     time.Now(),
		AlightStopID:  "Y",
		AlightTime:    time.Now().Add(time.Hour),
	}
	journeyID := services.EncodeJourneyID([]services.LegRef{ref})

	detail, err := testApp.Services.JourneyDetail.GetJourneyDetail(ctx, journeyID)
	require.NoError(t, err)
	require.Len(t, detail.Legs, 1)
	assert.Equal(t, "does-not-exist", detail.Legs[0].TripShortName)
	assert.Empty(t, detail.Legs[0].Stops)
}

func TestJourneyDetailService_GetJourneyDetail_InvalidIDIsAnError(t *testing.T) {
	_, err := testApp.Services.JourneyDetail.GetJourneyDetail(
		context.Background(), "not-a-real-journey-id!!",
	)
	require.ErrorIs(t, err, services.ErrInvalidJourneyID)
}

func TestGetJourneyDetail_Handler_EndToEnd(t *testing.T) {
	ctx := context.Background()
	windowStart := time.Now().UTC().Truncate(24 * time.Hour)
	require.NoError(
		t,
		testApp.Repositories.Feed.ImportFeed(ctx, detailFeed(windowStart)),
	)

	journeyID := services.EncodeJourneyID([]services.LegRef{detailLegRef(windowStart)})
	client := newTrainsTestClient(t)

	resp, err := client.GetJourneyDetail(ctx, connect.NewRequest(
		&trainsv1.GetJourneyDetailRequest{JourneyId: journeyID},
	))
	require.NoError(t, err)
	require.Len(t, resp.Msg.GetJourney().GetLegs(), 1)
	assert.Equal(t, "IC900", resp.Msg.GetJourney().GetLegs()[0].GetTripShortName())
	assert.Equal(t, journeyID, resp.Msg.GetJourney().GetJourneyId())
}

func TestGetJourneyDetail_Handler_EmptyJourneyIDIsInvalidArgument(t *testing.T) {
	client := newTrainsTestClient(t)
	_, err := client.GetJourneyDetail(
		context.Background(),
		connect.NewRequest(&trainsv1.GetJourneyDetailRequest{JourneyId: ""}),
	)
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	assert.Equal(t, connect.CodeInvalidArgument, connErr.Code())
}

func TestGetJourneyDetail_Handler_MalformedJourneyIDIsInvalidArgument(t *testing.T) {
	client := newTrainsTestClient(t)
	_, err := client.GetJourneyDetail(
		context.Background(),
		connect.NewRequest(
			&trainsv1.GetJourneyDetailRequest{JourneyId: "!!not-valid!!"},
		),
	)
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	assert.Equal(t, connect.CodeInvalidArgument, connErr.Code())
}

func TestSearchJourneys_ResponseCarriesJourneyID(t *testing.T) {
	ctx := context.Background()
	windowStart := time.Now().UTC().Truncate(24 * time.Hour)
	require.NoError(
		t,
		testApp.Repositories.Feed.ImportFeed(ctx, detailFeed(windowStart)),
	)
	_, err := testApp.Services.Journey.RefreshWindow(ctx, windowStart)
	require.NoError(t, err)

	client := newTrainsTestClient(t)
	resp, err := client.SearchJourneys(
		ctx,
		connect.NewRequest(&trainsv1.SearchJourneysRequest{
			OriginStopId:      "DA",
			DestinationStopId: "DB",
			Time: windowStart.Add(7*time.Hour + 55*time.Minute).
				Format(time.RFC3339),
			ArriveBy: false,
		}),
	)
	require.NoError(t, err)
	require.NotEmpty(t, resp.Msg.GetJourneys())
	assert.NotEmpty(t, resp.Msg.GetJourneys()[0].GetJourneyId())
}
