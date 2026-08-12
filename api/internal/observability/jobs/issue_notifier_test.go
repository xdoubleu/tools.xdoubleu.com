package jobs_test

import (
	"bytes"
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/digitalocean"
	"tools.xdoubleu.com/internal/github"
	"tools.xdoubleu.com/internal/logging"
	"tools.xdoubleu.com/internal/mailer"
	"tools.xdoubleu.com/internal/notifications"
	"tools.xdoubleu.com/internal/observability/jobs"
	"tools.xdoubleu.com/internal/sentryapi"
)

type fakeSentryClient struct {
	issues []sentryapi.Issue
	err    error
}

func (f fakeSentryClient) ListUnresolvedIssues(
	_ context.Context,
) ([]sentryapi.Issue, error) {
	return f.issues, f.err
}

func (f fakeSentryClient) ResolveIssue(_ context.Context, _ string) error {
	return nil
}

func (f fakeSentryClient) ListOrgs(_ context.Context) ([]sentryapi.Org, error) {
	return nil, nil
}

func (f fakeSentryClient) ListProjects(
	_ context.Context, _ string,
) ([]sentryapi.Project, error) {
	return nil, nil
}

func sentryIssue(id, title string) sentryapi.Issue {
	return sentryapi.Issue{
		ID:        id,
		Title:     title,
		Culprit:   "",
		Permalink: "",
		Count:     0,
		LastSeen:  time.Time{},
		Level:     "",
		Project:   "",
	}
}

type fakeDOClient struct {
	deployment *digitalocean.Deployment
	err        error
}

func (f fakeDOClient) LatestDeployment(
	_ context.Context,
) (*digitalocean.Deployment, error) {
	return f.deployment, f.err
}

func (f fakeDOClient) ListApps(_ context.Context) ([]digitalocean.App, error) {
	return nil, nil
}

func (f fakeDOClient) DeploymentLogs(
	_ context.Context, _ string, _ int,
) ([]digitalocean.ComponentLog, error) {
	return nil, nil
}

func (f fakeDOClient) DeploymentLogsStream(
	_ context.Context, _ string, _ int, _ func(digitalocean.ComponentLog) error,
) error {
	return nil
}

func doDeployment(id, phase string) *digitalocean.Deployment {
	return &digitalocean.Deployment{
		ID:        id,
		Phase:     phase,
		Cause:     "",
		CreatedAt: time.Time{},
		UpdatedAt: time.Time{},
	}
}

type fakeGithubClient struct {
	prs []github.PullRequest
	err error
}

func (f fakeGithubClient) ListFailingPullRequests(
	_ context.Context,
) ([]github.PullRequest, error) {
	return f.prs, f.err
}

func failingPR(headSHA string, labels ...string) github.PullRequest {
	return failingPRNumbered(42, headSHA, labels...)
}

func failingPRNumbered(
	number int64,
	headSHA string,
	labels ...string,
) github.PullRequest {
	return github.PullRequest{
		Number:    number,
		Title:     "Bump some-dep from 1.0.0 to 1.0.1",
		URL:       "https://gh/pr/" + headSHA,
		Author:    "renovate[bot]",
		UpdatedAt: time.Time{},
		HeadSHA:   headSHA,
		Labels:    labels,
		FailingChecks: []github.FailingCheck{
			{Name: "ci-pass", Conclusion: "failure", URL: ""},
		},
	}
}

type fakeMailer struct {
	sent []string
	err  error
}

func (f *fakeMailer) Send(_ context.Context, subject, _ string) error {
	if f.err != nil {
		return f.err
	}
	f.sent = append(f.sent, subject)
	return nil
}

func (f *fakeMailer) SendTo(_ context.Context, _, _, _ string) error {
	return nil
}

type fakeNotifiedRepo struct {
	keys map[string]bool
}

func newFakeNotifiedRepo() *fakeNotifiedRepo {
	return &fakeNotifiedRepo{keys: map[string]bool{}}
}

func (f *fakeNotifiedRepo) Exists(_ context.Context, key string) (bool, error) {
	return f.keys[key], nil
}

