package sentryapi

import "golang.org/x/oauth2"

// OAuthConfig builds the Sentry Integration Platform OAuth config. Tokens
// expire (~8h) and are refreshed by TokenSource.
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
		// org:read lists orgs for the picker; event:write lets ResolveIssue PUT
		// /api/0/issues/{id}/.
		Scopes: []string{"org:read", "project:read", "event:read", "event:write"},
	}
}
