package services

import (
	"testing"

	"github.com/MobilityData/gtfs-realtime-bindings/golang/gtfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"tools.xdoubleu.com/apps/trains/internal/models"
)

func ptr[T any](v T) *T { return &v }

func marshalFeed(t *testing.T, msg *gtfs.FeedMessage) []byte {
	t.Helper()
	body, err := proto.Marshal(msg)
	require.NoError(t, err)
	return body
}

func newFeedMessage(entities ...*gtfs.FeedEntity) *gtfs.FeedMessage {
	return &gtfs.FeedMessage{
		Header: &gtfs.FeedHeader{
			GtfsRealtimeVersion: ptr("2.0"),
			Incrementality:      nil,
			Timestamp:           nil,
		},
		Entity: entities,
	}
}

// TestDecodeTripUpdates_FullCancellation covers the trip-level CANCELED
// case from issue #1393's table — the whole trip is off, not just one stop.
func TestDecodeTripUpdates_FullCancellation(t *testing.T) {
	//nolint:exhaustruct //only the fields under test are set
	msg := newFeedMessage(&gtfs.FeedEntity{
		Id: ptr("e1"),
		TripUpdate: &gtfs.TripUpdate{
			Trip: &gtfs.TripDescriptor{
				TripId:               ptr("trip-1"),
				ScheduleRelationship: gtfs.TripDescriptor_CANCELED.Enum(),
			},
		},
	})

	trips, err := decodeTripUpdates(marshalFeed(t, msg))
	require.NoError(t, err)
	require.Contains(t, trips, "trip-1")
	assert.Equal(t, models.DelayCancelled, trips["trip-1"].State)
}

// TestDecodeTripUpdates_PartialCancellation covers a stop-level SKIPPED
// call on an otherwise-SCHEDULED trip — the trip itself still runs.
func TestDecodeTripUpdates_PartialCancellation(t *testing.T) {
	//nolint:exhaustruct //only the fields under test are set
	msg := newFeedMessage(&gtfs.FeedEntity{
		Id: ptr("e1"),
		TripUpdate: &gtfs.TripUpdate{
			Trip: &gtfs.TripDescriptor{TripId: ptr("trip-1")},
			StopTimeUpdate: []*gtfs.TripUpdate_StopTimeUpdate{
				{
					StopId:               ptr("stop-1"),
					StopSequence:         ptr(uint32(3)),
					ScheduleRelationship: gtfs.TripUpdate_StopTimeUpdate_SKIPPED.Enum(),
				},
			},
		},
	})

	trips, err := decodeTripUpdates(marshalFeed(t, msg))
	require.NoError(t, err)
	trip := trips["trip-1"]
	assert.Equal(t, models.DelayOnTime, trip.State)
	require.Len(t, trip.StopCalls, 1)
	assert.Equal(t, models.DelaySkipped, trip.StopCalls[0].State)
}

// TestDecodeTripUpdates_NoData covers the common case (issue #1393: 68% of
// calls in one sample) — no live information, which must never read as
// on-time.
func TestDecodeTripUpdates_NoData(t *testing.T) {
	//nolint:exhaustruct //only the fields under test are set
	msg := newFeedMessage(&gtfs.FeedEntity{
		Id: ptr("e1"),
		TripUpdate: &gtfs.TripUpdate{
			Trip: &gtfs.TripDescriptor{TripId: ptr("trip-1")},
			StopTimeUpdate: []*gtfs.TripUpdate_StopTimeUpdate{
				{
					StopId:               ptr("stop-1"),
					ScheduleRelationship: gtfs.TripUpdate_StopTimeUpdate_NO_DATA.Enum(),
				},
			},
		},
	})

	trips, err := decodeTripUpdates(marshalFeed(t, msg))
	require.NoError(t, err)
	call := trips["trip-1"].StopCalls[0]
	assert.Equal(t, models.DelayUnknown, call.State)
	assert.Nil(t, call.ArrivalDelay)
	assert.Nil(t, call.DepartureDelay)
}

