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
	githubWorkflowRunDurationSeconds = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "github_workflow_run_duration_seconds",
		// Label is "workflow", not "job": the api scrape job already carries a
		// "job" label and infra/prometheus.yml only relabels job_duration_seconds'
		// collision, not this one.
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
	sentryUnresolvedIssues = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "sentry_unresolved_issues",
		Help: "Unresolved Sentry issues across the configured projects. Backs " +
			"the IssueSentryUnresolved Grafana alert directly (rather than that " +
			"rule querying the grafana-sentry-datasource plugin itself), since " +
			"no Grafana SSE expression can evaluate the Sentry Issues " +
			"endpoint's wide-series response (grafana/sentry-datasource#266).",
	})
	postgresSchemaSizeBytes = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "postgres_schema_size_bytes",
		Help: "On-disk size of each database schema in bytes. postgres_exporter " +
			"only exposes per-database size, so this is the per-schema breakdown.",
	}, []string{"schema"})
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

// knownRoutines is the fixed set of scheduled claude.ai routines whose
// liveness automated_action_seconds_since_last_open tracks, one series per
// name. api has no visibility into claude.ai's own routines UI/schedule, so
// this list — and each one's cadence, encoded as a threshold in
// infra/grafana/provisioning/alerting/rules.yml's AutomatedRoutineMissed
// rule — has to be hardcoded and kept in sync by hand against
// docs/spec-routine-*.md.
//
//nolint:gochecknoglobals //small fixed list, read-only, mirrors the collectors above
var knownRoutines = []string{
	"nightly-maintenance-sweep",
	"ready-issues-executor",
	"red-pr-repair",
}

// neverOpenedSentinelSeconds is the value automated_action_seconds_since_last_open
// reports for a routine that has never once opened an automated_actions row
// (repositories.AutomatedActionsRepository.MostRecentOpenedAt returns
// database.ErrResourceNotFound). It is deliberately far past any routine's
// alert threshold rather than 0, since "never started" is the unhealthy case
// this gauge exists to catch, not the healthy one.
const neverOpenedSentinelSeconds = 1 << 30

// mainBranch is the branch label the workflow-run gauge reports on — only
// failures on the default branch are tracked.
const mainBranch = "main"

// millisPerSecond converts github.WorkflowRun.DurationMs to the seconds unit
// the github_workflow_run_duration_seconds gauge reports in.
const millisPerSecond = 1000

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

// schemaSizer is the subset of *repositories.DBStatsRepository the
// per-schema size gauge needs.
type schemaSizer interface {
	SchemaSizes(ctx context.Context) ([]models.SchemaStats, error)
}

// sentryIssuesLister is the subset of sentryapi.Client the unresolved-issues
// gauge needs.
type sentryIssuesLister interface {
	ListUnresolvedIssues(ctx context.Context) ([]sentryapi.Issue, error)
}

// oldestOpenAutomatedActionGetter is the subset of
// *repositories.AutomatedActionsRepository the stalled-routine gauge needs.
type oldestOpenAutomatedActionGetter interface {
	OldestOpenFiredAt(ctx context.Context) (time.Time, error)
}

// mostRecentOpenedAtGetter is the subset of
// *repositories.AutomatedActionsRepository the never-started-routine gauge
// needs.
type mostRecentOpenedAtGetter interface {
	MostRecentOpenedAt(ctx context.Context, routineName string) (time.Time, error)
}

// automatedActionGetter is the subset of
// *repositories.AutomatedActionsRepository the collector needs across both
// automated-action gauges.
type automatedActionGetter interface {
	oldestOpenAutomatedActionGetter
	mostRecentOpenedAtGetter
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
// GitHub, Sentry, the latest storage snapshot, per-schema database sizes,
// the oldest still-open global.automated_actions row, and each known
// routine's most recent open row. A provider that isn't connected leaves its
// gauge untouched rather than resetting it to zero or failing the run; other
// errors are logged and skipped. Run always returns nil.
type IssueSignalCollectorJob struct {
	gh              issueSignalGithubClient
	sentry          sentryIssuesLister
	storageSnapshot latestStorageSnapshotGetter
	schemaSizes     schemaSizer
	automatedAction automatedActionGetter
}

func NewIssueSignalCollectorJob(
	gh issueSignalGithubClient,
	sentry sentryIssuesLister,
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
	j.collectSentryUnresolvedIssues(ctx, logger)
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

// collectSentryUnresolvedIssues sets sentryUnresolvedIssues from
// sentryapi.Client.ListUnresolvedIssues — the same client and method
// WeeklyDigestJob and the get_sentry_issues/resolve_sentry_issue MCP tools
// already use. The gauge backs the IssueSentryUnresolved Grafana rule
// directly rather than that rule querying the grafana-sentry-datasource
// plugin's Issues endpoint itself, since no Grafana SSE expression can
// consume that endpoint's wide-series response (confirmed live and recorded
// in adr-0022 Phase 18).
func (j *IssueSignalCollectorJob) collectSentryUnresolvedIssues(
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

// collectAutomatedActionAge sets the stalled-routine gauge to 0 when no
// automated_actions row is currently open (the healthy state) rather than
// leaving it at whatever a prior run last observed.
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

// collectRoutineLiveness sets automated_action_seconds_since_last_open for
// every known routine — the "did it even start" complement to
// collectAutomatedActionAge above (which only ever sees a routine that has
// already opened at least one row). A routine that has never opened a row
// reports neverOpenedSentinelSeconds rather than 0, since that absence is
// itself the failure mode this gauge exists to surface.
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
