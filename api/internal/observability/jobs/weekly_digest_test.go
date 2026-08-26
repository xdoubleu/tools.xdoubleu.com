package jobs_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/github"
	"tools.xdoubleu.com/internal/observability/jobs"
	"tools.xdoubleu.com/internal/repositories"
	"tools.xdoubleu.com/internal/sentryapi"
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

func TestWeeklyDigestSendsAllClearWhenNothingWrong(t *testing.T) {
	mail := &fakeMailer{sent: nil, err: nil}
	notifSvc := testNotifications(t, mail)

	job := jobs.NewWeeklyDigestJob(
		fakeSentryClient{issues: nil, err: nil},
		fakeGithubClient{prs: nil, err: nil, alerts: nil, alertsErr: nil},
		fakeFeedsLister{unhealthy: nil, err: nil},
		notifSvc,
		alwaysEnabledSettings{},
	)
	require.NoError(t, job.Run(t.Context(), testLogger()))
	notifSvc.WaitUntilDone()

	assert.Len(t, mail.sent, 1)
}

func TestWeeklyDigestAlwaysSendsEvenWhenPreviouslySeen(t *testing.T) {
	sentry := fakeSentryClient{
		issues: []sentryapi.Issue{sentryIssue("1", "boom")}, err: nil,
	}
	mail := &fakeMailer{sent: nil, err: nil}
	notifSvc := testNotifications(t, mail)

	job := jobs.NewWeeklyDigestJob(
		sentry,
		fakeGithubClient{prs: nil, err: nil, alerts: nil, alertsErr: nil},
		fakeFeedsLister{unhealthy: nil, err: nil},
		notifSvc,
		alwaysEnabledSettings{},
	)
	require.NoError(t, job.Run(t.Context(), testLogger()))
	require.NoError(t, job.Run(t.Context(), testLogger()))
	notifSvc.WaitUntilDone()

	assert.Len(t, mail.sent, 2)
}

func TestWeeklyDigestIncludesUnhealthyFeeds(t *testing.T) {
	mail := &fakeMailer{sent: nil, err: nil}
	notifSvc := testNotifications(t, mail)

	job := jobs.NewWeeklyDigestJob(
		fakeSentryClient{issues: nil, err: nil},
		fakeGithubClient{prs: nil, err: nil, alerts: nil, alertsErr: nil},
		fakeFeedsLister{unhealthy: []jobs.UnhealthyFeed{
			{
				Title: "My Feed", URL: "https://example.com/feed",
				LastError: "timeout", ConsecutiveFailures: 4,
			},
		}, err: nil},
		notifSvc,
		alwaysEnabledSettings{},
	)
	require.NoError(t, job.Run(t.Context(), testLogger()))
	notifSvc.WaitUntilDone()

	assert.Len(t, mail.sent, 1)
}

func TestWeeklyDigestSentryNotConfiguredDoesNotBlockOthers(t *testing.T) {
	mail := &fakeMailer{sent: nil, err: nil}
	notifSvc := testNotifications(t, mail)

	job := jobs.NewWeeklyDigestJob(
		fakeSentryClient{issues: nil, err: sentryapi.ErrNotConfigured},
		fakeGithubClient{prs: nil, err: nil, alerts: nil, alertsErr: nil},
		fakeFeedsLister{unhealthy: nil, err: nil},
		notifSvc,
		alwaysEnabledSettings{},
	)
	require.NoError(t, job.Run(t.Context(), testLogger()))
	notifSvc.WaitUntilDone()

	assert.Len(t, mail.sent, 1)
}

func TestWeeklyDigestGithubOnlyIncludesDependencyPRs(t *testing.T) {
	mail := &fakeMailer{sent: nil, err: nil}
	notifSvc := testNotifications(t, mail)

	job := jobs.NewWeeklyDigestJob(
		fakeSentryClient{issues: nil, err: nil},
		fakeGithubClient{
			prs: []github.PullRequest{
				failingPR("sha1"),
			}, err: nil, alerts: nil, alertsErr: nil,
		},
		fakeFeedsLister{unhealthy: nil, err: nil},
		notifSvc,
		alwaysEnabledSettings{},
	)
	require.NoError(t, job.Run(t.Context(), testLogger()))
	notifSvc.WaitUntilDone()

	assert.Len(t, mail.sent, 1)
}

