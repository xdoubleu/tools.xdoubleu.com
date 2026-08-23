package jobs_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/logging"
	"tools.xdoubleu.com/internal/models"
	"tools.xdoubleu.com/internal/observability"
	"tools.xdoubleu.com/internal/observability/jobs"
)

type stubScraper struct {
	sample observability.HostSample
	err    error
}

func (s stubScraper) Scrape(context.Context) (observability.HostSample, error) {
	return s.sample, s.err
}

type fakeHostMetricsInserter struct {
	inserted   *models.HostMetricSample
	insertErr  error
	pruneCalls int
	pruneErr   error
}

func (f *fakeHostMetricsInserter) Insert(
	_ context.Context, sample models.HostMetricSample,
) error {
	if f.insertErr != nil {
		return f.insertErr
	}
	f.inserted = &sample
	return nil
}

func (f *fakeHostMetricsInserter) PruneOlderThan(context.Context, time.Time) error {
	f.pruneCalls++
	return f.pruneErr
}

type fakeLogsPruner struct {
	pruneCalls int
	pruneErr   error
}

func (f *fakeLogsPruner) PruneOlderThan(context.Context, time.Time) error {
	f.pruneCalls++
	return f.pruneErr
}

func TestHostMetricsSnapshotJob_InsertsAndPrunes(t *testing.T) {
	scraper := stubScraper{ //nolint:exhaustruct // err intentionally zero
		sample: observability.HostSample{
			CPUPercent: 10, MemoryPercent: 20, DiskPercent: 30,
		},
	}
	metrics := &fakeHostMetricsInserter{} //nolint:exhaustruct // fixture
	logs := &fakeLogsPruner{}             //nolint:exhaustruct // fixture

	job := jobs.NewHostMetricsSnapshotJob(scraper, metrics, logs)
	require.NoError(t, job.Run(t.Context(), logging.NewNopLogger()))

	require.NotNil(t, metrics.inserted)
	assert.InDelta(t, 10, metrics.inserted.CPUPercent, 0.001)
	assert.WithinDuration(t, time.Now(), metrics.inserted.SampledAt, time.Second)
	assert.Equal(t, 1, metrics.pruneCalls)
	assert.Equal(t, 1, logs.pruneCalls)
}

func TestHostMetricsSnapshotJob_ScrapeErrorPropagates(t *testing.T) {
	scraper := stubScraper{err: assert.AnError} //nolint:exhaustruct // fixture
	metrics := &fakeHostMetricsInserter{}       //nolint:exhaustruct // fixture
	logs := &fakeLogsPruner{}                   //nolint:exhaustruct // fixture

	job := jobs.NewHostMetricsSnapshotJob(scraper, metrics, logs)
	err := job.Run(t.Context(), logging.NewNopLogger())
	require.ErrorIs(t, err, assert.AnError)
	assert.Nil(t, metrics.inserted)
	assert.Zero(t, metrics.pruneCalls)
}

func TestHostMetricsSnapshotJob_InsertErrorPropagates(t *testing.T) {
	scraper := stubScraper{ //nolint:exhaustruct // fixture
		sample: observability.HostSample{
			CPUPercent: 10, MemoryPercent: 20, DiskPercent: 30,
		},
	}
	//nolint:exhaustruct // fixture
	metrics := &fakeHostMetricsInserter{insertErr: assert.AnError}
	logs := &fakeLogsPruner{} //nolint:exhaustruct // fixture

	job := jobs.NewHostMetricsSnapshotJob(scraper, metrics, logs)
	err := job.Run(t.Context(), logging.NewNopLogger())
	require.ErrorIs(t, err, assert.AnError)
	assert.Zero(t, metrics.pruneCalls)
	assert.Zero(t, logs.pruneCalls)
}

func TestHostMetricsSnapshotJob_MetricsPruneErrorPropagates(t *testing.T) {
	scraper := stubScraper{ //nolint:exhaustruct // fixture
		sample: observability.HostSample{
			CPUPercent: 10, MemoryPercent: 20, DiskPercent: 30,
		},
	}
	//nolint:exhaustruct // fixture
	metrics := &fakeHostMetricsInserter{pruneErr: assert.AnError}
	logs := &fakeLogsPruner{} //nolint:exhaustruct // fixture

	job := jobs.NewHostMetricsSnapshotJob(scraper, metrics, logs)
	err := job.Run(t.Context(), logging.NewNopLogger())
	require.ErrorIs(t, err, assert.AnError)
	assert.Zero(t, logs.pruneCalls)
}

func TestHostMetricsSnapshotJob_LogsPruneErrorPropagates(t *testing.T) {
	scraper := stubScraper{ //nolint:exhaustruct // fixture
		sample: observability.HostSample{
			CPUPercent: 10, MemoryPercent: 20, DiskPercent: 30,
		},
	}
	metrics := &fakeHostMetricsInserter{} //nolint:exhaustruct // fixture
	//nolint:exhaustruct // fixture
	logs := &fakeLogsPruner{pruneErr: assert.AnError}

	job := jobs.NewHostMetricsSnapshotJob(scraper, metrics, logs)
	err := job.Run(t.Context(), logging.NewNopLogger())
	require.ErrorIs(t, err, assert.AnError)
}

func TestHostMetricsSnapshotJob_IDAndSchedule(t *testing.T) {
	job := jobs.NewHostMetricsSnapshotJob(
		stubScraper{},              //nolint:exhaustruct // fixture
		&fakeHostMetricsInserter{}, //nolint:exhaustruct // fixture
		&fakeLogsPruner{},          //nolint:exhaustruct // fixture
	)
	assert.Equal(t, "host-metrics-snapshot", job.ID())
	assert.Equal(t, 60*time.Second, job.RunEvery())
}
