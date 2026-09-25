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

// Issue-signal gauges, refreshed on a timer (never at scrape time) because
// the GitHub and Sentry APIs are rate-limited.
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
	githubWorkflowRunDurationSeconds = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "github_workflow_run_duration_seconds",
		// Label is "workflow", not "job", which collides with the scrape job label.
		Help: "Duration of the most recent completed run of each GitHub Actions " +
			"workflow on the default branch, in seconds.",
	}, []string{"workflow"})
	githubOpenSecurityAlerts = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "github_open_security_alerts",
		Help: "Open Dependabot, code-scanning and secret-scanning alerts, by severity.",
	}, []string{"severity"})
	r2OrphanedObjects = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "r2_orphaned_objects",
		Help: "Orphaned R2 storage objects in the latest storage snapshot.",
	})
	r2StorageBytes = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "r2_storage_bytes",
		Help: "Total R2 storage size in bytes from the latest storage snapshot.",
	})
	postgresSchemaSizeBytes = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "postgres_schema_size_bytes",
		Help: "On-disk size of each database schema in bytes. postgres_exporter " +
			"only exposes per-database size, so this is the per-schema breakdown.",
	}, []string{"schema"})
	sentryUnresolvedIssues = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "sentry_unresolved_issues",
		Help: "Unresolved Sentry issues across every project the connected " +
			"token can see.",
	})
	automatedActionOldestOpenAgeSeconds = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "automated_action_oldest_open_age_seconds",
		Help: "Age in seconds of the longest-open global.automated_actions row " +
			"(finished_at IS NULL) — a self-healing routine that fired but never " +
			"closed out. 0 when none are open.",
	})
	automatedActionSecondsSinceLastOpen = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "automated_action_seconds_since_last_open",
		Help: "Seconds since routine last opened a global.automated_actions row " +
			"via record_action(mode=open), by routine name — detects a scheduled " +
			"routine that never even started (e.g. its claude.ai trigger itself " +
			"failed to fire), which a stalled-but-opened row can't catch. A " +
			"routine that has never opened a row at all reports a very large " +
			"value rather than 0, so it reads as overdue rather than healthy.",
	}, []string{"routine"})
)

// knownRoutines are the claude.ai routines tracked by
// automated_action_seconds_since_last_open. Keep in sync by hand with
// docs/spec-routine-*.md and the AutomatedRoutineMissed thresholds.
//
//nolint:gochecknoglobals //small fixed list, read-only, mirrors the collectors above
var knownRoutines = []string{
	"nightly-maintenance-sweep",
	"ready-issues-executor",
	"red-pr-repair",
}

// neverOpenedSentinelSeconds is reported for a routine that never opened a
// row; it's far past every threshold because "never started" is unhealthy.
const neverOpenedSentinelSeconds = 1 << 30

const mainBranch = "main"

const millisPerSecond = 1000

type failingPRLister interface {
	ListFailingPullRequests(ctx context.Context) ([]github.PullRequest, error)
}

type securityAlertLister interface {
	ListSecurityAlerts(ctx context.Context) ([]github.SecurityAlert, error)
}

type latestStorageSnapshotGetter interface {
	Latest(ctx context.Context) (*models.StorageSnapshot, error)
}

type schemaSizer interface {
	SchemaSizes(ctx context.Context) ([]models.SchemaStats, error)
}

type oldestOpenAutomatedActionGetter interface {
	OldestOpenFiredAt(ctx context.Context) (time.Time, error)
}

type mostRecentOpenedAtGetter interface {
	MostRecentOpenedAt(ctx context.Context, routineName string) (time.Time, error)
}

type automatedActionGetter interface {
	oldestOpenAutomatedActionGetter
	mostRecentOpenedAtGetter
}

type workflowRunsLister interface {
	ListWorkflowRuns(ctx context.Context) ([]github.WorkflowRun, error)
}

type issueSignalGithubClient interface {
	failingPRLister
	securityAlertLister
	workflowRunsLister
}

type unresolvedIssueLister interface {
	ListUnresolvedIssues(ctx context.Context) ([]sentryapi.Issue, error)
}

// runEvery is the shared poll interval of this package's timer jobs.
const runEvery = 5 * time.Minute

// IssueSignalCollectorJob refreshes the issue-signal gauges. A disconnected
// provider leaves its gauge untouched; other errors are logged. Run always
// returns nil.
type IssueSignalCollectorJob struct {
	gh              issueSignalGithubClient
	sentry          unresolvedIssueLister
	storageSnapshot latestStorageSnapshotGetter
	schemaSizes     schemaSizer
	automatedAction automatedActionGetter
}

