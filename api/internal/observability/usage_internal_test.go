package observability

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xdoubleu/essentia/v4/pkg/logging"

	"tools.xdoubleu.com/internal/models"
)

type fakeUsageStore struct {
	flushed  []models.UsageEntry
	flushErr error
	pruned   int
}

func (f *fakeUsageStore) Flush(
	_ context.Context,
	entries []models.UsageEntry,
) error {
	if f.flushErr != nil {
		return f.flushErr
	}
	f.flushed = append(f.flushed, entries...)
	return nil
}

func (f *fakeUsageStore) PruneOlderThan(_ context.Context, _ time.Time) error {
	f.pruned++
	return nil
}

func newTestRecorder(store usageStore) *UsageRecorder {
	//nolint:exhaustruct //mu and lastPrune start zero-valued on purpose
	return &UsageRecorder{
		logger: logging.NewNopLogger(),
		repo:   store,
		counts: make(map[usageKey]int64),
	}
}

func TestUsageRecorderAccumulatesAndFlushes(t *testing.T) {
	store := &fakeUsageStore{flushed: nil, flushErr: nil, pruned: 0}
	rec := newTestRecorder(store)

	rec.Record("books", "LibraryService/ListBooks")
	rec.Record("books", "LibraryService/ListBooks")
	rec.Record("games", "GamesService/ListGames")

	require.NoError(t, rec.Flush(t.Context()))

	require.Len(t, store.flushed, 2)
	counts := map[string]int64{}
	for _, e := range store.flushed {
		counts[e.App+":"+e.Endpoint] = e.Count
	}
	assert.Equal(t, int64(2), counts["books:LibraryService/ListBooks"])
	assert.Equal(t, int64(1), counts["games:GamesService/ListGames"])
}

func TestUsageRecorderFlushClearsCounts(t *testing.T) {
	store := &fakeUsageStore{flushed: nil, flushErr: nil, pruned: 0}
	rec := newTestRecorder(store)

	rec.Record("todos", "root")
	require.NoError(t, rec.Flush(t.Context()))
	require.Len(t, store.flushed, 1)

	// A second flush with no new records must not re-send.
	store.flushed = nil
	require.NoError(t, rec.Flush(t.Context()))
	assert.Empty(t, store.flushed)
}

func TestUsageRecorderRestoresBatchOnFlushError(t *testing.T) {
	store := &fakeUsageStore{
		flushed:  nil,
		flushErr: errors.New("db down"),
		pruned:   0,
	}
	rec := newTestRecorder(store)

	rec.Record("books", "root")
	require.Error(t, rec.Flush(t.Context()))

	// Recover the store and flush again; the count must survive.
	store.flushErr = nil
	require.NoError(t, rec.Flush(t.Context()))
	require.Len(t, store.flushed, 1)
	assert.Equal(t, int64(1), store.flushed[0].Count)
}

func TestUsageRecorderPrunesOncePerInterval(t *testing.T) {
	store := &fakeUsageStore{flushed: nil, flushErr: nil, pruned: 0}
	rec := newTestRecorder(store)

	require.NoError(t, rec.Flush(t.Context()))
	require.NoError(t, rec.Flush(t.Context()))

	// Two flushes back-to-back should prune only once.
	assert.Equal(t, 1, store.pruned)
}
