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

// WeeklyDigestJob emails an admin, once a week, a summary of every issue
// IssueNotifierJob already alerted on at least once but that may still be
// open — a real-time alert fires only the first time an issue is seen, so
// this restates anything still unresolved a week later, plus feeds
// currently failing to poll. Unlike IssueNotifierJob there is no per-item
// dedup: every run sends, including a short "all clear" email when a source
// is enabled but has nothing to report, so a missing weekly email is itself
// a signal the job stopped running rather than being indistinguishable from
// "nothing to report". The one case that does suppress the send entirely is
// every source being disabled in global.notification_settings — an admin
// who turned everything off shouldn't still get an empty digest every week.
type WeeklyDigestJob struct {
	sentry        sentryapi.Client
	gh            failingPRLister
	feeds         unhealthyFeedLister
	notifications *notifications.Service
	settings      notificationSettingsRepo
}

func NewWeeklyDigestJob(
	sentry sentryapi.Client,
	gh failingPRLister,
	feeds unhealthyFeedLister,
	notifications *notifications.Service,
	settings notificationSettingsRepo,
) *WeeklyDigestJob {
	return &WeeklyDigestJob{
		sentry:        sentry,
		gh:            gh,
		feeds:         feeds,
		notifications: notifications,
		settings:      settings,
	}
}

func (j *WeeklyDigestJob) ID() string {
	return "weekly-digest"
}

func (j *WeeklyDigestJob) RunEvery() time.Duration {
	return weeklyDigestRunEvery
}

func (j *WeeklyDigestJob) Run(ctx context.Context, logger *slog.Logger) error {
	var sections []string
	var anyEnabled bool

	s, enabled := j.sentrySection(ctx, logger)
	anyEnabled = anyEnabled || enabled
	if s != "" {
		sections = append(sections, s)
	}

	s, enabled = j.githubSection(ctx, logger)
	anyEnabled = anyEnabled || enabled
	if s != "" {
		sections = append(sections, s)
	}

	s, enabled = j.feedsSection(ctx, logger)
	anyEnabled = anyEnabled || enabled
	if s != "" {
		sections = append(sections, s)
	}

	if !anyEnabled {
		return nil
	}

	body := "No open issues this week."
	if len(sections) > 0 {
		body = strings.Join(sections, "\n\n")
	}

	j.notifications.Enqueue(
		"[Weekly Digest] Open issues summary",
		body,
		func(_ context.Context, err error) error {
			if errors.Is(err, mailer.ErrNotConfigured) {
				return nil
			}
			return err
		},
	)
	return nil
}

// sentrySection returns the section's rendered text (empty when there's
// nothing to report) and whether the source is enabled — enabled is what
// Run uses to decide whether the weekly email should send at all, since an
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
