package observability

import (
	"bytes"
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const sampleHeapProfile = `heap profile: 3: 300 [10: 900] @ heap/1048576
1: 100 [2: 200] @ 0x1 0x2
#	0x1	pkg.small+0x10	/src/small.go:10
#	0x2	pkg.caller+0x20	/src/caller.go:20

1: 5000 [4: 9000] @ 0x3 0x4
#	0x3	pkg.big+0x30	/src/big.go:30
#	0x4	pkg.bigcaller+0x40	/src/bigcaller.go:40

# runtime.MemStats
# Alloc = 123
`

func TestTopHeapStacks_OrdersByInUseBytes(t *testing.T) {
	got := topHeapStacks(sampleHeapProfile, 1)

	assert.Equal(t, []string{
		"5000: pkg.big+0x30 /src/big.go:30 < pkg.bigcaller+0x40 /src/bigcaller.go:40",
	}, got)
}

func TestTopHeapStacks_CapsAtAvailableStacks(t *testing.T) {
	got := topHeapStacks(sampleHeapProfile, 10)

	require.Len(t, got, 2)
	assert.Contains(t, got[1], "pkg.small")
}

func TestTopHeapStacks_IgnoresGarbage(t *testing.T) {
	assert.Empty(t, topHeapStacks("not a profile\n# stray\n", 3))
}

func TestWatchHeap_LogsOnceAboveThreshold(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	done := make(chan struct{})
	go func() {
		WatchHeap(ctx, logger, 1, time.Millisecond)
		close(done)
	}()

	select {
	case <-done:
	case <-ctx.Done():
		t.Fatal("WatchHeap did not return after logging")
	}
	assert.Contains(t, buf.String(), "heap watchdog: heap in use above threshold")
}

func TestWatchHeap_StopsOnCancelBelowThreshold(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		WatchHeap(ctx, logger, 1<<62, time.Millisecond)
		close(done)
	}()
	cancel()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("WatchHeap ignored cancellation")
	}
	assert.Empty(t, buf.String())
}
