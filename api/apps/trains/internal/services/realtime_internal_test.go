package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/MobilityData/gtfs-realtime-bindings/golang/gtfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/trains/internal/mocks"
	"tools.xdoubleu.com/apps/trains/internal/models"
	"tools.xdoubleu.com/apps/trains/pkg/bmc"
	"tools.xdoubleu.com/internal/logging"
)

// stubTripResolver stands in for repositories.FeedRepository.ShortNamesByTripIDs,
// mapping the fixture trip_ids these tests publish to a static trip_short_name.
type stubTripResolver map[string]string

func (s stubTripResolver) ShortNamesByTripIDs(
	_ context.Context, ids []string,
) (map[string]string, error) {
	out := make(map[string]string, len(ids))
	for _, id := range ids {
		if name, ok := s[id]; ok {
			out[id] = name
		}
	}
	return out, nil
}

//nolint:gochecknoglobals //fixed test fixture
var testTripResolver = stubTripResolver{"trip-1": "L1", "trip-2": "L2"}

func tripUpdateBody(t *testing.T, tripID string) []byte {
	t.Helper()
	//nolint:exhaustruct //only the fields under test are set
	msg := newFeedMessage(&gtfs.FeedEntity{
		Id: ptr("e1"),
		TripUpdate: &gtfs.TripUpdate{
			Trip: &gtfs.TripDescriptor{TripId: ptr(tripID)},
		},
	})
	return marshalFeed(t, msg)
}

func alertBody(t *testing.T, id string) []byte {
	t.Helper()
	//nolint:exhaustruct //only the fields under test are set
	msg := newFeedMessage(&gtfs.FeedEntity{
		Id:    ptr(id),
		Alert: &gtfs.Alert{},
	})
	return marshalFeed(t, msg)
}

func newTestMock(t *testing.T, tripID, alertID string) *mocks.MockBMCClient {
	t.Helper()
	//nolint:exhaustruct //Result/Err/Calls/RealtimeErr not needed here
	return &mocks.MockBMCClient{
		RealtimeResults: map[string]*bmc.RealtimeResult{
			bmc.FeedTripUpdate: {Body: tripUpdateBody(t, tripID)},
			bmc.FeedAlert:      {Body: alertBody(t, alertID)},
		},
	}
}

func TestRealtimeService_Poll_BuildsSnapshot(t *testing.T) {
	m := newTestMock(t, "trip-1", "alert-1")
	svc := NewRealtimeService(logging.NewNopLogger(), m, testTripResolver)

	require.NoError(t, svc.Poll(context.Background()))

	snap := svc.Snapshot()
	require.Len(t, snap.Trips, 1)
	assert.Equal(t, 0, snap.UnresolvedTripCount)
	_, ok := snap.CallFor("L1", time.Now().In(brusselsLoc))
	assert.True(t, ok, "trip-1 should resolve to short name L1")
	require.Len(t, snap.Alerts, 1)
	assert.Equal(t, "alert-1", snap.Alerts[0].ID)
	assert.False(t, snap.FetchedAt.IsZero())

	// The alert feed is only polled every alertPollEvery cycles — the first
	// poll always fetches it, but a second immediate poll should not.
	require.NoError(t, svc.Poll(context.Background()))
	assert.Len(t, m.RealtimeCalls, 3) // trip,alert,trip
}

func TestRealtimeService_Poll_KeepsPriorAlertsBetweenAlertPolls(t *testing.T) {
	m := newTestMock(t, "trip-1", "alert-1")
	svc := NewRealtimeService(logging.NewNopLogger(), m, testTripResolver)

	require.NoError(t, svc.Poll(context.Background()))
	require.NoError(t, svc.Poll(context.Background()))

	snap := svc.Snapshot()
	require.Len(t, snap.Alerts, 1)
	assert.Equal(t, "alert-1", snap.Alerts[0].ID)
}

func TestRealtimeService_Poll_RateLimitedIsNotAnError(t *testing.T) {
	m := &mocks.MockBMCClient{ //nolint:exhaustruct //only RealtimeErr set
		RealtimeErr: &bmc.RateLimitedError{RetryAfter: 5 * time.Second},
	}
	svc := NewRealtimeService(logging.NewNopLogger(), m, testTripResolver)

	err := svc.Poll(context.Background())
	require.NoError(t, err)
	assert.Nil(t, svc.Snapshot().Trips)
}

func TestRealtimeService_Poll_UpstreamServerErrorIsNotAnError(t *testing.T) {
	m := &mocks.MockBMCClient{ //nolint:exhaustruct //only RealtimeErr set
		RealtimeErr: &bmc.UpstreamError{StatusCode: 503},
	}
	svc := NewRealtimeService(logging.NewNopLogger(), m, testTripResolver)

	require.NoError(t, svc.Poll(context.Background()))
}

