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

type fakeReconciler struct {
	n   int
	err error
}

func (f fakeReconciler) ReconcileBoard(context.Context) (int, error) {
	return f.n, f.err
}

func TestTheReconcileJobRunsEveryFiveMinutesAndReportsFailure(t *testing.T) {
	job := jobs.NewReconcileBoardJob(fakeReconciler{n: 2, err: nil})
	assert.Equal(t, "factory-reconcile-board", job.ID())
	assert.Equal(t, 5*time.Minute, job.RunEvery())
	require.NoError(t, job.Run(context.Background(), logging.NewNopLogger()))

	boom := errors.New("boom")
	job = jobs.NewReconcileBoardJob(fakeReconciler{n: 0, err: boom})
	require.ErrorIs(t, job.Run(context.Background(), logging.NewNopLogger()), boom)
}
