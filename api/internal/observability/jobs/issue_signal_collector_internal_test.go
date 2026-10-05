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

	"tools.xdoubleu.com/internal/database"
	"tools.xdoubleu.com/internal/github"
	"tools.xdoubleu.com/internal/models"
	"tools.xdoubleu.com/internal/sentryapi"
)

type stubGithubClient struct {
	prs       []github.PullRequest
	prsErr    error
	runs      []github.WorkflowRun
	runsErr   error
	alerts    []github.SecurityAlert
	alertsErr error
	vars      map[string]string
	varsErr   error
}

func (g stubGithubClient) ListFailingPullRequests(
	_ context.Context,
) ([]github.PullRequest, error) {
	return g.prs, g.prsErr
}

func (g stubGithubClient) ListRepositoryVariables(
	_ context.Context,
) (map[string]string, error) {
	return g.vars, g.varsErr
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

type stubAutomatedActionGetter struct {
	firedAt time.Time
	err     error

	// byRoutine, when non-nil, overrides firedAt/err per routine name.
	byRoutine map[string]stubAutomatedActionGetter
}

func (s stubAutomatedActionGetter) OldestOpenFiredAt(
	_ context.Context,
) (time.Time, error) {
	return s.firedAt, s.err
}

func (s stubAutomatedActionGetter) MostRecentOpenedAt(
	_ context.Context,
	routine string,
) (time.Time, error) {
	if s.byRoutine != nil {
		r, ok := s.byRoutine[routine]
		if !ok {
			return time.Time{}, database.ErrResourceNotFound
		}
		return r.firedAt, r.err
	}
	return s.firedAt, s.err
}

func (s stubAutomatedActionGetter) LatestRunMetrics(
	_ context.Context,
	_ time.Time,
) (map[string]models.RunMetrics, error) {
	return map[string]models.RunMetrics{}, nil
}

// stubRunMetricsGetter overrides LatestRunMetrics.
type stubRunMetricsGetter struct {
	stubAutomatedActionGetter

	metrics map[string]models.RunMetrics
	err     error
}

func (s stubRunMetricsGetter) LatestRunMetrics(
	_ context.Context,
	_ time.Time,
) (map[string]models.RunMetrics, error) {
	return s.metrics, s.err
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
			vars: nil, varsErr: nil,
		},
		stubSentryClient{issues: nil, err: nil},
		stubStorageGetter{snap: nil, err: database.ErrResourceNotFound},
		stubSchemaSizer{sizes: nil, err: nil},
		stubAutomatedActionGetter{firedAt: time.Now(), err: nil, byRoutine: nil},
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
	githubWorkflowRunFailed.Reset()
	githubOpenSecurityAlerts.Reset()
	githubSecurityAlertsFetchOK.Set(0)
	r2OrphanedObjects.Set(0)
	r2StorageBytes.Set(0)
	githubWorkflowRunDurationSeconds.Reset()
	postgresSchemaSizeBytes.Reset()
	automatedActionOldestOpenAgeSeconds.Set(0)
	automatedActionSecondsSinceLastOpen.Reset()
	automatedActionRoutinePaused.Reset()
	automatedActionLastRun.Reset()
	sentryUnresolvedIssues.Set(0)
	transactionP95Seconds.Reset()
}

func emptyStubJob(automatedAction automatedActionGetter) *IssueSignalCollectorJob {
	return NewIssueSignalCollectorJob(
		stubGithubClient{
			prs: nil, prsErr: nil, runs: nil, runsErr: nil,
			alerts: nil, alertsErr: nil,
			vars: nil, varsErr: nil,
		},
		stubSentryClient{issues: nil, err: nil},
		stubStorageGetter{snap: nil, err: database.ErrResourceNotFound},
		stubSchemaSizer{sizes: nil, err: nil},
		automatedAction,
		stubLatencyTrends{trends: nil, err: nil},
	)
}

