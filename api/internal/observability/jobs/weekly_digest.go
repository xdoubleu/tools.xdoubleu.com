package jobs

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"tools.xdoubleu.com/internal/github"
	"tools.xdoubleu.com/internal/mailer"
	"tools.xdoubleu.com/internal/notifications"
	"tools.xdoubleu.com/internal/repositories"
	"tools.xdoubleu.com/internal/sentryapi"
)

// weeklyDigestRunEvery is how often WeeklyDigestJob sends its summary.
const weeklyDigestRunEvery = 7 * 24 * time.Hour

// UnhealthyFeed is a feed currently failing to poll, as reported by the
// feeds app for the weekly digest (issue #1014).
type UnhealthyFeed struct {
	Title               string
	URL                 string
	LastError           string
	ConsecutiveFailures int
}

// unhealthyFeedLister is the subset of *feeds.Feeds this job needs. Kept as
// a narrow interface here (rather than importing apps/feeds directly) so
// this cross-app internal/ package never depends on an apps/* package —
// main.go wires the concrete *feeds.Feeds in via an adapter closure.
type unhealthyFeedLister interface {
	ListUnhealthy(ctx context.Context) ([]UnhealthyFeed, error)
}

// OpenFeedItem is one feed with unread items, as reported by the feeds app
// for the weekly digest's open-feed-items reminder (issue #1355).
type OpenFeedItem struct {
	Title string
	URL   string
	Count int
}

// openFeedItemsLister is the subset of *feeds.Feeds this job needs for the
// open-feed-items reminder — kept narrow for the same reason as
// unhealthyFeedLister above.
type openFeedItemsLister interface {
	ListOpenItems(ctx context.Context) ([]OpenFeedItem, error)
}

// WeeklyDigestJob emails an admin, once a week, a summary of every issue
// IssueNotifierJob already alerted on at least once but that may still be
// open — a real-time alert fires only the first time an issue is seen, so
// this restates anything still unresolved a week later.
//
// It sends two separate emails, not one combined digest: monitoring
// (Sentry, failing dependency PRs, security alerts, slow transactions) and
// feeds (feeds failing to poll, unread items). Monitoring and feeds are
// separate apps with separate audiences — an admin chasing a red CI check
// and a reader clearing a backlog of unread items are doing unrelated
// things, and folding both into one mail means neither can be skimmed,
// filtered or muted on its own.
//
// Unlike IssueNotifierJob there is no per-item dedup: every run sends,
// including a short "all clear" email when a source is enabled but has
// nothing to report, so a missing weekly email is itself a signal the job
// stopped running rather than being indistinguishable from "nothing to
// report". The one case that suppresses a send entirely is every source
// feeding that particular email being disabled in
// global.notification_settings — an admin who turned all of monitoring off
// shouldn't still get an empty monitoring digest every week, and the two
// emails make that call independently of each other.
type WeeklyDigestJob struct {
	sentry             sentryapi.Client
	gh                 issueNotifierGithubClient
	feeds              unhealthyFeedLister
	openFeedItems      openFeedItemsLister
	notifications      *notifications.Service
	settings           notificationSettingsRepo
	transactionLatency slowTransactionsRepo
}

func NewWeeklyDigestJob(
	sentry sentryapi.Client,
	gh issueNotifierGithubClient,
	feeds unhealthyFeedLister,
	openFeedItems openFeedItemsLister,
	notifications *notifications.Service,
	settings notificationSettingsRepo,
	transactionLatency slowTransactionsRepo,
) *WeeklyDigestJob {
	return &WeeklyDigestJob{
		sentry:             sentry,
		gh:                 gh,
		feeds:              feeds,
		openFeedItems:      openFeedItems,
		notifications:      notifications,
		settings:           settings,
		transactionLatency: transactionLatency,
	}
}

func (j *WeeklyDigestJob) ID() string {
	return "weekly-digest"
}

func (j *WeeklyDigestJob) RunEvery() time.Duration {
	return weeklyDigestRunEvery
}

// digest accumulates the sections of one outgoing email, along with whether
// any of the sources feeding it is enabled at all — an empty section list is
// ambiguous between "everything healthy" and "everything muted", and only
// the former should still send an all-clear.
type digest struct {
	subject    string
	emptyBody  string
	sections   []string
	anyEnabled bool
}

// add takes a section method's (text, enabled) pair directly, so registering
// a source reads as one line per source.
func (d *digest) add(section string, enabled bool) {
	d.anyEnabled = d.anyEnabled || enabled
	if section != "" {
		d.sections = append(d.sections, section)
	}
}

