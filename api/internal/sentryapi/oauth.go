package sentryapi

import "golang.org/x/oauth2"

// OAuthConfig builds the Sentry OAuth config used to let an admin connect
// this integration, via Sentry's Integration Platform ("public integration")
// authorization-code flow. Sentry access tokens expire (~8h) and rotate via
// the standard OAuth2 refresh grant, handled automatically by
// oauth2.Config.TokenSource.
func OAuthConfig(clientID, clientSecret, apiURL string) *oauth2.Config {
	return &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		//nolint:exhaustruct,gosec // endpoint URLs, not credentials
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://sentry.io/oauth/authorize/",
			TokenURL: "https://sentry.io/oauth/token/",
		},
		RedirectURL: apiURL + "/admin/oauth/sentry/callback",
		Scopes:      []string{"project:read", "event:read"},
	}
}
