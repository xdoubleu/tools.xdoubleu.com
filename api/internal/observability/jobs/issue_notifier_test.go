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
	"tools.xdoubleu.com/internal/mailer"
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

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))
}

func TestIssueNotifierSendsForNewSentryIssue(t *testing.T) {
	sentry := fakeSentryClient{
		issues: []sentryapi.Issue{sentryIssue("1", "boom")}, err: nil,
	}
	do := fakeDOClient{deployment: nil, err: nil}
	mail := &fakeMailer{sent: nil, err: nil}
	notified := newFakeNotifiedRepo()

	job := jobs.NewIssueNotifierJob(sentry, do, mail, notified)
	require.NoError(t, job.Run(t.Context(), testLogger()))

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

	job := jobs.NewIssueNotifierJob(sentry, do, mail, notified)
	require.NoError(t, job.Run(t.Context(), testLogger()))

	assert.Empty(t, mail.sent)
}

func TestIssueNotifierSentryNotConfiguredDoesNotBlockDO(t *testing.T) {
	sentry := fakeSentryClient{issues: nil, err: sentryapi.ErrNotConfigured}
	do := fakeDOClient{deployment: doDeployment("d1", "ERROR"), err: nil}
	mail := &fakeMailer{sent: nil, err: nil}
	notified := newFakeNotifiedRepo()

	job := jobs.NewIssueNotifierJob(sentry, do, mail, notified)
	require.NoError(t, job.Run(t.Context(), testLogger()))

	assert.Len(t, mail.sent, 1)
	assert.True(t, notified.keys["digitalocean:d1"])
}

func TestIssueNotifierDOIgnoresNonErrorPhase(t *testing.T) {
	sentry := fakeSentryClient{issues: nil, err: nil}
	do := fakeDOClient{deployment: doDeployment("d1", "ACTIVE"), err: nil}
	mail := &fakeMailer{sent: nil, err: nil}
	notified := newFakeNotifiedRepo()

	job := jobs.NewIssueNotifierJob(sentry, do, mail, notified)
	require.NoError(t, job.Run(t.Context(), testLogger()))

	assert.Empty(t, mail.sent)
}

func TestIssueNotifierDONotConfiguredSkipsSilently(t *testing.T) {
	sentry := fakeSentryClient{issues: nil, err: nil}
	do := fakeDOClient{deployment: nil, err: digitalocean.ErrNotConfigured}
	mail := &fakeMailer{sent: nil, err: nil}
	notified := newFakeNotifiedRepo()

	job := jobs.NewIssueNotifierJob(sentry, do, mail, notified)
	require.NoError(t, job.Run(t.Context(), testLogger()))

	assert.Empty(t, mail.sent)
}

func TestIssueNotifierMailerNotConfiguredDoesNotRecordAsNotified(t *testing.T) {
	sentry := fakeSentryClient{
		issues: []sentryapi.Issue{sentryIssue("1", "boom")}, err: nil,
	}
	do := fakeDOClient{deployment: nil, err: nil}
	mail := &fakeMailer{sent: nil, err: mailer.ErrNotConfigured}
	notified := newFakeNotifiedRepo()

	job := jobs.NewIssueNotifierJob(sentry, do, mail, notified)
	require.NoError(t, job.Run(t.Context(), testLogger()))

	assert.False(t, notified.keys["sentry:1"])
}

func TestIssueNotifierIDAndRunEvery(t *testing.T) {
	job := jobs.NewIssueNotifierJob(
		fakeSentryClient{issues: nil, err: nil},
		fakeDOClient{deployment: nil, err: nil},
		&fakeMailer{sent: nil, err: nil},
		newFakeNotifiedRepo(),
	)
	assert.Equal(t, "notify-new-issues", job.ID())
	assert.Positive(t, job.RunEvery())
}
