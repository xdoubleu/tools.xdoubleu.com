package todoist

import "golang.org/x/oauth2"

// OAuthConfig builds the Todoist OAuth2 config. Tokens are per user, stored in
// learningpaths.oauth_connections. Targets API v1 (/api/v1/); rate limit is
// 1000 requests / 15 min per token. The authorize endpoint is app.todoist.com
// and expects comma-separated scopes (moot with a single scope).
func OAuthConfig(clientID, clientSecret, apiURL string) *oauth2.Config {
	return &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		//nolint:exhaustruct,gosec // endpoint URLs, not credentials
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://app.todoist.com/oauth/authorize",
			TokenURL: "https://api.todoist.com/oauth/access_token",
		},
		RedirectURL: apiURL + "/learningpaths/oauth/todoist/callback",
		// task:add is create-only.
		Scopes: []string{"task:add"},
	}
}
