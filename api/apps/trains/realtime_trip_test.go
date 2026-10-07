package trains_test

import (
	"context"
	"testing"

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

// stopIDOnlyTripUpdateBody builds a GTFS-RT feed whose one stop update names
// stopID with a departure delay and no stop_sequence.
func stopIDOnlyTripUpdateBody(
	t *testing.T, tripID, stopID string, departureDelaySeconds int32,
) []byte {
	t.Helper()
	//nolint:exhaustruct //only the fields under test are set
	msg := &gtfs.FeedMessage{
		Header: &gtfs.FeedHeader{GtfsRealtimeVersion: proto.String("2.0")},
		Entity: []*gtfs.FeedEntity{{
			Id: proto.String("e1"),
			TripUpdate: &gtfs.TripUpdate{
				Trip: &gtfs.TripDescriptor{TripId: proto.String(tripID)},
				StopTimeUpdate: []*gtfs.TripUpdate_StopTimeUpdate{{
					StopId: proto.String(stopID),
					Departure: &gtfs.TripUpdate_StopTimeEvent{
						Delay: proto.Int32(departureDelaySeconds),
					},
				}},
			},
		}},
	}
	body, err := proto.Marshal(msg)
	require.NoError(t, err)
	return body
}

func pollStopIDOnlyDelay(ctx context.Context, t *testing.T) {
	t.Helper()
	require.NoError(
		t,
		testApp.Repositories.Feed.ImportFeed(ctx, detailFeed(trainsDayStart())),
	)
	t.Cleanup(func() { testBMC.RealtimeResults = nil })
	testBMC.RealtimeResults = map[string]*bmc.RealtimeResult{
		bmc.FeedTripUpdate: {Body: stopIDOnlyTripUpdateBody(t, "t_detail", "DA1", 180)},
		bmc.FeedAlert:      {Body: emptyAlertFeedBody(t)},
	}
	require.NoError(t, testApp.Services.Realtime.Poll(ctx))
}

// TestGetJourneyDetail_StopIDOnlyUpdatePropagates: an update without
// stop_sequence still lands, and later stops without one inherit its delay.
func TestGetJourneyDetail_StopIDOnlyUpdatePropagates(t *testing.T) {
	ctx := context.Background()
	pollStopIDOnlyDelay(ctx, t)

	journeyID := services.EncodeJourneyID(
		[]services.LegRef{detailLegRef(trainsDayStart())},
	)
	detail, err := testApp.Services.JourneyDetail.GetJourneyDetail(ctx, journeyID)
	require.NoError(t, err)
	require.Len(t, detail.Legs, 1)

	for _, s := range detail.Legs[0].Stops {
		assert.Equal(t, models.DelayDelayed, s.State, s.StopID)
		require.NotNil(t, s.DepartureDelay, s.StopID)
		assert.Equal(t, 180, *s.DepartureDelay, s.StopID)
	}
}

func TestGetRealtimeTrip_Handler_ReportsRawTripAndPlannedStops(t *testing.T) {
	ctx := context.Background()
	pollStopIDOnlyDelay(ctx, t)

	client := newTrainsTestClient(t)
	resp, err := client.GetRealtimeTrip(ctx, connect.NewRequest(
		&trainsv1.GetRealtimeTripRequest{TripShortName: "IC900"},
	))
	require.NoError(t, err)
	msg := resp.Msg

	assert.NotEmpty(t, msg.GetFetchedAt())
	assert.Equal(t, int32(1), msg.GetTripCount())
	assert.Zero(t, msg.GetUnresolvedTripCount())
	assert.Zero(t, msg.GetDuplicateTripCount())
	assert.Equal(t, map[string]int32{"delayed": 1}, msg.GetStopUpdateStatuses())

	assert.True(t, msg.GetFound())
	assert.Equal(t, "t_detail", msg.GetTripId())
	assert.Equal(t, "on_time", msg.GetStatus())
	require.Len(t, msg.GetStopUpdates(), 1)
	update := msg.GetStopUpdates()[0]
	assert.Equal(t, "DA1", update.GetStopId())
	assert.Zero(t, update.GetStopSequence())
	assert.Nil(t, update.ArrivalDelay, "an unpublished delay stays absent")
	assert.Equal(t, int32(180), update.GetDepartureDelay())
	assert.Empty(t, update.GetArrivalTime(), "the feed published only delays")

	assert.Equal(t, "t_detail", msg.GetPlannedTripId())
	require.Len(t, msg.GetPlannedStops(), 3)
	assert.Equal(t, "DB1", msg.GetPlannedStops()[2].GetStopId())
	assert.Equal(t, int32(3), msg.GetPlannedStops()[2].GetStopSequence())
}

func TestGetRealtimeTrip_Handler_SummaryOnlyWithoutTrainNumber(t *testing.T) {
	ctx := context.Background()
	pollStopIDOnlyDelay(ctx, t)

	client := newTrainsTestClient(t)
	resp, err := client.GetRealtimeTrip(ctx, connect.NewRequest(
		&trainsv1.GetRealtimeTripRequest{},
	))
	require.NoError(t, err)

	assert.Equal(t, int32(1), resp.Msg.GetTripCount())
	assert.False(t, resp.Msg.GetFound())
	assert.Empty(t, resp.Msg.GetStatus())
	assert.Empty(t, resp.Msg.GetPlannedTripId())
}

func TestGetRealtimeTrip_Handler_UnknownTrainIsNotFound(t *testing.T) {
	ctx := context.Background()
	pollStopIDOnlyDelay(ctx, t)

	client := newTrainsTestClient(t)
	resp, err := client.GetRealtimeTrip(ctx, connect.NewRequest(
		&trainsv1.GetRealtimeTripRequest{TripShortName: "IC-none"},
	))
	require.NoError(t, err)

	assert.False(t, resp.Msg.GetFound())
	assert.Empty(t, resp.Msg.GetStopUpdates())
	assert.Empty(t, resp.Msg.GetPlannedTripId())
}
