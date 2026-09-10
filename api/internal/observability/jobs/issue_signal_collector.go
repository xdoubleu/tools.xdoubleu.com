package jobs

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"tools.xdoubleu.com/internal/database"
	"tools.xdoubleu.com/internal/github"
	essentialogger "tools.xdoubleu.com/internal/logging"
	"tools.xdoubleu.com/internal/models"
	"tools.xdoubleu.com/internal/sentryapi"
)

// Issue-signal gauges, registered on client_golang's default registry which
// cmd/api's /metrics handler already serves (mirrors jobDuration in
// internal/observability/trackedjob.go and the histograms in
// internal/middleware/metrics.go). IssueSignalCollectorJob refreshes them on
// a timer — never at scrape time — because the GitHub and Sentry APIs behind
// them are rate-limited.
//
//nolint:gochecknoglobals //Prometheus collectors are process-wide by design
var (
	githubFailingPullRequests = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "github_failing_pull_requests",
		Help: "Open pull requests with at least one failing CI check.",
	})
	githubWorkflowRunFailed = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "github_workflow_run_failed",
		Help: "Recent GitHub Actions workflow runs that concluded in failure, by branch.",
	}, []string{"branch"})
	githubOpenSecurityAlerts = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "github_open_security_alerts",
		Help: "Open Dependabot, code-scanning and secret-scanning alerts, by severity.",
	}, []string{"severity"})
	sentryUnresolvedIssues = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "sentry_unresolved_issues",
		Help: "Unresolved Sentry issues in the configured project.",
	})
	r2OrphanedObjects = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "r2_orphaned_objects",
		Help: "Orphaned R2 storage objects in the latest storage snapshot.",
	})
	r2StorageBytes = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "r2_storage_bytes",
		Help: "Total R2 storage size in bytes from the latest storage snapshot.",
	})
)

// mainBranch is the branch label the workflow-run gauge reports on — only
// failures on the default branch are tracked.
const mainBranch = "main"

// failingPRLister is the subset of github.Client the failing-PR gauge needs.
type failingPRLister interface {
	ListFailingPullRequests(ctx context.Context) ([]github.PullRequest, error)
}

// securityAlertLister is the subset of github.Client the security-alert
// gauge needs.
type securityAlertLister interface {
	ListSecurityAlerts(ctx context.Context) ([]github.SecurityAlert, error)
}

// latestStorageSnapshotGetter is the subset of
// *repositories.StorageSnapshotsRepository the storage gauges need.
type latestStorageSnapshotGetter interface {
	Latest(ctx context.Context) (*models.StorageSnapshot, error)
}

// workflowRunsLister is the subset of github.Client the collector needs on
// top of failingPRLister and securityAlertLister.
type workflowRunsLister interface {
	ListWorkflowRuns(ctx context.Context) ([]github.WorkflowRun, error)
}

// issueSignalGithubClient is the subset of github.Client
// IssueSignalCollectorJob reads.
type issueSignalGithubClient interface {
	failingPRLister
	securityAlertLister
	workflowRunsLister
}

// runEvery is the poll interval shared by the timer-driven observability
// jobs in this package; "realtime" here means "within a few minutes", not
// sub-second.
const runEvery = 5 * time.Minute

// IssueSignalCollectorJob refreshes the issue-signal Prometheus gauges from
// GitHub, Sentry and the latest storage snapshot. A provider that isn't
// connected leaves its gauge untouched rather than resetting it to zero or
// failing the run; other errors are logged and skipped. Run always returns
// nil.
type IssueSignalCollectorJob struct {
	sentry          sentryapi.Client
	gh              issueSignalGithubClient
	storageSnapshot latestStorageSnapshotGetter
}

func NewIssueSignalCollectorJob(
	sentry sentryapi.Client,
	gh issueSignalGithubClient,
	storageSnapshot latestStorageSnapshotGetter,
) *IssueSignalCollectorJob {
	return &IssueSignalCollectorJob{
		sentry:          sentry,
		gh:              gh,
		storageSnapshot: storageSnapshot,
	}
}

func (j *IssueSignalCollectorJob) ID() string {
	return "collect-issue-signals"
}

