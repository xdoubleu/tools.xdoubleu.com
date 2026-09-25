package services

import (
	"context"
	"errors"
	"fmt"
	"net/url"
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

// stubTripResolver maps fixture trip_ids to trip_short_names.
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

// erroringTripResolver always fails with err.
type erroringTripResolver struct{ err error }

func (e erroringTripResolver) ShortNamesByTripIDs(
	_ context.Context, _ []string,
) (map[string]string, error) {
	return nil, e.err
}

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

// newTestMock publishes "trip-1"/"alert-1" fixtures.
func newTestMock(t *testing.T) *mocks.MockBMCClient {
	t.Helper()
	//nolint:exhaustruct //Result/Err/Calls/RealtimeErr not needed here
	return &mocks.MockBMCClient{
		RealtimeResults: map[string]*bmc.RealtimeResult{
			bmc.FeedTripUpdate: {Body: tripUpdateBody(t, "trip-1")},
			bmc.FeedAlert:      {Body: alertBody(t, "alert-1")},
		},
	}
}

func TestRealtimeService_Poll_BuildsSnapshot(t *testing.T) {
	m := newTestMock(t)
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

	// Alerts are fetched on the first poll only, not an immediate second one.
	require.NoError(t, svc.Poll(context.Background()))
	assert.Len(t, m.RealtimeCalls, 3) // trip,alert,trip
}

func TestRealtimeService_Poll_KeepsPriorAlertsBetweenAlertPolls(t *testing.T) {
	m := newTestMock(t)
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

func TestRealtimeService_Poll_UnexpectedContentTypeIsNotAnError(t *testing.T) {
	m := &mocks.MockBMCClient{ //nolint:exhaustruct //only RealtimeErr set
		RealtimeErr: &bmc.UnexpectedContentTypeError{
			Feed:        bmc.FeedTripUpdate,
			ContentType: "text/html; charset=utf-8",
		},
	}
	svc := NewRealtimeService(logging.NewNopLogger(), m, testTripResolver)

	err := svc.Poll(context.Background())
	require.NoError(t, err)
	assert.Nil(t, svc.Snapshot().Trips)
}

// TestRealtimeService_Poll_TimeoutIsNotAnError: a client timeout backs off
// rather than failing the job.
func TestRealtimeService_Poll_TimeoutIsNotAnError(t *testing.T) {
	m := &mocks.MockBMCClient{ //nolint:exhaustruct //only RealtimeErr set
		RealtimeErr: &url.Error{
			Op:  "Get",
			URL: "https://example.com/api/gtfs/feed/nmbssncb/rt/trip-update",
			Err: context.DeadlineExceeded,
		},
	}
	svc := NewRealtimeService(logging.NewNopLogger(), m, testTripResolver)

	require.NoError(t, svc.Poll(context.Background()))
	assert.Nil(t, svc.Snapshot().Trips)
}

// TestRealtimeService_Poll_TripResolveTimeoutIsNotAnError: a resolve cut off
// by the poll deadline backs off, keeping the previous snapshot.
func TestRealtimeService_Poll_TripResolveTimeoutIsNotAnError(t *testing.T) {
	m := newTestMock(t)
	svc := NewRealtimeService(
		logging.NewNopLogger(), m,
		erroringTripResolver{err: context.DeadlineExceeded},
	)

	require.NoError(t, svc.Poll(context.Background()))
	assert.Nil(
		t,
		svc.Snapshot().Trips,
		"the (empty) prior snapshot must be kept, not overwritten",
	)
}

// TestRealtimeService_Poll_TripResolveNonTimeoutErrorFails: other resolve
// errors still fail the job.
func TestRealtimeService_Poll_TripResolveNonTimeoutErrorFails(t *testing.T) {
	m := newTestMock(t)
	svc := NewRealtimeService(
		logging.NewNopLogger(), m,
		erroringTripResolver{err: errors.New("db: connection refused")},
	)

	require.Error(t, svc.Poll(context.Background()))
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
	// Force the alert fetch's error path on the first poll.
	m.RealtimeResults[bmc.FeedAlert] = nil
	svc := NewRealtimeService(logging.NewNopLogger(), &alertErroringMock{
		MockBMCClient: m,
		alertErr:      &bmc.UpstreamError{StatusCode: 400},
	}, testTripResolver)

	require.Error(t, svc.Poll(context.Background()))
}

// alertErroringMock fails the alert feed independently of trip updates.
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

// TestRealtimeService_Poll_FiresOnUpdateListeners: each listener runs once
// per successful poll, after the snapshot swap.
func TestRealtimeService_Poll_FiresOnUpdateListeners(t *testing.T) {
	m := newTestMock(t)
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
	assert.True(t, isBackoffable(&bmc.UnexpectedContentTypeError{
		Feed:        bmc.FeedTripUpdate,
		ContentType: "text/html; charset=utf-8",
	}))
	assert.False(t, isBackoffable(errors.New("boom")))

	// A client timeout (*url.Error with Timeout() true) backs off.
	timeoutErr := &url.Error{
		Op:  "Get",
		URL: "https://example.com",
		Err: context.DeadlineExceeded,
	}
	assert.True(t, isBackoffable(timeoutErr))

	// A non-timeout network error does not.
	nonTimeoutErr := &url.Error{
		Op:  "Get",
		URL: "https://example.com",
		Err: errors.New("connection refused"),
	}
	assert.False(t, isBackoffable(nonTimeoutErr))

	// pgx surfaces a query cut off by pollTimeout as a bare DeadlineExceeded.
	assert.True(t, isBackoffable(context.DeadlineExceeded))
	assert.True(t, isBackoffable(fmt.Errorf("query: %w", context.DeadlineExceeded)))
}
