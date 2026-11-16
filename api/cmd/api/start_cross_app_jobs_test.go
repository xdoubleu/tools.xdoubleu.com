package main

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/kit/logging"
	"tools.xdoubleu.com/internal/platform/jobqueue"
)

// duplicateCollectorJob collides with the collector job's ID to reach its AddJob
// error branch.
type duplicateCollectorJob struct{}

func (duplicateCollectorJob) ID() string { return "collect-issue-signals" }

func (duplicateCollectorJob) Run(context.Context, *slog.Logger) error {
	return nil
}

func TestStartCrossAppJobsDuplicateCollectorJobIDErrors(t *testing.T) {
	queue := jobqueue.NewJobQueue(
		t.Context(), logging.NewNopLogger(), 1, 8, testApp.db,
	)
	require.NoError(t, queue.AddJob(
		duplicateCollectorJob{}, func(string, bool, *time.Time) {},
	))

	//nolint:exhaustruct //only the fields startCrossAppJobs reads are set
	app := &Application{
		db:                            testApp.db,
		issueSignalCollectorJob:       testApp.issueSignalCollectorJob,
		transactionLatencySnapshotJob: testApp.transactionLatencySnapshotJob,
		weeklyDigestJob:               testApp.weeklyDigestJob,
		globalJobQueue:                queue,
	}

	err := startCrossAppJobs(app)
	require.ErrorContains(t, err, "already exists")
}

// namedJob collides with a registered job's ID to reach its AddJob error
// branch.
type namedJob string

func (j namedJob) ID() string { return string(j) }

func (namedJob) Run(context.Context, *slog.Logger) error { return nil }

func TestStartCrossAppJobsDuplicateLaterJobIDsError(t *testing.T) {
	for _, id := range []string{"weekly-digest", "prune-log-entries"} {
		t.Run(id, func(t *testing.T) {
			queue := jobqueue.NewJobQueue(
				t.Context(), logging.NewNopLogger(), 1, 8, testApp.db,
			)
			require.NoError(t, queue.AddJob(
				namedJob(id), func(string, bool, *time.Time) {},
			))

			//nolint:exhaustruct //only the fields startCrossAppJobs reads are set
			app := &Application{
				db:                            testApp.db,
				issueSignalCollectorJob:       testApp.issueSignalCollectorJob,
				transactionLatencySnapshotJob: testApp.transactionLatencySnapshotJob,
				weeklyDigestJob:               testApp.weeklyDigestJob,
				logPruneJob:                   testApp.logPruneJob,
				globalJobQueue:                queue,
			}

			require.ErrorContains(t, startCrossAppJobs(app), "already exists")
		})
	}
}