func (j *WeeklyDigestJob) Run(ctx context.Context, logger *slog.Logger) error {
	monitoring := &digest{
		subject:    "[Weekly Digest] Monitoring — open issues summary",
		emptyBody:  "No open monitoring issues this week.",
		sections:   nil,
		anyEnabled: false,
	}
	monitoring.add(j.sentrySection(ctx, logger))
	monitoring.add(j.githubSection(ctx, logger))
	monitoring.add(j.securityAlertsSection(ctx, logger))
	monitoring.add(j.slowTransactionsSection(ctx, logger))

	feeds := &digest{
		subject:    "[Weekly Digest] Feeds — open items and failing feeds",
		emptyBody:  "No feeds need attention this week.",
		sections:   nil,
		anyEnabled: false,
	}
	feeds.add(j.feedsSection(ctx, logger))
	feeds.add(j.openFeedItemsSection(ctx, logger))

	j.send(monitoring)
	j.send(feeds)

	return nil
}

// send enqueues one digest email, skipping it entirely when every source
// feeding it is disabled.
func (j *WeeklyDigestJob) send(d *digest) {
	if !d.anyEnabled {
		return
	}

	body := d.emptyBody
	if len(d.sections) > 0 {
		body = strings.Join(d.sections, "\n\n")
	}

	j.notifications.Enqueue(
		d.subject,
		body,
		func(_ context.Context, err error) error {
			if errors.Is(err, mailer.ErrNotConfigured) {
				return nil
			}
			return err
		},
	)
}

// sentrySection returns the section's rendered text (empty when there's
// nothing to report) and whether the source is enabled — enabled is what
// digest.add uses to decide whether that email should send at all, since an
// empty section is ambiguous between "healthy" and "disabled".
func (j *WeeklyDigestJob) sentrySection(
	ctx context.Context, logger *slog.Logger,
) (string, bool) {
	enabled, err := j.settings.IsEnabled(
		ctx,
		repositories.NotificationSourceSentryIssues,
	)
	if err != nil {
		logger.ErrorContext(ctx, "weekly-digest: failed to read sentry_issues setting",
			"error", err)
		return "", true
	}
	if !enabled {
		return "", false
	}

	issues, err := j.sentry.ListUnresolvedIssues(ctx)
	if errors.Is(err, sentryapi.ErrNotConfigured) {
		return "", true
	}
	if err != nil {
		logAPIErr(ctx, logger, "weekly-digest: failed to list sentry issues",
			err, sentryapi.IsTransientAPIError(err))
		return "", true
	}
	if len(issues) == 0 {
		return "", true
	}

	lines := make([]string, len(issues))
	for i, issue := range issues {
		lines[i] = fmt.Sprintf(
			"- [%s] %s (%s) — %s",
			issue.Level, issue.Title, issue.Project, issue.Permalink,
		)
	}
	return fmt.Sprintf("Sentry — %d unresolved issue(s):\n%s",
		len(issues), strings.Join(lines, "\n")), true
}

// githubSection follows sentrySection's (text, enabled) contract.
func (j *WeeklyDigestJob) githubSection(
	ctx context.Context, logger *slog.Logger,
) (string, bool) {
	enabled, err := j.settings.IsEnabled(
		ctx,
		repositories.NotificationSourceFailingDependencyPRs,
	)
	if err != nil {
		logger.ErrorContext(ctx,
			"weekly-digest: failed to read failing_dependency_prs setting",
			"error", err)
		return "", true
	}
	if !enabled {
		return "", false
	}

	prs, err := j.gh.ListFailingPullRequests(ctx)
	if errors.Is(err, github.ErrNotConfigured) {
		return "", true
	}
	if err != nil {
		logAPIErr(ctx, logger, "weekly-digest: failed to list failing pull requests",
			err, github.IsTransientAPIError(err))
		return "", true
	}

	var lines []string
	for _, pr := range prs {
		lines = append(lines, fmt.Sprintf(
			"- #%d %s — %s — failing: %s",
			pr.Number, pr.Title, pr.URL, failingCheckNames(pr.FailingChecks),
		))
	}
	if len(lines) == 0 {
		return "", true
	}

	return fmt.Sprintf("GitHub — %d dependency PR(s) failing CI:\n%s",
		len(lines), strings.Join(lines, "\n")), true
}

// feedsSection follows sentrySection's (text, enabled) contract.
func (j *WeeklyDigestJob) feedsSection(
	ctx context.Context, logger *slog.Logger,
) (string, bool) {
	enabled, err := j.settings.IsEnabled(
		ctx,
		repositories.NotificationSourceUnhealthyFeeds,
	)
	if err != nil {
		logger.ErrorContext(
			ctx,
			"weekly-digest: failed to read unhealthy_feeds setting",
			"error",
			err,
		)
		return "", true
	}
	if !enabled {
		return "", false
	}

	unhealthy, err := j.feeds.ListUnhealthy(ctx)
	if err != nil {
		logger.ErrorContext(ctx, "weekly-digest: failed to list unhealthy feeds",
			"error", err)
		return "", true
	}
	if len(unhealthy) == 0 {
		return "", true
	}

	lines := make([]string, len(unhealthy))
	for i, feed := range unhealthy {
		lines[i] = fmt.Sprintf(
			"- %s (%s) — %d consecutive failure(s): %s",
			feed.Title, feed.URL, feed.ConsecutiveFailures, feed.LastError,
		)
	}
	return fmt.Sprintf("Feeds — %d feed(s) failing to poll:\n%s",
		len(unhealthy), strings.Join(lines, "\n")), true
}

