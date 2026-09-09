package main

import (
	"crypto/rsa"
	"encoding/json"
	"net/http"

	"github.com/ory/fosite"

	"tools.xdoubleu.com/internal/models"
	"tools.xdoubleu.com/internal/oauth2as"
)

// oauth2asWiring bundles the embedded OAuth 2.1 / OpenID Connect authorization
// server (issues #1039, #1469) constructed in NewApplication.
type oauth2asWiring struct {
	store    *oauth2as.Store
	provider fosite.OAuth2Provider
	// oidcKey signs OIDC ID tokens; its public half is served at
	// oauth2JWKSPath and its key id is stamped into every ID token header.
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

// oauth2SessionUserResolver reads the accessToken cookie and resolves it —
// with the DB-managed role and display name overlaid (the GetCurrentUser
// two-layer pattern) — for oauth2as.AuthorizeHandler to re-verify the session
// server-side (defense in depth) and to build the OIDC ID-token claims once
// the web consent page has confirmed consent=allow.
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

		// Prefer the DB role/display-name/email; the bare auth-schema user
		// always resolves to RoleUser with no display name (api/CLAUDE.md).
		if dbUser, dbErr := app.appUsersRepo.GetByID(r.Context(), user.ID); dbErr == nil {
			resolved.Email = dbUser.Email
			resolved.DisplayName = dbUser.DisplayName
			resolved.IsAdmin = dbUser.Role == models.RoleAdmin
		}

		return resolved, true
	}
}

// oauth2MetadataHandler hand-rolls the RFC 8414 authorization-server metadata
// document (also served at openIDConfigurationPath as OIDC discovery) — see
// the AUTH_ISSUER config doc comment for why this api itself is the issuer.
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
			},
			// Dynamically-registered (MCP) clients only ever get
			// offline_access; the openid/profile/email scopes are reserved
			// for the static Grafana SSO client (issue #1469).
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
