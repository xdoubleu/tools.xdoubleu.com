package jobs

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/kit/database"
	"tools.xdoubleu.com/internal/models"
	"tools.xdoubleu.com/internal/platform/github"
	"tools.xdoubleu.com/internal/platform/sentryapi"
)

type stubGithubClient struct {
	prs       []github.PullRequest
	prsErr    error
	runs      []github.WorkflowRun
	runsErr   error
	alerts    []github.SecurityAlert
	alertsErr error
}

func (g stubGithubClient) ListFailingPullRequests(
	_ context.Context,
) ([]github.PullRequest, error) {
	return g.prs, g.prsErr
}

func (g stubGithubClient) ListFailingPullRequestsIn(
	_ context.Context, _ string, _ github.PRScope,
) ([]github.PullRequest, error) {
	return g.prs, g.prsErr
}

func (g stubGithubClient) ListWorkflowRuns(
	_ context.Context,
) ([]github.WorkflowRun, error) {
	return g.runs, g.runsErr
}

func (g stubGithubClient) ListSecurityAlerts(
	_ context.Context,
) ([]github.SecurityAlert, error) {
	return g.alerts, g.alertsErr
}

type stubSentryClient struct {
	issues []sentryapi.Issue
	err    error
}

func (s stubSentryClient) ListUnresolvedIssues(
	_ context.Context,
) ([]sentryapi.Issue, error) {
	return s.issues, s.err
}

type stubStorageGetter struct {
	snap *models.StorageSnapshot
	err  error
}

func (s stubStorageGetter) Latest(
	_ context.Context,
) (*models.StorageSnapshot, error) {
	return s.snap, s.err
}

type stubSchemaSizer struct {
	sizes []models.SchemaStats
	err   error
}

func (s stubSchemaSizer) SchemaSizes(
	_ context.Context,
) ([]models.SchemaStats, error) {
	return s.sizes, s.err
}

type stubLatencyTrends struct {
	trends []models.TransactionTrend
	err    error
}

func (s stubLatencyTrends) Trends(
	_ context.Context,
) ([]models.TransactionTrend, error) {
	return s.trends, s.err
}

func loggerWithBuf() (*slog.Logger, *bytes.Buffer) {
	buf := &bytes.Buffer{}
	return slog.New(slog.NewTextHandler(buf, nil)), buf
}

func TestLogAPIErr(t *testing.T) {
	t.Run("transient logs at warn", func(t *testing.T) {
		logger, buf := loggerWithBuf()
		logAPIErr(t.Context(), logger, "poll failed", errors.New("boom"), true)
		assert.Contains(t, buf.String(), "level=WARN")
	})
	t.Run("non-transient logs at error", func(t *testing.T) {
		logger, buf := loggerWithBuf()
		logAPIErr(t.Context(), logger, "poll failed", errors.New("boom"), false)
		assert.Contains(t, buf.String(), "level=ERROR")
	})
}

func failedRun(branch, conclusion string) github.WorkflowRun {
	//nolint:exhaustruct //only Branch/Conclusion drive the workflow-run gauge
	return github.WorkflowRun{Branch: branch, Conclusion: conclusion}
}

func completedRun(name string, durationMs int64) github.WorkflowRun {
	//nolint:exhaustruct //only these fields drive the duration gauge
	return github.WorkflowRun{
		Name:       name,
		Branch:     "main",
		Status:     "completed",
		Conclusion: "success",
		StartedAt:  time.Unix(durationMs, 0),
		DurationMs: durationMs,
	}
}

// mainCompletedRun builds a completed main-branch run; only these fields
// drive the workflow-run gauges.
func mainCompletedRun(
	name string, startedAt time.Time, conclusion string,
) github.WorkflowRun {
	//nolint:exhaustruct //only these fields drive the workflow-run gauges
	return github.WorkflowRun{
		Name:       name,
		Branch:     "main",
		Status:     "completed",
		Conclusion: conclusion,
		StartedAt:  startedAt,
	}
}

// inProgressMainRun builds a main-branch run that is still running, so its
// conclusion is empty and it must not count as the latest completed run.
func inProgressMainRun(startedAt time.Time) github.WorkflowRun {
	//nolint:exhaustruct //only these fields drive the workflow-run gauges
	return github.WorkflowRun{
		Branch:    "main",
		Status:    "in_progress",
		StartedAt: startedAt,
	}
}

