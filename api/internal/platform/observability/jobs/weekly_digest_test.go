package jobs_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/platform/observability/jobs"
	platformrepositories "tools.xdoubleu.com/internal/platform/repositories"
)

type fakeFeedsLister struct {
	unhealthy []jobs.UnhealthyFeed
	err       error
}

func (f fakeFeedsLister) ListUnhealthy(
	_ context.Context,
) ([]jobs.UnhealthyFeed, error) {
	return f.unhealthy, f.err
}

type fakeDigestItemsLister struct {
	items jobs.FeedDigestItems
	err   error
}

func (f fakeDigestItemsLister) ListDigestItems(
	_ context.Context,
) (jobs.FeedDigestItems, error) {
	return f.items, f.err
}

func noDigestItems() jobs.FeedDigestItems {
	return jobs.FeedDigestItems{Newest: nil, LastChance: nil}
}

func digestItem(title string, expires *time.Time) jobs.FeedDigestItem {
	return jobs.FeedDigestItem{
		Title:     title,
		FeedTitle: "My Feed",
		URL:       "https://example.com/" + title,
		ExpiresAt: expires,
	}
}

func newestOnly(title string) jobs.FeedDigestItems {
	return jobs.FeedDigestItems{
		Newest:     []jobs.FeedDigestItem{digestItem(title, nil)},
		LastChance: nil,
	}
}

func TestWeeklyDigestSendsAllClearWhenNothingWrong(t *testing.T) {
	mail := &fakeMailer{sent: nil, bodies: nil, err: nil}
	notifSvc := testNotifications(t, mail)

	job := jobs.NewWeeklyDigestJob(
		fakeFeedsLister{unhealthy: nil, err: nil},
		fakeDigestItemsLister{items: noDigestItems(), err: nil},
		notifSvc,
		alwaysEnabledSettings{},
	)
	require.NoError(t, job.Run(t.Context(), testLogger()))
	notifSvc.WaitUntilDone()

	require.Len(t, mail.sent, 1)
	assert.Contains(t, mail.sent[0], "Feeds")
}

func TestWeeklyDigestAlwaysSendsEvenWhenPreviouslySeen(t *testing.T) {
	mail := &fakeMailer{sent: nil, bodies: nil, err: nil}
	notifSvc := testNotifications(t, mail)

	job := jobs.NewWeeklyDigestJob(
		fakeFeedsLister{unhealthy: nil, err: nil},
		fakeDigestItemsLister{items: noDigestItems(), err: nil},
		notifSvc,
		alwaysEnabledSettings{},
	)
	require.NoError(t, job.Run(t.Context(), testLogger()))
	require.NoError(t, job.Run(t.Context(), testLogger()))
	notifSvc.WaitUntilDone()

	assert.Len(t, mail.sent, 2)
}

func TestWeeklyDigestIncludesUnhealthyFeeds(t *testing.T) {
	mail := &fakeMailer{sent: nil, bodies: nil, err: nil}
	notifSvc := testNotifications(t, mail)

	job := jobs.NewWeeklyDigestJob(
		fakeFeedsLister{unhealthy: []jobs.UnhealthyFeed{
			{
				Title: "My Feed", URL: "https://example.com/feed",
				LastError: "timeout", ConsecutiveFailures: 4,
			},
		}, err: nil},
		fakeDigestItemsLister{items: noDigestItems(), err: nil},
		notifSvc,
		alwaysEnabledSettings{},
	)
	require.NoError(t, job.Run(t.Context(), testLogger()))
	notifSvc.WaitUntilDone()

	require.Len(t, mail.sent, 1)
	assert.Contains(t, mail.bodies[0], "My Feed")
}

func TestWeeklyDigestFeedsErrorDoesNotFailRun(t *testing.T) {
	mail := &fakeMailer{sent: nil, bodies: nil, err: nil}
	notifSvc := testNotifications(t, mail)

	job := jobs.NewWeeklyDigestJob(
		fakeFeedsLister{unhealthy: nil, err: assert.AnError},
		fakeDigestItemsLister{items: noDigestItems(), err: nil},
		notifSvc,
		alwaysEnabledSettings{},
	)
	require.NoError(t, job.Run(t.Context(), testLogger()))
	notifSvc.WaitUntilDone()

	assert.Len(t, mail.sent, 1)
}

func TestWeeklyDigestIncludesNewestItems(t *testing.T) {
	mail := &fakeMailer{sent: nil, bodies: nil, err: nil}
	notifSvc := testNotifications(t, mail)

	job := jobs.NewWeeklyDigestJob(
		fakeFeedsLister{unhealthy: nil, err: nil},
		fakeDigestItemsLister{items: newestOnly("Post"), err: nil},
		notifSvc,
		alwaysEnabledSettings{},
	)
	require.NoError(t, job.Run(t.Context(), testLogger()))
	notifSvc.WaitUntilDone()

	require.Len(t, mail.sent, 1)
	assert.Contains(t, mail.bodies[0], "newest unread")
	assert.Contains(t, mail.bodies[0], "https://example.com/Post")
}

func TestWeeklyDigestListsLastChanceItemsWithExpiry(t *testing.T) {
	mail := &fakeMailer{sent: nil, bodies: nil, err: nil}
	notifSvc := testNotifications(t, mail)
	expires := time.Date(2026, 11, 3, 9, 30, 0, 0, time.UTC)

	job := jobs.NewWeeklyDigestJob(
		fakeFeedsLister{unhealthy: nil, err: nil},
		fakeDigestItemsLister{items: jobs.FeedDigestItems{
			Newest:     nil,
			LastChance: []jobs.FeedDigestItem{digestItem("Fading", &expires)},
		}, err: nil},
		notifSvc,
		alwaysEnabledSettings{},
	)
	require.NoError(t, job.Run(t.Context(), testLogger()))
	notifSvc.WaitUntilDone()

	require.Len(t, mail.sent, 1)
	assert.Contains(t, mail.bodies[0], "last chance")
	assert.Contains(t, mail.bodies[0], "https://example.com/Fading")
	assert.Contains(t, mail.bodies[0], "expires Tue 3 Nov 09:30 UTC")
	assert.NotContains(t, mail.bodies[0], "newest unread")
}

