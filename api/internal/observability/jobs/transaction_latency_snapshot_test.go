package jobs_test

import (
	"bytes"
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/logging"
	"tools.xdoubleu.com/internal/observability/jobs"
	"tools.xdoubleu.com/internal/sentryapi"
)

type stubStatsLister struct {
	stats []sentryapi.TransactionStat
	err   error
}

func (s stubStatsLister) ListTransactionStats(
	context.Context,
) ([]sentryapi.TransactionStat, error) {
	return s.stats, s.err
}

type fakeLatencyInserter struct {
	inserted []sentryapi.TransactionStat
	day      time.Time
	err      error
	calls    int
}

func (f *fakeLatencyInserter) Insert(
	_ context.Context, day time.Time, stats []sentryapi.TransactionStat,
) error {
	f.calls++
	f.day = day
	f.inserted = stats
	return f.err
}

func TestTransactionLatencySnapshotJob_InsertsStats(t *testing.T) {
	stats := []sentryapi.TransactionStat{
		{
			Transaction:   "GET /api/books",
			Project:       "proj",
			P95DurationMs: 123,
			RequestCount:  10,
		},
	}
	inserter := &fakeLatencyInserter{} //nolint:exhaustruct // zero values are the fixture
	job := jobs.NewTransactionLatencySnapshotJob(
		stubStatsLister{stats: stats}, //nolint:exhaustruct // err intentionally zero
		inserter,
	)

	require.NoError(t, job.Run(t.Context(), logging.NewNopLogger()))
	assert.Equal(t, 1, inserter.calls)
	assert.Equal(t, stats, inserter.inserted)
	assert.WithinDuration(t, time.Now(), inserter.day, time.Second)
}

func TestTransactionLatencySnapshotJob_NotConfiguredNoops(t *testing.T) {
	inserter := &fakeLatencyInserter{} //nolint:exhaustruct // zero values are the fixture
	job := jobs.NewTransactionLatencySnapshotJob(
		stubStatsLister{ //nolint:exhaustruct // stats intentionally zero
			err: sentryapi.ErrNotConfigured,
		},
		inserter,
	)

	require.NoError(t, job.Run(t.Context(), logging.NewNopLogger()))
	assert.Zero(
		t,
		inserter.calls,
		"must not insert anything when Sentry isn't configured",
	)
}

func TestTransactionLatencySnapshotJob_ListErrorPropagates(t *testing.T) {
	inserter := &fakeLatencyInserter{} //nolint:exhaustruct // zero values are the fixture
	job := jobs.NewTransactionLatencySnapshotJob(
		stubStatsLister{err: assert.AnError}, //nolint:exhaustruct // stats unused
		inserter,
	)

	err := job.Run(t.Context(), logging.NewNopLogger())
	require.ErrorIs(t, err, assert.AnError)
	assert.Zero(t, inserter.calls)
}

func TestTransactionLatencySnapshotJob_InsertErrorPropagates(t *testing.T) {
	inserter := &fakeLatencyInserter{ //nolint:exhaustruct // fixture
		err: assert.AnError,
	}
	job := jobs.NewTransactionLatencySnapshotJob(
		stubStatsLister{}, //nolint:exhaustruct // zero values are the fixture
		inserter,
	)

	err := job.Run(t.Context(), logging.NewNopLogger())
	require.ErrorIs(t, err, assert.AnError)
}

func TestTransactionLatencySnapshotJob_ZeroStatsWarns(t *testing.T) {
	inserter := &fakeLatencyInserter{} //nolint:exhaustruct // zero values are the fixture
	job := jobs.NewTransactionLatencySnapshotJob(
		stubStatsLister{}, //nolint:exhaustruct // zero values are the fixture
		inserter,
	)

	var buf bytes.Buffer
	logger := slog.New(logging.NewBufLogHandler(&buf, nil))

	require.NoError(t, job.Run(t.Context(), logger))
	assert.Equal(t, 1, inserter.calls)
	assert.Contains(t, buf.String(), "transaction latency snapshot got zero stats")
}

func TestTransactionLatencySnapshotJob_IDAndSchedule(t *testing.T) {
	job := jobs.NewTransactionLatencySnapshotJob(
		stubStatsLister{},      //nolint:exhaustruct // zero values are the fixture
		&fakeLatencyInserter{}, //nolint:exhaustruct // zero values are the fixture
	)
	assert.Equal(t, "transaction-latency-snapshot", job.ID())
	assert.Equal(t, 24*time.Hour, job.RunEvery())
}