func (f *fakeNotifiedRepo) Insert(_ context.Context, key string) error {
	f.keys[key] = true
	return nil
}

// erroringNotifiedRepo makes Exists always fail, for
// TestIssueNotifierNotifiedExistsErrorPropagates.
type erroringNotifiedRepo struct {
	err error
}

func (f *erroringNotifiedRepo) Exists(_ context.Context, _ string) (bool, error) {
	return false, f.err
}

func (f *erroringNotifiedRepo) Insert(_ context.Context, _ string) error {
	return nil
}

func testLogger() *slog.Logger {
	logger, _ := testLoggerWithBuf()
	return logger
}

func testLoggerWithBuf() (*slog.Logger, *bytes.Buffer) {
	buf := &bytes.Buffer{}
	return slog.New(slog.NewTextHandler(buf, nil)), buf
}

// testNotifications wraps mail in a notifications.Service for the job to
// enqueue onto; deliveries now happen on a background worker (issue #923),
// so tests must call WaitUntilDone before asserting on mail/notified state.
func testNotifications(t *testing.T, mail *fakeMailer) *notifications.Service {
	t.Helper()
	return notifications.New(t.Context(), logging.NewNopLogger(), mail)
}

func TestIssueNotifierSendsForNewSentryIssue(t *testing.T) {
	sentry := fakeSentryClient{
		issues: []sentryapi.Issue{sentryIssue("1", "boom")}, err: nil,
	}
	do := fakeDOClient{deployment: nil, err: nil}
	mail := &fakeMailer{sent: nil, err: nil}
	notified := newFakeNotifiedRepo()
	notifSvc := testNotifications(t, mail)

	job := jobs.NewIssueNotifierJob(
		sentry, do, fakeGithubClient{prs: nil, err: nil}, notifSvc, notified,
	)
	require.NoError(t, job.Run(t.Context(), testLogger()))
	notifSvc.WaitUntilDone()

	assert.Len(t, mail.sent, 1)
	assert.True(t, notified.keys["sentry:1"])
}

func TestIssueNotifierSkipsAlreadyNotifiedIssue(t *testing.T) {
	sentry := fakeSentryClient{
		issues: []sentryapi.Issue{sentryIssue("1", "boom")}, err: nil,
	}
	do := fakeDOClient{deployment: nil, err: nil}
	mail := &fakeMailer{sent: nil, err: nil}
	notified := newFakeNotifiedRepo()
	notified.keys["sentry:1"] = true
	notifSvc := testNotifications(t, mail)

	job := jobs.NewIssueNotifierJob(
		sentry, do, fakeGithubClient{prs: nil, err: nil}, notifSvc, notified,
	)
	require.NoError(t, job.Run(t.Context(), testLogger()))
	notifSvc.WaitUntilDone()

	assert.Empty(t, mail.sent)
}

func TestIssueNotifierSentryNotConfiguredDoesNotBlockDO(t *testing.T) {
	sentry := fakeSentryClient{issues: nil, err: sentryapi.ErrNotConfigured}
	do := fakeDOClient{deployment: doDeployment("d1", "ERROR"), err: nil}
	mail := &fakeMailer{sent: nil, err: nil}
	notified := newFakeNotifiedRepo()
	notifSvc := testNotifications(t, mail)

	job := jobs.NewIssueNotifierJob(
		sentry, do, fakeGithubClient{prs: nil, err: nil}, notifSvc, notified,
	)
	require.NoError(t, job.Run(t.Context(), testLogger()))
	notifSvc.WaitUntilDone()

	assert.Len(t, mail.sent, 1)
	assert.True(t, notified.keys["digitalocean:d1"])
}

func TestIssueNotifierDOIgnoresNonErrorPhase(t *testing.T) {
	sentry := fakeSentryClient{issues: nil, err: nil}
	do := fakeDOClient{deployment: doDeployment("d1", "ACTIVE"), err: nil}
	mail := &fakeMailer{sent: nil, err: nil}
	notified := newFakeNotifiedRepo()
	notifSvc := testNotifications(t, mail)

	job := jobs.NewIssueNotifierJob(
		sentry, do, fakeGithubClient{prs: nil, err: nil}, notifSvc, notified,
	)
	require.NoError(t, job.Run(t.Context(), testLogger()))
	notifSvc.WaitUntilDone()

	assert.Empty(t, mail.sent)
}

