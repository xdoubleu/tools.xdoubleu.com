package jobs

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"tools.xdoubleu.com/apps/reading/pkg/objectstore"
	"tools.xdoubleu.com/internal/models"
)

const (
	// scanInterval is how often the whole bucket is walked.
	scanInterval = 24 * time.Hour
	// staleUploadAge is how old a temp upload must be before it counts as
	// leaked (the upload flow normally finalizes within minutes).
	staleUploadAge = 7 * 24 * time.Hour
	booksPrefix    = "books/"
	uploadsMarker  = "/uploads/"
	coverSuffix    = "/cover.jpg"
	coverMissing   = "/cover.missing"
)

// objectLister is the slice of objectstore.Client the scan needs.
type objectLister interface {
	List(ctx context.Context, prefix string) ([]objectstore.ObjectInfo, error)
}

// storageKeyLister returns the R2 keys referenced by book files.
type storageKeyLister interface {
	AllStorageKeys(ctx context.Context) ([]string, error)
}

// snapshotStore persists a completed scan.
type snapshotStore interface {
	Insert(ctx context.Context, snap models.StorageSnapshot) error
}

// StorageScanJob walks the whole object-store bucket once a day and records a
// snapshot (total size, per-prefix breakdown, and — by diffing against the
// book_files table — orphaned objects and stale temp uploads) so the admin
// dashboard can flag when manual cleanup is worthwhile.
type StorageScanJob struct {
	store     objectLister
	bookFiles storageKeyLister
	snapshots snapshotStore
}

func NewStorageScanJob(
	store objectLister,
	bookFiles storageKeyLister,
	snapshots snapshotStore,
) *StorageScanJob {
	return &StorageScanJob{
		store:     store,
		bookFiles: bookFiles,
		snapshots: snapshots,
	}
}

func (j *StorageScanJob) ID() string { return "books-storage-scan" }

func (j *StorageScanJob) RunEvery() time.Duration { return scanInterval }

func (j *StorageScanJob) Run(ctx context.Context, logger *slog.Logger) error {
	objects, err := j.store.List(ctx, "")
	if err != nil {
		return err
	}

	keys, err := j.bookFiles.AllStorageKeys(ctx)
	if err != nil {
		return err
	}
	referenced := make(map[string]bool, len(keys))
	for _, k := range keys {
		referenced[k] = true
	}

	snap := buildSnapshot(objects, referenced, time.Now())

	logger.InfoContext(ctx, "storage scan complete",
		slog.Int64("objects", snap.ObjectCount),
		slog.Int64("orphans", snap.OrphanCount),
		slog.Int64("stale_uploads", snap.StaleUploadCount),
	)

	return j.snapshots.Insert(ctx, snap)
}

// buildSnapshot aggregates a bucket listing into a StorageSnapshot. It is pure
// so the classification logic can be unit-tested without a live bucket.
func buildSnapshot(
	objects []objectstore.ObjectInfo,
	referenced map[string]bool,
	now time.Time,
) models.StorageSnapshot {
	prefixes := map[string]*models.PrefixStat{}
	snap := models.StorageSnapshot{
		ScannedAt:            now,
		TotalSizeBytes:       0,
		ObjectCount:          0,
		OrphanSizeBytes:      0,
		OrphanCount:          0,
		StaleUploadSizeBytes: 0,
		StaleUploadCount:     0,
		PrefixBreakdown:      nil,
	}

	for _, obj := range objects {
		snap.TotalSizeBytes += obj.Size
		snap.ObjectCount++

		p := topPrefix(obj.Key)
		stat, ok := prefixes[p]
		if !ok {
			stat = &models.PrefixStat{Prefix: p, SizeBytes: 0, Count: 0}
			prefixes[p] = stat
		}
		stat.SizeBytes += obj.Size
		stat.Count++

		if isOrphan(obj.Key, referenced) {
			snap.OrphanSizeBytes += obj.Size
			snap.OrphanCount++
		}
		if isStaleUpload(obj, now) {
			snap.StaleUploadSizeBytes += obj.Size
			snap.StaleUploadCount++
		}
	}

	snap.PrefixBreakdown = make([]models.PrefixStat, 0, len(prefixes))
	for _, stat := range prefixes {
		snap.PrefixBreakdown = append(snap.PrefixBreakdown, *stat)
	}
	return snap
}

// isOrphan reports whether a books/<id>/… object is no longer referenced by
// any book file. Cover caches and negative-cache markers are legitimately
// unreferenced, so they never count as orphans.
func isOrphan(key string, referenced map[string]bool) bool {
	if !strings.HasPrefix(key, booksPrefix) {
		return false
	}
	if strings.HasSuffix(key, coverSuffix) || strings.HasSuffix(key, coverMissing) {
		return false
	}
	return !referenced[key]
}

func isStaleUpload(obj objectstore.ObjectInfo, now time.Time) bool {
	if !strings.Contains(obj.Key, uploadsMarker) {
		return false
	}
	return now.Sub(obj.LastModified) > staleUploadAge
}

// topPrefix returns the first path segment of a key (e.g. "books", "users"),
// falling back to "(root)" for keys with no slash.
func topPrefix(key string) string {
	if idx := strings.IndexByte(key, '/'); idx != -1 {
		return key[:idx]
	}
	return "(root)"
}