func TestDecodeTripUpdates_DelayedAndOnTime(t *testing.T) {
	//nolint:exhaustruct //only the fields under test are set
	msg := newFeedMessage(&gtfs.FeedEntity{
		Id: ptr("e1"),
		TripUpdate: &gtfs.TripUpdate{
			Trip: &gtfs.TripDescriptor{TripId: ptr("trip-1")},
			StopTimeUpdate: []*gtfs.TripUpdate_StopTimeUpdate{
				{
					StopId:    ptr("stop-1"),
					Arrival:   &gtfs.TripUpdate_StopTimeEvent{Delay: ptr(int32(120))},
					Departure: &gtfs.TripUpdate_StopTimeEvent{Delay: ptr(int32(120))},
				},
				{
					StopId:    ptr("stop-2"),
					Arrival:   &gtfs.TripUpdate_StopTimeEvent{Delay: ptr(int32(0))},
					Departure: &gtfs.TripUpdate_StopTimeEvent{Delay: ptr(int32(0))},
				},
			},
		},
	})

	trips, err := decodeTripUpdates(marshalFeed(t, msg))
	require.NoError(t, err)
	calls := trips["trip-1"].StopCalls
	require.Len(t, calls, 2)

	assert.Equal(t, models.DelayDelayed, calls[0].State)
	require.NotNil(t, calls[0].ArrivalDelay)
	assert.Equal(t, 120, *calls[0].ArrivalDelay)

	assert.Equal(t, models.DelayOnTime, calls[1].State)
	require.NotNil(t, calls[1].DepartureDelay)
	assert.Equal(t, 0, *calls[1].DepartureDelay)
}

func TestDecodeAlerts(t *testing.T) {
	//nolint:exhaustruct //only the fields under test are set
	msg := newFeedMessage(&gtfs.FeedEntity{
		Id: ptr("alert-1"),
		Alert: &gtfs.Alert{
			Cause:  gtfs.Alert_TECHNICAL_PROBLEM.Enum(),
			Effect: gtfs.Alert_SIGNIFICANT_DELAYS.Enum(),
			HeaderText: &gtfs.TranslatedString{
				Translation: []*gtfs.TranslatedString_Translation{
					{Text: ptr("Delays on line 50"), Language: ptr("en")},
				},
			},
			InformedEntity: []*gtfs.EntitySelector{
				{
					RouteId: ptr("route-1"),
					Trip:    &gtfs.TripDescriptor{TripId: ptr("trip-1")},
					StopId:  ptr("stop-1"),
				},
			},
		},
	})

	alerts, err := decodeAlerts(marshalFeed(t, msg))
	require.NoError(t, err)
	require.Len(t, alerts, 1)
	a := alerts[0]
	assert.Equal(t, "alert-1", a.ID)
	assert.Equal(t, "Delays on line 50", a.HeaderText)
	assert.Equal(t, []string{"trip-1"}, a.InformedTripIDs)
	assert.Equal(t, []string{"route-1"}, a.InformedRouteIDs)
	assert.Equal(t, []string{"stop-1"}, a.InformedStopIDs)
}

func TestDecodeTripUpdates_MalformedBody(t *testing.T) {
	_, err := decodeTripUpdates([]byte{0xff, 0xff, 0xff})
	require.Error(t, err)
}

// TestDecodeTripUpdates_ScheduledWithNoEvents covers a SCHEDULED (the
// default, unset value) stop-time update carrying neither an arrival nor a
// departure event at all — as distinct from NO_DATA, which is explicit.
func TestDecodeTripUpdates_ScheduledWithNoEvents(t *testing.T) {
	//nolint:exhaustruct //only the fields under test are set
	msg := newFeedMessage(&gtfs.FeedEntity{
		Id: ptr("e1"),
		TripUpdate: &gtfs.TripUpdate{
			Trip:      &gtfs.TripDescriptor{TripId: ptr("trip-1")},
			Timestamp: ptr(uint64(1_700_000_000)),
			StopTimeUpdate: []*gtfs.TripUpdate_StopTimeUpdate{
				{StopId: ptr("stop-1")},
			},
		},
	})

	trips, err := decodeTripUpdates(marshalFeed(t, msg))
	require.NoError(t, err)
	trip := trips["trip-1"]
	assert.Equal(t, models.DelayUnknown, trip.StopCalls[0].State)
	assert.Nil(t, trip.StopCalls[0].ArrivalDelay)
	assert.False(t, trip.Timestamp.IsZero())
}

func TestDecodeTripUpdates_NoTimestamp(t *testing.T) {
	//nolint:exhaustruct //only the fields under test are set
	msg := newFeedMessage(&gtfs.FeedEntity{
		Id: ptr("e1"),
		TripUpdate: &gtfs.TripUpdate{
			Trip: &gtfs.TripDescriptor{TripId: ptr("trip-1")},
		},
	})

	trips, err := decodeTripUpdates(marshalFeed(t, msg))
	require.NoError(t, err)
	assert.True(t, trips["trip-1"].Timestamp.IsZero())
}