func TestIssueNotifierDONotConfiguredSkipsSilently(t *testing.T) {
	sentry := fakeSentryClient{issues: nil, err: nil}
	do := fakeDOClient{deployment: nil, err: digitalocean.ErrNotConfigured}
	mail := &fakeMailer{sent: nil, err: nil}
	notified := newFakeNotifiedRepo()
	notifSvc := testNotifications(t, mail)

	job := jobs.NewIssueNotifierJob(
		sentry, do, fakeGithubClient{prs: nil, err: nil}, notifSvc, notified,
	)
	require.NoError(t, job.Run(t.Context(), testLogger()))
	notifSvc.WaitUntilDone()

	assert.Empty(t, mail.sent)
}

func TestIssueNotifierMailerNotConfiguredDoesNotRecordAsNotified(t *testing.T) {
	sentry := fakeSentryClient{
		issues: []sentryapi.Issue{sentryIssue("1", "boom")}, err: nil,
	}
	do := fakeDOClient{deployment: nil, err: nil}
	mail := &fakeMailer{sent: nil, err: mailer.ErrNotConfigured}
	notified := newFakeNotifiedRepo()
	notifSvc := testNotifications(t, mail)

	job := jobs.NewIssueNotifierJob(
		sentry, do, fakeGithubClient{prs: nil, err: nil}, notifSvc, notified,
	)
	require.NoError(t, job.Run(t.Context(), testLogger()))
	notifSvc.WaitUntilDone()

	assert.False(t, notified.keys["sentry:1"])
}

func TestIssueNotifierLogsWarnForTransientDOError(t *testing.T) {
	sentry := fakeSentryClient{issues: nil, err: nil}
	do := fakeDOClient{deployment: nil, err: context.DeadlineExceeded}
	mail := &fakeMailer{sent: nil, err: nil}
	notified := newFakeNotifiedRepo()
	logger, buf := testLoggerWithBuf()
	notifSvc := testNotifications(t, mail)

	job := jobs.NewIssueNotifierJob(
		sentry, do, fakeGithubClient{prs: nil, err: nil}, notifSvc, notified,
	)
	require.NoError(t, job.Run(t.Context(), logger))
	notifSvc.WaitUntilDone()

	assert.Contains(t, buf.String(), "level=WARN")
	assert.NotContains(t, buf.String(), "level=ERROR")
}

func TestIssueNotifierLogsErrorForNonTransientDOError(t *testing.T) {
	sentry := fakeSentryClient{issues: nil, err: nil}
	do := fakeDOClient{deployment: nil, err: assert.AnError}
	mail := &fakeMailer{sent: nil, err: nil}
	notified := newFakeNotifiedRepo()
	logger, buf := testLoggerWithBuf()
	notifSvc := testNotifications(t, mail)

	job := jobs.NewIssueNotifierJob(
		sentry, do, fakeGithubClient{prs: nil, err: nil}, notifSvc, notified,
	)
	require.NoError(t, job.Run(t.Context(), logger))
	notifSvc.WaitUntilDone()

	assert.Contains(t, buf.String(), "level=ERROR")
}

func TestIssueNotifierLogsWarnForTransientSentryError(t *testing.T) {
	sentry := fakeSentryClient{issues: nil, err: context.DeadlineExceeded}
	do := fakeDOClient{deployment: nil, err: nil}
	mail := &fakeMailer{sent: nil, err: nil}
	notified := newFakeNotifiedRepo()
	logger, buf := testLoggerWithBuf()
	notifSvc := testNotifications(t, mail)

	job := jobs.NewIssueNotifierJob(
		sentry, do, fakeGithubClient{prs: nil, err: nil}, notifSvc, notified,
	)
	require.NoError(t, job.Run(t.Context(), logger))
	notifSvc.WaitUntilDone()

	assert.Contains(t, buf.String(), "level=WARN")
	assert.NotContains(t, buf.String(), "level=ERROR")
}

