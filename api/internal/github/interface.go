package github

import (
	"context"
	"errors"
)

// ErrNotConfigured is returned when the GitHub token or repository is unset.
// Callers treat it as a degraded (not failed) state — the observability
// handlers return an empty section instead of an error.
var ErrNotConfigured = errors.New("github: not configured")

// Client is the subset of the GitHub REST API used for observability: the
// failing pull requests and open Dependabot/code-scanning/secret-scanning
// security alerts on the configured repository.
type Client interface {
	// ListFailingPullRequests returns the open pull requests that have at
	// least one failing CI check run on their head commit. Returns
	// ErrNotConfigured when no token/repo is set.
	ListFailingPullRequests(ctx context.Context) ([]PullRequest, error)
	// ListSecurityAlerts returns the repo's open Dependabot, code-scanning,
	// and secret-scanning alerts. Returns ErrNotConfigured when no
	// token/repo is set.
	ListSecurityAlerts(ctx context.Context) ([]SecurityAlert, error)
	// ListRepos returns the repositories visible to the connected account,
	// for the admin config picker. Returns oauthconn.ErrNotConnected when no
	// token is set — discovery must work before a repo is picked.
	ListRepos(ctx context.Context) ([]Repo, error)
	// ListWorkflowRuns returns the most recent pull-request and push (main
	// branch) GitHub Actions workflow runs on the configured repository.
	// Returns ErrNotConfigured when no token/repo is set.
	ListWorkflowRuns(ctx context.Context) ([]WorkflowRun, error)
	// ListWorkflowRunJobs returns the per-job breakdown of a single workflow
	// run. Returns ErrNotConfigured when no token/repo is set.
	ListWorkflowRunJobs(ctx context.Context, runID int64) ([]WorkflowJob, error)
}