// workflowRunsJob builds a collector whose only contributing input is runs.
func workflowRunsJob(runs []github.WorkflowRun) *IssueSignalCollectorJob {
	return newStubJob(
		stubGithubClient{
			prs: nil, prsErr: nil, runs: runs, runsErr: nil,
			alerts: nil, alertsErr: nil,
		},
		stubSentryClient{issues: nil, err: nil},
		stubStorageGetter{snap: nil, err: database.ErrResourceNotFound},
		stubSchemaSizer{sizes: nil, err: nil},
	)
}

func TestIssueSignalCollectorWorkflowRunFailedLatestMain(t *testing.T) {
	t.Run("latest completed main run failed sets 1", func(t *testing.T) {
		resetGauges()
		job := workflowRunsJob([]github.WorkflowRun{
			mainCompletedRun("CI", time.Unix(100, 0), "success"),
			mainCompletedRun("CI", time.Unix(200, 0), "failure"),
		})
		logger, _ := loggerWithBuf()
		require.NoError(t, job.Run(t.Context(), logger))
		assert.InDelta(t, 1.0, testutil.ToFloat64(
			githubWorkflowRunFailed.WithLabelValues("main")), 0)
	})
	t.Run("newer completed success clears an older failure", func(t *testing.T) {
		resetGauges()
		job := workflowRunsJob([]github.WorkflowRun{
			mainCompletedRun("CI", time.Unix(100, 0), "failure"),
			mainCompletedRun("CI", time.Unix(200, 0), "success"),
		})
		logger, _ := loggerWithBuf()
		require.NoError(t, job.Run(t.Context(), logger))
		assert.InDelta(t, 0.0, testutil.ToFloat64(
			githubWorkflowRunFailed.WithLabelValues("main")), 0)
	})
	t.Run("in-progress newer run does not clear a completed failure", func(t *testing.T) {
		resetGauges()
		job := workflowRunsJob([]github.WorkflowRun{
			mainCompletedRun("CI", time.Unix(200, 0), "failure"),
			inProgressMainRun(time.Unix(300, 0)),
		})
		logger, _ := loggerWithBuf()
		require.NoError(t, job.Run(t.Context(), logger))
		assert.InDelta(t, 1.0, testutil.ToFloat64(
			githubWorkflowRunFailed.WithLabelValues("main")), 0)
	})
	t.Run("no completed main run keeps 0", func(t *testing.T) {
		resetGauges()
		job := workflowRunsJob([]github.WorkflowRun{
			inProgressMainRun(time.Unix(300, 0)),
			failedRun("feature", "failure"),
		})
		logger, _ := loggerWithBuf()
		require.NoError(t, job.Run(t.Context(), logger))
		assert.InDelta(t, 0.0, testutil.ToFloat64(
			githubWorkflowRunFailed.WithLabelValues("main")), 0)
	})
	t.Run("feature-branch failure is ignored", func(t *testing.T) {
		resetGauges()
		job := workflowRunsJob([]github.WorkflowRun{
			mainCompletedRun("CI", time.Unix(100, 0), "success"),
			failedRun("feature", "failure"),
		})
		logger, _ := loggerWithBuf()
		require.NoError(t, job.Run(t.Context(), logger))
		assert.InDelta(t, 0.0, testutil.ToFloat64(
			githubWorkflowRunFailed.WithLabelValues("main")), 0)
	})
}

func alertWithSeverity(sev string) github.SecurityAlert {
	//nolint:exhaustruct //only Severity drives the security-alert gauge
	return github.SecurityAlert{Severity: sev}
}

func failingPRs(n int) []github.PullRequest {
	return make([]github.PullRequest, n)
}

// resetGauges clears the process-wide gauges between tests.
func resetGauges() {
	githubFailingPullRequests.Set(0)
	githubFailingPullRequestsByRepo.Reset()
	sentryUnresolvedIssuesByRepo.Reset()
	githubWorkflowRunFailed.Reset()
	githubOpenSecurityAlerts.Reset()
	githubSecurityAlertsFetchOK.Set(0)
	r2OrphanedObjects.Set(0)
	r2StorageBytes.Set(0)
	githubWorkflowRunDurationSeconds.Reset()
	postgresSchemaSizeBytes.Reset()
	sentryUnresolvedIssues.Set(0)
	sentryUnresolvedFetchOK.Set(0)
	transactionP95Seconds.Reset()
}

func emptyStubJob() *IssueSignalCollectorJob {
	return NewIssueSignalCollectorJob(
		stubGithubClient{
			prs: nil, prsErr: nil, runs: nil, runsErr: nil,
			alerts: nil, alertsErr: nil,
		},
		stubSentryClient{issues: nil, err: nil},
		stubStorageGetter{snap: nil, err: database.ErrResourceNotFound},
		stubSchemaSizer{sizes: nil, err: nil},
		stubLatencyTrends{trends: nil, err: nil},
	)
}

