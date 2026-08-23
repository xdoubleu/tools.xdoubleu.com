package models

import (
	"encoding/json"
	"errors"
	"time"
)

// ErrDecryptFailed means a stored OAuth token could not be decrypted with the
// currently configured ENCRYPTION_KEY (e.g. the key was rotated after the
// connection was authorized). The connection must be reconnected — there is
// no way to recover the existing encrypted bytes.
var ErrDecryptFailed = errors.New(
	"models: stored oauth token could not be decrypted",
)

// OAuthProvider identifies which external service an OAuth connection
// belongs to.
type OAuthProvider string

const (
	OAuthProviderGithub OAuthProvider = "github"
	OAuthProviderSentry OAuthProvider = "sentry"
)

// OAuthConnection is the admin-facing status of a provider's stored OAuth
// connection (global.oauth_connections). It never carries the raw token —
// that stays encrypted at rest and is only handled inside the repository.
type OAuthConnection struct {
	Provider    OAuthProvider
	ConnectedBy string
	ConnectedAt time.Time
	UpdatedAt   time.Time
	ExpiresAt   *time.Time // nil = non-expiring or unknown
	// Config is the admin-picked provider-specific identifier(s), stored as
	// opaque JSON — nil means "connected but not yet configured". Parsing
	// into a provider-specific shape happens at the client/handler layer.
	Config json.RawMessage
	// GrantedScope is the `scope` the provider returned with the token (empty
	// if the provider didn't echo one back). It is the provider's own
	// normalized view, not a faithful echo of what was asked for — GitHub
	// drops scopes a broader one already subsumes, returning just `repo` for
	// a `repo security_events` authorization — so it is diagnostic only and
	// must never decide whether a connection covers what a provider needs.
	GrantedScope string
	// RequestedScope is the space-separated set of scopes this connection was
	// authorized with, recorded from oauth2.Config.Scopes at connect time.
	// Compared against a provider's currently configured scopes to detect a
	// connection authorized before a required scope was added. Empty for rows
	// written before it was recorded; those fall back to GrantedScope.
	RequestedScope string
}
