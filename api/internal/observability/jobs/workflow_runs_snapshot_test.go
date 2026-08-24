package jobs_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/github"
	"tools.xdoubleu.com/internal/models"
	"tools.xdoubleu.com/internal/observability/jobs"
)

type fakeWorkflowRunLister struct {
	runs    []github.WorkflowRun
	jobs    map[int64][]github.WorkflowJob
	err     error
	jobsErr error
}

func (f fakeWorkflowRunLister) ListWorkflowRuns(
	_ context.Context,
) ([]github.WorkflowRun, error) {
	return f.runs, f.err
}

func (f fakeWorkflowRunLister) ListWorkflowRunJobs(
	_ context.Context, runID int64,
) ([]github.WorkflowJob, error) {
	if f.jobsErr != nil {
		return nil, f.jobsErr
	}
	return f.jobs[runID], nil
}

// fakeWorkflowRunStore.mu guards concurrent access — InsertRun runs on the
// test goroutine while the notification callback (Insert on notifiedRepo,
// a separate fake) runs on notifications.Service's background worker.
type fakeWorkflowRunStore struct {
	mu         sync.Mutex
	existing   map[int64]bool
	inserted   []models.WorkflowRunSample
	jobsByRun  map[int64][]models.WorkflowJobSample
	pruneCalls int
	insertErr  error
	pruneErr   error
}

func newFakeWorkflowRunStore(existing ...int64) *fakeWorkflowRunStore {
	m := map[int64]bool{}
	for _, id := range existing {
		m[id] = true
	}
	return &fakeWorkflowRunStore{ //nolint:exhaustruct // fixture
		existing:  m,
		jobsByRun: map[int64][]models.WorkflowJobSample{},
	}
}

func (f *fakeWorkflowRunStore) Exists(
	_ context.Context, runID int64,
) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.existing[runID], nil
}

func (f *fakeWorkflowRunStore) InsertRun(
	_ context.Context, run models.WorkflowRunSample,
) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.insertErr != nil {
		return f.insertErr
	}
	f.inserted = append(f.inserted, run)
	f.existing[run.RunID] = true
	return nil
}

func (f *fakeWorkflowRunStore) InsertJobs(
	_ context.Context, jobs []models.WorkflowJobSample,
) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(jobs) == 0 {
		return nil
	}
	f.jobsByRun[jobs[0].RunID] = jobs
	return nil
}

func (f *fakeWorkflowRunStore) PruneOlderThan(context.Context, time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.pruneCalls++
	return f.pruneErr
}

func completedRun(branch, conclusion string) github.WorkflowRun {
	return github.WorkflowRun{
		ID:         1,
		Name:       "CI",
		Event:      "push",
		Branch:     branch,
		Status:     "completed",
		Conclusion: conclusion,
		URL:        "https://github.com/x/y/actions/runs/1",
		StartedAt:  time.Now().Add(-time.Minute),
		DurationMs: 60_000,
	}
}

func TestWorkflowRunsSnapshotJob_RecordsNewCompletedRuns(t *testing.T) {
	gh := fakeWorkflowRunLister{ //nolint:exhaustruct // fixture
		runs: []github.WorkflowRun{completedRun("main", "success")},
	}
	store := newFakeWorkflowRunStore()
	mail := &fakeMailer{sent: nil, err: nil}
	notified := newFakeNotifiedRepo()
	notifSvc := testNotifications(t, mail)

	job := jobs.NewWorkflowRunsSnapshotJob(gh, store, notifSvc, notified)
	require.NoError(t, job.Run(t.Context(), testLogger()))
	notifSvc.WaitUntilDone()

	require.Len(t, store.inserted, 1)
	assert.Equal(t, int64(1), store.inserted[0].RunID)
	assert.Equal(t, 1, store.pruneCalls)
	assert.Empty(t, mail.sent)
}

func TestWorkflowRunsSnapshotJob_SkipsAlreadyRecordedRuns(t *testing.T) {
	gh := fakeWorkflowRunLister{ //nolint:exhaustruct // fixture
		runs: []github.WorkflowRun{completedRun("main", "success")},
	}
	store := newFakeWorkflowRunStore(1)
	notifSvc := testNotifications(t, &fakeMailer{sent: nil, err: nil})

	job := jobs.NewWorkflowRunsSnapshotJob(gh, store, notifSvc, newFakeNotifiedRepo())
	require.NoError(t, job.Run(t.Context(), testLogger()))
	notifSvc.WaitUntilDone()

	assert.Empty(t, store.inserted)
}

func TestWorkflowRunsSnapshotJob_SkipsInProgressRuns(t *testing.T) {
	run := completedRun("main", "")
	run.Status = "in_progress"
	//nolint:exhaustruct // fixture
	gh := fakeWorkflowRunLister{runs: []github.WorkflowRun{run}}
	store := newFakeWorkflowRunStore()
	notifSvc := testNotifications(t, &fakeMailer{sent: nil, err: nil})

	job := jobs.NewWorkflowRunsSnapshotJob(gh, store, notifSvc, newFakeNotifiedRepo())
	require.NoError(t, job.Run(t.Context(), testLogger()))
	notifSvc.WaitUntilDone()

	assert.Empty(t, store.inserted)
}

