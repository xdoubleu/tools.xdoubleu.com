package jobs

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"tools.xdoubleu.com/internal/kit/database"
	essentialogger "tools.xdoubleu.com/internal/kit/logging"
	"tools.xdoubleu.com/internal/models"
	"tools.xdoubleu.com/internal/platform/github"
	"tools.xdoubleu.com/internal/platform/reposignals"
	"tools.xdoubleu.com/internal/platform/sentryapi"
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
	githubFailingPullRequestsByRepo = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "github_failing_pull_requests_by_repo",
		Help: "Open pull requests in a registered repo's red-PR scope with at " +
			"least one failing CI check.",
	}, []string{"repo"})
	githubWorkflowRunFailed = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "github_workflow_run_failed",
		Help: "1 when the most recent completed workflow run on the main " +
			"branch failed, else 0.",
	}, []string{"branch"})
	githubWorkflowRunDurationSeconds = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "github_workflow_run_duration_seconds",
		// Label is "workflow", not "job", which collides with the scrape job label.
		Help: "Duration of the most recent completed run of each GitHub Actions " +
			"workflow on the default branch, in seconds.",
	}, []string{"workflow"})
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
		Help: "Unresolved error- and fatal-level Sentry issues across every " +
			"project the connected token can see.",
	})
	sentryUnresolvedIssuesByRepo = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "sentry_unresolved_issues_by_repo",
		Help: "Unresolved Sentry issues by the registered repo that owns their " +
			"project; " + unassignedRepo + " when no repo claims it.",
	}, []string{"repo"})
	sentryUnresolvedFetchOK = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "sentry_unresolved_issues_fetch_ok",
		Help: "1 when the last Sentry unresolved-issue fetch succeeded, 0 when " +
			"Sentry isn't configured, the fetch failed, or none has run yet, so " +
			"sentry_unresolved_issues is stale and its 0 can't be trusted.",
	})
	transactionP95Seconds = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "transaction_p95_seconds",
		Help: "Average daily p95 of each regressing Sentry transaction, by " +
			"window: recent (last 7 days) or prior (the 7 before). Only " +
			"transactions get_slow_transactions lists as trending are set.",
	}, []string{"project", "transaction", "window"})
)

const mainBranch = "main"

// unassignedRepo is the repo label of Sentry issues no repo claims.
const unassignedRepo = "unassigned"

const millisPerSecond = 1000