func TestIssueNotifierLogsErrorForNonTransientSentryError(t *testing.T) {
	sentry := fakeSentryClient{issues: nil, err: assert.AnError}
	do := fakeDOClient{deployment: nil, err: nil}
	mail := &fakeMailer{sent: nil, err: nil}
	notified := newFakeNotifiedRepo()
	logger, buf := testLoggerWithBuf()
	notifSvc := testNotifications(t, mail)

	job := jobs.NewIssueNotifierJob(
		sentry, do, fakeGithubClient{prs: nil, err: nil}, notifSvc, notified,
	)
	require.NoError(t, job.Run(t.Context(), logger))
	notifSvc.WaitUntilDone()

	assert.Contains(t, buf.String(), "level=ERROR")
}

func TestIssueNotifierIDAndRunEvery(t *testing.T) {
	job := jobs.NewIssueNotifierJob(
		fakeSentryClient{issues: nil, err: nil},
		fakeDOClient{deployment: nil, err: nil},
		fakeGithubClient{prs: nil, err: nil},
		testNotifications(t, &fakeMailer{sent: nil, err: nil}),
		newFakeNotifiedRepo(),
	)
	assert.Equal(t, "notify-new-issues", job.ID())
	assert.Positive(t, job.RunEvery())
}

func TestIssueNotifierNotifiedExistsErrorPropagates(t *testing.T) {
	sentry := fakeSentryClient{
		issues: []sentryapi.Issue{sentryIssue("42", "kaboom")}, err: nil,
	}
	do := fakeDOClient{deployment: nil, err: nil}
	mail := &fakeMailer{sent: nil, err: nil}
	notified := &erroringNotifiedRepo{err: assert.AnError}
	notifSvc := testNotifications(t, mail)

	job := jobs.NewIssueNotifierJob(
		sentry, do, fakeGithubClient{prs: nil, err: nil}, notifSvc, notified,
	)
	err := job.Run(t.Context(), testLogger())

	require.ErrorIs(t, err, assert.AnError)
	assert.Empty(t, mail.sent)
}

func TestIssueNotifierRealSendErrorIsNotMarkedAsNotified(t *testing.T) {
	sentry := fakeSentryClient{
		issues: []sentryapi.Issue{sentryIssue("1", "boom")}, err: nil,
	}
	do := fakeDOClient{deployment: nil, err: nil}
	mail := &fakeMailer{sent: nil, err: assert.AnError}
	notified := newFakeNotifiedRepo()
	notifSvc := testNotifications(t, mail)

	job := jobs.NewIssueNotifierJob(
		sentry, do, fakeGithubClient{prs: nil, err: nil}, notifSvc, notified,
	)
	require.NoError(t, job.Run(t.Context(), testLogger()))
	notifSvc.WaitUntilDone()

	assert.False(t, notified.keys["sentry:1"])
}

func TestIssueNotifierSendsForFailingDependencyPR(t *testing.T) {
	sentry := fakeSentryClient{issues: nil, err: nil}
	do := fakeDOClient{deployment: nil, err: nil}
	gh := fakeGithubClient{
		prs: []github.PullRequest{failingPR("sha1", "dependencies")}, err: nil,
	}
	mail := &fakeMailer{sent: nil, err: nil}
	notified := newFakeNotifiedRepo()
	notifSvc := testNotifications(t, mail)

	job := jobs.NewIssueNotifierJob(sentry, do, gh, notifSvc, notified)
	require.NoError(t, job.Run(t.Context(), testLogger()))
	notifSvc.WaitUntilDone()

	assert.Len(t, mail.sent, 1)
	assert.True(t, notified.keys["github:pr:42:sha1"])
}

func TestIssueNotifierGithubNoFailingPRs(t *testing.T) {
	sentry := fakeSentryClient{issues: nil, err: nil}
	do := fakeDOClient{deployment: nil, err: nil}
	gh := fakeGithubClient{prs: []github.PullRequest{}, err: nil}
	mail := &fakeMailer{sent: nil, err: nil}
	notified := newFakeNotifiedRepo()
	notifSvc := testNotifications(t, mail)

	job := jobs.NewIssueNotifierJob(sentry, do, gh, notifSvc, notified)
	require.NoError(t, job.Run(t.Context(), testLogger()))
	notifSvc.WaitUntilDone()

	assert.Empty(t, mail.sent)
}

