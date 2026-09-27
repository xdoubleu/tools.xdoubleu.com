package jobs_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/logging"
	"tools.xdoubleu.com/internal/observability/jobs"
)

type fakeLogPruner struct {
	cutoff time.Time
	err    error
}

func (f *fakeLogPruner) PruneOlderThan(_ context.Context, cutoff time.Time) error {
	f.cutoff = cutoff
	return f.err
}

func TestLogPruneJob_PrunesPastRetention(t *testing.T) {
	pruner := &fakeLogPruner{} //nolint:exhaustruct // zero values are the fixture
	job := jobs.NewLogPruneJob(pruner)

	require.NoError(t, job.Run(t.Context(), logging.NewNopLogger()))
	assert.Equal(t, "prune-log-entries", job.ID())
	assert.Equal(t, 24*time.Hour, job.RunEvery())
	assert.WithinDuration(
		t,
		time.Now().Add(-jobs.LogRetention),
		pruner.cutoff,
		time.Minute,
	)
}

func TestLogPruneJob_PropagatesError(t *testing.T) {
	job := jobs.NewLogPruneJob(
		&fakeLogPruner{cutoff: time.Time{}, err: errors.New("boom")},
	)
	assert.Error(t, job.Run(t.Context(), logging.NewNopLogger()))
}
