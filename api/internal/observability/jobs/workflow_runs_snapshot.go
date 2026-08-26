package jobs

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"tools.xdoubleu.com/internal/github"
	"tools.xdoubleu.com/internal/mailer"
	"tools.xdoubleu.com/internal/models"
	"tools.xdoubleu.com/internal/notifications"
	"tools.xdoubleu.com/internal/repositories"
)

// workflowRunsSnapshotRunEvery matches IssueNotifierJob's cadence, so a
// main-branch CI failure is caught within a few minutes.
const workflowRunsSnapshotRunEvery = 5 * time.Minute

// workflowRunsRetention bounds global.workflow_run_samples/
// global.workflow_job_samples.
const workflowRunsRetention = 90 * 24 * time.Hour

// mainBranch is the branch a main-failure alert fires on — deploys run
// straight off a push to it with no re-test (root CLAUDE.md's CI section),
// so a failure here is always a genuine incident, not routine PR noise.
const mainBranch = "main"

// failureConclusion is the workflow-run conclusion a main-branch run alerts
// on.
const failureConclusion = "failure"

// mainFailureMaxAge bounds how old a failing main run can be and still
// trigger an email. GitHub's runs endpoint has no server-side date filter
// (ListWorkflowRuns just returns the most recent page per event type), so on
// a quiet repo a run far older than this cutoff can still show up as
// "recently seen" for the first time — this stops that from reading as a
// fresh incident.
const mainFailureMaxAge = 48 * time.Hour

// workflowRunLister is the subset of github.Client this job needs to fetch
// recent runs.
type workflowRunLister interface {
	ListWorkflowRuns(ctx context.Context) ([]github.WorkflowRun, error)
	ListWorkflowRunJobs(ctx context.Context, runID int64) ([]github.WorkflowJob, error)
}

// workflowRunStore is the subset of *repositories.WorkflowRunsRepository this
// job needs.
type workflowRunStore interface {
	Exists(ctx context.Context, runID int64) (bool, error)
	InsertRun(ctx context.Context, run models.WorkflowRunSample) error
	InsertJobs(ctx context.Context, jobs []models.WorkflowJobSample) error
	PruneOlderThan(ctx context.Context, cutoff time.Time) error
}

// WorkflowRunsSnapshotJob polls GitHub Actions workflow runs every ~5
// minutes, persists newly-completed ones (plus their per-job breakdown) into
// global.workflow_run_samples/global.workflow_job_samples, and emails an
// admin the first time a run on mainBranch fails — deduped via the same
// global.notified_issues mechanism IssueNotifierJob uses (issue #1217), and
// gated by NotificationSourceFailingMainCI in global.notification_settings
// like every other admin notification (issue #1214).
type WorkflowRunsSnapshotJob struct {
	gh            workflowRunLister
	store         workflowRunStore
	notifications *notifications.Service
	notified      notifiedRepo
	settings      notificationSettingsRepo
}

func NewWorkflowRunsSnapshotJob(
	gh workflowRunLister,
	store workflowRunStore,
	notifications *notifications.Service,
	notified notifiedRepo,
	settings notificationSettingsRepo,
) *WorkflowRunsSnapshotJob {
	return &WorkflowRunsSnapshotJob{
		gh:            gh,
		store:         store,
		notifications: notifications,
		notified:      notified,
		settings:      settings,
	}
}

func (j *WorkflowRunsSnapshotJob) ID() string {
	return "workflow-runs-snapshot"
}

func (j *WorkflowRunsSnapshotJob) RunEvery() time.Duration {
	return workflowRunsSnapshotRunEvery
}

func (j *WorkflowRunsSnapshotJob) Run(ctx context.Context, logger *slog.Logger) error {
	runs, err := j.gh.ListWorkflowRuns(ctx)
	if errors.Is(err, github.ErrNotConfigured) {
		return nil
	}
	if err != nil {
		logAPIErr(ctx, logger, "workflow-runs-snapshot: failed to list workflow runs",
			err, github.IsTransientAPIError(err))
		return nil
	}

	for _, run := range runs {
		if run.Status != "completed" {
			continue
		}
		if err = j.recordRun(ctx, logger, run); err != nil {
			return err
		}
	}

	cutoff := time.Now().Add(-workflowRunsRetention)
	return j.store.PruneOlderThan(ctx, cutoff)
}