func TestWeeklyDigestFeedsErrorDoesNotFailRun(t *testing.T) {
	mail := &fakeMailer{sent: nil, err: nil}
	notifSvc := testNotifications(t, mail)

	job := jobs.NewWeeklyDigestJob(
		fakeSentryClient{issues: nil, err: nil},
		fakeGithubClient{prs: nil, err: nil, alerts: nil, alertsErr: nil},
		fakeFeedsLister{unhealthy: nil, err: assert.AnError},
		notifSvc,
		alwaysEnabledSettings{},
	)
	require.NoError(t, job.Run(t.Context(), testLogger()))
	notifSvc.WaitUntilDone()

	assert.Len(t, mail.sent, 1)
}

func TestWeeklyDigestGithubNotConfiguredSkipsSilently(t *testing.T) {
	mail := &fakeMailer{sent: nil, err: nil}
	notifSvc := testNotifications(t, mail)

	job := jobs.NewWeeklyDigestJob(
		fakeSentryClient{issues: nil, err: nil},
		fakeGithubClient{prs: nil, err: github.ErrNotConfigured, alerts: nil, alertsErr: nil},
		fakeFeedsLister{unhealthy: nil, err: nil},
		notifSvc,
		alwaysEnabledSettings{},
	)
	require.NoError(t, job.Run(t.Context(), testLogger()))
	notifSvc.WaitUntilDone()

	assert.Len(t, mail.sent, 1)
}

func TestWeeklyDigestSentryGenericErrorSkipsSilently(t *testing.T) {
	mail := &fakeMailer{sent: nil, err: nil}
	notifSvc := testNotifications(t, mail)

	job := jobs.NewWeeklyDigestJob(
		fakeSentryClient{issues: nil, err: assert.AnError},
		fakeGithubClient{prs: nil, err: nil, alerts: nil, alertsErr: nil},
		fakeFeedsLister{unhealthy: nil, err: nil},
		notifSvc,
		alwaysEnabledSettings{},
	)
	require.NoError(t, job.Run(t.Context(), testLogger()))
	notifSvc.WaitUntilDone()

	assert.Len(t, mail.sent, 1)
}

func TestWeeklyDigestGithubGenericErrorSkipsSilently(t *testing.T) {
	mail := &fakeMailer{sent: nil, err: nil}
	notifSvc := testNotifications(t, mail)

	job := jobs.NewWeeklyDigestJob(
		fakeSentryClient{issues: nil, err: nil},
		fakeGithubClient{prs: nil, err: assert.AnError, alerts: nil, alertsErr: nil},
		fakeFeedsLister{unhealthy: nil, err: nil},
		notifSvc,
		alwaysEnabledSettings{},
	)
	require.NoError(t, job.Run(t.Context(), testLogger()))
	notifSvc.WaitUntilDone()

	assert.Len(t, mail.sent, 1)
}

func TestWeeklyDigestGithubIgnoresNonDependencyPR(t *testing.T) {
	mail := &fakeMailer{sent: nil, err: nil}
	notifSvc := testNotifications(t, mail)

	job := jobs.NewWeeklyDigestJob(
		fakeSentryClient{issues: nil, err: nil},
		fakeGithubClient{
			prs: []github.PullRequest{
				failingPR("sha1", "not-dependencies"),
			}, err: nil, alerts: nil, alertsErr: nil,
		},
		fakeFeedsLister{unhealthy: nil, err: nil},
		notifSvc,
		alwaysEnabledSettings{},
	)
	require.NoError(t, job.Run(t.Context(), testLogger()))
	notifSvc.WaitUntilDone()

	assert.Len(t, mail.sent, 1)
}

