package github

import (
	"context"
	"errors"
)

// ErrNotConfigured means no GitHub token or repo is set; callers degrade to an
// empty section.
var ErrNotConfigured = errors.New("github: not configured")

// ErrAccessDenied wraps an upstream 401/403, typically a missing token scope or
// app permission; the wrapped message carries GitHub's reason.
var ErrAccessDenied = errors.New("github: access denied")

// Client is the subset of the GitHub REST API used for observability.
type Client interface {
	// ListFailingPullRequests returns open PRs with a failing check on their head.
	ListFailingPullRequests(ctx context.Context) ([]PullRequest, error)
	// ListFailingPullRequestsIn is the same for repo ("owner/name"), limited to
	// the PRs in scope.
	ListFailingPullRequestsIn(
		ctx context.Context, repo string, scope PRScope,
	) ([]PullRequest, error)
	// ListSecurityAlerts returns open Dependabot, code-scanning and secret-scanning
	// alerts.
	ListSecurityAlerts(ctx context.Context) ([]SecurityAlert, error)
	// ListRepos returns repos visible to the connection, for the config picker; it
	// works before a repo is picked.
	ListRepos(ctx context.Context) ([]Repo, error)
	// ListWorkflowRuns returns recent PR and main-push workflow runs.
	ListWorkflowRuns(ctx context.Context) ([]WorkflowRun, error)
	// ListWorkflowRunJobs returns a workflow run's jobs.
	ListWorkflowRunJobs(ctx context.Context, runID int64) ([]WorkflowJob, error)
	// GetJobLogs returns one job's full plain-text log.
	GetJobLogs(ctx context.Context, jobID int64) (string, error)
	// DismissSecurityAlert dismisses one alert; reason must be valid for alertType
	// (ErrInvalidDismissReason otherwise).
	DismissSecurityAlert(
		ctx context.Context,
		alertType SecurityAlertType,
		alertNumber int64,
		reason string,
	) error
	// ListProjectIssuesByStatus returns open issues on the owner's Projects (v2)
	// board projectNumber whose Status matches (case-insensitive).
	ListProjectIssuesByStatus(
		ctx context.Context, projectNumber int64, status string,
	) ([]ProjectIssue, error)
}