// RunEvery reuses the package-level runEvery (5 minutes).
func (j *IssueSignalCollectorJob) RunEvery() time.Duration {
	return runEvery
}

// logAPIErr logs a poll failure at Warn (transient, self-heals on the next
// poll) or Error (reaches Sentry, needs a look) depending on whether the
// client classified the error as a known-benign shape.
func logAPIErr(
	ctx context.Context, logger *slog.Logger, msg string, err error, transient bool,
) {
	if transient {
		logger.WarnContext(ctx, msg, essentialogger.ErrAttr(err))
		return
	}
	logger.ErrorContext(ctx, msg, essentialogger.ErrAttr(err))
}

func (j *IssueSignalCollectorJob) Run(
	ctx context.Context,
	logger *slog.Logger,
) error {
	j.collectFailingPullRequests(ctx, logger)
	j.collectWorkflowRuns(ctx, logger)
	j.collectSecurityAlerts(ctx, logger)
	j.collectSentryIssues(ctx, logger)
	j.collectStorage(ctx, logger)
	return nil
}

func (j *IssueSignalCollectorJob) collectFailingPullRequests(
	ctx context.Context,
	logger *slog.Logger,
) {
	prs, err := j.gh.ListFailingPullRequests(ctx)
	if errors.Is(err, github.ErrNotConfigured) {
		return
	}
	if err != nil {
		logAPIErr(ctx, logger,
			"issue-signal-collector: failed to list failing pull requests",
			err, github.IsTransientAPIError(err))
		return
	}
	githubFailingPullRequests.Set(float64(len(prs)))
}

func (j *IssueSignalCollectorJob) collectWorkflowRuns(
	ctx context.Context,
	logger *slog.Logger,
) {
	runs, err := j.gh.ListWorkflowRuns(ctx)
	if errors.Is(err, github.ErrNotConfigured) {
		return
	}
	if err != nil {
		logAPIErr(ctx, logger,
			"issue-signal-collector: failed to list workflow runs",
			err, github.IsTransientAPIError(err))
		return
	}

	failed := 0
	for _, run := range runs {
		if run.Branch == mainBranch && run.Conclusion == "failure" {
			failed++
		}
	}
	githubWorkflowRunFailed.Reset()
	githubWorkflowRunFailed.WithLabelValues(mainBranch).Set(float64(failed))
}

func (j *IssueSignalCollectorJob) collectSecurityAlerts(
	ctx context.Context,
	logger *slog.Logger,
) {
	alerts, err := j.gh.ListSecurityAlerts(ctx)
	if errors.Is(err, github.ErrNotConfigured) {
		return
	}
	if err != nil {
		logAPIErr(ctx, logger,
			"issue-signal-collector: failed to list security alerts",
			err, github.IsTransientAPIError(err))
		return
	}

	bySeverity := make(map[string]int, len(alerts))
	for _, alert := range alerts {
		bySeverity[alert.Severity]++
	}
	githubOpenSecurityAlerts.Reset()
	for severity, count := range bySeverity {
		githubOpenSecurityAlerts.WithLabelValues(severity).Set(float64(count))
	}
}

func (j *IssueSignalCollectorJob) collectSentryIssues(
	ctx context.Context,
	logger *slog.Logger,
) {
	issues, err := j.sentry.ListUnresolvedIssues(ctx)
	if errors.Is(err, sentryapi.ErrNotConfigured) {
		return
	}
	if err != nil {
		logAPIErr(ctx, logger,
			"issue-signal-collector: failed to list sentry issues",
			err, sentryapi.IsTransientAPIError(err))
		return
	}
	sentryUnresolvedIssues.Set(float64(len(issues)))
}

func (j *IssueSignalCollectorJob) collectStorage(
	ctx context.Context,
	logger *slog.Logger,
) {
	snap, err := j.storageSnapshot.Latest(ctx)
	if errors.Is(err, database.ErrResourceNotFound) {
		return
	}
	if err != nil {
		logger.ErrorContext(ctx,
			"issue-signal-collector: failed to load storage snapshot",
			essentialogger.ErrAttr(err))
		return
	}
	r2OrphanedObjects.Set(float64(snap.OrphanCount))
	r2StorageBytes.Set(float64(snap.TotalSizeBytes))
}
