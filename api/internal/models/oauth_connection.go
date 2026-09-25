package models

import (
	"encoding/json"
	"errors"
	"time"
)

// ErrDecryptFailed means the stored token can't be decrypted with the current
// ENCRYPTION_KEY; the connection must be reconnected.
var ErrDecryptFailed = errors.New(
	"models: stored oauth token could not be decrypted",
)

// OAuthProvider identifies a connection's external service.
type OAuthProvider string

const (
	OAuthProviderGithub OAuthProvider = "github"
	OAuthProviderSentry OAuthProvider = "sentry"
	// OAuthProviderTodoist is stored per user in learningpaths.oauth_connections;
	// it exists so that repository can reuse oauthconn.
	OAuthProviderTodoist OAuthProvider = "todoist"
)

// OAuthConnection is a provider connection's admin-facing status; it never
// carries the raw token.
type OAuthConnection struct {
	Provider    OAuthProvider
	ConnectedBy string
	ConnectedAt time.Time
	UpdatedAt   time.Time
	ExpiresAt   *time.Time // nil = non-expiring or unknown
	// Config is the admin-picked provider config as opaque JSON; nil means not
	// configured yet.
	Config json.RawMessage
	// GrantedScope is the provider's normalized echo of the scope (GitHub drops
	// subsumed scopes). Diagnostic only; never use it to decide coverage.
	GrantedScope string
	// RequestedScope is the scope requested at connect time, compared against the
	// provider's current scopes. Empty on old rows, which fall back to
	// GrantedScope.
	RequestedScope string
}
