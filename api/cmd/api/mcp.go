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
	"tools.xdoubleu.com/internal/oauth2as"
)

// OAuth 2.1 plumbing shared by MCP endpoints: the api is both the resource
// server and its own authorization server (internal/oauth2as). Server and tool
// registration live in mcp_apps.go.

const (
	mcpServerVersion = "1.0.0"

	// mcpUserExtraKey stashes the resolved user on the go-sdk TokenInfo.
	mcpUserExtraKey = "user"

	// mcpTokenTTL is the nominal freshness reported for a validated token.
	mcpTokenTTL = time.Hour

	// rootResourceMetadataPath is the RFC 9728 metadata at the well-known root.
	rootResourceMetadataPath = "/.well-known/oauth-protected-resource"
)

// mcpAuthServerIssuer is internal/oauth2as's issuer URL.
func (app *Application) mcpAuthServerIssuer() string {
	return app.config.AuthIssuer
}

// mcpResourceMetadataFor builds the RFC 9728 metadata for mcpPath.
func (app *Application) mcpResourceMetadataFor(
	mcpPath, resourceName string,
) *oauthex.ProtectedResourceMetadata {
	//nolint:exhaustruct // only the discovery fields are relevant
	return &oauthex.ProtectedResourceMetadata{
		Resource:               app.config.APIURL + mcpPath,
		AuthorizationServers:   []string{app.mcpAuthServerIssuer()},
		BearerMethodsSupported: []string{"header"},
		ResourceName:           resourceName,
		// Advertising offline_access makes clients request a refresh token.
		ScopesSupported: []string{oauth2as.OfflineAccessScope},
	}
}

// mcpTokenVerifier validates a Bearer token with the cookie middleware's
// resolution and enrichment, stashing the user for mcpUserContext.
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
		// ResolveToken only accepts current tokens, so a nominal expiry is safe.
		//nolint:exhaustruct // scopes are not used by this resource
		return &mcpauth.TokenInfo{
			UserID:     user.ID,
			Expiration: time.Now().Add(mcpTokenTTL),
			Extra:      map[string]any{mcpUserExtraKey: *user},
		}, nil
	}
}

// mcpUserContext puts the verified user under UserContextKey so tool access
// gates work like the Connect handlers'.
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

// mcpBearerRoute wraps an MCP handler in Bearer verification and user
// promotion.
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
