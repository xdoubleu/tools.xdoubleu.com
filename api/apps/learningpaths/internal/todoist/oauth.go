package todoist

import (
	"strings"

	"golang.org/x/oauth2"
)

// RequiredScopes are what the reminder pipeline needs: read/write tasks and
// projects (includes task:add), delete tasks, delete a path's project.
func RequiredScopes() []string {
	return []string{"data:read_write", "data:delete", "project:delete"}
}

// ScopeParam is scopes as Todoist's authorize endpoint expects them:
// comma-separated, where golang.org/x/oauth2 joins with spaces.
func ScopeParam(scopes []string) string {
	return strings.Join(scopes, ",")
}

// OAuthConfig builds the Todoist OAuth2 config. Tokens are per user, stored in
// learningpaths.oauth_connections. Targets API v1 (/api/v1/); rate limit is
// 1000 requests / 15 min per token. The authorize endpoint is app.todoist.com;
// build its URL with ScopeParam.
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
		Scopes:      RequiredScopes(),
	}
}
