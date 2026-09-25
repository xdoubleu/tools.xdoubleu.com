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

// NewProvider builds the embedded OAuth 2.1 / OIDC server: authorization code
// and refresh grants, PKCE required, RS256 ID tokens for openid clients.
// Access tokens are opaque HMAC tokens; oidcKey signs only ID tokens.
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
		// OIDC must come after the authorize-code handler: it adds an ID token to the
		// code that handler issued.
		compose.OpenIDConnectExplicitFactory,
		compose.OpenIDConnectRefreshFactory,
		// Required: without it IntrospectToken always fails, breaking
		// ResolveAccessToken.
		compose.OAuth2TokenIntrospectionFactory,
	)
}