func TestIssueSignalCollectorRoutineRunMetrics(t *testing.T) {
	resetGauges()
	// A routine missing from the latest result must not keep a stale value.
	automatedActionLastRun.WithLabelValues("retired", "requests").Set(9)

	job := emptyStubJob(stubRunMetricsGetter{
		stubAutomatedActionGetter: stubAutomatedActionGetter{
			firedAt: time.Now(), err: nil, byRoutine: nil,
		},
		metrics: map[string]models.RunMetrics{
			"red-pr-repair": {
				Requests:          14,
				InputTokens:       52000,
				OutputTokens:      3100,
				ReasoningTokens:   900,
				CacheReadTokens:   40000,
				CostUSD:           0.021,
				DurationSeconds:   95,
				ToolCalls:         22,
				ToolErrors:        2,
				RepeatedToolCalls: 1,
			},
		},
		err: nil,
	})

	logger, _ := loggerWithBuf()
	require.NoError(t, job.Run(t.Context(), logger))

	for metric, want := range map[string]float64{
		"requests":            14,
		"input_tokens":        52000,
		"output_tokens":       3100,
		"reasoning_tokens":    900,
		"cache_read_tokens":   40000,
		"cost_usd":            0.021,
		"duration_seconds":    95,
		"tool_calls":          22,
		"tool_errors":         2,
		"repeated_tool_calls": 1,
	} {
		assert.InDelta(t, want, testutil.ToFloat64(
			automatedActionLastRun.WithLabelValues("red-pr-repair", metric)),
			1e-9, metric)
	}
	assert.Equal(t, 10, testutil.CollectAndCount(automatedActionLastRun))
}

func TestIssueSignalCollectorRoutineRunMetricsErrorKeepsGauges(t *testing.T) {
	resetGauges()
	automatedActionLastRun.WithLabelValues("red-pr-repair", "requests").Set(7)

	job := emptyStubJob(stubRunMetricsGetter{
		stubAutomatedActionGetter: stubAutomatedActionGetter{
			firedAt: time.Now(), err: nil, byRoutine: nil,
		},
		metrics: nil,
		err:     errors.New("boom"),
	})

	logger, buf := loggerWithBuf()
	require.NoError(t, job.Run(t.Context(), logger))

	assert.InDelta(t, 7.0, testutil.ToFloat64(
		automatedActionLastRun.WithLabelValues("red-pr-repair", "requests")), 0)
	assert.Contains(t, buf.String(), "failed to load routine run metrics")
}

