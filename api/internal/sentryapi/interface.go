package sentryapi

import (
	"context"
	"errors"
)

// ErrNotConfigured is returned when the Sentry org, project or auth token is
// unset. Callers treat it as a degraded (not failed) state — the observability
// handlers return an empty section instead of an error.
var ErrNotConfigured = errors.New("sentryapi: not configured")

// Client is the subset of the Sentry REST API used for observability: the list
// of unresolved issues on the configured project.
type Client interface {
	// ListUnresolvedIssues returns the unresolved issues of the configured
	// project. Returns ErrNotConfigured when org/project/token is unset.
	ListUnresolvedIssues(ctx context.Context) ([]Issue, error)
	// ListOrgs returns the organizations visible to the connected account,
	// for the admin config picker. Returns oauthconn.ErrNotConnected when no
	// token is set — discovery must work before an org/project is picked.
	ListOrgs(ctx context.Context) ([]Org, error)
	// ListProjects returns the projects within org visible to the connected
	// account, for the admin config picker.
	ListProjects(ctx context.Context, org string) ([]Project, error)
}
