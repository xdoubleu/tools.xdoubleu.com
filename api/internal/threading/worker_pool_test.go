package threading_test

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/logging"
	"tools.xdoubleu.com/internal/threading"
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
