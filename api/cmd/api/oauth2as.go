package main

import (
	"encoding/json"
	"net/http"

	"github.com/ory/fosite"

	"tools.xdoubleu.com/internal/oauth2as"
)

// oauth2asWiring bundles the embedded OAuth 2.1 authorization server (issue
// #1039, replacing Supabase for the MCP OAuth flow) constructed in
// NewApplication.
type oauth2asWiring struct {
	store    *oauth2as.Store
	provider fosite.OAuth2Provider
}

const (
	oauth2AuthorizePath = "/oauth2/authorize"
	//nolint:gosec // route path, not a credential
	oauth2TokenPath       = "/oauth2/token"
	oauth2RegisterPath    = "/oauth2/register"
	oauth2ConsentInfoPath = "/oauth2/consent-info"
	oauth2MetadataPath    = "/.well-known/oauth-authorization-server"
)

// oauth2SessionUserResolver reads the accessToken cookie and resolves it to
// a user ID via the already-wired auth service, for oauth2as.AuthorizeHandler
// to re-verify the session server-side (defense in depth) once the web
// consent page has confirmed consent=allow.
func (app *Application) oauth2SessionUserResolver() oauth2as.SessionUserResolver {
	return func(r *http.Request) (string, bool) {
		cookie, err := r.Cookie("accessToken")
		if err != nil {
			return "", false
		}
		user, err := app.auth.GetUser(r.Context(), cookie.Value)
		if err != nil {
			return "", false
		}
		return user.ID, true
	}
}

// oauth2MetadataHandler hand-rolls the RFC 8414 / RFC 9728
// authorization-server metadata document — see the AUTH_ISSUER config doc
// comment for why this api itself is the issuer.
func (app *Application) oauth2MetadataHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		issuer := app.config.AuthIssuer
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"issuer":                   issuer,
			"authorization_endpoint":   issuer + oauth2AuthorizePath,
			"token_endpoint":           issuer + oauth2TokenPath,
			"registration_endpoint":    issuer + oauth2RegisterPath,
			"response_types_supported": []string{"code"},
			"grant_types_supported": []string{
				"authorization_code",
				"refresh_token",
			},
			"code_challenge_methods_supported":      []string{"S256"},
			"token_endpoint_auth_methods_supported": []string{"none"},
		})
	}
}
