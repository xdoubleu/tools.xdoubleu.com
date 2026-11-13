package jobs_test

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/factory/internal/jobs"
)

type fakeCollector struct{ err error }

func (f fakeCollector) CollectSignals(context.Context) error { return f.err }

func TestTheSignalsJobRunsEveryFiveMinutes(t *testing.T) {
	job := jobs.NewCollectSignalsJob(fakeCollector{err: nil})
	assert.Equal(t, "factory-collect-signals", job.ID())
	assert.Equal(t, 5*time.Minute, job.RunEvery())
	require.NoError(t, job.Run(context.Background(), slog.Default()))

	boom := errors.New("boom")
	require.ErrorIs(t, jobs.NewCollectSignalsJob(fakeCollector{err: boom}).
		Run(context.Background(), slog.Default()), boom)
}