// securityAlertsSection follows sentrySection's (text, enabled) contract.
func (j *WeeklyDigestJob) securityAlertsSection(
	ctx context.Context, logger *slog.Logger,
) (string, bool) {
	enabled, err := j.settings.IsEnabled(
		ctx,
		repositories.NotificationSourceSecurityAlerts,
	)
	if err != nil {
		logger.ErrorContext(ctx,
			"weekly-digest: failed to read security_alerts setting",
			"error", err)
		return "", true
	}
	if !enabled {
		return "", false
	}

	alerts, err := j.gh.ListSecurityAlerts(ctx)
	if errors.Is(err, github.ErrNotConfigured) {
		return "", true
	}
	if err != nil {
		logAPIErr(ctx, logger, "weekly-digest: failed to list security alerts",
			err, github.IsTransientAPIError(err))
		return "", true
	}
	if len(alerts) == 0 {
		return "", true
	}

	lines := make([]string, len(alerts))
	for i, alert := range alerts {
		lines[i] = fmt.Sprintf(
			"- [%s] %s alert #%d — %s — %s",
			alert.Severity, alert.Type, alert.Number, alert.Summary, alert.URL,
		)
	}
	return fmt.Sprintf("Security — %d open alert(s):\n%s",
		len(alerts), strings.Join(lines, "\n")), true
}

// slowTransactionsSection follows sentrySection's (text, enabled) contract.
// Unlike IssueNotifierJob's per-item dedup, this reports every currently
// slow transaction on every run — the digest restates current state, it
// doesn't track "seen before".
func (j *WeeklyDigestJob) slowTransactionsSection(
	ctx context.Context, logger *slog.Logger,
) (string, bool) {
	enabled, err := j.settings.IsEnabled(
		ctx,
		repositories.NotificationSourceSlowTransactions,
	)
	if err != nil {
		logger.ErrorContext(ctx,
			"weekly-digest: failed to read slow_transactions setting",
			"error", err)
		return "", true
	}
	if !enabled {
		return "", false
	}

	slow, err := currentlySlowTransactions(ctx, j.transactionLatency)
	if err != nil {
		logger.ErrorContext(ctx,
			"weekly-digest: failed to load transaction trends",
			"error", err)
		return "", true
	}
	if len(slow) == 0 {
		return "", true
	}

	lines := make([]string, len(slow))
	for i, t := range slow {
		lines[i] = fmt.Sprintf(
			"- %s (%s) — %.0fms p95 (was %.0fms, +%.0f%%)",
			t.Transaction,
			t.Project,
			t.RecentAvgP95Ms,
			t.PriorAvgP95Ms,
			t.PctChange*pctChangeToPercent,
		)
	}
	return fmt.Sprintf("Slow transactions — %d over threshold:\n%s",
		len(slow), strings.Join(lines, "\n")), true
}

// openFeedItemsSection follows sentrySection's (text, enabled) contract.
// Unlike feedsSection (feeds failing to poll), this restates every feed
// with at least one open (unread, non-dismissed) item, across all users —
// a nudge that items are piling up unread rather than a health problem
// (issue #1355).
func (j *WeeklyDigestJob) openFeedItemsSection(
	ctx context.Context, logger *slog.Logger,
) (string, bool) {
	enabled, err := j.settings.IsEnabled(
		ctx,
		repositories.NotificationSourceOpenFeedItems,
	)
	if err != nil {
		logger.ErrorContext(ctx,
			"weekly-digest: failed to read open_feed_items setting",
			"error", err)
		return "", true
	}
	if !enabled {
		return "", false
	}

	open, err := j.openFeedItems.ListOpenItems(ctx)
	if err != nil {
		logger.ErrorContext(ctx, "weekly-digest: failed to list open feed items",
			"error", err)
		return "", true
	}
	if len(open) == 0 {
		return "", true
	}

	total := 0
	lines := make([]string, len(open))
	for i, feed := range open {
		total += feed.Count
		lines[i] = fmt.Sprintf(
			"- %s (%s) — %d unread",
			feed.Title,
			feed.URL,
			feed.Count,
		)
	}
	return fmt.Sprintf("Feeds — %d unread item(s) across %d feed(s):\n%s",
		total, len(open), strings.Join(lines, "\n")), true
}