func TestWeeklyDigestFeedItemsErrorOmitsSection(t *testing.T) {
	mail := &fakeMailer{sent: nil, bodies: nil, err: nil}
	notifSvc := testNotifications(t, mail)

	job := jobs.NewWeeklyDigestJob(
		fakeFeedsLister{unhealthy: nil, err: nil},
		fakeDigestItemsLister{items: noDigestItems(), err: assert.AnError},
		notifSvc,
		alwaysEnabledSettings{},
	)
	require.NoError(t, job.Run(t.Context(), testLogger()))
	notifSvc.WaitUntilDone()

	assert.Len(t, mail.sent, 1)
}

func TestWeeklyDigestOmitsFeedItemsSectionForDisabledSource(t *testing.T) {
	mail := &fakeMailer{sent: nil, bodies: nil, err: nil}
	notifSvc := testNotifications(t, mail)
	//nolint:exhaustive //only unhealthy_feeds needs to be enabled here
	settings := disabledSourceSettings{
		enabled: map[platformrepositories.NotificationSource]bool{
			platformrepositories.NotificationSourceUnhealthyFeeds: true,
		},
	}

	job := jobs.NewWeeklyDigestJob(
		fakeFeedsLister{unhealthy: nil, err: nil},
		fakeDigestItemsLister{items: newestOnly("Post"), err: nil},
		notifSvc,
		settings,
	)
	require.NoError(t, job.Run(t.Context(), testLogger()))
	notifSvc.WaitUntilDone()

	require.Len(t, mail.sent, 1)
	assert.NotContains(t, mail.bodies[0], "https://example.com/Post")
}

func TestWeeklyDigestOmitsUnhealthyFeedsSectionForDisabledSource(t *testing.T) {
	mail := &fakeMailer{sent: nil, bodies: nil, err: nil}
	notifSvc := testNotifications(t, mail)
	//nolint:exhaustive //only open_feed_items needs to be enabled here
	settings := disabledSourceSettings{
		enabled: map[platformrepositories.NotificationSource]bool{
			platformrepositories.NotificationSourceOpenFeedItems: true,
		},
	}

	job := jobs.NewWeeklyDigestJob(
		fakeFeedsLister{unhealthy: []jobs.UnhealthyFeed{
			{
				Title: "My Feed", URL: "https://example.com/feed",
				LastError: "timeout", ConsecutiveFailures: 4,
			},
		}, err: nil},
		fakeDigestItemsLister{items: noDigestItems(), err: nil},
		notifSvc,
		settings,
	)
	require.NoError(t, job.Run(t.Context(), testLogger()))
	notifSvc.WaitUntilDone()

	require.Len(t, mail.sent, 1)
	assert.NotContains(t, mail.bodies[0], "https://example.com/Post")
}

func TestWeeklyDigestSkipsSendWhenAllSourcesDisabled(t *testing.T) {
	feeds := fakeFeedsLister{unhealthy: []jobs.UnhealthyFeed{
		{
			Title: "My Feed", URL: "https://example.com/feed",
			LastError: "timeout", ConsecutiveFailures: 4,
		},
	}, err: nil}
	mail := &fakeMailer{sent: nil, bodies: nil, err: nil}
	notifSvc := testNotifications(t, mail)
	settings := disabledSourceSettings{
		enabled: make(map[platformrepositories.NotificationSource]bool),
	}

	job := jobs.NewWeeklyDigestJob(
		feeds,
		fakeDigestItemsLister{items: noDigestItems(), err: nil},
		notifSvc,
		settings,
	)
	require.NoError(t, job.Run(t.Context(), testLogger()))
	notifSvc.WaitUntilDone()

	assert.Empty(t, mail.sent)
}

func TestWeeklyDigestSettingsErrorOmitsSection(t *testing.T) {
	mail := &fakeMailer{sent: nil, bodies: nil, err: nil}
	notifSvc := testNotifications(t, mail)

	job := jobs.NewWeeklyDigestJob(
		fakeFeedsLister{unhealthy: nil, err: nil},
		fakeDigestItemsLister{items: newestOnly("Post"), err: nil},
		notifSvc,
		settingsErrFake{err: assert.AnError},
	)
	require.NoError(t, job.Run(t.Context(), testLogger()))
	notifSvc.WaitUntilDone()

	// IsEnabled errors fail open (enabled) but render empty, so the all-clear
	// still sends.
	require.Len(t, mail.sent, 1)
	assert.NotContains(t, mail.bodies[0], "https://example.com/Post")
}

type settingsErrFake struct {
	err error
}

func (s settingsErrFake) IsEnabled(
	_ context.Context,
	_ platformrepositories.NotificationSource,
) (bool, error) {
	return false, s.err
}

func TestWeeklyDigestID(t *testing.T) {
	job := jobs.NewWeeklyDigestJob(
		fakeFeedsLister{unhealthy: nil, err: nil},
		fakeDigestItemsLister{items: noDigestItems(), err: nil},
		testNotifications(t, &fakeMailer{sent: nil, bodies: nil, err: nil}),
		alwaysEnabledSettings{},
	)
	assert.Equal(t, "weekly-digest", job.ID())
	assert.Equal(t, 7*24*time.Hour, job.RunEvery())
}
