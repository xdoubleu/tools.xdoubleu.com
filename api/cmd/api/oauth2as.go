package main

import (
	"crypto/rsa"
	"encoding/json"
	"net/http"

	"github.com/ory/fosite"

	"tools.xdoubleu.com/internal/models"
	"tools.xdoubleu.com/internal/oauth2as"
)

// oauth2asWiring bundles the embedded OAuth 2.1 / OIDC authorization server.
type oauth2asWiring struct {
	store    *oauth2as.Store
	provider fosite.OAuth2Provider
	// oidcKey signs ID tokens; its public half is served at oauth2JWKSPath.
	oidcKey *rsa.PrivateKey
}

func (w *oauth2asWiring) oidcKeyID() string {
	return oauth2as.OIDCKeyID(w.oidcKey)
}

const (
	oauth2AuthorizePath = "/oauth2/authorize"
	//nolint:gosec // route path, not a credential
	oauth2TokenPath       = "/oauth2/token"
	oauth2RegisterPath    = "/oauth2/register"
	oauth2ConsentInfoPath = "/oauth2/consent-info"

	oauth2JWKSPath          = "/oauth2/jwks"
	oauth2MetadataPath      = "/.well-known/oauth-authorization-server"
	openIDConfigurationPath = "/.well-known/openid-configuration"
)

// oauth2SessionUserResolver resolves the accessToken cookie, with DB role and
// display name overlaid, for AuthorizeHandler to re-verify the session and
// build ID-token claims.
func (app *Application) oauth2SessionUserResolver() oauth2as.SessionUserResolver {
	return func(r *http.Request) (oauth2as.ResolvedUser, bool) {
		cookie, err := r.Cookie("accessToken")
		if err != nil {
			return oauth2as.ResolvedUser{}, false //nolint:exhaustruct // not-found sentinel
		}
		user, err := app.auth.GetUser(r.Context(), cookie.Value)
		if err != nil {
			return oauth2as.ResolvedUser{}, false //nolint:exhaustruct // not-found sentinel
		}

		resolved := oauth2as.ResolvedUser{
			ID:          user.ID,
			Email:       user.Email,
			DisplayName: user.DisplayName,
			IsAdmin:     user.Role == models.RoleAdmin,
		}

		// The bare auth-schema user always has RoleUser and no display name.
		if dbUser, dbErr := app.appUsersRepo.GetByID(r.Context(), user.ID); dbErr == nil {
			resolved.Email = dbUser.Email
			resolved.DisplayName = dbUser.DisplayName
			resolved.IsAdmin = dbUser.Role == models.RoleAdmin
		}

		return resolved, true
	}
}

// oauth2MetadataHandler serves the RFC 8414 metadata (also the OIDC discovery
// document).
func (app *Application) oauth2MetadataHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		issuer := app.config.AuthIssuer
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"issuer":                   issuer,
			"authorization_endpoint":   issuer + oauth2AuthorizePath,
			"token_endpoint":           issuer + oauth2TokenPath,
			"registration_endpoint":    issuer + oauth2RegisterPath,
			"jwks_uri":                 issuer + oauth2JWKSPath,
			"response_types_supported": []string{"code"},
			"grant_types_supported": []string{
				"authorization_code",
				"refresh_token",
				"client_credentials",
			},
			// Dynamic (MCP) clients only get offline_access; the OIDC scopes are for the
			// static Grafana SSO client.
			"scopes_supported":                      oauth2as.SupportedScopes,
			"subject_types_supported":               []string{"public"},
			"id_token_signing_alg_values_supported": []string{"RS256"},
			"code_challenge_methods_supported":      []string{"S256"},
			"token_endpoint_auth_methods_supported": []string{
				"none", "client_secret_basic", "client_secret_post",
			},
		})
	}
}
