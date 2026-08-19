package oauth2as

import (
	"time"

	"github.com/ory/fosite"
	"github.com/ory/fosite/compose"

	"tools.xdoubleu.com/internal/config"
)

const (
	accessTokenLifespan   = time.Hour
	refreshTokenLifespan  = 30 * 24 * time.Hour
	authorizeCodeLifespan = 10 * time.Minute
)

// NewProvider builds the embedded OAuth 2.1 authorization server (issue
// #1039): authorization code grant + refresh token grant, PKCE required on
// every client (public-client-only — this server never issues confidential
// client secrets).
func NewProvider(cfg config.Config, store fosite.Storage) fosite.OAuth2Provider {
	//nolint:exhaustruct //remaining Config fields use library defaults
	fc := &fosite.Config{
		GlobalSecret:                   []byte(cfg.OAuthHMACSecret),
		AccessTokenLifespan:            accessTokenLifespan,
		RefreshTokenLifespan:           refreshTokenLifespan,
		AuthorizeCodeLifespan:          authorizeCodeLifespan,
		EnforcePKCE:                    true,
		EnablePKCEPlainChallengeMethod: false,
	}

	strategy := compose.NewOAuth2HMACStrategy(fc)

	return compose.Compose(
		fc,
		store,
		strategy,
		compose.OAuth2AuthorizeExplicitFactory,
		compose.OAuth2RefreshTokenGrantFactory,
		compose.OAuth2PKCEFactory,
	)
}