func latencyTrendsJob(trends stubLatencyTrends) *IssueSignalCollectorJob {
	return NewIssueSignalCollectorJob(
		stubGithubClient{
			prs: nil, prsErr: nil, runs: nil, runsErr: nil,
			alerts: nil, alertsErr: nil,
		},
		stubSentryClient{issues: nil, err: nil},
		stubStorageGetter{snap: nil, err: database.ErrResourceNotFound},
		stubSchemaSizer{sizes: nil, err: nil},
		trends,
	)
}

func TestIssueSignalCollectorLatencyTrends(t *testing.T) {
	resetGauges()
	// A transaction that stopped regressing must not keep a stale value.
	transactionP95Seconds.WithLabelValues("tools-api", "recovered", "recent").Set(9)

	job := latencyTrendsJob(stubLatencyTrends{
		trends: []models.TransactionTrend{{
			Transaction:    "poll-feeds",
			Project:        "tools-api",
			PriorAvgP95Ms:  4500,
			RecentAvgP95Ms: 16000,
			PctChange:      2.56,
		}},
		err: nil,
	})

	logger, _ := loggerWithBuf()
	require.NoError(t, job.Run(t.Context(), logger))

	assert.InDelta(t, 16.0, testutil.ToFloat64(
		transactionP95Seconds.WithLabelValues("tools-api", "poll-feeds", "recent")),
		1e-9)
	assert.InDelta(t, 4.5, testutil.ToFloat64(
		transactionP95Seconds.WithLabelValues("tools-api", "poll-feeds", "prior")),
		1e-9)
	assert.Equal(t, 2, testutil.CollectAndCount(transactionP95Seconds))
}

func TestIssueSignalCollectorLatencyTrendsErrorKeepsGauge(t *testing.T) {
	resetGauges()
	transactionP95Seconds.WithLabelValues("tools-api", "poll-feeds", "recent").Set(16)

	job := latencyTrendsJob(stubLatencyTrends{trends: nil, err: errors.New("boom")})

	logger, buf := loggerWithBuf()
	require.NoError(t, job.Run(t.Context(), logger))

	assert.InDelta(t, 16.0, testutil.ToFloat64(
		transactionP95Seconds.WithLabelValues("tools-api", "poll-feeds", "recent")), 0)
	assert.Contains(t, buf.String(), "failed to load transaction latency trends")
}

func newStubJob(
	gh stubGithubClient,
	sentry stubSentryClient,
	storage stubStorageGetter,
	schemas stubSchemaSizer,
) *IssueSignalCollectorJob {
	return NewIssueSignalCollectorJob(
		gh, sentry, storage, schemas,
		stubLatencyTrends{trends: nil, err: nil},
	)
}

func TestIssueSignalCollectorIDAndRunEvery(t *testing.T) {
	job := newStubJob(
		stubGithubClient{
			prs: nil, prsErr: nil, runs: nil, runsErr: nil,
			alerts: nil, alertsErr: nil,
		},
		stubSentryClient{issues: nil, err: nil},
		stubStorageGetter{snap: nil, err: nil},
		stubSchemaSizer{sizes: nil, err: nil},
	)
	assert.Equal(t, "collect-issue-signals", job.ID())
	assert.Positive(t, job.RunEvery())
}

