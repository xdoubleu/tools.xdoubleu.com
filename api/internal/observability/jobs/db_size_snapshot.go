package jobs

import (
	"context"
	"log/slog"
	"time"

	"tools.xdoubleu.com/internal/models"
)

// dbSizeSnapshotRunEvery is how often DBSizeSnapshotJob samples per-table
// database size — daily is plenty for growth trends, unlike
// HostMetricsSnapshotJob's 60s resolution.
const dbSizeSnapshotRunEvery = 24 * time.Hour

// dbSizeSamplesRetention bounds global.db_size_samples, matching
// workflowRunsRetention.
const dbSizeSamplesRetention = 90 * 24 * time.Hour

// tableSizeScraper is the slice of *repositories.DBStatsRepository this job
// needs to read live per-table sizes.
type tableSizeScraper interface {
	TableSizes(ctx context.Context) ([]models.TableSizeSample, error)
}

// dbSizeStore is the slice of *repositories.DBSizeSamplesRepository this job
// needs.
type dbSizeStore interface {
	InsertBatch(
		ctx context.Context,
		sampledAt time.Time,
		samples []models.TableSizeSample,
	) error
	PruneOlderThan(ctx context.Context, cutoff time.Time) error
}

// DBSizeSnapshotJob samples every table's on-disk size once a day and
// inserts one global.db_size_samples row per table, then prunes rows past
// dbSizeSamplesRetention (issue #1282) — the read side of "which tables are
// growing rapidly", a question get_database_stats' live-only query couldn't
// answer.
type DBSizeSnapshotJob struct {
	scraper tableSizeScraper
	store   dbSizeStore
}

func NewDBSizeSnapshotJob(
	scraper tableSizeScraper, store dbSizeStore,
) *DBSizeSnapshotJob {
	return &DBSizeSnapshotJob{scraper: scraper, store: store}
}

func (j *DBSizeSnapshotJob) ID() string {
	return "db-size-snapshot"
}

func (j *DBSizeSnapshotJob) RunEvery() time.Duration {
	return dbSizeSnapshotRunEvery
}

func (j *DBSizeSnapshotJob) Run(ctx context.Context, _ *slog.Logger) error {
	samples, err := j.scraper.TableSizes(ctx)
	if err != nil {
		return err
	}

	if err = j.store.InsertBatch(ctx, time.Now(), samples); err != nil {
		return err
	}

	cutoff := time.Now().Add(-dbSizeSamplesRetention)
	return j.store.PruneOlderThan(ctx, cutoff)
}
