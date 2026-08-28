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

	"tools.xdoubleu.com/internal/logging"
	"tools.xdoubleu.com/internal/models"
	"tools.xdoubleu.com/internal/sentrytools"
	"tools.xdoubleu.com/internal/threading"
)

// newSentryTestCtx returns a context carrying a hub whose transport captures
// every finished transaction event, and the transport to read them back
// from. TracesSampleRate/EnableTracing must both be set or sentry-go drops
// transactions before they ever reach the transport (see Span.sample).
func newSentryTestCtx(t *testing.T) (context.Context, *sentry.Hub) {
	t.Helper()

	client, err := sentry.NewClient(sentry.ClientOptions{
		Dsn:              "http://whatever@example.com/1337",
		EnableTracing:    true,
		TracesSampleRate: 1.0,
		Transport:        sentrytools.MockedSentryClientOptions().Transport,
	})
	require.NoError(t, err)

	hub := sentry.NewHub(client, sentry.NewScope())
	return sentry.SetHubOnContext(t.Context(), hub), hub
}

type fakeInserter struct {
	runs      []models.JobRun
	insertErr error
}

func (f *fakeInserter) Insert(_ context.Context, run models.JobRun) error {
	f.runs = append(f.runs, run)
	return f.insertErr
}

// fakeJob has no RunEvery method, so it's trigger-only — see threading.Scheduled.
type fakeJob struct {
	err    error
	panics bool
}

func (f fakeJob) ID() string { return "fake" }

func (f fakeJob) Run(_ context.Context, _ *slog.Logger) error {
	if f.panics {
		panic("kaboom")
	}
	return f.err
}

type fakeScheduledJob struct {
	fakeJob
}

func (f fakeScheduledJob) RunEvery() time.Duration { return time.Hour }

func newTestTrackedJob(inner threading.Job, repo *fakeInserter) *TrackedJob {
	return &TrackedJob{inner: inner, repo: repo}
}

func TestTrackedJobDelegates(t *testing.T) {
	job := newTestTrackedJob(
		fakeJob{err: nil, panics: false},
		&fakeInserter{runs: nil, insertErr: nil},
	)

	assert.Equal(t, "fake", job.ID())
}

func TestNewTrackedJob_ScheduledInnerStaysScheduled(t *testing.T) {
	job := NewTrackedJob(fakeScheduledJob{fakeJob{err: nil, panics: false}}, nil)

	scheduled, ok := job.(threading.Scheduled)
	require.True(t, ok)
	assert.Equal(t, time.Hour, scheduled.RunEvery())
	assert.Equal(t, "fake", job.ID())
}

func TestNewTrackedJob_TriggerOnlyInnerStaysTriggerOnly(t *testing.T) {
	job := NewTrackedJob(fakeJob{err: nil, panics: false}, nil)

	_, ok := job.(threading.Scheduled)
	assert.False(t, ok)
	assert.Equal(t, "fake", job.ID())
}

func TestTrackedJobRecordsSuccess(t *testing.T) {
	repo := &fakeInserter{runs: nil, insertErr: nil}
	job := newTestTrackedJob(fakeJob{err: nil, panics: false}, repo)

	err := job.Run(t.Context(), logging.NewNopLogger())
	require.NoError(t, err)

	require.Len(t, repo.runs, 1)
	assert.Equal(t, "fake", repo.runs[0].JobID)
	assert.True(t, repo.runs[0].Success)
	assert.Empty(t, repo.runs[0].Error)
	assert.WithinDuration(t, time.Now(), repo.runs[0].StartedAt, time.Second)
}

func TestTrackedJobRecordsFailure(t *testing.T) {
	repo := &fakeInserter{runs: nil, insertErr: nil}
	job := newTestTrackedJob(fakeJob{err: errors.New("boom"), panics: false}, repo)

	err := job.Run(t.Context(), logging.NewNopLogger())
	require.EqualError(t, err, "boom")

	require.Len(t, repo.runs, 1)
	assert.False(t, repo.runs[0].Success)
	assert.Equal(t, "boom", repo.runs[0].Error)
}

func TestTrackedJobRecoversPanic(t *testing.T) {
	repo := &fakeInserter{runs: nil, insertErr: nil}
	job := newTestTrackedJob(fakeJob{err: nil, panics: true}, repo)

	err := job.Run(t.Context(), logging.NewNopLogger())
	require.ErrorContains(t, err, "kaboom")

	require.Len(t, repo.runs, 1)
	assert.False(t, repo.runs[0].Success)
	assert.Contains(t, repo.runs[0].Error, "kaboom")
}

func TestTrackedJobRun_StartsAndFinishesTransactionNamedForJobOnSuccess(t *testing.T) {
	ctx, hub := newSentryTestCtx(t)
	repo := &fakeInserter{runs: nil, insertErr: nil}
	job := newTestTrackedJob(fakeJob{err: nil, panics: false}, repo)

	err := job.Run(ctx, logging.NewNopLogger())
	require.NoError(t, err)

	events := sentrytools.MockedHubEvents(hub)
	require.Len(t, events, 1)
	assert.Equal(t, "fake", events[0].Transaction)
	trace, ok := events[0].Contexts["trace"]
	require.True(t, ok)
	assert.Equal(t, sentry.SpanStatusOK, trace["status"])
}

func TestTrackedJobRun_StartsAndFinishesTransactionNamedForJobOnFailure(t *testing.T) {
	ctx, hub := newSentryTestCtx(t)
	repo := &fakeInserter{runs: nil, insertErr: nil}
	job := newTestTrackedJob(fakeJob{err: errors.New("boom"), panics: false}, repo)

	err := job.Run(ctx, logging.NewNopLogger())
	require.EqualError(t, err, "boom")

	events := sentrytools.MockedHubEvents(hub)
	require.Len(t, events, 1)
	assert.Equal(t, "fake", events[0].Transaction)
	trace, ok := events[0].Contexts["trace"]
	require.True(t, ok)
	assert.Equal(t, sentry.SpanStatusInternalError, trace["status"])
}

func TestTrackedJobInsertFailureDoesNotFailJob(t *testing.T) {
	repo := &fakeInserter{runs: nil, insertErr: errors.New("db down")}
	job := newTestTrackedJob(fakeJob{err: nil, panics: false}, repo)

	err := job.Run(t.Context(), logging.NewNopLogger())
	assert.NoError(t, err)
}