func TestIssueNotifierGithubHandlesMultiplePRsMixedLabels(t *testing.T) {
	sentry := fakeSentryClient{issues: nil, err: nil}
	do := fakeDOClient{deployment: nil, err: nil}
	gh := fakeGithubClient{
		prs: []github.PullRequest{
			failingPRNumbered(1, "sha1", "bug"),
			failingPRNumbered(2, "sha2", "dependencies"),
			failingPRNumbered(3, "sha3", "dependencies", "go"),
		},
		err: nil,
	}
	mail := &fakeMailer{sent: nil, err: nil}
	notified := newFakeNotifiedRepo()
	notifSvc := testNotifications(t, mail)

	job := jobs.NewIssueNotifierJob(sentry, do, gh, notifSvc, notified)
	require.NoError(t, job.Run(t.Context(), testLogger()))
	notifSvc.WaitUntilDone()

	assert.Len(t, mail.sent, 2)
	assert.False(t, notified.keys["github:pr:1:sha1"])
	assert.True(t, notified.keys["github:pr:2:sha2"])
	assert.True(t, notified.keys["github:pr:3:sha3"])
}

func TestIssueNotifierIgnoresNonDependencyFailingPR(t *testing.T) {
	sentry := fakeSentryClient{issues: nil, err: nil}
	do := fakeDOClient{deployment: nil, err: nil}
	gh := fakeGithubClient{
		prs: []github.PullRequest{failingPR("sha1", "bug")}, err: nil,
	}
	mail := &fakeMailer{sent: nil, err: nil}
	notified := newFakeNotifiedRepo()
	notifSvc := testNotifications(t, mail)

	job := jobs.NewIssueNotifierJob(sentry, do, gh, notifSvc, notified)
	require.NoError(t, job.Run(t.Context(), testLogger()))
	notifSvc.WaitUntilDone()

	assert.Empty(t, mail.sent)
}

func TestIssueNotifierSkipsAlreadyNotifiedDependencyPR(t *testing.T) {
	sentry := fakeSentryClient{issues: nil, err: nil}
	do := fakeDOClient{deployment: nil, err: nil}
	gh := fakeGithubClient{
		prs: []github.PullRequest{failingPR("sha1", "dependencies")}, err: nil,
	}
	mail := &fakeMailer{sent: nil, err: nil}
	notified := newFakeNotifiedRepo()
	notified.keys["github:pr:42:sha1"] = true
	notifSvc := testNotifications(t, mail)

	job := jobs.NewIssueNotifierJob(sentry, do, gh, notifSvc, notified)
	require.NoError(t, job.Run(t.Context(), testLogger()))
	notifSvc.WaitUntilDone()

	assert.Empty(t, mail.sent)
}

func TestIssueNotifierRenotifiesDependencyPROnNewHeadSHA(t *testing.T) {
	sentry := fakeSentryClient{issues: nil, err: nil}
	do := fakeDOClient{deployment: nil, err: nil}
	gh := fakeGithubClient{
		prs: []github.PullRequest{failingPR("sha2", "dependencies")}, err: nil,
	}
	mail := &fakeMailer{sent: nil, err: nil}
	notified := newFakeNotifiedRepo()
	notified.keys["github:pr:42:sha1"] = true
	notifSvc := testNotifications(t, mail)

	job := jobs.NewIssueNotifierJob(sentry, do, gh, notifSvc, notified)
	require.NoError(t, job.Run(t.Context(), testLogger()))
	notifSvc.WaitUntilDone()

	assert.Len(t, mail.sent, 1)
	assert.True(t, notified.keys["github:pr:42:sha2"])
}