type failingPRLister interface {
	ListFailingPullRequests(ctx context.Context) ([]github.PullRequest, error)
	ListFailingPullRequestsIn(
		ctx context.Context, repo string, scope github.PRScope,
	) ([]github.PullRequest, error)
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

type latencyTrendsGetter interface {
	Trends(ctx context.Context) ([]models.TransactionTrend, error)
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
	latencyTrends   latencyTrendsGetter
	repos           reposignals.Source
}

// WithRepos makes the PR and Sentry signals follow the registered repos' own
// scopes. Without it, or while none is registered, they cover the admin-picked
// GitHub repo and every picked Sentry project.
func (j *IssueSignalCollectorJob) WithRepos(
	repos reposignals.Source,
) *IssueSignalCollectorJob {
	j.repos = repos
	return j
}

// monitoredRepos lists the registered repos; none when unwired or unreadable.
func (j *IssueSignalCollectorJob) monitoredRepos(
	ctx context.Context, logger *slog.Logger,
) []reposignals.Repo {
	if j.repos == nil {
		return nil
	}
	repos, err := j.repos.MonitoredRepos(ctx)
	if err != nil {
		logger.WarnContext(ctx,
			"issue-signal-collector: failed to list monitored repos",
			essentialogger.ErrAttr(err))
		return nil
	}
	return repos
}

func NewIssueSignalCollectorJob(
	gh issueSignalGithubClient,
	sentry unresolvedIssueLister,
	storageSnapshot latestStorageSnapshotGetter,
	schemaSizes schemaSizer,
	latencyTrends latencyTrendsGetter,
) *IssueSignalCollectorJob {
	// Seed the fetch-ok gauge to 1: a fresh process has no fetch to have
	// failed yet, and the gauge's zero value would otherwise pend
	// SecurityAlertsUnreadable on every deploy until the first run.
	githubSecurityAlertsFetchOK.Set(1)
	return &IssueSignalCollectorJob{
		gh:              gh,
		sentry:          sentry,
		storageSnapshot: storageSnapshot,
		schemaSizes:     schemaSizes,
		latencyTrends:   latencyTrends,
		repos:           nil,
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
	j.collectLatencyTrends(ctx, logger)
	return nil
}

func (j *IssueSignalCollectorJob) collectFailingPullRequests(
	ctx context.Context,
	logger *slog.Logger,
) {
	repos := j.monitoredRepos(ctx, logger)
	if len(repos) == 0 {
		j.collectPickedRepoFailingPullRequests(ctx, logger)
		return
	}

	counts := make(map[string]int, len(repos))
	total := 0
	for _, r := range repos {
		prs, err := j.gh.ListFailingPullRequestsIn(ctx, r.Slug, github.PRScope{
			Label:        r.RedPRLabel,
			BranchPrefix: r.RedPRBranchPrefix,
		})
		if errors.Is(err, github.ErrNotConfigured) {
			return
		}
		if err != nil {
			// A partial total would read as the others having recovered, so
			// leave every gauge as it was.
			logAPIErr(ctx, logger,
				"issue-signal-collector: failed to list failing pull requests",
				err, github.IsTransientAPIError(err))
			return
		}
		counts[r.Slug] = len(prs)
		total += len(prs)
	}
	githubFailingPullRequests.Set(float64(total))
	githubFailingPullRequestsByRepo.Reset()
	for slug, n := range counts {
		githubFailingPullRequestsByRepo.WithLabelValues(slug).Set(float64(n))
	}
}

// collectPickedRepoFailingPullRequests is the fallback while no repo is
// registered: the admin-picked GitHub repo under the default scope.
func (j *IssueSignalCollectorJob) collectPickedRepoFailingPullRequests(
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

	// mainFailed is 1 iff the most recent completed run on main concluded
	// failure. Counting every failure in ListWorkflowRuns' last-20-push window
	// kept the gauge set for ~8h after a transient red push, so IssueMainCIRed
	// fired while main was green.
	mainFailed := 0.0
	haveMainRun := false
	var latestMain github.WorkflowRun
	latestByWorkflow := make(map[string]github.WorkflowRun)
	for _, run := range runs {
		if run.Branch == mainBranch && run.Status == "completed" {
			if !haveMainRun || run.StartedAt.After(latestMain.StartedAt) {
				haveMainRun = true
				latestMain = run
			}
		}
		if run.Branch != mainBranch || run.Status != "completed" {
			continue
		}
		if cur, ok := latestByWorkflow[run.Name]; !ok ||
			run.StartedAt.After(cur.StartedAt) {
			latestByWorkflow[run.Name] = run
		}
	}
	if haveMainRun && latestMain.Conclusion == "failure" {
		mainFailed = 1
	}
	githubWorkflowRunFailed.Reset()
	githubWorkflowRunFailed.WithLabelValues(mainBranch).Set(mainFailed)

	githubWorkflowRunDurationSeconds.Reset()
	for name, run := range latestByWorkflow {
		githubWorkflowRunDurationSeconds.WithLabelValues(name).
			Set(float64(run.DurationMs) / millisPerSecond)
	}
}

// collectSentryIssues sets sentry_unresolved_issues and records the fetch
// outcome in sentry_unresolved_issues_fetch_ok; the count is left untouched on
// failure, so consumers must read it only while fetch_ok is 1. It's a gauge
// because Grafana can't alert on the grafana-sentry-datasource Issues query
// directly.
func (j *IssueSignalCollectorJob) collectSentryIssues(
	ctx context.Context,
	logger *slog.Logger,
) {
	issues, err := j.sentry.ListUnresolvedIssues(ctx)
	if errors.Is(err, sentryapi.ErrNotConfigured) {
		sentryUnresolvedFetchOK.Set(0)
		return
	}
	if err != nil {
		sentryUnresolvedFetchOK.Set(0)
		logAPIErr(ctx, logger,
			"issue-signal-collector: failed to list unresolved Sentry issues",
			err, sentryapi.IsTransientAPIError(err))
		return
	}
	issues = errorLevelIssues(issues)
	sentryUnresolvedIssues.Set(float64(len(issues)))
	sentryUnresolvedFetchOK.Set(1)

	repos := j.monitoredRepos(ctx, logger)
	sentryUnresolvedIssuesByRepo.Reset()
	for _, is := range issues {
		repo := reposignals.RepoForSentryProject(repos, is.Project)
		if repo == "" {
			repo = unassignedRepo
		}
		sentryUnresolvedIssuesByRepo.WithLabelValues(repo).Inc()
	}
}

// errorLevelIssues drops info-, warning- and debug-level issues, which aren't
// outages; an unset level is kept.
func errorLevelIssues(issues []sentryapi.Issue) []sentryapi.Issue {
	kept := make([]sentryapi.Issue, 0, len(issues))
	for _, is := range issues {
		switch is.Level {
		case "info", "warning", "debug":
		default:
			kept = append(kept, is)
		}
	}
	return kept
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

// collectLatencyTrends sets transaction_p95_seconds, which feeds Grafana's
// TransactionLatencyRegression. A fetch error leaves the gauge untouched.
func (j *IssueSignalCollectorJob) collectLatencyTrends(
	ctx context.Context,
	logger *slog.Logger,
) {
	trends, err := j.latencyTrends.Trends(ctx)
	if err != nil {
		logger.ErrorContext(ctx,
			"issue-signal-collector: failed to load transaction latency trends",
			essentialogger.ErrAttr(err))
		return
	}
	transactionP95Seconds.Reset()
	for _, t := range trends {
		transactionP95Seconds.WithLabelValues(t.Project, t.Transaction, "recent").
			Set(t.RecentAvgP95Ms / millisPerSecond)
		transactionP95Seconds.WithLabelValues(t.Project, t.Transaction, "prior").
			Set(t.PriorAvgP95Ms / millisPerSecond)
	}
}
