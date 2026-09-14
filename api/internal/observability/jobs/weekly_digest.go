package jobs

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"tools.xdoubleu.com/internal/mailer"
	"tools.xdoubleu.com/internal/notifications"
	"tools.xdoubleu.com/internal/repositories"
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

// WeeklyDigestJob emails an admin, once a week, a personal reading/feed
// backlog reminder: feeds currently failing to poll, plus feeds with
// unread items. Everything else this job used to cover (Sentry, failing
// dependency PRs, security alerts, slow transactions) is now alerted on in
// real time by Grafana and delivered to Slack instead — see adr-0010.
//
// There is no per-item dedup: every run sends,
// including a short "all clear" email when a source is enabled but has
// nothing to report, so a missing weekly email is itself a signal the job
// stopped running rather than being indistinguishable from "nothing to
// report". The one case that suppresses a send entirely is every source
// feeding the email being disabled in global.notification_settings — an
// admin who turned both feed sources off shouldn't still get an empty
// digest every week.

// notificationSettingsRepo is the subset of
// *repositories.NotificationSettingsRepository the digest checks before
// sending each section.
type notificationSettingsRepo interface {
	IsEnabled(
		ctx context.Context,
		source repositories.NotificationSource,
	) (bool, error)
}

type WeeklyDigestJob struct {
	feeds         unhealthyFeedLister
	openFeedItems openFeedItemsLister
	notifications *notifications.Service
	settings      notificationSettingsRepo
}

func NewWeeklyDigestJob(
	feeds unhealthyFeedLister,
	openFeedItems openFeedItemsLister,
	notifications *notifications.Service,
	settings notificationSettingsRepo,
) *WeeklyDigestJob {
	return &WeeklyDigestJob{
		feeds:         feeds,
		openFeedItems: openFeedItems,
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
	feeds := &digest{
		subject:    "[Weekly Digest] Feeds — open items and failing feeds",
		emptyBody:  "No feeds need attention this week.",
		sections:   nil,
		anyEnabled: false,
	}
	feeds.add(j.feedsSection(ctx, logger))
	feeds.add(j.openFeedItemsSection(ctx, logger))

	j.send(feeds)

	return nil
}

// send enqueues the digest email, skipping it entirely when every source
// feeding it is disabled. It uses EnqueueEmail, not Enqueue: the digest is
// email-only regardless of the global email/Slack switch (docs/adr-0019).
func (j *WeeklyDigestJob) send(d *digest) {
	if !d.anyEnabled {
		return
	}

	body := d.emptyBody
	if len(d.sections) > 0 {
		body = strings.Join(d.sections, "\n\n")
	}

	j.notifications.EnqueueEmail(
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

// feedsSection returns the section's rendered text (empty when there's
// nothing to report) and whether the source is enabled — enabled is what
// digest.add uses to decide whether the email should send at all, since an
// empty section is ambiguous between "healthy" and "disabled".
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

// openFeedItemsSection follows feedsSection's (text, enabled) contract.
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