func TestRealtimeService_Poll_UpstreamClientErrorFails(t *testing.T) {
	m := &mocks.MockBMCClient{ //nolint:exhaustruct //only RealtimeErr set
		RealtimeErr: &bmc.UpstreamError{StatusCode: 400},
	}
	svc := NewRealtimeService(logging.NewNopLogger(), m, testTripResolver)

	require.Error(t, svc.Poll(context.Background()))
}

func TestRealtimeService_Poll_DecodeFailureSurfaces(t *testing.T) {
	m := &mocks.MockBMCClient{ //nolint:exhaustruct //only RealtimeResults set
		RealtimeResults: map[string]*bmc.RealtimeResult{
			bmc.FeedTripUpdate: {Body: []byte{0xff, 0xff, 0xff}},
		},
	}
	svc := NewRealtimeService(logging.NewNopLogger(), m, testTripResolver)

	require.Error(t, svc.Poll(context.Background()))
}

func TestRealtimeService_Poll_AlertFetchClientErrorFails(t *testing.T) {
	m := &mocks.MockBMCClient{ //nolint:exhaustruct //only fields under test are set
		RealtimeResults: map[string]*bmc.RealtimeResult{
			bmc.FeedTripUpdate: {Body: tripUpdateBody(t, "trip-1")},
		},
		RealtimeErr: nil,
	}
	// The alert feed always 400s; the trip-update feed always succeeds by
	// returning a plain result — force the alert branch to hit its error
	// path on the very first poll (which always fetches alerts).
	m.RealtimeResults[bmc.FeedAlert] = nil
	svc := NewRealtimeService(logging.NewNopLogger(), &alertErroringMock{
		MockBMCClient: m,
		alertErr:      &bmc.UpstreamError{StatusCode: 400},
	}, testTripResolver)

	require.Error(t, svc.Poll(context.Background()))
}

// alertErroringMock lets the alert feed fail independently of the
// trip-update feed, which mocks.MockBMCClient's single RealtimeErr can't
// express.
type alertErroringMock struct {
	*mocks.MockBMCClient
	alertErr error
}

func (m *alertErroringMock) FetchRealtime(
	ctx context.Context,
	feed string,
) (*bmc.RealtimeResult, error) {
	if feed == bmc.FeedAlert {
		return nil, m.alertErr
	}
	return m.MockBMCClient.FetchRealtime(ctx, feed)
}

// TestRealtimeService_Poll_FiresOnUpdateListeners covers the hook
// JourneyWSService's PushAll is wired through (issue #1394): every
// registered listener must run once per successful poll, after the
// snapshot is already swapped in.
func TestRealtimeService_Poll_FiresOnUpdateListeners(t *testing.T) {
	m := newTestMock(t, "trip-1", "alert-1")
	svc := NewRealtimeService(logging.NewNopLogger(), m, testTripResolver)

	var calls int
	svc.OnUpdate(func() { calls++ })
	svc.OnUpdate(func() { calls++ })

	require.NoError(t, svc.Poll(context.Background()))
	assert.Equal(t, 2, calls)

	require.NoError(t, svc.Poll(context.Background()))
	assert.Equal(t, 4, calls)
}

func TestCorrelateTripUpdates(t *testing.T) {
	raw := map[string]models.TripUpdate{
		//nolint:exhaustruct //only the fields under test are set
		"rt-a": {TripID: "rt-a", StartDate: "20240301"},
		//nolint:exhaustruct //an undated update falls back to fallbackDate
		"rt-b": {TripID: "rt-b"},
		//nolint:exhaustruct //no static match -> counted, not stored
		"rt-ghost": {TripID: "rt-ghost"},
	}
	shortNames := map[string]string{"rt-a": "IC100", "rt-b": "IC200"}

	trips, unresolved := correlateTripUpdates(raw, shortNames, "20240401")

	assert.Equal(t, 1, unresolved)
	require.Len(t, trips, 2)
	_, hasDated := trips[models.TripKey{ShortName: "IC100", Date: "20240301"}]
	assert.True(t, hasDated, "a dated update keeps its own start_date")
	_, hasFallback := trips[models.TripKey{ShortName: "IC200", Date: "20240401"}]
	assert.True(t, hasFallback, "an undated update takes the fallback date")
}

func TestIsBackoffable(t *testing.T) {
	assert.True(t, isBackoffable(&bmc.RateLimitedError{RetryAfter: time.Second}))
	assert.True(t, isBackoffable(&bmc.UpstreamError{StatusCode: 502}))
	assert.False(t, isBackoffable(&bmc.UpstreamError{StatusCode: 404}))
	assert.False(t, isBackoffable(errors.New("boom")))
}
