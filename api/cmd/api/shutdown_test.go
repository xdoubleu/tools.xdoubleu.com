package main

import (
	"context"
	"syscall"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/logging"
)

func TestNewRootContext_CancelledOnSIGTERM(t *testing.T) {
	ctx, stop := newRootContext()
	defer stop()

	require.NoError(t, syscall.Kill(syscall.Getpid(), syscall.SIGTERM))

	select {
	case <-ctx.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("the root context outlived SIGTERM")
	}
}

func TestShutdownBudgetFitsKamalStopWait(t *testing.T) {
	t.Parallel()

	total := httpShutdownTimeout + drainTimeout + usageFlushTimeout +
		sentryFlushTimeout + logFlushTimeout
	assert.Less(t, total, kamalStopWait)
}

func TestApplicationShutdown_FlushesUsageRecordedWhileDraining(t *testing.T) {
	rootCtx, cancelRoot := context.WithCancel(context.Background())
	app := NewApplication(rootCtx, logging.NewNopLogger(), testApp.config, testApp.db)

	appName := "shutdown-test-" + uuid.NewString()
	t.Cleanup(func() {
		_, _ = testApp.db.Exec(context.Background(),
			"DELETE FROM global.usage_daily WHERE app = $1", appName)
	})

	cancelRoot()
	// A request still draining after SIGTERM.
	app.usage.Record(appName, "/drained", 42)

	require.NoError(t, app.Shutdown(context.Background()))

	var count, bytes int64
	require.NoError(t, testApp.db.QueryRow(t.Context(),
		"SELECT count, bytes FROM global.usage_daily WHERE app = $1", appName,
	).Scan(&count, &bytes))
	assert.Equal(t, int64(1), count)
	assert.Equal(t, int64(42), bytes)
}