// TestIssueNotifierGithubNotifiedExistsErrorPropagates mirrors
// TestIssueNotifierNotifiedExistsErrorPropagates for the github path: a
// failed dedup lookup must abort the run instead of being swallowed.
func TestIssueNotifierGithubNotifiedExistsErrorPropagates(t *testing.T) {
	sentry := fakeSentryClient{issues: nil, err: nil}
	do := fakeDOClient{deployment: nil, err: nil}
	gh := fakeGithubClient{
		prs: []github.PullRequest{failingPR("sha1", "dependencies")}, err: nil,
	}
	mail := &fakeMailer{sent: nil, err: nil}
	notified := &erroringNotifiedRepo{err: assert.AnError}
	notifSvc := testNotifications(t, mail)

	job := jobs.NewIssueNotifierJob(sentry, do, gh, notifSvc, notified)
	err := job.Run(t.Context(), testLogger())

	require.ErrorIs(t, err, assert.AnError)
	assert.Empty(t, mail.sent)
}

// TestIssueNotifierDOErrorStopsRunBeforeGithub asserts Run now short-circuits
// on a genuine (non-degraded) notifyDigitalOcean error instead of falling
// through to notifyGithub — the DO and github steps used to be a single
// tail return, so this branch didn't exist before notifyGithub was added.
func TestIssueNotifierDOErrorStopsRunBeforeGithub(t *testing.T) {
	sentry := fakeSentryClient{issues: nil, err: nil}
	do := fakeDOClient{deployment: doDeployment("d1", "ERROR"), err: nil}
	gh := fakeGithubClient{
		prs: []github.PullRequest{failingPR("sha1", "dependencies")}, err: nil,
	}
	mail := &fakeMailer{sent: nil, err: nil}
	notified := &erroringNotifiedRepo{err: assert.AnError}
	notifSvc := testNotifications(t, mail)

	job := jobs.NewIssueNotifierJob(sentry, do, gh, notifSvc, notified)
	err := job.Run(t.Context(), testLogger())

	require.ErrorIs(t, err, assert.AnError)
	assert.Empty(t, mail.sent)
}

func TestIssueNotifierGithubNotConfiguredSkipsSilently(t *testing.T) {
	sentry := fakeSentryClient{issues: nil, err: nil}
	do := fakeDOClient{deployment: nil, err: nil}
	gh := fakeGithubClient{prs: nil, err: github.ErrNotConfigured}
	mail := &fakeMailer{sent: nil, err: nil}
	notified := newFakeNotifiedRepo()
	notifSvc := testNotifications(t, mail)

	job := jobs.NewIssueNotifierJob(sentry, do, gh, notifSvc, notified)
	require.NoError(t, job.Run(t.Context(), testLogger()))
	notifSvc.WaitUntilDone()

	assert.Empty(t, mail.sent)
}

func TestIssueNotifierLogsWarnForTransientGithubError(t *testing.T) {
	sentry := fakeSentryClient{issues: nil, err: nil}
	do := fakeDOClient{deployment: nil, err: nil}
	gh := fakeGithubClient{prs: nil, err: context.DeadlineExceeded}
	mail := &fakeMailer{sent: nil, err: nil}
	notified := newFakeNotifiedRepo()
	logger, buf := testLoggerWithBuf()
	notifSvc := testNotifications(t, mail)

	job := jobs.NewIssueNotifierJob(sentry, do, gh, notifSvc, notified)
	require.NoError(t, job.Run(t.Context(), logger))
	notifSvc.WaitUntilDone()

	assert.Contains(t, buf.String(), "level=WARN")
	assert.NotContains(t, buf.String(), "level=ERROR")
}

func TestIssueNotifierLogsErrorForNonTransientGithubError(t *testing.T) {
	sentry := fakeSentryClient{issues: nil, err: nil}
	do := fakeDOClient{deployment: nil, err: nil}
	gh := fakeGithubClient{prs: nil, err: assert.AnError}
	mail := &fakeMailer{sent: nil, err: nil}
	notified := newFakeNotifiedRepo()
	logger, buf := testLoggerWithBuf()
	notifSvc := testNotifications(t, mail)

	job := jobs.NewIssueNotifierJob(sentry, do, gh, notifSvc, notified)
	require.NoError(t, job.Run(t.Context(), logger))
	notifSvc.WaitUntilDone()

	assert.Contains(t, buf.String(), "level=ERROR")
}
