package github

import "golang.org/x/oauth2"

// OAuthConfig builds the GitHub OAuth App config. Classic tokens usually never
// expire; TokenSource handles either case without special-casing.
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
		// read:project lets ListProjectIssuesByStatus read a user-owned Projects (v2)
		// board's Status field over GraphQL.
		Scopes: []string{"repo", "security_events", "read:project"},
	}
}