func TestWorkflowRunsSnapshotJob_RecordsJobBreakdown(t *testing.T) {
	//nolint:exhaustruct // fixture
	gh := fakeWorkflowRunLister{
		runs: []github.WorkflowRun{completedRun("main", "success")},
		jobs: map[int64][]github.WorkflowJob{
			1: {{
				Name:        "test",
				Status:      "completed",
				Conclusion:  "success",
				StartedAt:   time.Now().Add(-time.Minute),
				CompletedAt: time.Now(),
				DurationMs:  60_000,
			}},
		},
	}
	store := newFakeWorkflowRunStore()
	notifSvc := testNotifications(t, &fakeMailer{sent: nil, err: nil})

	job := jobs.NewWorkflowRunsSnapshotJob(gh, store, notifSvc, newFakeNotifiedRepo())
	require.NoError(t, job.Run(t.Context(), testLogger()))
	notifSvc.WaitUntilDone()

	require.Len(t, store.jobsByRun[1], 1)
	assert.Equal(t, "test", store.jobsByRun[1][0].JobName)
}

func TestWorkflowRunsSnapshotJob_AlertsOnMainFailure(t *testing.T) {
	gh := fakeWorkflowRunLister{ //nolint:exhaustruct // fixture
		runs: []github.WorkflowRun{completedRun("main", "failure")},
	}
	store := newFakeWorkflowRunStore()
	mail := &fakeMailer{sent: nil, err: nil}
	notified := newFakeNotifiedRepo()
	notifSvc := testNotifications(t, mail)

	job := jobs.NewWorkflowRunsSnapshotJob(gh, store, notifSvc, notified)
	require.NoError(t, job.Run(t.Context(), testLogger()))
	notifSvc.WaitUntilDone()

	assert.Len(t, mail.sent, 1)
	assert.True(t, notified.keys["workflow_run:main_failure:1"])
}

func TestWorkflowRunsSnapshotJob_DoesNotAlertOnNonMainFailure(t *testing.T) {
	gh := fakeWorkflowRunLister{ //nolint:exhaustruct // fixture
		runs: []github.WorkflowRun{completedRun("feature-branch", "failure")},
	}
	store := newFakeWorkflowRunStore()
	mail := &fakeMailer{sent: nil, err: nil}
	notifSvc := testNotifications(t, mail)

	job := jobs.NewWorkflowRunsSnapshotJob(
		gh, store, notifSvc, newFakeNotifiedRepo(),
	)
	require.NoError(t, job.Run(t.Context(), testLogger()))
	notifSvc.WaitUntilDone()

	assert.Empty(t, mail.sent)
}

func TestWorkflowRunsSnapshotJob_ListErrorDoesNotFailRun(t *testing.T) {
	gh := fakeWorkflowRunLister{err: assert.AnError} //nolint:exhaustruct // fixture
	store := newFakeWorkflowRunStore()
	notifSvc := testNotifications(t, &fakeMailer{sent: nil, err: nil})

	job := jobs.NewWorkflowRunsSnapshotJob(
		gh, store, notifSvc, newFakeNotifiedRepo(),
	)
	require.NoError(t, job.Run(t.Context(), testLogger()))
	notifSvc.WaitUntilDone()

	assert.Empty(t, store.inserted)
}

func TestWorkflowRunsSnapshotJob_NotConfiguredIsNotAnError(t *testing.T) {
	//nolint:exhaustruct // fixture
	gh := fakeWorkflowRunLister{err: github.ErrNotConfigured}
	store := newFakeWorkflowRunStore()
	notifSvc := testNotifications(t, &fakeMailer{sent: nil, err: nil})

	job := jobs.NewWorkflowRunsSnapshotJob(
		gh, store, notifSvc, newFakeNotifiedRepo(),
	)
	require.NoError(t, job.Run(t.Context(), testLogger()))
	notifSvc.WaitUntilDone()

	assert.Zero(t, store.pruneCalls)
}

func TestWorkflowRunsSnapshotJob_InsertErrorPropagates(t *testing.T) {
	gh := fakeWorkflowRunLister{ //nolint:exhaustruct // fixture
		runs: []github.WorkflowRun{completedRun("main", "success")},
	}
	store := newFakeWorkflowRunStore()
	store.insertErr = assert.AnError
	notifSvc := testNotifications(t, &fakeMailer{sent: nil, err: nil})

	job := jobs.NewWorkflowRunsSnapshotJob(
		gh, store, notifSvc, newFakeNotifiedRepo(),
	)
	err := job.Run(t.Context(), testLogger())
	require.ErrorIs(t, err, assert.AnError)
}

func TestWorkflowRunsSnapshotJob_PruneErrorPropagates(t *testing.T) {
	gh := fakeWorkflowRunLister{ //nolint:exhaustruct // fixture
		runs: []github.WorkflowRun{completedRun("main", "success")},
	}
	store := newFakeWorkflowRunStore()
	store.pruneErr = assert.AnError
	notifSvc := testNotifications(t, &fakeMailer{sent: nil, err: nil})

	job := jobs.NewWorkflowRunsSnapshotJob(
		gh, store, notifSvc, newFakeNotifiedRepo(),
	)
	err := job.Run(t.Context(), testLogger())
	require.ErrorIs(t, err, assert.AnError)
}

func TestWorkflowRunsSnapshotJob_IDAndSchedule(t *testing.T) {
	job := jobs.NewWorkflowRunsSnapshotJob(
		fakeWorkflowRunLister{}, //nolint:exhaustruct // fixture
		newFakeWorkflowRunStore(),
		testNotifications(t, &fakeMailer{sent: nil, err: nil}),
		newFakeNotifiedRepo(),
	)
	assert.Equal(t, "workflow-runs-snapshot", job.ID())
	assert.Equal(t, 5*time.Minute, job.RunEvery())
}
