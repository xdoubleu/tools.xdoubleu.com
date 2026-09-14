// Package anthropicadmin is a thin client for the Anthropic Admin API's
// Claude Code Analytics endpoint (issue #1591). It authenticates with a
// single, rarely-rotated Admin API key (config.AnthropicAdminAPIKey) rather
// than the per-admin OAuth connections internal/github and internal/sentryapi
// use — there is exactly one Anthropic organization this codebase reports
// usage for, not a per-admin picked resource.
package anthropicadmin

import (
	"context"
	"errors"
)

// ErrNotConfigured is returned when no Admin API key is set. Callers treat
// it as a degraded (not failed) state — the collector job skips its update
// instead of erroring, mirroring internal/github.ErrNotConfigured and
// internal/sentryapi.ErrNotConfigured.
var ErrNotConfigured = errors.New("anthropicadmin: not configured")

// Client is the subset of the Anthropic Admin API used for observability:
// the Claude Code Analytics usage report.
type Client interface {
	// GetClaudeCodeUsage returns every Claude Code usage record (one per
	// actor, model-breakdown nested inside) for the single UTC day date
	// falls on, following pagination to completion. Returns
	// ErrNotConfigured when no Admin API key is set.
	GetClaudeCodeUsage(ctx context.Context, date string) ([]UsageRecord, error)
}
