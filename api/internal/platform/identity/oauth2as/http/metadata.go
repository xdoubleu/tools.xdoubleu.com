package http

import (
	"context"
	"encoding/json"
	nethttp "net/http"

	"tools.xdoubleu.com/internal/platform/identity/oauth2as"
)

// Sessions resolves the browser session behind an accessToken cookie, with the
// user's current role and display name.
type Sessions interface {
	SessionUser(ctx context.Context, accessToken string) (oauth2as.ResolvedUser, error)
}

// NewSessionUserResolver resolves the accessToken cookie for AuthorizeHandler
// to re-verify the session and build ID-token claims.
func NewSessionUserResolver(sessions Sessions) SessionUserResolver {
	return func(r *nethttp.Request) (oauth2as.ResolvedUser, bool) {
		cookie, err := r.Cookie("accessToken")
		if err != nil {
			return oauth2as.ResolvedUser{}, false //nolint:exhaustruct // not-found sentinel
		}
		user, err := sessions.SessionUser(r.Context(), cookie.Value)
		if err != nil {
			return oauth2as.ResolvedUser{}, false //nolint:exhaustruct // not-found sentinel
		}
		return user, true
	}
}

// MetadataHandler serves the RFC 8414 metadata (also the OIDC discovery
// document) for issuer.
func MetadataHandler(issuer string) nethttp.HandlerFunc {
	return func(w nethttp.ResponseWriter, _ *nethttp.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"issuer":                   issuer,
			"authorization_endpoint":   issuer + oauth2as.AuthorizePath,
			"token_endpoint":           issuer + oauth2as.TokenPath,
			"registration_endpoint":    issuer + oauth2as.RegisterPath,
			"jwks_uri":                 issuer + oauth2as.JWKSPath,
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
