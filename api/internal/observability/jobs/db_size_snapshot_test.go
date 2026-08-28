package jobs_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/logging"
	"tools.xdoubleu.com/internal/models"
	"tools.xdoubleu.com/internal/observability/jobs"
)

type stubTableSizeScraper struct {
	samples []models.TableSizeSample
	err     error
}

func (s stubTableSizeScraper) TableSizes(
	context.Context,
) ([]models.TableSizeSample, error) {
	return s.samples, s.err
}

type fakeDBSizeStore struct {
	insertedAt      time.Time
	insertedSamples []models.TableSizeSample
	insertErr       error
	pruneCalls      int
	pruneErr        error
}

func (f *fakeDBSizeStore) InsertBatch(
	_ context.Context, sampledAt time.Time, samples []models.TableSizeSample,
) error {
	if f.insertErr != nil {
		return f.insertErr
	}
	f.insertedAt = sampledAt
	f.insertedSamples = samples
	return nil
}

func (f *fakeDBSizeStore) PruneOlderThan(context.Context, time.Time) error {
	f.pruneCalls++
	return f.pruneErr
}

func TestDBSizeSnapshotJob_InsertsAndPrunes(t *testing.T) {
	samples := []models.TableSizeSample{
		{SchemaName: "public", TableName: "foo", SizeBytes: 100},
	}
	scraper := stubTableSizeScraper{samples: samples} //nolint:exhaustruct // fixture
	store := &fakeDBSizeStore{}                       //nolint:exhaustruct // fixture

	job := jobs.NewDBSizeSnapshotJob(scraper, store)
	require.NoError(t, job.Run(t.Context(), logging.NewNopLogger()))

	assert.Equal(t, samples, store.insertedSamples)
	assert.WithinDuration(t, time.Now(), store.insertedAt, time.Second)
	assert.Equal(t, 1, store.pruneCalls)
}

func TestDBSizeSnapshotJob_ScrapeErrorPropagates(t *testing.T) {
	scraper := stubTableSizeScraper{err: assert.AnError} //nolint:exhaustruct // fixture
	store := &fakeDBSizeStore{}                          //nolint:exhaustruct // fixture

	job := jobs.NewDBSizeSnapshotJob(scraper, store)
	err := job.Run(t.Context(), logging.NewNopLogger())
	require.ErrorIs(t, err, assert.AnError)
	assert.Nil(t, store.insertedSamples)
	assert.Zero(t, store.pruneCalls)
}

func TestDBSizeSnapshotJob_InsertErrorPropagates(t *testing.T) {
	scraper := stubTableSizeScraper{ //nolint:exhaustruct // fixture
		samples: []models.TableSizeSample{
			{SchemaName: "public", TableName: "foo", SizeBytes: 1},
		},
	}
	store := &fakeDBSizeStore{insertErr: assert.AnError} //nolint:exhaustruct // fixture

	job := jobs.NewDBSizeSnapshotJob(scraper, store)
	err := job.Run(t.Context(), logging.NewNopLogger())
	require.ErrorIs(t, err, assert.AnError)
	assert.Zero(t, store.pruneCalls)
}

func TestDBSizeSnapshotJob_PruneErrorPropagates(t *testing.T) {
	scraper := stubTableSizeScraper{ //nolint:exhaustruct // fixture
		samples: []models.TableSizeSample{
			{SchemaName: "public", TableName: "foo", SizeBytes: 1},
		},
	}
	store := &fakeDBSizeStore{pruneErr: assert.AnError} //nolint:exhaustruct // fixture

	job := jobs.NewDBSizeSnapshotJob(scraper, store)
	err := job.Run(t.Context(), logging.NewNopLogger())
	require.ErrorIs(t, err, assert.AnError)
}

func TestDBSizeSnapshotJob_IDAndSchedule(t *testing.T) {
	job := jobs.NewDBSizeSnapshotJob(
		stubTableSizeScraper{}, //nolint:exhaustruct // fixture
		&fakeDBSizeStore{},     //nolint:exhaustruct // fixture
	)
	assert.Equal(t, "db-size-snapshot", job.ID())
	assert.Equal(t, 24*time.Hour, job.RunEvery())
}
