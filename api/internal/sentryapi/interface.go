package sentryapi

import (
	"context"
	"errors"
)

// ErrNotConfigured means the Sentry org, project or token is unset; callers
// degrade to an empty section.
var ErrNotConfigured = errors.New("sentryapi: not configured")

// ErrReauthRequired means the connection is configured but its scope is stale;
// an admin must reconnect.
var ErrReauthRequired = errors.New(
	"sentryapi: sentry connection needs to be reauthorized (missing a required scope)",
)

// Client is the subset of the Sentry REST API used for observability.
type Client interface {
	// ListUnresolvedIssues returns the configured projects' unresolved issues.
	ListUnresolvedIssues(ctx context.Context) ([]Issue, error)
	// ResolveIssue resolves an issue by Issue.ID.
	ResolveIssue(ctx context.Context, issueID string) error
	// ListOrgs lists orgs for the picker; works before anything is picked.
	ListOrgs(ctx context.Context) ([]Org, error)
	// ListProjects lists org's projects for the picker.
	ListProjects(ctx context.Context, org string) ([]Project, error)
	// ListTransactionStats returns 24h p95 and count per transaction on the
	// configured projects.
	ListTransactionStats(ctx context.Context) ([]TransactionStat, error)
}
