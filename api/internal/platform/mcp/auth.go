package mcp

import (
	"context"
	"fmt"
	"net/http"
	"time"

	mcpauth "github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/oauthex"

	"tools.xdoubleu.com/internal/kit/constants"
	"tools.xdoubleu.com/internal/models"
	"tools.xdoubleu.com/internal/platform/oauth2as"
)

// OAuth 2.1 plumbing shared by MCP endpoints: the api is both the resource
// server and its own authorization server (internal/platform/oauth2as). Server
// and tool registration live in cmd/api/mcp_apps.go.

const (
	serverVersion = "1.0.0"

	// userExtraKey stashes the resolved user on the go-sdk TokenInfo.
	userExtraKey = "user"

	// tokenTTL is the nominal freshness reported for a validated token.
	tokenTTL = time.Hour

	// RootResourceMetadataPath is the RFC 9728 metadata at the well-known root.
	RootResourceMetadataPath = "/.well-known/oauth-protected-resource"
)

// TokenResolver resolves a bearer token to its enriched user.
type TokenResolver interface {
	ResolveToken(ctx context.Context, token string) (*models.User, error)
}

// Resource is the MCP resource server in front of the authorization server at
// Issuer.
type Resource struct {
	Tokens TokenResolver
	// APIURL is the public base URL the MCP paths hang off.
	APIURL string
	// Issuer is the authorization server's issuer URL.
	Issuer string
}

// ResourceMetadataFor builds the RFC 9728 metadata for mcpPath.
func (r Resource) ResourceMetadataFor(
	mcpPath, resourceName string,
) *oauthex.ProtectedResourceMetadata {
	//nolint:exhaustruct // only the discovery fields are relevant
	return &oauthex.ProtectedResourceMetadata{
		Resource:               r.APIURL + mcpPath,
		AuthorizationServers:   []string{r.Issuer},
		BearerMethodsSupported: []string{"header"},
		ResourceName:           resourceName,
		// Advertising offline_access makes clients request a refresh token.
		ScopesSupported: []string{oauth2as.OfflineAccessScope},
	}
}

// tokenVerifier validates a Bearer token with the cookie middleware's
// resolution and enrichment, stashing the user for userContext.
func (r Resource) tokenVerifier() mcpauth.TokenVerifier {
	return func(
		ctx context.Context,
		token string,
		_ *http.Request,
	) (*mcpauth.TokenInfo, error) {
		user, err := r.Tokens.ResolveToken(ctx, token)
		if err != nil {
			return nil, fmt.Errorf("%w: %w", mcpauth.ErrInvalidToken, err)
		}
		// ResolveToken only accepts current tokens, so a nominal expiry is safe.
		//nolint:exhaustruct // scopes are not used by this resource
		return &mcpauth.TokenInfo{
			UserID:     user.ID,
			Expiration: time.Now().Add(tokenTTL),
			Extra:      map[string]any{userExtraKey: *user},
		}, nil
	}
}

// userContext puts the verified user under UserContextKey so tool access
// gates work like the Connect handlers'.
func userContext(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		info := mcpauth.TokenInfoFromContext(req.Context())
		user, ok := info.Extra[userExtraKey].(models.User)
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		ctx := context.WithValue(req.Context(), constants.UserContextKey, user)
		ctx = context.WithValue(ctx, constants.AuthChannelContextKey, constants.ChannelMCP)
		next.ServeHTTP(w, req.WithContext(ctx))
	})
}

// BearerRoute wraps an MCP handler in Bearer verification and user promotion.
func (r Resource) BearerRoute(
	resourceMetadataURL string,
	inner http.Handler,
) http.Handler {
	bearer := mcpauth.RequireBearerToken(
		r.tokenVerifier(),
		&mcpauth.RequireBearerTokenOptions{
			ResourceMetadataURL: resourceMetadataURL,
			Scopes:              nil,
			// Zero values keep strict expiration: the verifier always sets an
			// Expiration, and no clock-skew tolerance is applied.
			AllowMissingExpiration: false,
			ClockSkew:              0,
		},
	)
	return bearer(userContext(inner))
}
