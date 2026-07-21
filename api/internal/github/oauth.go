package github

import "golang.org/x/oauth2"

// OAuthConfig builds the GitHub OAuth App config used to let an admin connect
// this integration. GitHub's classic OAuth App tokens don't expire (no
// refresh_token/expires_in in the token response) unless the owning org has
// enabled "expire user authorization tokens" — either way,
// oauth2.Config.TokenSource handles it correctly with no special-casing:
// zero Expiry means "never expires", and a returned refresh_token is honored
// automatically via the standard OAuth2 refresh grant against TokenURL.
func OAuthConfig(clientID, clientSecret, apiURL string) *oauth2.Config {
	return &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		//nolint:exhaustruct,gosec // endpoint URLs, not credentials
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://github.com/login/oauth/authorize",
			TokenURL: "https://github.com/login/oauth/access_token",
		},
		RedirectURL: apiURL + "/admin/oauth/github/callback",
		Scopes:      []string{"repo"},
	}
}
