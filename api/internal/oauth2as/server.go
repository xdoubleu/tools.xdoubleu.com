package oauth2as

import (
	"crypto/rsa"
	"time"

	fosite "authelia.com/provider/oauth2"
	"authelia.com/provider/oauth2/compose"
	hoauth2 "authelia.com/provider/oauth2/handler/oauth2"
	"authelia.com/provider/oauth2/token/jose"
	"authelia.com/provider/oauth2/token/jwt"

	"tools.xdoubleu.com/internal/config"
)

const (
	accessTokenLifespan   = time.Hour
	refreshTokenLifespan  = 30 * 24 * time.Hour
	authorizeCodeLifespan = 10 * time.Minute
	idTokenLifespan       = time.Hour
)

// hmacTokenPrefix keeps the "ory_at_"/"ory_rt_"/"ory_ac_" prefixes so tokens
// issued before the switch from ory/fosite still validate.
const hmacTokenPrefix = "ory_%s_"

// NewProvider builds the embedded OAuth 2.1 / OIDC server: authorization code
// and refresh grants, PKCE required, RS256 ID tokens for openid clients, and
// client_credentials for confidential clients registered with that grant.
// Access tokens are opaque HMAC tokens; oidcKey signs only ID tokens.
func NewProvider(
	cfg config.Config,
	store fosite.Storage,
	oidcKey *rsa.PrivateKey,
) fosite.Provider {
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

	signer := &jwt.DefaultStrategy{
		Config: fc,
		Issuer: jwt.NewDefaultIssuerUnverifiedFromJWKS(&jose.JSONWebKeySet{
			//nolint:exhaustruct //only the private key and its identity are needed
			Keys: []jose.JSONWebKey{{
				Key:       oidcKey,
				KeyID:     OIDCKeyID(oidcKey),
				Algorithm: string(jose.RS256),
				Use:       jwt.JSONWebTokenUseSignature,
			}},
		}),
	}

	strategy := &compose.CommonStrategy{
		CoreStrategy:               hoauth2.NewHMACCoreStrategy(fc, hmacTokenPrefix),
		OpenIDConnectTokenStrategy: compose.NewOpenIDConnectStrategy(nil, signer, fc),
		Strategy:                   signer,
	}

	return compose.Compose(
		fc,
		store,
		strategy,
		compose.OAuth2AuthorizeExplicitFactory,
		compose.OAuth2RefreshTokenGrantFactory,
		compose.OAuth2ClientCredentialsGrantFactory,
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
