package jobs_test

import (
	"bytes"
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/books/internal/jobs"
	"tools.xdoubleu.com/apps/books/pkg/objectstore"
	"tools.xdoubleu.com/internal/logging"
	"tools.xdoubleu.com/internal/models"
)

type fakeKeyLister struct {
	keys []string
	err  error
}

func (f fakeKeyLister) AllStorageKeys(_ context.Context) ([]string, error) {
	return f.keys, f.err
}

type fakeSnapshotStore struct {
	saved *models.StorageSnapshot
	err   error
}

func (f *fakeSnapshotStore) Insert(
	_ context.Context,
	snap models.StorageSnapshot,
) error {
	if f.err != nil {
		return f.err
	}
	f.saved = &snap
	return nil
}

func put(t *testing.T, store *objectstore.FakeClient, key, data string) {
	t.Helper()
	require.NoError(t, store.Put(
		t.Context(), key,
		bytes.NewReader([]byte(data)), int64(len(data)), "application/octet-stream",
	))
}

func TestStorageScanClassifiesObjects(t *testing.T) {
	store := objectstore.NewFake()
	// Referenced book file.
	put(t, store, "books/b1/abc.epub", "epubcontent")
	// Cover cache + negative marker — never orphans even if unreferenced.
	put(t, store, "books/b1/cover.jpg", "img")
	put(t, store, "books/b2/cover.missing", "")
	// Orphaned book object — under books/ but not referenced.
	put(t, store, "books/b3/orphan.epub", "leaked")
	// Fresh temp upload — not stale.
	put(t, store, "users/u1/uploads/fresh.epub", "fresh")
	// Stale temp upload — older than the 7-day threshold.
	store.PutAt(
		"users/u1/uploads/stale.epub",
		[]byte("staledata"),
		time.Now().Add(-30*24*time.Hour),
	)

	snapStore := &fakeSnapshotStore{saved: nil, err: nil}
	job := jobs.NewStorageScanJob(
		store,
		fakeKeyLister{keys: []string{"books/b1/abc.epub"}, err: nil},
		snapStore,
	)

	err := job.Run(t.Context(), logging.NewNopLogger())
	require.NoError(t, err)

	snap := snapStore.saved
	require.NotNil(t, snap)
	assert.Equal(t, int64(6), snap.ObjectCount)
	// Only books/b3/orphan.epub is an orphan.
	assert.Equal(t, int64(1), snap.OrphanCount)
	assert.Equal(t, int64(len("leaked")), snap.OrphanSizeBytes)
	assert.Equal(t, []string{"books/b3/orphan.epub"}, snap.OrphanKeys)
	// Only the stale upload counts.
	assert.Equal(t, int64(1), snap.StaleUploadCount)
	assert.Equal(t, int64(len("staledata")), snap.StaleUploadSizeBytes)

	// Prefix breakdown covers both top-level prefixes.
	prefixes := map[string]int64{}
	for _, p := range snap.PrefixBreakdown {
		prefixes[p.Prefix] = p.Count
	}
	assert.Equal(t, int64(4), prefixes["books"])
	assert.Equal(t, int64(2), prefixes["users"])
}

func TestStorageScanIDAndSchedule(t *testing.T) {
	job := jobs.NewStorageScanJob(
		objectstore.NewFake(),
		fakeKeyLister{keys: nil, err: nil},
		&fakeSnapshotStore{saved: nil, err: nil},
	)
	assert.Equal(t, "books-storage-scan", job.ID())
	assert.Equal(t, 24*time.Hour, job.RunEvery())
}

func TestStorageScanOrphanKeysCapped(t *testing.T) {
	store := objectstore.NewFake()
	for i := range 60 {
		put(t, store, "books/b/"+strconv.Itoa(i)+".epub", "leaked")
	}

	snapStore := &fakeSnapshotStore{saved: nil, err: nil}
	job := jobs.NewStorageScanJob(
		store,
		fakeKeyLister{keys: nil, err: nil},
		snapStore,
	)

	require.NoError(t, job.Run(t.Context(), logging.NewNopLogger()))
	snap := snapStore.saved
	require.NotNil(t, snap)
	// Every orphan is counted, but the retained key list is capped.
	assert.Equal(t, int64(60), snap.OrphanCount)
	assert.Len(t, snap.OrphanKeys, 50)
}

