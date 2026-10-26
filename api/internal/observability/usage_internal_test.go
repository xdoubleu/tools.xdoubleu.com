package observability

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/logging"
	"tools.xdoubleu.com/internal/models"
)

type fakeUsageStore struct {
	mu       sync.Mutex
	flushed  []models.UsageEntry
	flushErr error
	pruned   int
}

// Flush fails on a cancelled ctx, as the real repository would.
func (f *fakeUsageStore) Flush(
	ctx context.Context,
	entries []models.UsageEntry,
) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	if f.flushErr != nil {
		return f.flushErr
	}
	f.flushed = append(f.flushed, entries...)
	return nil
}

func (f *fakeUsageStore) PruneOlderThan(_ context.Context, _ time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.pruned++
	return nil
}

func (f *fakeUsageStore) flushedLen() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.flushed)
}

func newFakeUsageStore() *fakeUsageStore {
	//nolint:exhaustruct //mu and slice/err start zero-valued on purpose
	return &fakeUsageStore{}
}

func newTestRecorder(store usageStore) *UsageRecorder {
	//nolint:exhaustruct //mu and lastPrune start zero-valued on purpose
	return &UsageRecorder{
		logger: logging.NewNopLogger(),
		repo:   store,
		counts: make(map[usageKey]usageCounter),
	}
}

func TestUsageRecorderAccumulatesAndFlushes(t *testing.T) {
	store := newFakeUsageStore()
	rec := newTestRecorder(store)

	rec.Record("books", "LibraryService/ListBooks", 100)
	rec.Record("books", "LibraryService/ListBooks", 250)
	rec.Record("games", "GamesService/ListGames", 40)

	require.NoError(t, rec.Flush(t.Context()))

	require.Len(t, store.flushed, 2)
	counts := map[string]int64{}
	bytes := map[string]int64{}
	for _, e := range store.flushed {
		counts[e.App+":"+e.Endpoint] = e.Count
		bytes[e.App+":"+e.Endpoint] = e.Bytes
	}
	assert.Equal(t, int64(2), counts["books:LibraryService/ListBooks"])
	assert.Equal(t, int64(1), counts["games:GamesService/ListGames"])
	assert.Equal(t, int64(350), bytes["books:LibraryService/ListBooks"])
	assert.Equal(t, int64(40), bytes["games:GamesService/ListGames"])
}

func TestUsageRecorderFlushClearsCounts(t *testing.T) {
	store := newFakeUsageStore()
	rec := newTestRecorder(store)

	rec.Record("recipes", "root", 0)
	require.NoError(t, rec.Flush(t.Context()))
	require.Len(t, store.flushed, 1)

	store.flushed = nil
	require.NoError(t, rec.Flush(t.Context()))
	assert.Empty(t, store.flushed)
}

func TestUsageRecorderRestoresBatchOnFlushError(t *testing.T) {
	store := newFakeUsageStore()
	store.flushErr = errors.New("db down")
	rec := newTestRecorder(store)

	rec.Record("books", "root", 0)
	require.Error(t, rec.Flush(t.Context()))

	store.flushErr = nil
	require.NoError(t, rec.Flush(t.Context()))
	require.Len(t, store.flushed, 1)
	assert.Equal(t, int64(1), store.flushed[0].Count)
}

func TestUsageRecorderPrunesOncePerInterval(t *testing.T) {
	store := newFakeUsageStore()
	rec := newTestRecorder(store)

	require.NoError(t, rec.Flush(t.Context()))
	require.NoError(t, rec.Flush(t.Context()))

	assert.Equal(t, 1, store.pruned)
}

func TestUsageRecorderFlushTickLogsErrorAndSurvives(t *testing.T) {
	store := newFakeUsageStore()
	store.flushErr = errors.New("db down")
	rec := newTestRecorder(store)

	rec.Record("books", "root", 0)
	rec.flushTick(t.Context())

	store.flushErr = nil
	require.NoError(t, rec.Flush(t.Context()))
	assert.Equal(t, 1, store.flushedLen())
}

func TestUsageRecorderStopFlushesCountsRecordedAfterTheRootContextEnds(t *testing.T) {
	store := newFakeUsageStore()
	rec := newTestRecorder(store)

	rootCtx, cancelRoot := context.WithCancel(context.Background())
	rec.Start(rootCtx, time.Hour)
	cancelRoot()
	// A request still draining after SIGTERM.
	rec.Record("books", "root", 0)

	require.NoError(t, rec.Stop(t.Context()))
	assert.Equal(t, 1, store.flushedLen())
}

func TestUsageRecorderTicksFlushAfterTheRootContextEnds(t *testing.T) {
	store := newFakeUsageStore()
	rec := newTestRecorder(store)

	rootCtx, cancelRoot := context.WithCancel(context.Background())
	cancelRoot()
	rec.Start(rootCtx, 5*time.Millisecond)
	rec.Record("books", "root", 0)

	require.Eventually(t, func() bool {
		return store.flushedLen() == 1
	}, time.Second, 5*time.Millisecond)
	require.NoError(t, rec.Stop(t.Context()))
}

func TestUsageRecorderStopWithoutStartIsANoop(t *testing.T) {
	store := newFakeUsageStore()
	rec := newTestRecorder(store)

	require.NoError(t, rec.Stop(t.Context()))
	assert.Equal(t, 0, store.pruned)
}
