package oauth2as

import (
	"context"
	"crypto/rsa"
	"time"

	"github.com/ory/fosite"
	"github.com/ory/fosite/compose"
	"github.com/ory/fosite/token/jwt"

	"tools.xdoubleu.com/internal/config"
)

const (
	accessTokenLifespan   = time.Hour
	refreshTokenLifespan  = 30 * 24 * time.Hour
	authorizeCodeLifespan = 10 * time.Minute
	idTokenLifespan       = time.Hour
)

// NewProvider builds the embedded OAuth 2.1 / OpenID Connect authorization
// server (issues #1039, #1469): authorization code grant + refresh token
// grant, PKCE required on every client, plus OIDC ID tokens (RS256) for
// clients that request the openid scope. Public clients authenticate with
// PKCE alone; confidential clients (the Grafana SSO client) additionally
// present a client_secret.
//
// oidcKey signs ID tokens; its public half is served at /oauth2/jwks. Access
// tokens remain opaque HMAC tokens introspected via ResolveAccessToken — only
// the ID token is asymmetric.
func NewProvider(
	cfg config.Config,
	store fosite.Storage,
	oidcKey *rsa.PrivateKey,
) fosite.OAuth2Provider {
	//nolint:exhaustruct //remaining Config fields use library defaults
	fc := &fosite.Config{
		GlobalSecret:                   []byte(cfg.OAuthHMACSecret),
		AccessTokenLifespan:            accessTokenLifespan,
		RefreshTokenLifespan:           refreshTokenLifespan,
		AuthorizeCodeLifespan:          authorizeCodeLifespan,
		IDTokenLifespan:                idTokenLifespan,
		IDTokenIssuer:                  cfg.AuthIssuer,
		EnforcePKCE:                    true,
		EnablePKCEPlainChallengeMethod: false,
	}

	keyGetter := func(context.Context) (any, error) { return oidcKey, nil }

	strategy := &compose.CommonStrategy{
		CoreStrategy:               compose.NewOAuth2HMACStrategy(fc),
		OpenIDConnectTokenStrategy: compose.NewOpenIDConnectStrategy(keyGetter, fc),
		Signer:                     &jwt.DefaultSigner{GetPrivateKey: keyGetter},
	}

	return compose.Compose(
		fc,
		store,
		strategy,
		compose.OAuth2AuthorizeExplicitFactory,
		compose.OAuth2RefreshTokenGrantFactory,
		compose.OAuth2PKCEFactory,
		// OIDC handlers must be composed after the OAuth2 authorize-code
		// handler above: they run on the same authorize/token requests and
		// layer an ID token on top of the code the core handler issued.
		compose.OpenIDConnectExplicitFactory,
		compose.OpenIDConnectRefreshFactory,
		// Without this, IntrospectToken has no registered validation
		// strategy and every call fails with ErrRequestUnauthorized ("no
		// suitable validation strategy") regardless of the token's
		// validity — silently breaking resolver.go's ResolveAccessToken,
		// the only thing that lets this api verify a bearer token it
		// issued itself as an OAuth 2.1 resource server.
		compose.OAuth2TokenIntrospectionFactory,
	)
}
