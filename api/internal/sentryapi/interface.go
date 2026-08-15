package sentryapi

import (
	"context"
	"errors"
)

// ErrNotConfigured is returned when the Sentry org, project or auth token is
// unset. Callers treat it as a degraded (not failed) state — the observability
// handlers return an empty section instead of an error.
var ErrNotConfigured = errors.New("sentryapi: not configured")

// ErrReauthRequired is returned by ResolveIssue when the connection is
// already configured (org/projects picked) but the stored token's granted
// scope no longer covers what's required — i.e. an admin needs to
// reauthorize via the existing Connect flow, not "not connected" at all.
var ErrReauthRequired = errors.New(
	"sentryapi: sentry connection needs to be reauthorized (missing a required scope)",
)

// Client is the subset of the Sentry REST API used for observability: the list
// of unresolved issues on the configured project.
type Client interface {
	// ListUnresolvedIssues returns the unresolved issues of the configured
	// project. Returns ErrNotConfigured when org/project/token is unset.
	ListUnresolvedIssues(ctx context.Context) ([]Issue, error)
	// ResolveIssue marks the given Sentry issue (by its Issue.ID) as
	// resolved. Returns ErrNotConfigured when org/project/token is unset, or
	// ErrReauthRequired when configured but the granted OAuth scope is stale.
	ResolveIssue(ctx context.Context, issueID string) error
	// ListOrgs returns the organizations visible to the connected account,
	// for the admin config picker. Returns oauthconn.ErrNotConnected when no
	// token is set — discovery must work before an org/project is picked.
	ListOrgs(ctx context.Context) ([]Org, error)
	// ListProjects returns the projects within org visible to the connected
	// account, for the admin config picker.
	ListProjects(ctx context.Context, org string) ([]Project, error)
	// ListTransactionStats returns p95 duration + request count over the
	// last 24h for transactions (API endpoints/frontend pages) on the
	// configured projects, sourced from Sentry's org-level Events
	// (Discover) API. Returns ErrNotConfigured when org/project/token is
	// unset.
	ListTransactionStats(ctx context.Context) ([]TransactionStat, error)
}
