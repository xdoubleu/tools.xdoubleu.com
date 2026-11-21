package oauth2as

import (
	"crypto/rsa"

	fosite "authelia.com/provider/oauth2"
)

// Wiring bundles the embedded OAuth 2.1 / OIDC authorization server.
type Wiring struct {
	Store    *Store
	Provider fosite.Provider
	// OIDCKey signs ID tokens; its public half is served at JWKSPath.
	OIDCKey *rsa.PrivateKey
}

// OIDCKeyID is the key ID of OIDCKey.
func (w *Wiring) OIDCKeyID() string {
	return OIDCKeyID(w.OIDCKey)
}

// Routes the authorization server is served on.
const (
	AuthorizePath = "/oauth2/authorize"
	//nolint:gosec // route path, not a credential
	TokenPath       = "/oauth2/token"
	RegisterPath    = "/oauth2/register"
	ConsentInfoPath = "/oauth2/consent-info"

	JWKSPath                = "/oauth2/jwks"
	MetadataPath            = "/.well-known/oauth-authorization-server"
	OpenIDConfigurationPath = "/.well-known/openid-configuration"
)
