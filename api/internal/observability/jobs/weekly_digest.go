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

const weeklyDigestRunEvery = 7 * 24 * time.Hour

// UnhealthyFeed is a feed currently failing to poll.
type UnhealthyFeed struct {
	Title               string
	URL                 string
	LastError           string
	ConsecutiveFailures int
}

// unhealthyFeedLister keeps this internal/ package from importing apps/feeds;
// main.go adapts *feeds.Feeds.
type unhealthyFeedLister interface {
	ListUnhealthy(ctx context.Context) ([]UnhealthyFeed, error)
}

// OpenFeedItem is a feed with unread items.
type OpenFeedItem struct {
	Title string
	URL   string
	Count int
}

type openFeedItemsLister interface {
	ListOpenItems(ctx context.Context) ([]OpenFeedItem, error)
}

// WeeklyDigestJob emails an admin a weekly feed-backlog reminder (failing
// feeds, feeds with unread items). Every run sends, including an "all clear",
// so a missing email signals a stopped job; it's skipped only when every
// source is disabled in global.notification_settings.

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

// digest collects sections plus whether any source is enabled: an empty
// digest is ambiguous between healthy and muted.
type digest struct {
	subject    string
	emptyBody  string
	sections   []string
	anyEnabled bool
}

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

// send enqueues the digest unless every source is disabled.
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

// feedsSection returns (text, enabled); text is empty when nothing to report.
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

// openFeedItemsSection lists every feed with open items, across all users;
// same (text, enabled) contract as feedsSection.
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
