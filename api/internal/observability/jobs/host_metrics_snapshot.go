package jobs

import (
	"context"
	"log/slog"
	"time"

	"tools.xdoubleu.com/internal/models"
	"tools.xdoubleu.com/internal/observability"
)

// hostMetricsSnapshotRunEvery is how often HostMetricsSnapshotJob scrapes
// node_exporter.
const hostMetricsSnapshotRunEvery = 60 * time.Second

// hostMetricsRetention bounds global.host_metric_samples/global.log_entries
// — the DB is self-hosted now, not billed per-byte, but unbounded growth
// still isn't wanted (issue #1040).
const hostMetricsRetention = 30 * 24 * time.Hour

// hostSampleScraper is the slice of *observability.HostMetricsScraper this
// job needs.
type hostSampleScraper interface {
	Scrape(ctx context.Context) (observability.HostSample, error)
}

// hostMetricsInserter is the slice of *repositories.HostMetricsRepository
// this job needs.
type hostMetricsInserter interface {
	Insert(ctx context.Context, sample models.HostMetricSample) error
	PruneOlderThan(ctx context.Context, cutoff time.Time) error
}

// logsPruner is the slice of *repositories.LogsRepository this job needs —
// only pruning, since log inserts happen independently of this job (in
// process for api, over HTTP for web).
type logsPruner interface {
	PruneOlderThan(ctx context.Context, cutoff time.Time) error
}

// HostMetricsSnapshotJob scrapes node_exporter every ~60s and inserts one
// global.host_metric_samples row (issue #1040), then prunes both
// host_metric_samples and log_entries past hostMetricsRetention.
type HostMetricsSnapshotJob struct {
	scraper hostSampleScraper
	metrics hostMetricsInserter
	logs    logsPruner
}

func NewHostMetricsSnapshotJob(
	scraper hostSampleScraper, metrics hostMetricsInserter, logs logsPruner,
) *HostMetricsSnapshotJob {
	return &HostMetricsSnapshotJob{scraper: scraper, metrics: metrics, logs: logs}
}

func (j *HostMetricsSnapshotJob) ID() string {
	return "host-metrics-snapshot"
}

func (j *HostMetricsSnapshotJob) RunEvery() time.Duration {
	return hostMetricsSnapshotRunEvery
}

func (j *HostMetricsSnapshotJob) Run(ctx context.Context, _ *slog.Logger) error {
	sample, err := j.scraper.Scrape(ctx)
	if err != nil {
		return err
	}

	row := models.HostMetricSample{
		SampledAt:     time.Now(),
		CPUPercent:    sample.CPUPercent,
		MemoryPercent: sample.MemoryPercent,
		DiskPercent:   sample.DiskPercent,
	}
	if err = j.metrics.Insert(ctx, row); err != nil {
		return err
	}

	cutoff := time.Now().Add(-hostMetricsRetention)
	if err = j.metrics.PruneOlderThan(ctx, cutoff); err != nil {
		return err
	}
	return j.logs.PruneOlderThan(ctx, cutoff)
}
