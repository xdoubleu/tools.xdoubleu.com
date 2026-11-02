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

// FeedDigestItem is an unread feed item listed in the digest; ExpiresAt is
// set for items about to expire.
type FeedDigestItem struct {
	Title     string
	FeedTitle string
	URL       string
	ExpiresAt *time.Time
}

// FeedDigestItems are the digest's newest and last-chance item lists.
type FeedDigestItems struct {
	Newest     []FeedDigestItem
	LastChance []FeedDigestItem
}

type feedDigestItemsLister interface {
	ListDigestItems(ctx context.Context) (FeedDigestItems, error)
}

// WeeklyDigestJob emails an admin a weekly feed digest (failing feeds, the
// newest unread items, items about to expire). Every run sends, including an
// "all clear", so a missing email signals a stopped job; it's skipped only when every
// source is disabled in global.notification_settings.

type notificationSettingsRepo interface {
	IsEnabled(
		ctx context.Context,
		source repositories.NotificationSource,
	) (bool, error)
}

type WeeklyDigestJob struct {
	feeds         unhealthyFeedLister
	digestItems   feedDigestItemsLister
	notifications *notifications.Service
	settings      notificationSettingsRepo
}

func NewWeeklyDigestJob(
	feeds unhealthyFeedLister,
	digestItems feedDigestItemsLister,
	notifications *notifications.Service,
	settings notificationSettingsRepo,
) *WeeklyDigestJob {
	return &WeeklyDigestJob{
		feeds:         feeds,
		digestItems:   digestItems,
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
		subject:    "[Weekly Digest] Feeds — newest, last chance and failing feeds",
		emptyBody:  "No feeds need attention this week.",
		sections:   nil,
		anyEnabled: false,
	}
	feeds.add(j.feedsSection(ctx, logger))
	feeds.add(j.feedItemsSection(ctx, logger))

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

// feedItemsSection lists the newest unread items and those about to expire,
// across all users; same (text, enabled) contract as feedsSection.
func (j *WeeklyDigestJob) feedItemsSection(
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

	items, err := j.digestItems.ListDigestItems(ctx)
	if err != nil {
		logger.ErrorContext(ctx, "weekly-digest: failed to list feed items",
			"error", err)
		return "", true
	}

	var sections []string
	if len(items.Newest) > 0 {
		sections = append(sections, "Feeds — newest unread:\n"+
			formatDigestItems(items.Newest))
	}
	if len(items.LastChance) > 0 {
		sections = append(sections, "Feeds — last chance, expiring soon:\n"+
			formatDigestItems(items.LastChance))
	}
	return strings.Join(sections, "\n\n"), true
}

func formatDigestItems(items []FeedDigestItem) string {
	lines := make([]string, len(items))
	for i, item := range items {
		lines[i] = fmt.Sprintf("- %s (%s) — %s", item.Title, item.FeedTitle, item.URL)
		if item.ExpiresAt != nil {
			lines[i] += " — expires " + item.ExpiresAt.UTC().Format("Mon 2 Jan 15:04 UTC")
		}
	}
	return strings.Join(lines, "\n")
}
