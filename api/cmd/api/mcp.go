package main

import (
	"context"
	"fmt"
	"net/http"
	"time"

	mcpauth "github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/oauthex"

	"tools.xdoubleu.com/internal/constants"
	"tools.xdoubleu.com/internal/models"
)

// This file holds the OAuth 2.1 plumbing shared by every MCP endpoint: the api
// is both the resource server (Bearer verification + protected-resource
// metadata) and, since issue #1039, its own authorization server
// (internal/oauth2as, replacing Supabase). The server construction and tool
// registration for the single combined MCP server live in mcp_apps.go.

const (
	mcpServerVersion = "1.0.0"

	// mcpUserExtraKey stashes the resolved user on the go-sdk TokenInfo so the
	// user-context middleware can promote it for the tools' access gates.
	mcpUserExtraKey = "user"

	// mcpTokenTTL is the nominal freshness window reported to the go-sdk bearer
	// middleware for a token we just validated.
	mcpTokenTTL = time.Hour

	// rootResourceMetadataPath is the RFC 9728 metadata document at the
	// well-known root, for clients that probe it without a resource path.
	rootResourceMetadataPath = "/.well-known/oauth-protected-resource"
)

// mcpAuthServerIssuer is this api's own OAuth 2.1 authorization-server issuer
// (internal/oauth2as) that clients discover from the protected-resource
// metadata.
func (app *Application) mcpAuthServerIssuer() string {
	return app.config.AuthIssuer
}

// mcpResourceMetadataFor builds the RFC 9728 protected-resource metadata for the
// MCP endpoint at mcpPath: the resource URL, the Supabase authorization server,
// and a human-readable resource name. Shared by every MCP endpoint.
func (app *Application) mcpResourceMetadataFor(
	mcpPath, resourceName string,
) *oauthex.ProtectedResourceMetadata {
	//nolint:exhaustruct // only the discovery fields are relevant
	return &oauthex.ProtectedResourceMetadata{
		Resource:               app.config.APIURL + mcpPath,
		AuthorizationServers:   []string{app.mcpAuthServerIssuer()},
		BearerMethodsSupported: []string{"header"},
		ResourceName:           resourceName,
	}
}

// mcpTokenVerifier validates a Bearer access token by reusing the same
// token resolution + admin-role enrichment as the cookie middleware, and
// stashes the resolved user for mcpUserContext to promote into the request
// context.
func (app *Application) mcpTokenVerifier() mcpauth.TokenVerifier {
	return func(
		ctx context.Context,
		token string,
		_ *http.Request,
	) (*mcpauth.TokenInfo, error) {
		user, err := app.auth.ResolveToken(ctx, token)
		if err != nil {
			return nil, fmt.Errorf("%w: %w", mcpauth.ErrInvalidToken, err)
		}
		// ResolveToken only succeeds for a token currently accepted, so
		// a nominal near-future expiration satisfies the go-sdk's freshness
		// check; the token is re-validated on the next cache miss anyway.
		//nolint:exhaustruct // scopes are not used by this resource
		return &mcpauth.TokenInfo{
			UserID:     user.ID,
			Expiration: time.Now().Add(mcpTokenTTL),
			Extra:      map[string]any{mcpUserExtraKey: *user},
		}, nil
	}
}

// mcpUserContext promotes the user resolved by the Bearer verifier onto the
// request context under UserContextKey, so the tools' access gates work
// exactly as they do for the cookie-authenticated Connect handlers.
func (app *Application) mcpUserContext(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		info := mcpauth.TokenInfoFromContext(r.Context())
		user, ok := info.Extra[mcpUserExtraKey].(models.User)
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		ctx := context.WithValue(r.Context(), constants.UserContextKey, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// mcpBearerRoute wraps an MCP handler in the OAuth 2.1 resource-server gate:
// Bearer verification (whose 401 challenge points at resourceMetadataURL) then
// promotion of the resolved user onto the request context. Shared by every MCP
// endpoint.
func (app *Application) mcpBearerRoute(
	resourceMetadataURL string,
	inner http.Handler,
) http.Handler {
	bearer := mcpauth.RequireBearerToken(
		app.mcpTokenVerifier(),
		&mcpauth.RequireBearerTokenOptions{
			ResourceMetadataURL: resourceMetadataURL,
			Scopes:              nil,
		},
	)
	return bearer(app.mcpUserContext(inner))
}
