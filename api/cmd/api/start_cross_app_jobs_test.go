package main

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/jobqueue"
	"tools.xdoubleu.com/internal/logging"
)

// duplicateSweepJob collides with the sweep job's ID to reach its AddJob
// error branch.
type duplicateSweepJob struct{}

func (duplicateSweepJob) ID() string { return "sweep-automated-actions" }

func (duplicateSweepJob) Run(context.Context, *slog.Logger) error {
	return nil
}

func TestStartCrossAppJobsDuplicateSweepJobIDErrors(t *testing.T) {
	queue := jobqueue.NewJobQueue(
		t.Context(), logging.NewNopLogger(), 1, 8, testApp.db,
	)
	require.NoError(t, queue.AddJob(
		duplicateSweepJob{}, func(string, bool, *time.Time) {},
	))

	//nolint:exhaustruct //only the fields startCrossAppJobs reads are set
	app := &Application{
		db:                            testApp.db,
		issueSignalCollectorJob:       testApp.issueSignalCollectorJob,
		automatedActionSweepJob:       testApp.automatedActionSweepJob,
		transactionLatencySnapshotJob: testApp.transactionLatencySnapshotJob,
		weeklyDigestJob:               testApp.weeklyDigestJob,
		globalJobQueue:                queue,
	}

	err := startCrossAppJobs(app)
	require.ErrorContains(t, err, "already exists")
}
