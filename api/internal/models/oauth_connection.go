package models

import "time"

// OAuthProvider identifies which external service an OAuth connection
// belongs to.
type OAuthProvider string

const (
	OAuthProviderGithub       OAuthProvider = "github"
	OAuthProviderSentry       OAuthProvider = "sentry"
	OAuthProviderDigitalOcean OAuthProvider = "digitalocean"
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
}
