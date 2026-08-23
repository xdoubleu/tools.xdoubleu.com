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
	// ListFailingMainRuns returns the repo's completed GitHub Actions runs on
	// main that have a non-passing conclusion. Returns ErrNotConfigured when
	// no token/repo is set.
	ListFailingMainRuns(ctx context.Context) ([]WorkflowRun, error)
	// ListRepos returns the repositories visible to the connected account,
	// for the admin config picker. Returns oauthconn.ErrNotConnected when no
	// token is set — discovery must work before a repo is picked.
	ListRepos(ctx context.Context) ([]Repo, error)
}
