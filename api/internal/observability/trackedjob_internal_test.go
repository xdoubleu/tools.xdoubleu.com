package observability

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xdoubleu/essentia/v4/pkg/logging"
)

// fakeTransport captures every event sentry.Client would otherwise send, so
// tests can assert on the check-ins TrackedJob produces without a network
// call.
type fakeTransport struct {
	events []*sentry.Event
}

func (f *fakeTransport) Flush(time.Duration) bool              { return true }
func (f *fakeTransport) FlushWithContext(context.Context) bool { return true }
func (f *fakeTransport) Configure(sentry.ClientOptions)        {}
func (f *fakeTransport) Close()                                {}
func (f *fakeTransport) SendEvent(event *sentry.Event) {
	f.events = append(f.events, event)
}

// checkIns returns the CheckIn payload of every captured event, in order.
func (f *fakeTransport) checkIns() []*sentry.CheckIn {
	checkIns := make([]*sentry.CheckIn, 0, len(f.events))
	for _, e := range f.events {
		checkIns = append(checkIns, e.CheckIn)
	}
	return checkIns
}

// testHubContext builds a context carrying a hub bound to a fake transport,
// so TrackedJob's check-ins are captured instead of sent over the network.
func testHubContext(t *testing.T) (context.Context, *fakeTransport) {
	t.Helper()
	transport := &fakeTransport{events: nil}
	//nolint:exhaustruct // only Dsn/Transport matter for this test client
	client, err := sentry.NewClient(sentry.ClientOptions{
		Dsn:       "https://public@example.com/1",
		Transport: transport,
	})
	require.NoError(t, err)
	hub := sentry.NewHub(client, sentry.NewScope())
	return sentry.SetHubOnContext(t.Context(), hub), transport
}

type fakeJob struct {
	err    error
	panics bool
}

func (f fakeJob) ID() string { return "fake" }

func (f fakeJob) RunEvery() time.Duration { return time.Hour }

func (f fakeJob) Run(_ context.Context, _ *slog.Logger) error {
	if f.panics {
		panic("kaboom")
	}
	return f.err
}

func TestTrackedJobDelegates(t *testing.T) {
	job := NewTrackedJob(fakeJob{err: nil, panics: false})

	assert.Equal(t, "fake", job.ID())
	assert.Equal(t, time.Hour, job.RunEvery())
}

func TestTrackedJobRecordsSuccess(t *testing.T) {
	ctx, transport := testHubContext(t)
	job := NewTrackedJob(fakeJob{err: nil, panics: false})

	err := job.Run(ctx, logging.NewNopLogger())
	require.NoError(t, err)

	checkIns := transport.checkIns()
	require.Len(t, checkIns, 2, "in_progress, then ok")
	assert.Equal(t, sentry.CheckInStatusInProgress, checkIns[0].Status)
	assert.Equal(t, sentry.CheckInStatusOK, checkIns[1].Status)
	assert.Equal(t, "fake", checkIns[1].MonitorSlug)
	assert.Equal(
		t,
		checkIns[0].ID,
		checkIns[1].ID,
		"second check-in must link to the first",
	)
}

func TestTrackedJobRecordsFailure(t *testing.T) {
	ctx, transport := testHubContext(t)
	job := NewTrackedJob(fakeJob{err: errors.New("boom"), panics: false})

	err := job.Run(ctx, logging.NewNopLogger())
	require.EqualError(t, err, "boom")

	checkIns := transport.checkIns()
	require.Len(t, checkIns, 2)
	assert.Equal(t, sentry.CheckInStatusError, checkIns[1].Status)
}

func TestTrackedJobRecoversPanic(t *testing.T) {
	ctx, transport := testHubContext(t)
	job := NewTrackedJob(fakeJob{err: nil, panics: true})

	err := job.Run(ctx, logging.NewNopLogger())
	require.ErrorContains(t, err, "kaboom")

	checkIns := transport.checkIns()
	require.Len(t, checkIns, 2)
	assert.Equal(t, sentry.CheckInStatusError, checkIns[1].Status)
}

func TestTrackedJobNoHubInContext(t *testing.T) {
	job := NewTrackedJob(fakeJob{err: nil, panics: false})

	// No hub on ctx and no global client configured: CaptureCheckIn is a
	// no-op both times, including the nil-ID second call — must not panic.
	err := job.Run(context.Background(), logging.NewNopLogger())
	require.NoError(t, err)
}