func TestWeeklyDigestOmitsSectionForDisabledSource(t *testing.T) {
	sentry := fakeSentryClient{
		issues: []sentryapi.Issue{sentryIssue("1", "boom")}, err: nil,
	}
	gh := fakeGithubClient{
		prs: []github.PullRequest{failingPR("sha1", "dependencies")}, err: nil,
		alerts: nil, alertsErr: nil,
	}
	mail := &fakeMailer{sent: nil, err: nil}
	notifSvc := testNotifications(t, mail)
	//nolint:exhaustive //only failing_dependency_prs needs to be enabled here
	settings := disabledSourceSettings{
		enabled: map[repositories.NotificationSource]bool{
			repositories.NotificationSourceFailingDependencyPRs: true,
		},
	}

	job := jobs.NewWeeklyDigestJob(
		sentry,
		gh,
		fakeFeedsLister{unhealthy: nil, err: nil},
		notifSvc,
		settings,
	)
	require.NoError(t, job.Run(t.Context(), testLogger()))
	notifSvc.WaitUntilDone()

	require.Len(t, mail.sent, 1)
	assert.NotContains(t, mail.sent[0], "boom")
}

// TestWeeklyDigestSkipsSendWhenAllSourcesDisabled covers the gap in issue
// #1214's original settings gate: each *Section was already omitted when
// disabled, but the digest email itself still always sent — even an empty
// "no open issues" email — regardless of whether every source had been
// explicitly turned off. An admin who disabled everything shouldn't keep
// getting a weekly email with nothing in it.
func TestWeeklyDigestSkipsSendWhenAllSourcesDisabled(t *testing.T) {
	sentry := fakeSentryClient{
		issues: []sentryapi.Issue{sentryIssue("1", "boom")}, err: nil,
	}
	gh := fakeGithubClient{
		prs: []github.PullRequest{failingPR("sha1", "dependencies")}, err: nil,
		alerts: nil, alertsErr: nil,
	}
	feeds := fakeFeedsLister{unhealthy: []jobs.UnhealthyFeed{
		{
			Title: "My Feed", URL: "https://example.com/feed",
			LastError: "timeout", ConsecutiveFailures: 4,
		},
	}, err: nil}
	mail := &fakeMailer{sent: nil, err: nil}
	notifSvc := testNotifications(t, mail)
	settings := disabledSourceSettings{
		enabled: map[repositories.NotificationSource]bool{},
	}

	job := jobs.NewWeeklyDigestJob(sentry, gh, feeds, notifSvc, settings)
	require.NoError(t, job.Run(t.Context(), testLogger()))
	notifSvc.WaitUntilDone()

	assert.Empty(t, mail.sent)
}

func TestWeeklyDigestSettingsErrorOmitsSection(t *testing.T) {
	sentry := fakeSentryClient{
		issues: []sentryapi.Issue{sentryIssue("1", "boom")}, err: nil,
	}
	mail := &fakeMailer{sent: nil, err: nil}
	notifSvc := testNotifications(t, mail)

	job := jobs.NewWeeklyDigestJob(
		sentry,
		fakeGithubClient{prs: nil, err: nil, alerts: nil, alertsErr: nil},
		fakeFeedsLister{unhealthy: nil, err: nil},
		notifSvc,
		settingsErrFake{err: assert.AnError},
	)
	require.NoError(t, job.Run(t.Context(), testLogger()))
	notifSvc.WaitUntilDone()

	require.Len(t, mail.sent, 1)
	assert.NotContains(t, mail.sent[0], "boom")
}

// settingsErrFake makes every IsEnabled call fail, for tests exercising the
// settings-lookup-error path.
type settingsErrFake struct {
	err error
}

func (s settingsErrFake) IsEnabled(
	_ context.Context,
	_ repositories.NotificationSource,
) (bool, error) {
	return false, s.err
}

