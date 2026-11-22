package threading_test

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/kit/logging"
	"tools.xdoubleu.com/internal/kit/threading"
)

func TestWorkerPoolRunsWorkAndStops(t *testing.T) {
	t.Parallel()

	pool := threading.NewWorkerPool(context.Background(), logging.NewNopLogger(), 1, 1)
	assert.Eventually(t, pool.Active, time.Second, 10*time.Millisecond)

	done := make(chan struct{})
	pool.EnqueueWork(func(_ context.Context, _ *slog.Logger) error {
		close(done)
		return nil
	})

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("work was never executed")
	}

	pool.WaitUntilDone()
	assert.False(t, pool.IsWorkRemaining())

	pool.Stop()
	assert.Eventually(t, func() bool {
		return !pool.Active()
	}, time.Second, 10*time.Millisecond)
}

func TestWorkerPoolRecoversPanickingWork(t *testing.T) {
	t.Parallel()

	var buf syncBuffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))
	pool := threading.NewWorkerPool(context.Background(), logger, 1, 2)

	pool.EnqueueWork(func(_ context.Context, _ *slog.Logger) error {
		panic("boom")
	})
	ran := make(chan struct{})
	pool.EnqueueWork(func(_ context.Context, _ *slog.Logger) error {
		close(ran)
		return nil
	})

	select {
	case <-ran:
	case <-time.After(time.Second):
		t.Fatal("the worker died with the panicking work")
	}
	require.NoError(t, pool.Wait(t.Context()))
	assert.Contains(t, buf.String(), "level=ERROR")
	assert.Contains(t, buf.String(), "boom")
}

func TestWorkerPoolWaitGivesUpWhenContextIsDone(t *testing.T) {
	t.Parallel()

	pool := threading.NewWorkerPool(context.Background(), logging.NewNopLogger(), 1, 1)
	release := make(chan struct{})
	defer close(release)
	pool.EnqueueWork(func(_ context.Context, _ *slog.Logger) error {
		<-release
		return nil
	})

	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Millisecond)
	defer cancel()
	require.ErrorIs(t, pool.Wait(ctx), context.DeadlineExceeded)
}

type recordHandler struct {
	slog.Handler
	records chan slog.Record
}

func (h recordHandler) Handle(_ context.Context, r slog.Record) error {
	h.records <- r
	return nil
}

func TestWorkerLogsFailedWorkWithErrorChain(t *testing.T) {
	t.Parallel()

	records := make(chan slog.Record, 1)
	logger := slog.New(recordHandler{
		Handler: slog.NewTextHandler(io.Discard, nil),
		records: records,
	})
	pool := threading.NewWorkerPool(context.Background(), logger, 1, 1)

	pool.EnqueueWork(func(_ context.Context, _ *slog.Logger) error {
		return fmt.Errorf("x: %w", context.Canceled)
	})

	select {
	case r := <-records:
		assert.Equal(t, "work item failed", r.Message)
		var logged error
		r.Attrs(func(a slog.Attr) bool {
			if err, ok := a.Value.Any().(error); ok && a.Key == "error" {
				logged = err
			}
			return true
		})
		require.ErrorIs(t, logged, context.Canceled)
	case <-time.After(time.Second):
		t.Fatal("failure was never logged")
	}
}
