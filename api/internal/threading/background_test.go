package threading_test

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/logging"
	internalsentry "tools.xdoubleu.com/internal/sentrytools"
	"tools.xdoubleu.com/internal/threading"
)

// Not parallel: points the global Sentry hub at a mocked transport.
func TestBackgroundRecoversAndReportsPanickingTask(t *testing.T) {
	require.NoError(t, sentry.Init(internalsentry.MockedSentryClientOptions()))
	var buf syncBuffer
	logger := slog.New(internalsentry.NewLogHandler(
		"test", slog.NewTextHandler(&buf, nil),
	))
	bg := threading.NewBackground(logger, 0)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	bg.Go(req.Context(), "kobo-cover-warm", func(context.Context) {
		panic("boom")
	})
	require.NoError(t, bg.Shutdown(t.Context()))

	assert.Contains(t, buf.String(), "level=ERROR")
	assert.Contains(t, buf.String(), "task=kobo-cover-warm")
	assert.Contains(t, buf.String(), "boom")

	events := internalsentry.MockedHubEvents(sentry.CurrentHub())
	require.Len(t, events, 1)
	assert.Equal(t, "kobo-cover-warm", events[0].Tags["task"])
	require.NotEmpty(t, events[0].Exception)
	assert.Contains(t, events[0].Exception[0].Value, "boom")
}

func TestBackgroundTaskOutlivesItsRequest(t *testing.T) {
	t.Parallel()

	bg := threading.NewBackground(logging.NewNopLogger(), 0)
	reqCtx, cancelReq := context.WithCancel(t.Context())
	started := make(chan struct{})
	var taskErr atomic.Value

	bg.Go(reqCtx, "outlives", func(ctx context.Context) {
		close(started)
		time.Sleep(20 * time.Millisecond)
		taskErr.Store(ctx.Err() == nil)
	})
	<-started
	cancelReq()

	require.NoError(t, bg.Shutdown(t.Context()))
	assert.Equal(t, true, taskErr.Load())
}

func TestBackgroundShutdownWaitsForRunningTasks(t *testing.T) {
	t.Parallel()

	bg := threading.NewBackground(logging.NewNopLogger(), 0)
	release := make(chan struct{})
	var finished atomic.Bool
	bg.Go(t.Context(), "slow", func(context.Context) {
		<-release
		finished.Store(true)
	})

	shutdown := make(chan error, 1)
	go func() { shutdown <- bg.Shutdown(t.Context()) }()

	select {
	case err := <-shutdown:
		t.Fatalf("Shutdown returned before the task finished: %v", err)
	case <-time.After(50 * time.Millisecond):
	}

	close(release)
	require.NoError(t, <-shutdown)
	assert.True(t, finished.Load())
}

func TestBackgroundShutdownCancelsTasksAtItsDeadline(t *testing.T) {
	t.Parallel()

	bg := threading.NewBackground(logging.NewNopLogger(), 0)
	cancelled := make(chan struct{})
	bg.Go(t.Context(), "stuck", func(ctx context.Context) {
		<-ctx.Done()
		close(cancelled)
	})

	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Millisecond)
	defer cancel()
	require.ErrorIs(t, bg.Shutdown(ctx), context.DeadlineExceeded)

	select {
	case <-cancelled:
	case <-time.After(time.Second):
		t.Fatal("the stuck task was never cancelled")
	}
}

func TestBackgroundShutdownDropsTasksStillWaitingForASlot(t *testing.T) {
	t.Parallel()

	bg := threading.NewBackground(logging.NewNopLogger(), 1)
	holding := make(chan struct{})
	bg.Go(t.Context(), "holder", func(ctx context.Context) {
		close(holding)
		<-ctx.Done()
	})
	<-holding
	var queuedRan atomic.Bool
	bg.Go(t.Context(), "queued", func(context.Context) { queuedRan.Store(true) })

	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Millisecond)
	defer cancel()
	require.ErrorIs(t, bg.Shutdown(ctx), context.DeadlineExceeded)

	require.NoError(t, bg.Shutdown(t.Context()))
	assert.False(t, queuedRan.Load())
}

func TestBackgroundLimitBoundsConcurrentTasks(t *testing.T) {
	t.Parallel()

	bg := threading.NewBackground(logging.NewNopLogger(), 2)
	var running, peak atomic.Int32
	for range 6 {
		bg.Go(t.Context(), "bounded", func(context.Context) {
			n := running.Add(1)
			for {
				p := peak.Load()
				if n <= p || peak.CompareAndSwap(p, n) {
					break
				}
			}
			time.Sleep(10 * time.Millisecond)
			running.Add(-1)
		})
	}

	require.NoError(t, bg.Shutdown(t.Context()))
	assert.Equal(t, int32(2), peak.Load())
}