func TestStorageScanDeletesGracedOrphans(t *testing.T) {
	store := objectstore.NewFake()
	// Orphan old enough to clear the grace period — should be deleted.
	store.PutAt(
		"books/b1/graced-orphan.epub",
		[]byte("leaked"),
		time.Now().Add(-2*time.Hour),
	)

	snapStore := &fakeSnapshotStore{saved: nil, err: nil}
	job := jobs.NewStorageScanJob(
		store,
		fakeKeyLister{keys: nil, err: nil},
		snapStore,
	)

	require.NoError(t, job.Run(t.Context(), logging.NewNopLogger()))

	snap := snapStore.saved
	require.NotNil(t, snap)
	assert.Equal(t, int64(1), snap.OrphanCount)
	assert.Equal(t, int64(1), snap.DeletedOrphanCount)
	assert.Equal(t, int64(len("leaked")), snap.DeletedOrphanSizeBytes)

	_, stillExists := store.GetContent("books/b1/graced-orphan.epub")
	assert.False(
		t,
		stillExists,
		"graced orphan should have been deleted from the store",
	)
}

func TestStorageScanKeepsFreshOrphans(t *testing.T) {
	store := objectstore.NewFake()
	// Fresh orphan, e.g. an in-flight upload whose book_files row hasn't
	// committed yet — must never be deleted within the grace period.
	put(t, store, "books/b1/fresh-orphan.epub", "leaked")

	snapStore := &fakeSnapshotStore{saved: nil, err: nil}
	job := jobs.NewStorageScanJob(
		store,
		fakeKeyLister{keys: nil, err: nil},
		snapStore,
	)

	require.NoError(t, job.Run(t.Context(), logging.NewNopLogger()))

	snap := snapStore.saved
	require.NotNil(t, snap)
	assert.Equal(t, int64(1), snap.OrphanCount)
	assert.Equal(t, int64(0), snap.DeletedOrphanCount)

	_, stillExists := store.GetContent("books/b1/fresh-orphan.epub")
	assert.True(t, stillExists, "fresh orphan must survive within the grace period")
}

func TestStorageScanOrphanDeleteFailureIsLoggedNotFatal(t *testing.T) {
	objectstore.SetBackoffBase(time.Millisecond)
	store := objectstore.NewFake()
	store.PutAt(
		"books/b1/graced-orphan.epub",
		[]byte("leaked"),
		time.Now().Add(-2*time.Hour),
	)
	// More failures than DeleteWithRetry's own attempt budget, so every
	// retry is exhausted and the delete is left genuinely failed.
	store.FailNextDeletes(10, assert.AnError)

	snapStore := &fakeSnapshotStore{saved: nil, err: nil}
	job := jobs.NewStorageScanJob(
		store,
		fakeKeyLister{keys: nil, err: nil},
		snapStore,
	)

	require.NoError(t, job.Run(t.Context(), logging.NewNopLogger()))

	snap := snapStore.saved
	require.NotNil(t, snap)
	assert.Equal(t, int64(1), snap.OrphanCount)
	assert.Equal(t, int64(0), snap.DeletedOrphanCount)

	_, stillExists := store.GetContent("books/b1/graced-orphan.epub")
	assert.True(
		t,
		stillExists,
		"a persistently-failing delete must leave the object in place, not silently drop it",
	)
}

func TestStorageScanEmptyBucket(t *testing.T) {
	snapStore := &fakeSnapshotStore{saved: nil, err: nil}
	job := jobs.NewStorageScanJob(
		objectstore.NewFake(),
		fakeKeyLister{keys: nil, err: nil},
		snapStore,
	)

	require.NoError(t, job.Run(t.Context(), logging.NewNopLogger()))
	require.NotNil(t, snapStore.saved)
	assert.Equal(t, int64(0), snapStore.saved.ObjectCount)
	assert.Empty(t, snapStore.saved.PrefixBreakdown)
}
