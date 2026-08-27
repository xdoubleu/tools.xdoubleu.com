package jobs_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/github"
	"tools.xdoubleu.com/internal/logging"
	"tools.xdoubleu.com/internal/mailer"
	"tools.xdoubleu.com/internal/notifications"
	"tools.xdoubleu.com/internal/observability/jobs"
	"tools.xdoubleu.com/internal/repositories"
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

func (f fakeSentryClient) ListTransactionStats(
	_ context.Context,
) ([]sentryapi.TransactionStat, error) {
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

type fakeGithubClient struct {
	prs       []github.PullRequest
	err       error
	alerts    []github.SecurityAlert
	alertsErr error
}

func (f fakeGithubClient) ListFailingPullRequests(
	_ context.Context,
) ([]github.PullRequest, error) {
	return f.prs, f.err
}

func (f fakeGithubClient) ListSecurityAlerts(
	_ context.Context,
) ([]github.SecurityAlert, error) {
	return f.alerts, f.alertsErr
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

// fakeNotifiedRepo.keys is read from the test goroutine (Exists, called
// synchronously in notifyGithub's per-PR loop) and written from the
// notifications.Service background worker (Insert, via the async
// OnResult callback) whenever a test's job.Run processes more than one PR
// without a WaitUntilDone between them -- a plain map races there.
type fakeNotifiedRepo struct {
	mu        sync.Mutex
	keys      map[string]bool
	insertErr error
}

func newFakeNotifiedRepo() *fakeNotifiedRepo {
	return &fakeNotifiedRepo{mu: sync.Mutex{}, keys: map[string]bool{}, insertErr: nil}
}

func (f *fakeNotifiedRepo) Exists(_ context.Context, key string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.keys[key], nil
}

func (f *fakeNotifiedRepo) Insert(_ context.Context, key string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.insertErr != nil {
		return f.insertErr
	}
	f.keys[key] = true
	return nil
}

// alwaysEnabledSettings makes every notification source enabled, for tests
// that aren't exercising the settings gate itself.
type alwaysEnabledSettings struct{}

func (alwaysEnabledSettings) IsEnabled(
	_ context.Context,
	_ repositories.NotificationSource,
) (bool, error) {
	return true, nil
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
	mail := &fakeMailer{sent: nil, err: nil}
	notified := newFakeNotifiedRepo()
	notifSvc := testNotifications(t, mail)

	job := jobs.NewIssueNotifierJob(
		sentry,
		fakeGithubClient{prs: nil, err: nil, alerts: nil, alertsErr: nil},
		notifSvc,
		notified,
		alwaysEnabledSettings{},
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
	mail := &fakeMailer{sent: nil, err: nil}
	notified := newFakeNotifiedRepo()
	notified.keys["sentry:1"] = true
	notifSvc := testNotifications(t, mail)

	job := jobs.NewIssueNotifierJob(
		sentry,
		fakeGithubClient{prs: nil, err: nil, alerts: nil, alertsErr: nil},
		notifSvc,
		notified,
		alwaysEnabledSettings{},
	)
	require.NoError(t, job.Run(t.Context(), testLogger()))
	notifSvc.WaitUntilDone()

	assert.Empty(t, mail.sent)
}

func TestIssueNotifierMailerNotConfiguredDoesNotRecordAsNotified(t *testing.T) {
	sentry := fakeSentryClient{
		issues: []sentryapi.Issue{sentryIssue("1", "boom")}, err: nil,
	}
	mail := &fakeMailer{sent: nil, err: mailer.ErrNotConfigured}
	notified := newFakeNotifiedRepo()
	notifSvc := testNotifications(t, mail)

	job := jobs.NewIssueNotifierJob(
		sentry,
		fakeGithubClient{prs: nil, err: nil, alerts: nil, alertsErr: nil},
		notifSvc,
		notified,
		alwaysEnabledSettings{},
	)
	require.NoError(t, job.Run(t.Context(), testLogger()))
	notifSvc.WaitUntilDone()

	assert.False(t, notified.keys["sentry:1"])
}

func TestIssueNotifierLogsWarnForTransientSentryError(t *testing.T) {
	sentry := fakeSentryClient{issues: nil, err: context.DeadlineExceeded}
	mail := &fakeMailer{sent: nil, err: nil}
	notified := newFakeNotifiedRepo()
	logger, buf := testLoggerWithBuf()
	notifSvc := testNotifications(t, mail)

	job := jobs.NewIssueNotifierJob(
		sentry,
		fakeGithubClient{prs: nil, err: nil, alerts: nil, alertsErr: nil},
		notifSvc,
		notified,
		alwaysEnabledSettings{},
	)
	require.NoError(t, job.Run(t.Context(), logger))
	notifSvc.WaitUntilDone()

	assert.Contains(t, buf.String(), "level=WARN")
	assert.NotContains(t, buf.String(), "level=ERROR")
}

func TestIssueNotifierLogsErrorForNonTransientSentryError(t *testing.T) {
	sentry := fakeSentryClient{issues: nil, err: assert.AnError}
	mail := &fakeMailer{sent: nil, err: nil}
	notified := newFakeNotifiedRepo()
	logger, buf := testLoggerWithBuf()
	notifSvc := testNotifications(t, mail)

	job := jobs.NewIssueNotifierJob(
		sentry,
		fakeGithubClient{prs: nil, err: nil, alerts: nil, alertsErr: nil},
		notifSvc,
		notified,
		alwaysEnabledSettings{},
	)
	require.NoError(t, job.Run(t.Context(), logger))
	notifSvc.WaitUntilDone()

	assert.Contains(t, buf.String(), "level=ERROR")
}

func TestIssueNotifierIDAndRunEvery(t *testing.T) {
	job := jobs.NewIssueNotifierJob(
		fakeSentryClient{issues: nil, err: nil},
		fakeGithubClient{prs: nil, err: nil, alerts: nil, alertsErr: nil},
		testNotifications(t, &fakeMailer{sent: nil, err: nil}),
		newFakeNotifiedRepo(),
		alwaysEnabledSettings{},
	)
	assert.Equal(t, "notify-new-issues", job.ID())
	assert.Positive(t, job.RunEvery())
}

func TestIssueNotifierNotifiedExistsErrorPropagates(t *testing.T) {
	sentry := fakeSentryClient{
		issues: []sentryapi.Issue{sentryIssue("42", "kaboom")}, err: nil,
	}
	mail := &fakeMailer{sent: nil, err: nil}
	notified := &erroringNotifiedRepo{err: assert.AnError}
	notifSvc := testNotifications(t, mail)

	job := jobs.NewIssueNotifierJob(
		sentry,
		fakeGithubClient{prs: nil, err: nil, alerts: nil, alertsErr: nil},
		notifSvc,
		notified,
		alwaysEnabledSettings{},
	)
	err := job.Run(t.Context(), testLogger())

	require.ErrorIs(t, err, assert.AnError)
	assert.Empty(t, mail.sent)
}

func TestIssueNotifierRealSendErrorIsNotMarkedAsNotified(t *testing.T) {
	sentry := fakeSentryClient{
		issues: []sentryapi.Issue{sentryIssue("1", "boom")}, err: nil,
	}
	mail := &fakeMailer{sent: nil, err: assert.AnError}
	notified := newFakeNotifiedRepo()
	notifSvc := testNotifications(t, mail)

	job := jobs.NewIssueNotifierJob(
		sentry,
		fakeGithubClient{prs: nil, err: nil, alerts: nil, alertsErr: nil},
		notifSvc,
		notified,
		alwaysEnabledSettings{},
	)
	require.NoError(t, job.Run(t.Context(), testLogger()))
	notifSvc.WaitUntilDone()

	assert.False(t, notified.keys["sentry:1"])
}

func TestIssueNotifierSendsForFailingDependencyPR(t *testing.T) {
	sentry := fakeSentryClient{issues: nil, err: nil}
	gh := fakeGithubClient{
		prs: []github.PullRequest{failingPR("sha1", "dependencies")}, err: nil,
		alerts: nil, alertsErr: nil,
	}
	mail := &fakeMailer{sent: nil, err: nil}
	notified := newFakeNotifiedRepo()
	notifSvc := testNotifications(t, mail)

	job := jobs.NewIssueNotifierJob(
		sentry,
		gh,
		notifSvc,
		notified,
		alwaysEnabledSettings{},
	)
	require.NoError(t, job.Run(t.Context(), testLogger()))
	notifSvc.WaitUntilDone()

	assert.Len(t, mail.sent, 1)
	assert.True(t, notified.keys["github:pr:42:sha1"])
}

func TestIssueNotifierGithubNoFailingPRs(t *testing.T) {
	sentry := fakeSentryClient{issues: nil, err: nil}
	gh := fakeGithubClient{
		prs: []github.PullRequest{}, err: nil, alerts: nil, alertsErr: nil,
	}
	mail := &fakeMailer{sent: nil, err: nil}
	notified := newFakeNotifiedRepo()
	notifSvc := testNotifications(t, mail)

	job := jobs.NewIssueNotifierJob(
		sentry,
		gh,
		notifSvc,
		notified,
		alwaysEnabledSettings{},
	)
	require.NoError(t, job.Run(t.Context(), testLogger()))
	notifSvc.WaitUntilDone()

	assert.Empty(t, mail.sent)
}

func TestIssueNotifierGithubHandlesMultiplePRs(t *testing.T) {
	sentry := fakeSentryClient{issues: nil, err: nil}
	gh := fakeGithubClient{
		prs: []github.PullRequest{
			failingPRNumbered(2, "sha2", "dependencies"),
			failingPRNumbered(3, "sha3", "dependencies", "go"),
		},
		err: nil, alerts: nil, alertsErr: nil,
	}
	mail := &fakeMailer{sent: nil, err: nil}
	notified := newFakeNotifiedRepo()
	notifSvc := testNotifications(t, mail)

	job := jobs.NewIssueNotifierJob(
		sentry,
		gh,
		notifSvc,
		notified,
		alwaysEnabledSettings{},
	)
	require.NoError(t, job.Run(t.Context(), testLogger()))
	notifSvc.WaitUntilDone()

	assert.Len(t, mail.sent, 2)
	assert.True(t, notified.keys["github:pr:2:sha2"])
	assert.True(t, notified.keys["github:pr:3:sha3"])
}

func TestIssueNotifierSkipsAlreadyNotifiedDependencyPR(t *testing.T) {
	sentry := fakeSentryClient{issues: nil, err: nil}
	gh := fakeGithubClient{
		prs: []github.PullRequest{failingPR("sha1", "dependencies")}, err: nil,
		alerts: nil, alertsErr: nil,
	}
	mail := &fakeMailer{sent: nil, err: nil}
	notified := newFakeNotifiedRepo()
	notified.keys["github:pr:42:sha1"] = true
	notifSvc := testNotifications(t, mail)

	job := jobs.NewIssueNotifierJob(
		sentry,
		gh,
		notifSvc,
		notified,
		alwaysEnabledSettings{},
	)
	require.NoError(t, job.Run(t.Context(), testLogger()))
	notifSvc.WaitUntilDone()

	assert.Empty(t, mail.sent)
}

func TestIssueNotifierRenotifiesDependencyPROnNewHeadSHA(t *testing.T) {
	sentry := fakeSentryClient{issues: nil, err: nil}
	gh := fakeGithubClient{
		prs: []github.PullRequest{failingPR("sha2", "dependencies")}, err: nil,
		alerts: nil, alertsErr: nil,
	}
	mail := &fakeMailer{sent: nil, err: nil}
	notified := newFakeNotifiedRepo()
	notified.keys["github:pr:42:sha1"] = true
	notifSvc := testNotifications(t, mail)

	job := jobs.NewIssueNotifierJob(
		sentry,
		gh,
		notifSvc,
		notified,
		alwaysEnabledSettings{},
	)
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
	gh := fakeGithubClient{
		prs: []github.PullRequest{failingPR("sha1", "dependencies")}, err: nil,
		alerts: nil, alertsErr: nil,
	}
	mail := &fakeMailer{sent: nil, err: nil}
	notified := &erroringNotifiedRepo{err: assert.AnError}
	notifSvc := testNotifications(t, mail)

	job := jobs.NewIssueNotifierJob(
		sentry,
		gh,
		notifSvc,
		notified,
		alwaysEnabledSettings{},
	)
	err := job.Run(t.Context(), testLogger())

	require.ErrorIs(t, err, assert.AnError)
	assert.Empty(t, mail.sent)
}

func TestIssueNotifierGithubNotConfiguredSkipsSilently(t *testing.T) {
	sentry := fakeSentryClient{issues: nil, err: nil}
	gh := fakeGithubClient{
		prs: nil, err: github.ErrNotConfigured, alerts: nil, alertsErr: nil,
	}
	mail := &fakeMailer{sent: nil, err: nil}
	notified := newFakeNotifiedRepo()
	notifSvc := testNotifications(t, mail)

	job := jobs.NewIssueNotifierJob(
		sentry,
		gh,
		notifSvc,
		notified,
		alwaysEnabledSettings{},
	)
	require.NoError(t, job.Run(t.Context(), testLogger()))
	notifSvc.WaitUntilDone()

	assert.Empty(t, mail.sent)
}

func TestIssueNotifierLogsWarnForTransientGithubError(t *testing.T) {
	sentry := fakeSentryClient{issues: nil, err: nil}
	gh := fakeGithubClient{
		prs: nil, err: context.DeadlineExceeded, alerts: nil, alertsErr: nil,
	}
	mail := &fakeMailer{sent: nil, err: nil}
	notified := newFakeNotifiedRepo()
	logger, buf := testLoggerWithBuf()
	notifSvc := testNotifications(t, mail)

	job := jobs.NewIssueNotifierJob(
		sentry,
		gh,
		notifSvc,
		notified,
		alwaysEnabledSettings{},
	)
	require.NoError(t, job.Run(t.Context(), logger))
	notifSvc.WaitUntilDone()

	assert.Contains(t, buf.String(), "level=WARN")
	assert.NotContains(t, buf.String(), "level=ERROR")
}

func TestIssueNotifierLogsErrorForNonTransientGithubError(t *testing.T) {
	sentry := fakeSentryClient{issues: nil, err: nil}
	gh := fakeGithubClient{prs: nil, err: assert.AnError, alerts: nil, alertsErr: nil}
	mail := &fakeMailer{sent: nil, err: nil}
	notified := newFakeNotifiedRepo()
	logger, buf := testLoggerWithBuf()
	notifSvc := testNotifications(t, mail)

	job := jobs.NewIssueNotifierJob(
		sentry,
		gh,
		notifSvc,
		notified,
		alwaysEnabledSettings{},
	)
	require.NoError(t, job.Run(t.Context(), logger))
	notifSvc.WaitUntilDone()

	assert.Contains(t, buf.String(), "level=ERROR")
}

// disabledSourceSettings reports every source as disabled except those
// listed, for tests exercising the per-source settings gate.
type disabledSourceSettings struct {
	enabled map[repositories.NotificationSource]bool
}

func (d disabledSourceSettings) IsEnabled(
	_ context.Context,
	source repositories.NotificationSource,
) (bool, error) {
	return d.enabled[source], nil
}

func TestIssueNotifierSkipsSentryWhenSourceDisabled(t *testing.T) {
	sentry := fakeSentryClient{
		issues: []sentryapi.Issue{sentryIssue("1", "boom")}, err: nil,
	}
	mail := &fakeMailer{sent: nil, err: nil}
	notified := newFakeNotifiedRepo()
	notifSvc := testNotifications(t, mail)
	settings := disabledSourceSettings{
		enabled: map[repositories.NotificationSource]bool{},
	}

	gh := fakeGithubClient{prs: nil, err: nil, alerts: nil, alertsErr: nil}
	job := jobs.NewIssueNotifierJob(sentry, gh, notifSvc, notified, settings)
	require.NoError(t, job.Run(t.Context(), testLogger()))
	notifSvc.WaitUntilDone()

	assert.Empty(t, mail.sent)
	assert.False(t, notified.keys["sentry:1"])
}

func TestIssueNotifierSkipsGithubWhenSourceDisabled(t *testing.T) {
	sentry := fakeSentryClient{issues: nil, err: nil}
	gh := fakeGithubClient{
		prs: []github.PullRequest{failingPR("sha1", "dependencies")}, err: nil,
		alerts: nil, alertsErr: nil,
	}
	mail := &fakeMailer{sent: nil, err: nil}
	notified := newFakeNotifiedRepo()
	notifSvc := testNotifications(t, mail)
	//nolint:exhaustive //only sentry_issues needs to be listed as enabled here
	settings := disabledSourceSettings{
		enabled: map[repositories.NotificationSource]bool{
			repositories.NotificationSourceSentryIssues: true,
		},
	}

	job := jobs.NewIssueNotifierJob(sentry, gh, notifSvc, notified, settings)
	require.NoError(t, job.Run(t.Context(), testLogger()))
	notifSvc.WaitUntilDone()

	assert.Empty(t, mail.sent)
	assert.False(t, notified.keys["github:pr:42:sha1"])
}

func securityAlert(alertType github.SecurityAlertType) github.SecurityAlert {
	return github.SecurityAlert{
		Type:                  alertType,
		Number:                5,
		PackageName:           "",
		Ecosystem:             "",
		Severity:              "high",
		Summary:               "vulnerable dependency",
		URL:                   "https://gh/alert/" + string(alertType),
		CreatedAt:             time.Time{},
		RuleID:                "",
		FilePath:              "",
		Line:                  0,
		SecretTypeDisplayName: "",
	}
}

func TestIssueNotifierSendsForSecurityAlert(t *testing.T) {
	sentry := fakeSentryClient{issues: nil, err: nil}
	gh := fakeGithubClient{
		prs: nil, err: nil,
		alerts: []github.SecurityAlert{
			securityAlert(github.SecurityAlertTypeDependabot),
		},
		alertsErr: nil,
	}
	mail := &fakeMailer{sent: nil, err: nil}
	notified := newFakeNotifiedRepo()
	notifSvc := testNotifications(t, mail)

	job := jobs.NewIssueNotifierJob(
		sentry,
		gh,
		notifSvc,
		notified,
		alwaysEnabledSettings{},
	)
	require.NoError(t, job.Run(t.Context(), testLogger()))
	notifSvc.WaitUntilDone()

	assert.Len(t, mail.sent, 1)
	assert.True(t, notified.keys["security:dependabot:5"])
}

// TestIssueNotifierSecurityAlertDedupKeyIncludesType asserts two alerts that
// share a number but differ in type (numbers aren't unique across the three
// alert sources) both notify, since the dedup key must include the type.
func TestIssueNotifierSecurityAlertDedupKeyIncludesType(t *testing.T) {
	sentry := fakeSentryClient{issues: nil, err: nil}
	gh := fakeGithubClient{
		prs: nil, err: nil,
		alerts: []github.SecurityAlert{
			securityAlert(github.SecurityAlertTypeDependabot),
			securityAlert(github.SecurityAlertTypeCodeScanning),
		},
		alertsErr: nil,
	}
	mail := &fakeMailer{sent: nil, err: nil}
	notified := newFakeNotifiedRepo()
	notifSvc := testNotifications(t, mail)

	job := jobs.NewIssueNotifierJob(
		sentry,
		gh,
		notifSvc,
		notified,
		alwaysEnabledSettings{},
	)
	require.NoError(t, job.Run(t.Context(), testLogger()))
	notifSvc.WaitUntilDone()

	assert.Len(t, mail.sent, 2)
	assert.True(t, notified.keys["security:dependabot:5"])
	assert.True(t, notified.keys["security:code_scanning:5"])
}

func TestIssueNotifierSkipsAlreadyNotifiedSecurityAlert(t *testing.T) {
	sentry := fakeSentryClient{issues: nil, err: nil}
	gh := fakeGithubClient{
		prs: nil, err: nil,
		alerts: []github.SecurityAlert{
			securityAlert(github.SecurityAlertTypeDependabot),
		},
		alertsErr: nil,
	}
	mail := &fakeMailer{sent: nil, err: nil}
	notified := newFakeNotifiedRepo()
	notified.keys["security:dependabot:5"] = true
	notifSvc := testNotifications(t, mail)

	job := jobs.NewIssueNotifierJob(
		sentry,
		gh,
		notifSvc,
		notified,
		alwaysEnabledSettings{},
	)
	require.NoError(t, job.Run(t.Context(), testLogger()))
	notifSvc.WaitUntilDone()

	assert.Empty(t, mail.sent)
}

func TestIssueNotifierSecurityAlertsNotConfiguredSkipsSilently(t *testing.T) {
	sentry := fakeSentryClient{issues: nil, err: nil}
	gh := fakeGithubClient{
		prs: nil, err: nil, alerts: nil, alertsErr: github.ErrNotConfigured,
	}
	mail := &fakeMailer{sent: nil, err: nil}
	notified := newFakeNotifiedRepo()
	notifSvc := testNotifications(t, mail)

	job := jobs.NewIssueNotifierJob(
		sentry,
		gh,
		notifSvc,
		notified,
		alwaysEnabledSettings{},
	)
	require.NoError(t, job.Run(t.Context(), testLogger()))
	notifSvc.WaitUntilDone()

	assert.Empty(t, mail.sent)
}

// TestIssueNotifierSecurityAlertsUpstreamErrorSkipsSilently asserts a
// non-ErrNotConfigured failure from ListSecurityAlerts is logged and
// swallowed (self-heals on the next poll) rather than failing the run.
func TestIssueNotifierSecurityAlertsUpstreamErrorSkipsSilently(t *testing.T) {
	sentry := fakeSentryClient{issues: nil, err: nil}
	gh := fakeGithubClient{
		prs: nil, err: nil, alerts: nil, alertsErr: errors.New("upstream down"),
	}
	mail := &fakeMailer{sent: nil, err: nil}
	notified := newFakeNotifiedRepo()
	notifSvc := testNotifications(t, mail)

	job := jobs.NewIssueNotifierJob(
		sentry,
		gh,
		notifSvc,
		notified,
		alwaysEnabledSettings{},
	)
	require.NoError(t, job.Run(t.Context(), testLogger()))
	notifSvc.WaitUntilDone()

	assert.Empty(t, mail.sent)
}

func TestIssueNotifierSkipsSecurityAlertsWhenSourceDisabled(t *testing.T) {
	sentry := fakeSentryClient{issues: nil, err: nil}
	gh := fakeGithubClient{
		prs: nil, err: nil,
		alerts: []github.SecurityAlert{
			securityAlert(github.SecurityAlertTypeDependabot),
		},
		alertsErr: nil,
	}
	mail := &fakeMailer{sent: nil, err: nil}
	notified := newFakeNotifiedRepo()
	notifSvc := testNotifications(t, mail)
	//nolint:exhaustive //only sentry_issues/failing_dependency_prs need to be listed here
	settings := disabledSourceSettings{
		enabled: map[repositories.NotificationSource]bool{
			repositories.NotificationSourceSentryIssues:         true,
			repositories.NotificationSourceFailingDependencyPRs: true,
		},
	}

	job := jobs.NewIssueNotifierJob(sentry, gh, notifSvc, notified, settings)
	require.NoError(t, job.Run(t.Context(), testLogger()))
	notifSvc.WaitUntilDone()

	assert.Empty(t, mail.sent)
	assert.False(t, notified.keys["security:dependabot:5"])
}
