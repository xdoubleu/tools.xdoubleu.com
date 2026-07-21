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
		// org:read is required by GET /api/0/organizations/, which the admin
		// config picker calls first to list orgs (project:read/event:read
		// cover the projects and issues endpoints).
		Scopes: []string{"org:read", "project:read", "event:read"},
	}
}
