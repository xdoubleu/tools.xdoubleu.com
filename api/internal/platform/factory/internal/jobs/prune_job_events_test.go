package jobs_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/kit/logging"
	"tools.xdoubleu.com/internal/platform/factory/internal/jobs"
)

type fakePruner struct {
	err error
}

func (f fakePruner) PruneEvents(context.Context) (int64, error) { return 3, f.err }

func TestThePruneJobRunsDailyAndReportsFailure(t *testing.T) {
	job := jobs.NewPruneJobEventsJob(fakePruner{err: nil})
	assert.Equal(t, "factory-prune-job-events", job.ID())
	assert.Equal(t, 24*time.Hour, job.RunEvery())
	require.NoError(t, job.Run(context.Background(), logging.NewNopLogger()))

	boom := errors.New("boom")
	job = jobs.NewPruneJobEventsJob(fakePruner{err: boom})
	require.ErrorIs(t, job.Run(context.Background(), logging.NewNopLogger()), boom)
}