func latencyTrendsJob(trends stubLatencyTrends) *IssueSignalCollectorJob {
	return NewIssueSignalCollectorJob(
		stubGithubClient{
			prs: nil, prsErr: nil, runs: nil, runsErr: nil,
			alerts: nil, alertsErr: nil,
			vars: nil, varsErr: nil,
		},
		stubSentryClient{issues: nil, err: nil},
		stubStorageGetter{snap: nil, err: database.ErrResourceNotFound},
		stubSchemaSizer{sizes: nil, err: nil},
		stubAutomatedActionGetter{firedAt: time.Now(), err: nil, byRoutine: nil},
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
	automatedAction stubAutomatedActionGetter,
) *IssueSignalCollectorJob {
	return NewIssueSignalCollectorJob(
		gh, sentry, storage, schemas, automatedAction,
		stubLatencyTrends{trends: nil, err: nil},
	)
}

func TestIssueSignalCollectorIDAndRunEvery(t *testing.T) {
	job := newStubJob(
		stubGithubClient{
			prs: nil, prsErr: nil, runs: nil, runsErr: nil,
			alerts: nil, alertsErr: nil,
			vars: nil, varsErr: nil,
		},
		stubSentryClient{issues: nil, err: nil},
		stubStorageGetter{snap: nil, err: nil},
		stubSchemaSizer{sizes: nil, err: nil},
		stubAutomatedActionGetter{firedAt: time.Time{}, err: nil, byRoutine: nil},
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
			vars:      nil, varsErr: nil,
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
		stubAutomatedActionGetter{
			firedAt:   time.Now().Add(-time.Hour),
			err:       nil,
			byRoutine: nil,
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
	assert.InDelta(t, 3600.0,
		testutil.ToFloat64(automatedActionOldestOpenAgeSeconds), 5)
	for _, routine := range knownRoutines {
		assert.InDelta(t, 3600.0, testutil.ToFloat64(
			automatedActionSecondsSinceLastOpen.WithLabelValues(routine)), 5)
	}
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
			vars:      nil,
			varsErr:   github.ErrNotConfigured,
		},
		stubSentryClient{issues: nil, err: sentryapi.ErrNotConfigured},
		stubStorageGetter{snap: nil, err: database.ErrResourceNotFound},
		stubSchemaSizer{sizes: nil, err: nil},
		stubAutomatedActionGetter{
			firedAt:   time.Time{},
			err:       database.ErrResourceNotFound,
			byRoutine: nil,
		},
	)

	logger, buf := loggerWithBuf()
	require.NoError(t, job.Run(t.Context(), logger))

	assert.InDelta(t, 11.0, testutil.ToFloat64(githubFailingPullRequests), 0)
	assert.InDelta(t, 33.0, testutil.ToFloat64(r2OrphanedObjects), 0)
	assert.InDelta(t, 44.0, testutil.ToFloat64(r2StorageBytes), 0)
	assert.InDelta(t, 22.0, testutil.ToFloat64(sentryUnresolvedIssues), 0)
	assert.InDelta(t, 0.0,
		testutil.ToFloat64(automatedActionOldestOpenAgeSeconds), 0)
	for _, routine := range knownRoutines {
		assert.InDelta(t, float64(neverOpenedSentinelSeconds), testutil.ToFloat64(
			automatedActionSecondsSinceLastOpen.WithLabelValues(routine)), 0)
	}
	assert.Empty(t, buf.String())
}

func TestIssueSignalCollectorRoutineLivenessMixedStates(t *testing.T) {
	resetGauges()
	job := newStubJob(
		stubGithubClient{
			prs: nil, prsErr: nil, runs: nil, runsErr: nil,
			alerts: nil, alertsErr: nil,
			vars: nil, varsErr: nil,
		},
		stubSentryClient{issues: nil, err: nil},
		stubStorageGetter{snap: nil, err: database.ErrResourceNotFound},
		stubSchemaSizer{sizes: nil, err: nil},
		stubAutomatedActionGetter{
			firedAt: time.Time{},
			err:     database.ErrResourceNotFound,
			byRoutine: map[string]stubAutomatedActionGetter{
				"nightly-maintenance-sweep": {
					firedAt:   time.Now().Add(-30 * time.Minute),
					err:       nil,
					byRoutine: nil,
				},
				"ready-issues-executor": {
					firedAt:   time.Time{},
					err:       database.ErrResourceNotFound,
					byRoutine: nil,
				},
				// "red-pr-repair" is omitted to exercise byRoutine's not-found fallback.
			},
		},
	)

	logger, buf := loggerWithBuf()
	require.NoError(t, job.Run(t.Context(), logger))

	assert.InDelta(t, 1800.0, testutil.ToFloat64(
		automatedActionSecondsSinceLastOpen.
			WithLabelValues("nightly-maintenance-sweep")), 5)
	assert.InDelta(t, float64(neverOpenedSentinelSeconds), testutil.ToFloat64(
		automatedActionSecondsSinceLastOpen.
			WithLabelValues("ready-issues-executor")), 0)
	assert.InDelta(t, float64(neverOpenedSentinelSeconds), testutil.ToFloat64(
		automatedActionSecondsSinceLastOpen.
			WithLabelValues("red-pr-repair")), 0)
	assert.Empty(t, buf.String())
}

func TestIssueSignalCollectorRoutineLivenessLogsNonNotFoundError(t *testing.T) {
	resetGauges()
	boom := errors.New("db unreachable")
	job := newStubJob(
		stubGithubClient{
			prs: nil, prsErr: nil, runs: nil, runsErr: nil,
			alerts: nil, alertsErr: nil,
			vars: nil, varsErr: nil,
		},
		stubSentryClient{issues: nil, err: nil},
		stubStorageGetter{snap: nil, err: database.ErrResourceNotFound},
		stubSchemaSizer{sizes: nil, err: nil},
		stubAutomatedActionGetter{
			firedAt:   time.Time{},
			err:       boom,
			byRoutine: nil,
		},
	)

	logger, buf := loggerWithBuf()
	require.NoError(t, job.Run(t.Context(), logger))
	assert.Contains(t, buf.String(), "most recent open")
}

func TestIssueSignalCollectorRoutinePausedStates(t *testing.T) {
	resetGauges()
	job := newStubJob(
		stubGithubClient{
			prs: nil, prsErr: nil, runs: nil, runsErr: nil,
			alerts: nil, alertsErr: nil,
			vars: map[string]string{
				"ROUTINE_NIGHTLY_MAINTENANCE_SWEEP_ENABLED": "true",
				"ROUTINE_READY_ISSUES_EXECUTOR_ENABLED":     "false",
				// "red-pr-repair" has no variable at all — paused like "false".
			},
			varsErr: nil,
		},
		stubSentryClient{issues: nil, err: nil},
		stubStorageGetter{snap: nil, err: database.ErrResourceNotFound},
		stubSchemaSizer{sizes: nil, err: nil},
		stubAutomatedActionGetter{firedAt: time.Now(), err: nil, byRoutine: nil},
	)

	logger, buf := loggerWithBuf()
	require.NoError(t, job.Run(t.Context(), logger))

	assert.InDelta(t, 0.0, testutil.ToFloat64(
		automatedActionRoutinePaused.
			WithLabelValues("nightly-maintenance-sweep")), 0)
	assert.InDelta(t, 1.0, testutil.ToFloat64(
		automatedActionRoutinePaused.
			WithLabelValues("ready-issues-executor")), 0)
	assert.InDelta(t, 1.0, testutil.ToFloat64(
		automatedActionRoutinePaused.
			WithLabelValues("red-pr-repair")), 0)
	assert.Empty(t, buf.String())
}

func TestIssueSignalCollectorRoutinePausedErrorLeavesGauge(t *testing.T) {
	resetGauges()
	automatedActionRoutinePaused.WithLabelValues("nightly-maintenance-sweep").Set(1)

	boom := errors.New("upstream down")
	job := newStubJob(
		stubGithubClient{
			prs: nil, prsErr: nil, runs: nil, runsErr: nil,
			alerts: nil, alertsErr: nil,
			vars: nil, varsErr: boom,
		},
		stubSentryClient{issues: nil, err: nil},
		stubStorageGetter{snap: nil, err: database.ErrResourceNotFound},
		stubSchemaSizer{sizes: nil, err: nil},
		stubAutomatedActionGetter{firedAt: time.Now(), err: nil, byRoutine: nil},
	)

	logger, buf := loggerWithBuf()
	require.NoError(t, job.Run(t.Context(), logger))

	assert.InDelta(t, 1.0, testutil.ToFloat64(
		automatedActionRoutinePaused.
			WithLabelValues("nightly-maintenance-sweep")), 0)
	assert.Contains(t, buf.String(), "failed to list repository variables")
}

func TestIssueSignalCollectorRoutinePausedNotConfiguredIsSilent(t *testing.T) {
	resetGauges()
	job := newStubJob(
		stubGithubClient{
			prs: nil, prsErr: github.ErrNotConfigured,
			runs: nil, runsErr: github.ErrNotConfigured,
			alerts: nil, alertsErr: github.ErrNotConfigured,
			vars: nil, varsErr: github.ErrNotConfigured,
		},
		stubSentryClient{issues: nil, err: sentryapi.ErrNotConfigured},
		stubStorageGetter{snap: nil, err: database.ErrResourceNotFound},
		stubSchemaSizer{sizes: nil, err: nil},
		stubAutomatedActionGetter{
			firedAt: time.Time{}, err: database.ErrResourceNotFound,
			byRoutine: nil,
		},
	)

	logger, buf := loggerWithBuf()
	require.NoError(t, job.Run(t.Context(), logger))
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
			vars:      nil,
			varsErr:   boom,
		},
		stubSentryClient{issues: nil, err: boom},
		stubStorageGetter{snap: nil, err: boom},
		stubSchemaSizer{sizes: nil, err: boom},
		stubAutomatedActionGetter{firedAt: time.Time{}, err: boom, byRoutine: nil},
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
			vars:      nil,
			varsErr:   nil,
		},
		stubSentryClient{issues: nil, err: nil},
		stubStorageGetter{snap: nil, err: database.ErrResourceNotFound},
		stubSchemaSizer{sizes: nil, err: nil},
		stubAutomatedActionGetter{
			firedAt:   time.Time{},
			err:       database.ErrResourceNotFound,
			byRoutine: nil,
		},
	)

	logger, buf := loggerWithBuf()
	require.NoError(t, job.Run(t.Context(), logger))

	assert.InDelta(t, 1.0, testutil.ToFloat64(githubFailingPullRequests), 0)
	assert.Empty(t, buf.String())
}