// recordRun inserts run if it hasn't been seen before, fetches and inserts
// its per-job breakdown, then alerts if it's a failing mainBranch run.
func (j *WorkflowRunsSnapshotJob) recordRun(
	ctx context.Context, logger *slog.Logger, run github.WorkflowRun,
) error {
	exists, err := j.store.Exists(ctx, run.ID)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}

	completedAt := run.StartedAt.Add(time.Duration(run.DurationMs) * time.Millisecond)
	sample := models.WorkflowRunSample{
		RunID:        run.ID,
		WorkflowName: run.Name,
		Branch:       run.Branch,
		Event:        run.Event,
		Conclusion:   run.Conclusion,
		URL:          run.URL,
		DurationMs:   run.DurationMs,
		StartedAt:    run.StartedAt,
		CompletedAt:  completedAt,
	}
	if err = j.store.InsertRun(ctx, sample); err != nil {
		return err
	}

	if err = j.recordJobs(ctx, logger, run.ID); err != nil {
		return err
	}

	isRecent := time.Since(completedAt) <= mainFailureMaxAge
	if run.Branch == mainBranch && run.Conclusion == failureConclusion && isRecent {
		return j.notifyMainFailure(ctx, run)
	}
	return nil
}

func (j *WorkflowRunsSnapshotJob) recordJobs(
	ctx context.Context, logger *slog.Logger, runID int64,
) error {
	jobs, err := j.gh.ListWorkflowRunJobs(ctx, runID)
	if errors.Is(err, github.ErrNotConfigured) {
		return nil
	}
	if err != nil {
		logAPIErr(ctx, logger, "workflow-runs-snapshot: failed to list run jobs",
			err, github.IsTransientAPIError(err))
		return nil
	}

	samples := make([]models.WorkflowJobSample, len(jobs))
	for i, job := range jobs {
		samples[i] = models.WorkflowJobSample{
			RunID:       runID,
			JobName:     job.Name,
			Conclusion:  job.Conclusion,
			DurationMs:  job.DurationMs,
			StartedAt:   job.StartedAt,
			CompletedAt: job.CompletedAt,
		}
	}
	return j.store.InsertJobs(ctx, samples)
}

// notifyMainFailure emails an admin once per failing run — the dedup key is
// the run id, not just "main failed", so each incident is reported
// separately.
func (j *WorkflowRunsSnapshotJob) notifyMainFailure(
	ctx context.Context, run github.WorkflowRun,
) error {
	enabled, err := j.settings.IsEnabled(
		ctx,
		repositories.NotificationSourceFailingMainCI,
	)
	if err != nil {
		return err
	}
	if !enabled {
		return nil
	}

	key := fmt.Sprintf("workflow_run:main_failure:%d", run.ID)

	exists, err := j.notified.Exists(ctx, key)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}

	subject := fmt.Sprintf("[CI] %s failed on main", run.Name)
	body := fmt.Sprintf(
		"A workflow run on main failed — this should never happen since "+
			"main deploys straight off a passing push.\n\n%s\n%s",
		run.Name, run.URL,
	)
	j.notifications.Enqueue(subject, body, func(ctx context.Context, err error) error {
		if errors.Is(err, mailer.ErrNotConfigured) {
			return nil
		}
		// Mark the run notified even on a send error: a transient Resend
		// failure (rate-limit, network blip) must not leave the dedup key
		// unwritten, or this same run gets retried and re-emailed on every
		// subsequent 5-minute poll forever. The error is still returned so
		// the worker pool logs it.
		if insertErr := j.notified.Insert(ctx, key); insertErr != nil {
			return insertErr
		}
		return err
	})
	return nil
}
