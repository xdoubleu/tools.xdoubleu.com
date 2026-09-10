package jobs_test

import (
	"bytes"
	"context"
	"log/slog"
	"testing"
	"time"

	"tools.xdoubleu.com/internal/github"
	"tools.xdoubleu.com/internal/logging"
	"tools.xdoubleu.com/internal/models"
	"tools.xdoubleu.com/internal/notifications"
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

func sentryIssue() sentryapi.Issue {
	return sentryapi.Issue{
		ID:        "1",
		Title:     "boom",
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

func failingPR(labels ...string) github.PullRequest {
	return github.PullRequest{
		Number:    42,
		Title:     "Bump some-dep from 1.0.0 to 1.0.1",
		URL:       "https://gh/pr/sha1",
		Author:    "renovate[bot]",
		UpdatedAt: time.Time{},
		HeadSHA:   "sha1",
		Labels:    labels,
		FailingChecks: []github.FailingCheck{
			{Name: "ci-pass", Conclusion: "failure", URL: ""},
		},
	}
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

// fakeMailer records both halves of every send: sent holds subjects,
// bodies the matching body at the same index. WeeklyDigestJob sends two
// separate emails per run, and only the bodies distinguish which content
// landed in which — a subject-only fake made assertions about that
// vacuously pass.
type fakeMailer struct {
	sent   []string
	bodies []string
	err    error
}

func (f *fakeMailer) Send(_ context.Context, subject, body string) error {
	if f.err != nil {
		return f.err
	}
	f.sent = append(f.sent, subject)
	f.bodies = append(f.bodies, body)
	return nil
}

func (f *fakeMailer) SendTo(_ context.Context, _, _, _ string) error {
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

// disabledSourceSettings reports enabled state from an explicit per-source
// map, for tests exercising the settings gate.
type disabledSourceSettings struct {
	enabled map[repositories.NotificationSource]bool
}

func (d disabledSourceSettings) IsEnabled(
	_ context.Context,
	source repositories.NotificationSource,
) (bool, error) {
	return d.enabled[source], nil
}

// fakeSlowTransactionsRepo is a test double for
// *repositories.TransactionLatencyRepository's Trends method.
type fakeSlowTransactionsRepo struct {
	trends []models.TransactionTrend
	err    error
}

func (f fakeSlowTransactionsRepo) Trends(
	_ context.Context,
) ([]models.TransactionTrend, error) {
	return f.trends, f.err
}

// noSlowTransactions mirrors an empty regression list, used by every test
// not exercising slowTransactionsSection itself.
//
//nolint:gochecknoglobals // read-only test fixture, mirrors alwaysEnabledSettings{}
var noSlowTransactions = fakeSlowTransactionsRepo{trends: nil, err: nil}

func slowTrend(
	project, transaction string,
	recentP95Ms float64,
) models.TransactionTrend {
	return models.TransactionTrend{
		Transaction:    transaction,
		Project:        project,
		PriorAvgP95Ms:  1000,
		RecentAvgP95Ms: recentP95Ms,
		PctChange:      (recentP95Ms - 1000) / 1000,
	}
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
// enqueue onto; deliveries happen on a background worker (issue #923), so
// tests must call WaitUntilDone before asserting on mail state.
func testNotifications(t *testing.T, mail *fakeMailer) *notifications.Service {
	t.Helper()
	return notifications.NewEmailOnly(t.Context(), logging.NewNopLogger(), mail)
}