func NewIssueSignalCollectorJob(
	gh issueSignalGithubClient,
	sentry unresolvedIssueLister,
	storageSnapshot latestStorageSnapshotGetter,
	schemaSizes schemaSizer,
	automatedAction automatedActionGetter,
) *IssueSignalCollectorJob {
	return &IssueSignalCollectorJob{
		gh:              gh,
		sentry:          sentry,
		storageSnapshot: storageSnapshot,
		schemaSizes:     schemaSizes,
		automatedAction: automatedAction,
	}
}

func (j *IssueSignalCollectorJob) ID() string {
	return "collect-issue-signals"
}

// RunEvery returns the shared runEvery.
func (j *IssueSignalCollectorJob) RunEvery() time.Duration {
	return runEvery
}

// logAPIErr logs transient errors at Warn and others at Error (Sentry).
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
	j.collectSchemaSizes(ctx, logger)
	j.collectAutomatedActionAge(ctx, logger)
	j.collectRoutineLiveness(ctx, logger)
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
	latestByWorkflow := make(map[string]github.WorkflowRun)
	for _, run := range runs {
		if run.Branch == mainBranch && run.Conclusion == "failure" {
			failed++
		}
		if run.Branch != mainBranch || run.Status != "completed" {
			continue
		}
		if cur, ok := latestByWorkflow[run.Name]; !ok ||
			run.StartedAt.After(cur.StartedAt) {
			latestByWorkflow[run.Name] = run
		}
	}
	githubWorkflowRunFailed.Reset()
	githubWorkflowRunFailed.WithLabelValues(mainBranch).Set(float64(failed))

	githubWorkflowRunDurationSeconds.Reset()
	for name, run := range latestByWorkflow {
		githubWorkflowRunDurationSeconds.WithLabelValues(name).
			Set(float64(run.DurationMs) / millisPerSecond)
	}
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

// collectSentryIssues sets sentry_unresolved_issues. It's a gauge because
// Grafana can't alert on the grafana-sentry-datasource Issues query directly.
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
			"issue-signal-collector: failed to list unresolved Sentry issues",
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

func (j *IssueSignalCollectorJob) collectSchemaSizes(
	ctx context.Context,
	logger *slog.Logger,
) {
	sizes, err := j.schemaSizes.SchemaSizes(ctx)
	if err != nil {
		logger.ErrorContext(ctx,
			"issue-signal-collector: failed to load schema sizes",
			essentialogger.ErrAttr(err))
		return
	}
	postgresSchemaSizeBytes.Reset()
	for _, s := range sizes {
		postgresSchemaSizeBytes.WithLabelValues(s.Name).Set(float64(s.SizeBytes))
	}
}

// collectAutomatedActionAge sets the gauge to 0 when no row is open.
func (j *IssueSignalCollectorJob) collectAutomatedActionAge(
	ctx context.Context,
	logger *slog.Logger,
) {
	firedAt, err := j.automatedAction.OldestOpenFiredAt(ctx)
	if errors.Is(err, database.ErrResourceNotFound) {
		automatedActionOldestOpenAgeSeconds.Set(0)
		return
	}
	if err != nil {
		logger.ErrorContext(ctx,
			"issue-signal-collector: failed to load oldest open automated action",
			essentialogger.ErrAttr(err))
		return
	}
	automatedActionOldestOpenAgeSeconds.Set(time.Since(firedAt).Seconds())
}

// collectRoutineLiveness sets automated_action_seconds_since_last_open per
// routine; neverOpenedSentinelSeconds when it never opened a row.
func (j *IssueSignalCollectorJob) collectRoutineLiveness(
	ctx context.Context,
	logger *slog.Logger,
) {
	for _, routine := range knownRoutines {
		firedAt, err := j.automatedAction.MostRecentOpenedAt(ctx, routine)
		if errors.Is(err, database.ErrResourceNotFound) {
			automatedActionSecondsSinceLastOpen.
				WithLabelValues(routine).Set(neverOpenedSentinelSeconds)
			continue
		}
		if err != nil {
			logger.ErrorContext(ctx,
				"issue-signal-collector: failed to load most recent open "+
					"automated action",
				slog.String("routine", routine), essentialogger.ErrAttr(err))
			continue
		}
		automatedActionSecondsSinceLastOpen.
			WithLabelValues(routine).Set(time.Since(firedAt).Seconds())
	}
}
