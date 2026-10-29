package obs

import (
	"golang.org/x/oauth2"

	"tools.xdoubleu.com/internal/config"
	"tools.xdoubleu.com/internal/github"
	"tools.xdoubleu.com/internal/models"
	"tools.xdoubleu.com/internal/sentryapi"
)

// oauthProviderDef maps a provider name to its config builder.
type oauthProviderDef struct {
	provider models.OAuthProvider
	conf     func(cfg config.Config) *oauth2.Config
}

//nolint:gochecknoglobals // fixed provider table, not runtime-configurable
var oauthProviders = map[string]oauthProviderDef{
	"github": {
		provider: models.OAuthProviderGithub,
		conf:     oauthConfigForGithub,
	},
	"sentry": {
		provider: models.OAuthProviderSentry,
		conf:     oauthConfigForSentry,
	},
}

func oauthConfigForGithub(cfg config.Config) *oauth2.Config {
	return github.OAuthConfig(
		cfg.GithubOAuthClientID,
		cfg.GithubOAuthClientSecret,
		cfg.APIURL,
	)
}

func oauthConfigForSentry(cfg config.Config) *oauth2.Config {
	return sentryapi.OAuthConfig(
		cfg.SentryOAuthClientID,
		cfg.SentryOAuthClientSecret,
		cfg.APIURL,
	)
}

// oauthProviderConfig resolves name to its provider and a config built from
// cfg. ok is false for an unknown provider.
func oauthProviderConfig(
	name string, cfg config.Config,
) (oauthProviderDef, bool) {
	def, ok := oauthProviders[name]
	if ok {
		return oauthProviderDef{provider: def.provider, conf: def.conf}, true
	}
	return oauthProviderDef{}, false
}

// oauthProviderFor resolves name to the provider enum, or nil.
func oauthProviderFor(name string) (models.OAuthProvider, bool) {
	def, ok := oauthProviders[name]
	return def.provider, ok
}