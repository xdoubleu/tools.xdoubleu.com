package github

import (
	"context"
	"errors"
)

// ErrNotConfigured is returned when the GitHub token or repository is unset.
// Callers treat it as a degraded (not failed) state — the observability
// handlers return an empty section instead of an error.
var ErrNotConfigured = errors.New("github: not configured")

// Client is the subset of the GitHub REST API used for observability: the list
// of open issues on the configured repository.
type Client interface {
	// ListOpenIssues returns the open issues (pull requests excluded) of the
	// configured repository. Returns ErrNotConfigured when no token/repo is set.
	ListOpenIssues(ctx context.Context) ([]Issue, error)
}
