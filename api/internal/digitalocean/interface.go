package digitalocean

import (
	"context"
	"errors"
)

// ErrNotConfigured is returned when the DigitalOcean access token or app ID is
// unset. Callers treat it as a degraded (not failed) state — the observability
// handlers return an empty section instead of an error.
var ErrNotConfigured = errors.New("digitalocean: not configured")

// Client is the subset of the DigitalOcean App Platform API used for
// observability: the latest deployment's phase and health.
type Client interface {
	// LatestDeployment returns the most recent deployment of the configured
	// app, or nil when the app has no deployments yet. Returns ErrNotConfigured
	// when the token/app ID is unset.
	LatestDeployment(ctx context.Context) (*Deployment, error)
	// ListApps returns the apps visible to the connected account, for the
	// admin config picker. Returns oauthconn.ErrNotConnected when no token is
	// set — discovery must work before an app ID is picked.
	ListApps(ctx context.Context) ([]App, error)
	// DeploymentLogs returns the BUILD, DEPLOY, RUN, and RUN_RESTARTED log
	// text for every service component of the given deployment, so both a
	// failed deploy and the component's runtime behavior can be analyzed.
	// When the requested deployment is not the one currently serving traffic,
	// the app's active deployment's runtime logs are appended too — that is
	// the only place a failed deploy's "what is the app actually doing"
	// answer lives. tailLines bounds the live backlog replayed per component;
	// 0 picks the default. A component/type pair with no logs is omitted
	// rather than erroring. Returns ErrNotConfigured when the token/app ID is
	// unset.
	DeploymentLogs(
		ctx context.Context, deploymentID string, tailLines int,
	) ([]ComponentLog, error)
	// DeploymentLogsStream is DeploymentLogs' incremental counterpart: yield is
	// called once per component/type pair as soon as it resolves, instead of
	// the caller waiting for every component to finish before seeing anything.
	// Used by the GetDeployLogs Connect handler so the first byte reaches the
	// client well before DigitalOcean App Platform's edge request timeout,
	// regardless of how long the slowest component takes.
	DeploymentLogsStream(
		ctx context.Context,
		deploymentID string,
		tailLines int,
		yield func(ComponentLog) error,
	) error
}
