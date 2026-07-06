package jobs_test

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xdoubleu/essentia/v4/pkg/logging"

	"tools.xdoubleu.com/apps/books/internal/jobs"
	"tools.xdoubleu.com/apps/books/pkg/objectstore"
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
