package digitalocean

import "golang.org/x/oauth2"

// OAuthConfig builds the DigitalOcean OAuth config used to let an admin
// connect this integration. DO access tokens expire (~30d) and rotate via
// the standard OAuth2 refresh grant, handled automatically by
// oauth2.Config.TokenSource.
func OAuthConfig(clientID, clientSecret, apiURL string) *oauth2.Config {
	return &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		//nolint:exhaustruct,gosec // endpoint URLs, not credentials
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://cloud.digitalocean.com/v1/oauth/authorize",
			TokenURL: "https://cloud.digitalocean.com/v1/oauth/token",
		},
		RedirectURL: apiURL + "/admin/oauth/digitalocean/callback",
		Scopes:      []string{"read"},
	}
}
