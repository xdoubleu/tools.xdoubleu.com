package jobs_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/observability/jobs"
	"tools.xdoubleu.com/internal/repositories"
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

type fakeOpenFeedItemsLister struct {
	open []jobs.OpenFeedItem
	err  error
}

func (f fakeOpenFeedItemsLister) ListOpenItems(
	_ context.Context,
) ([]jobs.OpenFeedItem, error) {
	return f.open, f.err
}

func TestWeeklyDigestSendsAllClearWhenNothingWrong(t *testing.T) {
	mail := &fakeMailer{sent: nil, bodies: nil, err: nil}
	notifSvc := testNotifications(t, mail)

	job := jobs.NewWeeklyDigestJob(
		fakeFeedsLister{unhealthy: nil, err: nil},
		fakeOpenFeedItemsLister{open: nil, err: nil},
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
		fakeOpenFeedItemsLister{open: nil, err: nil},
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
		fakeOpenFeedItemsLister{open: nil, err: nil},
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
		fakeOpenFeedItemsLister{open: nil, err: nil},
		notifSvc,
		alwaysEnabledSettings{},
	)
	require.NoError(t, job.Run(t.Context(), testLogger()))
	notifSvc.WaitUntilDone()

	assert.Len(t, mail.sent, 1)
}

func TestWeeklyDigestIncludesOpenFeedItems(t *testing.T) {
	mail := &fakeMailer{sent: nil, bodies: nil, err: nil}
	notifSvc := testNotifications(t, mail)

	job := jobs.NewWeeklyDigestJob(
		fakeFeedsLister{unhealthy: nil, err: nil},
		fakeOpenFeedItemsLister{open: []jobs.OpenFeedItem{
			{Title: "My Feed", URL: "https://example.com/feed", Count: 3},
		}, err: nil},
		notifSvc,
		alwaysEnabledSettings{},
	)
	require.NoError(t, job.Run(t.Context(), testLogger()))
	notifSvc.WaitUntilDone()

	require.Len(t, mail.sent, 1)
	assert.Contains(t, mail.bodies[0], "My Feed")
}

func TestWeeklyDigestOpenFeedItemsErrorOmitsSection(t *testing.T) {
	mail := &fakeMailer{sent: nil, bodies: nil, err: nil}
	notifSvc := testNotifications(t, mail)

	job := jobs.NewWeeklyDigestJob(
		fakeFeedsLister{unhealthy: nil, err: nil},
		fakeOpenFeedItemsLister{open: nil, err: assert.AnError},
		notifSvc,
		alwaysEnabledSettings{},
	)
	require.NoError(t, job.Run(t.Context(), testLogger()))
	notifSvc.WaitUntilDone()

	assert.Len(t, mail.sent, 1)
}

func TestWeeklyDigestOmitsOpenFeedItemsSectionForDisabledSource(t *testing.T) {
	mail := &fakeMailer{sent: nil, bodies: nil, err: nil}
	notifSvc := testNotifications(t, mail)
	//nolint:exhaustive //only unhealthy_feeds needs to be enabled here
	settings := disabledSourceSettings{
		enabled: map[repositories.NotificationSource]bool{
			repositories.NotificationSourceUnhealthyFeeds: true,
		},
	}

	job := jobs.NewWeeklyDigestJob(
		fakeFeedsLister{unhealthy: nil, err: nil},
		fakeOpenFeedItemsLister{open: []jobs.OpenFeedItem{
			{Title: "My Feed", URL: "https://example.com/feed", Count: 3},
		}, err: nil},
		notifSvc,
		settings,
	)
	require.NoError(t, job.Run(t.Context(), testLogger()))
	notifSvc.WaitUntilDone()

	require.Len(t, mail.sent, 1)
	assert.NotContains(t, mail.bodies[0], "My Feed")
}

func TestWeeklyDigestOmitsUnhealthyFeedsSectionForDisabledSource(t *testing.T) {
	mail := &fakeMailer{sent: nil, bodies: nil, err: nil}
	notifSvc := testNotifications(t, mail)
	//nolint:exhaustive //only open_feed_items needs to be enabled here
	settings := disabledSourceSettings{
		enabled: map[repositories.NotificationSource]bool{
			repositories.NotificationSourceOpenFeedItems: true,
		},
	}

	job := jobs.NewWeeklyDigestJob(
		fakeFeedsLister{unhealthy: []jobs.UnhealthyFeed{
			{
				Title: "My Feed", URL: "https://example.com/feed",
				LastError: "timeout", ConsecutiveFailures: 4,
			},
		}, err: nil},
		fakeOpenFeedItemsLister{open: nil, err: nil},
		notifSvc,
		settings,
	)
	require.NoError(t, job.Run(t.Context(), testLogger()))
	notifSvc.WaitUntilDone()

	require.Len(t, mail.sent, 1)
	assert.NotContains(t, mail.bodies[0], "My Feed")
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
		enabled: map[repositories.NotificationSource]bool{},
	}

	job := jobs.NewWeeklyDigestJob(
		feeds,
		fakeOpenFeedItemsLister{open: nil, err: nil},
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
		fakeOpenFeedItemsLister{open: []jobs.OpenFeedItem{
			{Title: "My Feed", URL: "https://example.com/feed", Count: 3},
		}, err: nil},
		notifSvc,
		settingsErrFake{err: assert.AnError},
	)
	require.NoError(t, job.Run(t.Context(), testLogger()))
	notifSvc.WaitUntilDone()

	// IsEnabled errors fail open (enabled) but render empty, so the all-clear
	// still sends.
	require.Len(t, mail.sent, 1)
	assert.NotContains(t, mail.bodies[0], "My Feed")
}

type settingsErrFake struct {
	err error
}

func (s settingsErrFake) IsEnabled(
	_ context.Context,
	_ repositories.NotificationSource,
) (bool, error) {
	return false, s.err
}

func TestWeeklyDigestID(t *testing.T) {
	job := jobs.NewWeeklyDigestJob(
		fakeFeedsLister{unhealthy: nil, err: nil},
		fakeOpenFeedItemsLister{open: nil, err: nil},
		testNotifications(t, &fakeMailer{sent: nil, bodies: nil, err: nil}),
		alwaysEnabledSettings{},
	)
	assert.Equal(t, "weekly-digest", job.ID())
	assert.Equal(t, 7*24*time.Hour, job.RunEvery())
}
