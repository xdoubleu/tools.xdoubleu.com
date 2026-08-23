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
// currently failing to poll. Unlike IssueNotifierJob there is no dedup:
// every run sends, including a short "all clear" email when nothing is
// wrong, so a missing weekly email is itself a signal the job stopped
// running rather than being indistinguishable from "nothing to report".
type WeeklyDigestJob struct {
	sentry        sentryapi.Client
	gh            failingPRLister
	feeds         unhealthyFeedLister
	notifications *notifications.Service
}

func NewWeeklyDigestJob(
	sentry sentryapi.Client,
	gh failingPRLister,
	feeds unhealthyFeedLister,
	notifications *notifications.Service,
) *WeeklyDigestJob {
	return &WeeklyDigestJob{
		sentry:        sentry,
		gh:            gh,
		feeds:         feeds,
		notifications: notifications,
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

	if s := j.sentrySection(ctx, logger); s != "" {
		sections = append(sections, s)
	}
	if s := j.githubSection(ctx, logger); s != "" {
		sections = append(sections, s)
	}
	if s := j.feedsSection(ctx, logger); s != "" {
		sections = append(sections, s)
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

func (j *WeeklyDigestJob) sentrySection(
	ctx context.Context, logger *slog.Logger,
) string {
	issues, err := j.sentry.ListUnresolvedIssues(ctx)
	if errors.Is(err, sentryapi.ErrNotConfigured) {
		return ""
	}
	if err != nil {
		logAPIErr(ctx, logger, "weekly-digest: failed to list sentry issues",
			err, sentryapi.IsTransientAPIError(err))
		return ""
	}
	if len(issues) == 0 {
		return ""
	}

	lines := make([]string, len(issues))
	for i, issue := range issues {
		lines[i] = fmt.Sprintf(
			"- [%s] %s (%s) — %s",
			issue.Level, issue.Title, issue.Project, issue.Permalink,
		)
	}
	return fmt.Sprintf("Sentry — %d unresolved issue(s):\n%s",
		len(issues), strings.Join(lines, "\n"))
}

func (j *WeeklyDigestJob) githubSection(
	ctx context.Context, logger *slog.Logger,
) string {
	prs, err := j.gh.ListFailingPullRequests(ctx)
	if errors.Is(err, github.ErrNotConfigured) {
		return ""
	}
	if err != nil {
		logAPIErr(ctx, logger, "weekly-digest: failed to list failing pull requests",
			err, github.IsTransientAPIError(err))
		return ""
	}

	var lines []string
	for _, pr := range prs {
		if !pr.HasLabel(dependenciesLabel) {
			continue
		}
		lines = append(lines, fmt.Sprintf(
			"- #%d %s — %s — failing: %s",
			pr.Number, pr.Title, pr.URL, failingCheckNames(pr.FailingChecks),
		))
	}
	if len(lines) == 0 {
		return ""
	}

	return fmt.Sprintf("GitHub — %d dependency PR(s) failing CI:\n%s",
		len(lines), strings.Join(lines, "\n"))
}

func (j *WeeklyDigestJob) feedsSection(
	ctx context.Context, logger *slog.Logger,
) string {
	unhealthy, err := j.feeds.ListUnhealthy(ctx)
	if err != nil {
		logger.ErrorContext(ctx, "weekly-digest: failed to list unhealthy feeds",
			"error", err)
		return ""
	}
	if len(unhealthy) == 0 {
		return ""
	}

	lines := make([]string, len(unhealthy))
	for i, feed := range unhealthy {
		lines[i] = fmt.Sprintf(
			"- %s (%s) — %d consecutive failure(s): %s",
			feed.Title, feed.URL, feed.ConsecutiveFailures, feed.LastError,
		)
	}
	return fmt.Sprintf("Feeds — %d feed(s) failing to poll:\n%s",
		len(unhealthy), strings.Join(lines, "\n"))
}