func TestWeeklyDigestIncludesSecurityAlerts(t *testing.T) {
	mail := &fakeMailer{sent: nil, err: nil}
	notifSvc := testNotifications(t, mail)

	job := jobs.NewWeeklyDigestJob(
		fakeSentryClient{issues: nil, err: nil},
		fakeGithubClient{
			prs: nil, err: nil,
			alerts:    []github.SecurityAlert{securityAlert(github.SecurityAlertTypeDependabot)},
			alertsErr: nil,
		},
		fakeFeedsLister{unhealthy: nil, err: nil},
		notifSvc,
		alwaysEnabledSettings{},
	)
	require.NoError(t, job.Run(t.Context(), testLogger()))
	notifSvc.WaitUntilDone()

	require.Len(t, mail.sent, 1)
}

func TestWeeklyDigestSecurityAlertsNotConfiguredSkipsSilently(t *testing.T) {
	mail := &fakeMailer{sent: nil, err: nil}
	notifSvc := testNotifications(t, mail)

	job := jobs.NewWeeklyDigestJob(
		fakeSentryClient{issues: nil, err: nil},
		fakeGithubClient{prs: nil, err: nil, alerts: nil, alertsErr: github.ErrNotConfigured},
		fakeFeedsLister{unhealthy: nil, err: nil},
		notifSvc,
		alwaysEnabledSettings{},
	)
	require.NoError(t, job.Run(t.Context(), testLogger()))
	notifSvc.WaitUntilDone()

	assert.Len(t, mail.sent, 1)
}

// TestWeeklyDigestSecurityAlertsUpstreamErrorSkipsSilently asserts a
// non-ErrNotConfigured failure from ListSecurityAlerts omits the section
// (self-heals on the next run) rather than failing the digest send.
func TestWeeklyDigestSecurityAlertsUpstreamErrorSkipsSilently(t *testing.T) {
	mail := &fakeMailer{sent: nil, err: nil}
	notifSvc := testNotifications(t, mail)

	job := jobs.NewWeeklyDigestJob(
		fakeSentryClient{issues: nil, err: nil},
		fakeGithubClient{
			prs: nil, err: nil, alerts: nil, alertsErr: assert.AnError,
		},
		fakeFeedsLister{unhealthy: nil, err: nil},
		notifSvc,
		alwaysEnabledSettings{},
	)
	require.NoError(t, job.Run(t.Context(), testLogger()))
	notifSvc.WaitUntilDone()

	assert.Len(t, mail.sent, 1)
}

func TestWeeklyDigestOmitsSecurityAlertsSectionForDisabledSource(t *testing.T) {
	gh := fakeGithubClient{
		prs: nil, err: nil,
		alerts:    []github.SecurityAlert{securityAlert(github.SecurityAlertTypeDependabot)},
		alertsErr: nil,
	}
	mail := &fakeMailer{sent: nil, err: nil}
	notifSvc := testNotifications(t, mail)
	//nolint:exhaustive //only sentry_issues needs to be enabled here
	settings := disabledSourceSettings{
		enabled: map[repositories.NotificationSource]bool{
			repositories.NotificationSourceSentryIssues: true,
		},
	}

	job := jobs.NewWeeklyDigestJob(
		fakeSentryClient{issues: nil, err: nil},
		gh,
		fakeFeedsLister{unhealthy: nil, err: nil},
		notifSvc,
		settings,
	)
	require.NoError(t, job.Run(t.Context(), testLogger()))
	notifSvc.WaitUntilDone()

	require.Len(t, mail.sent, 1)
	assert.NotContains(t, mail.sent[0], "vulnerable dependency")
}

func TestWeeklyDigestID(t *testing.T) {
	job := jobs.NewWeeklyDigestJob(
		fakeSentryClient{issues: nil, err: nil},
		fakeGithubClient{prs: nil, err: nil, alerts: nil, alertsErr: nil},
		fakeFeedsLister{unhealthy: nil, err: nil},
		testNotifications(t, &fakeMailer{sent: nil, err: nil}),
		alwaysEnabledSettings{},
	)
	assert.Equal(t, "weekly-digest", job.ID())
	assert.Equal(t, 7*24*time.Hour, job.RunEvery())
}