func TestIssueSignalCollectorConnectedSetsGauges(t *testing.T) {
	resetGauges()
	job := newStubJob(
		stubGithubClient{
			prs:    failingPRs(2),
			prsErr: nil,
			runs: []github.WorkflowRun{
				failedRun("feature", "failure"),
				completedRun("CI", 300),
				completedRun("CI", 420),
				completedRun("Deploy", 90),
				mainCompletedRun("triage", time.Unix(500, 0), "failure"),
			},
			runsErr: nil,
			alerts: []github.SecurityAlert{
				alertWithSeverity("high"),
				alertWithSeverity("high"),
				alertWithSeverity("low"),
			},
			alertsErr: nil,
		},
		stubSentryClient{
			//nolint:exhaustruct //only the count of issues drives the gauge
			issues: []sentryapi.Issue{{}, {}, {}},
			err:    nil,
		},
		stubStorageGetter{
			//nolint:exhaustruct //only OrphanCount/TotalSizeBytes are read
			snap: &models.StorageSnapshot{OrphanCount: 7, TotalSizeBytes: 123456},
			err:  nil,
		},
		stubSchemaSizer{
			sizes: []models.SchemaStats{
				{Name: "books", SizeBytes: 5000, TableCount: 4},
				{Name: "games", SizeBytes: 2000, TableCount: 3},
			},
			err: nil,
		},
	)

	logger, _ := loggerWithBuf()
	require.NoError(t, job.Run(t.Context(), logger))

	assert.InDelta(t, 2.0, testutil.ToFloat64(githubFailingPullRequests), 0)
	assert.InDelta(t, 1.0,
		testutil.ToFloat64(githubWorkflowRunFailed.WithLabelValues("main")), 0)
	assert.InDelta(t, 2.0,
		testutil.ToFloat64(githubOpenSecurityAlerts.WithLabelValues("high")), 0)
	assert.InDelta(t, 1.0,
		testutil.ToFloat64(githubOpenSecurityAlerts.WithLabelValues("low")), 0)
	assert.InDelta(t, 3.0, testutil.ToFloat64(sentryUnresolvedIssues), 0)
	assert.InDelta(t, 7.0, testutil.ToFloat64(r2OrphanedObjects), 0)
	assert.InDelta(t, 123456.0, testutil.ToFloat64(r2StorageBytes), 0)
	assert.InDelta(t, 0.42, testutil.ToFloat64(
		githubWorkflowRunDurationSeconds.WithLabelValues("CI")), 1e-9)
	assert.InDelta(t, 0.09, testutil.ToFloat64(
		githubWorkflowRunDurationSeconds.WithLabelValues("Deploy")), 1e-9)
	assert.InDelta(t, 5000.0, testutil.ToFloat64(
		postgresSchemaSizeBytes.WithLabelValues("books")), 0)
	assert.InDelta(t, 2000.0, testutil.ToFloat64(
		postgresSchemaSizeBytes.WithLabelValues("games")), 0)
}

func TestIssueSignalCollectorNotConnectedLeavesGaugesUntouched(t *testing.T) {
	resetGauges()
	githubFailingPullRequests.Set(11)
	r2OrphanedObjects.Set(33)
	r2StorageBytes.Set(44)
	sentryUnresolvedIssues.Set(22)

	job := newStubJob(
		stubGithubClient{
			prs:       nil,
			prsErr:    github.ErrNotConfigured,
			runs:      nil,
			runsErr:   github.ErrNotConfigured,
			alerts:    nil,
			alertsErr: github.ErrNotConfigured,
		},
		stubSentryClient{issues: nil, err: sentryapi.ErrNotConfigured},
		stubStorageGetter{snap: nil, err: database.ErrResourceNotFound},
		stubSchemaSizer{sizes: nil, err: nil},
	)

	logger, buf := loggerWithBuf()
	require.NoError(t, job.Run(t.Context(), logger))

	assert.InDelta(t, 11.0, testutil.ToFloat64(githubFailingPullRequests), 0)
	assert.InDelta(t, 33.0, testutil.ToFloat64(r2OrphanedObjects), 0)
	assert.InDelta(t, 44.0, testutil.ToFloat64(r2StorageBytes), 0)
	assert.InDelta(t, 22.0, testutil.ToFloat64(sentryUnresolvedIssues), 0)
	assert.Empty(t, buf.String())
}

func TestIssueSignalCollectorTransientErrorsAreLoggedNotFatal(t *testing.T) {
	resetGauges()
	boom := errors.New("upstream down")
	job := newStubJob(
		stubGithubClient{
			prs:       nil,
			prsErr:    boom,
			runs:      nil,
			runsErr:   boom,
			alerts:    nil,
			alertsErr: boom,
		},
		stubSentryClient{issues: nil, err: boom},
		stubStorageGetter{snap: nil, err: boom},
		stubSchemaSizer{sizes: nil, err: boom},
	)

	logger, buf := loggerWithBuf()
	require.NoError(t, job.Run(t.Context(), logger))
	assert.Contains(t, buf.String(), "issue-signal-collector")
}

func TestIssueSignalCollectorPartialProviderStillCollectsOthers(t *testing.T) {
	resetGauges()
	job := newStubJob(
		stubGithubClient{
			prs:       failingPRs(1),
			prsErr:    nil,
			runs:      nil,
			runsErr:   nil,
			alerts:    nil,
			alertsErr: nil,
		},
		stubSentryClient{issues: nil, err: nil},
		stubStorageGetter{snap: nil, err: database.ErrResourceNotFound},
		stubSchemaSizer{sizes: nil, err: nil},
	)

	logger, buf := loggerWithBuf()
	require.NoError(t, job.Run(t.Context(), logger))

	assert.InDelta(t, 1.0, testutil.ToFloat64(githubFailingPullRequests), 0)
	assert.Empty(t, buf.String())
}
