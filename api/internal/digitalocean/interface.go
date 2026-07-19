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
}
