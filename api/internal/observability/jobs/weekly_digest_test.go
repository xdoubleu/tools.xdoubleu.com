package jobs_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/github"
	"tools.xdoubleu.com/internal/observability/jobs"
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
		fakeGithubClient{prs: nil, err: nil},
		fakeFeedsLister{unhealthy: nil, err: nil},
		notifSvc,
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
		fakeGithubClient{prs: nil, err: nil},
		fakeFeedsLister{unhealthy: nil, err: nil},
		notifSvc,
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
		fakeGithubClient{prs: nil, err: nil},
		fakeFeedsLister{unhealthy: []jobs.UnhealthyFeed{
			{
				Title: "My Feed", URL: "https://example.com/feed",
				LastError: "timeout", ConsecutiveFailures: 4,
			},
		}, err: nil},
		notifSvc,
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
		fakeGithubClient{prs: nil, err: nil},
		fakeFeedsLister{unhealthy: nil, err: nil},
		notifSvc,
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
		fakeGithubClient{prs: []github.PullRequest{
			failingPR("sha1"),
		}, err: nil},
		fakeFeedsLister{unhealthy: nil, err: nil},
		notifSvc,
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
		fakeGithubClient{prs: nil, err: nil},
		fakeFeedsLister{unhealthy: nil, err: assert.AnError},
		notifSvc,
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
		fakeGithubClient{prs: nil, err: github.ErrNotConfigured},
		fakeFeedsLister{unhealthy: nil, err: nil},
		notifSvc,
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
		fakeGithubClient{prs: nil, err: nil},
		fakeFeedsLister{unhealthy: nil, err: nil},
		notifSvc,
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
		fakeGithubClient{prs: nil, err: assert.AnError},
		fakeFeedsLister{unhealthy: nil, err: nil},
		notifSvc,
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
		fakeGithubClient{prs: []github.PullRequest{
			failingPR("sha1", "not-dependencies"),
		}, err: nil},
		fakeFeedsLister{unhealthy: nil, err: nil},
		notifSvc,
	)
	require.NoError(t, job.Run(t.Context(), testLogger()))
	notifSvc.WaitUntilDone()

	assert.Len(t, mail.sent, 1)
}

func TestWeeklyDigestID(t *testing.T) {
	job := jobs.NewWeeklyDigestJob(
		fakeSentryClient{issues: nil, err: nil},
		fakeGithubClient{prs: nil, err: nil},
		fakeFeedsLister{unhealthy: nil, err: nil},
		testNotifications(t, &fakeMailer{sent: nil, err: nil}),
	)
	assert.Equal(t, "weekly-digest", job.ID())
	assert.Equal(t, 7*24*time.Hour, job.RunEvery())
}
