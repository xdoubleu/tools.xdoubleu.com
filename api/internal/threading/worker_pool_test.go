package threading_test

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

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
