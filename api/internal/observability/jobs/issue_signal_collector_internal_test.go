package jobs

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/database"
	"tools.xdoubleu.com/internal/github"
	"tools.xdoubleu.com/internal/models"
	"tools.xdoubleu.com/internal/sentryapi"
)

type stubSentryClient struct {
	issues []sentryapi.Issue
	err    error
}

func (s stubSentryClient) ListUnresolvedIssues(
	_ context.Context,
) ([]sentryapi.Issue, error) {
	return s.issues, s.err
}
func (s stubSentryClient) ResolveIssue(_ context.Context, _ string) error { return nil }

func (s stubSentryClient) ListOrgs(_ context.Context) ([]sentryapi.Org, error) {
	return nil, nil
}

func (s stubSentryClient) ListProjects(
	_ context.Context, _ string,
) ([]sentryapi.Project, error) {
	return nil, nil
}

func (s stubSentryClient) ListTransactionStats(
	_ context.Context,
) ([]sentryapi.TransactionStat, error) {
	return nil, nil
}

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

type stubStorageGetter struct {
	snap *models.StorageSnapshot
	err  error
}

func (s stubStorageGetter) Latest(
	_ context.Context,
) (*models.StorageSnapshot, error) {
	return s.snap, s.err
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

func alertWithSeverity(sev string) github.SecurityAlert {
	//nolint:exhaustruct //only Severity drives the security-alert gauge
	return github.SecurityAlert{Severity: sev}
}

func unresolvedIssues(n int) []sentryapi.Issue {
	out := make([]sentryapi.Issue, n)
	return out
}

func failingPRs(n int) []github.PullRequest {
	return make([]github.PullRequest, n)
}

// resetGauges clears every issue-signal gauge so one test's writes don't
// leak into another's assertions (the collectors are process-wide).
func resetGauges() {
	githubFailingPullRequests.Set(0)
	githubWorkflowRunFailed.Reset()
	githubOpenSecurityAlerts.Reset()
	sentryUnresolvedIssues.Set(0)
	r2OrphanedObjects.Set(0)
	r2StorageBytes.Set(0)
}

func newStubJob(
	sentry stubSentryClient,
	gh stubGithubClient,
	storage stubStorageGetter,
) *IssueSignalCollectorJob {
	return NewIssueSignalCollectorJob(sentry, gh, storage)
}

func TestIssueSignalCollectorIDAndRunEvery(t *testing.T) {
	job := newStubJob(
		stubSentryClient{issues: nil, err: nil},
		stubGithubClient{
			prs: nil, prsErr: nil, runs: nil, runsErr: nil,
			alerts: nil, alertsErr: nil,
		},
		stubStorageGetter{snap: nil, err: nil},
	)
	assert.Equal(t, "collect-issue-signals", job.ID())
	assert.Positive(t, job.RunEvery())
}

func TestIssueSignalCollectorConnectedSetsGauges(t *testing.T) {
	resetGauges()
	job := newStubJob(
		stubSentryClient{issues: unresolvedIssues(3), err: nil},
		stubGithubClient{
			prs:    failingPRs(2),
			prsErr: nil,
			runs: []github.WorkflowRun{
				failedRun("main", "failure"),
				failedRun("main", "failure"),
				failedRun("main", "success"),
				failedRun("feature", "failure"),
			},
			runsErr: nil,
			alerts: []github.SecurityAlert{
				alertWithSeverity("high"),
				alertWithSeverity("high"),
				alertWithSeverity("low"),
			},
			alertsErr: nil,
		},
		stubStorageGetter{
			//nolint:exhaustruct //only OrphanCount/TotalSizeBytes are read
			snap: &models.StorageSnapshot{OrphanCount: 7, TotalSizeBytes: 123456},
			err:  nil,
		},
	)

	logger, _ := loggerWithBuf()
	require.NoError(t, job.Run(t.Context(), logger))

	assert.InDelta(t, 2.0, testutil.ToFloat64(githubFailingPullRequests), 0)
	assert.InDelta(t, 2.0,
		testutil.ToFloat64(githubWorkflowRunFailed.WithLabelValues("main")), 0)
	assert.InDelta(t, 2.0,
		testutil.ToFloat64(githubOpenSecurityAlerts.WithLabelValues("high")), 0)
	assert.InDelta(t, 1.0,
		testutil.ToFloat64(githubOpenSecurityAlerts.WithLabelValues("low")), 0)
	assert.InDelta(t, 3.0, testutil.ToFloat64(sentryUnresolvedIssues), 0)
	assert.InDelta(t, 7.0, testutil.ToFloat64(r2OrphanedObjects), 0)
	assert.InDelta(t, 123456.0, testutil.ToFloat64(r2StorageBytes), 0)
}

func TestIssueSignalCollectorNotConnectedLeavesGaugesUntouched(t *testing.T) {
	resetGauges()
	githubFailingPullRequests.Set(11)
	sentryUnresolvedIssues.Set(22)
	r2OrphanedObjects.Set(33)
	r2StorageBytes.Set(44)

	job := newStubJob(
		stubSentryClient{issues: nil, err: sentryapi.ErrNotConfigured},
		stubGithubClient{
			prs:       nil,
			prsErr:    github.ErrNotConfigured,
			runs:      nil,
			runsErr:   github.ErrNotConfigured,
			alerts:    nil,
			alertsErr: github.ErrNotConfigured,
		},
		stubStorageGetter{snap: nil, err: database.ErrResourceNotFound},
	)

	logger, buf := loggerWithBuf()
	require.NoError(t, job.Run(t.Context(), logger))

	assert.InDelta(t, 11.0, testutil.ToFloat64(githubFailingPullRequests), 0)
	assert.InDelta(t, 22.0, testutil.ToFloat64(sentryUnresolvedIssues), 0)
	assert.InDelta(t, 33.0, testutil.ToFloat64(r2OrphanedObjects), 0)
	assert.InDelta(t, 44.0, testutil.ToFloat64(r2StorageBytes), 0)
	assert.Empty(t, buf.String())
}

func TestIssueSignalCollectorTransientErrorsAreLoggedNotFatal(t *testing.T) {
	resetGauges()
	boom := errors.New("upstream down")
	job := newStubJob(
		stubSentryClient{issues: nil, err: boom},
		stubGithubClient{
			prs:       nil,
			prsErr:    boom,
			runs:      nil,
			runsErr:   boom,
			alerts:    nil,
			alertsErr: boom,
		},
		stubStorageGetter{snap: nil, err: boom},
	)

	logger, buf := loggerWithBuf()
	require.NoError(t, job.Run(t.Context(), logger))
	assert.Contains(t, buf.String(), "issue-signal-collector")
}

func TestIssueSignalCollectorPartialProviderStillCollectsOthers(t *testing.T) {
	resetGauges()
	job := newStubJob(
		stubSentryClient{issues: nil, err: sentryapi.ErrNotConfigured},
		stubGithubClient{
			prs:       failingPRs(1),
			prsErr:    nil,
			runs:      nil,
			runsErr:   nil,
			alerts:    nil,
			alertsErr: nil,
		},
		stubStorageGetter{snap: nil, err: database.ErrResourceNotFound},
	)

	logger, buf := loggerWithBuf()
	require.NoError(t, job.Run(t.Context(), logger))

	assert.InDelta(t, 1.0, testutil.ToFloat64(githubFailingPullRequests), 0)
	assert.Empty(t, buf.String())
}
