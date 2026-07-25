package main

import (
	"context"
	"fmt"
	"net/http"
	"time"

	mcpauth "github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/modelcontextprotocol/go-sdk/oauthex"
	"google.golang.org/protobuf/proto"

	"tools.xdoubleu.com/internal/constants"
	"tools.xdoubleu.com/internal/mcptools"
	"tools.xdoubleu.com/internal/models"
)

// The observability MCP server exposes the read-only ObservabilityService
// signals to a local Claude CLI over streamable-HTTP. Every tool wraps a shared
// internal ObservabilityService read method — no write RPC is reachable, so the
// server is read-only by construction. It sits behind OAuth 2.1: the api is the
// resource server (Bearer verification + protected-resource metadata), Supabase
// is the authorization server.

const (
	mcpServerName    = "tools-observability"
	mcpServerVersion = "1.0.0"

	// mcpUserExtraKey stashes the resolved user on the go-sdk TokenInfo so the
	// user-context middleware can promote it for the tools' admin gate.
	mcpUserExtraKey = "user"

	// mcpTokenTTL is the nominal freshness window reported to the go-sdk bearer
	// middleware for a token we just validated against Supabase.
	mcpTokenTTL = time.Hour

	monitoringMCPPath = "/monitoring/mcp"
	// resourceMetadataPath is the resource-scoped RFC 9728 metadata document,
	// referenced from the WWW-Authenticate challenge. rootResourceMetadataPath
	// is the same document at the well-known root for clients that probe it.
	resourceMetadataPath     = "/.well-known/oauth-protected-resource/monitoring/mcp"
	rootResourceMetadataPath = "/.well-known/oauth-protected-resource"
)

// windowArgs is the input for the two windowed stats tools; noArgs is the empty
// input for the rest. Both are structs so their inferred JSON schema is an
// object, as the MCP spec requires.
type windowArgs struct {
	WindowDays int32 `json:"window_days,omitempty" jsonschema:"days to look back"`
}

type noArgs struct{}

func (app *Application) mcpResourceURL() string {
	return app.config.APIURL + monitoringMCPPath
}

// mcpAuthServerIssuer is the Supabase OAuth 2.1 authorization-server issuer that
// clients discover from the protected-resource metadata.
func (app *Application) mcpAuthServerIssuer() string {
	return "https://" + app.config.SupabaseProjRef + ".supabase.co/auth/v1"
}

func (app *Application) mcpResourceMetadataURL() string {
	return app.config.APIURL + resourceMetadataPath
}

func (app *Application) mcpResourceMetadata() *oauthex.ProtectedResourceMetadata {
	return app.mcpResourceMetadataFor(
		monitoringMCPPath, "tools.xdoubleu.com observability",
	)
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

// mcpTokenVerifier validates a Bearer access token by reusing the same Supabase
// token resolution + admin-role enrichment as the cookie middleware, and stashes
// the resolved user for mcpUserContext to promote into the request context.
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
		// ResolveToken only succeeds for a token Supabase currently accepts, so
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
// request context under UserContextKey, so the tools' requireAdmin gate works
// exactly as it does for the cookie-authenticated Connect handlers.
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

// monitoringMCPRoute is the fully gated MCP endpoint: Bearer verification (with
// the WWW-Authenticate challenge pointing at the resource metadata) → user
// promotion → the streamable-HTTP MCP handler.
func (app *Application) monitoringMCPRoute() http.Handler {
	return app.mcpBearerRoute(
		app.mcpResourceMetadataURL(), app.monitoringMCPHandler(),
	)
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

func (app *Application) monitoringMCPHandler() http.Handler {
	srv := app.newMonitoringMCPServer()
	//nolint:exhaustruct // Stateless is the only option this read-only server sets
	return mcp.NewStreamableHTTPHandler(
		func(*http.Request) *mcp.Server { return srv },
		&mcp.StreamableHTTPOptions{Stateless: true},
	)
}

func (app *Application) newMonitoringMCPServer() *mcp.Server {
	h := &obsConnectHandler{app: app}
	//nolint:exhaustruct // only Name/Version identify the server
	srv := mcp.NewServer(&mcp.Implementation{
		Name:    mcpServerName,
		Version: mcpServerVersion,
	}, nil)

	addObsTool(srv, "get_job_stats",
		"Background job run statistics and recent runs (global.job_runs).",
		func(ctx context.Context, a windowArgs) (proto.Message, error) {
			return h.jobStats(ctx, a.WindowDays)
		})
	addObsTool(srv, "get_usage_stats",
		"Per-day request counts by app and endpoint (global.usage_daily).",
		func(ctx context.Context, a windowArgs) (proto.Message, error) {
			return h.usageStats(ctx, a.WindowDays)
		})
	addObsTool(srv, "get_storage_stats",
		"Latest R2 object-store snapshot plus recent history.",
		func(ctx context.Context, _ noArgs) (proto.Message, error) {
			return h.storageStats(ctx)
		})
	addObsTool(srv, "get_database_stats",
		"Total database size and per-schema sizes (live pg_* queries).",
		func(ctx context.Context, _ noArgs) (proto.Message, error) {
			return h.databaseStats(ctx)
		})
	addObsTool(srv, "get_github_issues",
		"Open GitHub issues for the repository.",
		func(ctx context.Context, _ noArgs) (proto.Message, error) {
			return h.githubIssues(ctx), nil
		})
	addObsTool(srv, "get_sentry_issues",
		"Unresolved Sentry issues for the project.",
		func(ctx context.Context, _ noArgs) (proto.Message, error) {
			return h.sentryIssues(ctx), nil
		})
	addObsTool(srv, "get_deploy_status",
		"Phase and health of the latest DigitalOcean deployment.",
		func(ctx context.Context, _ noArgs) (proto.Message, error) {
			return h.deployStatus(ctx), nil
		})

	return srv
}

// addObsTool registers one read-only tool. It applies the admin gate uniformly
// and marshals the shared method's proto response to JSON text content, so the
// tool bodies stay a thin wrapper over the ObservabilityService read methods.
func addObsTool[In any](
	srv *mcp.Server,
	name, description string,
	produce func(context.Context, In) (proto.Message, error),
) {
	//nolint:exhaustruct // name/description are the only fields tools need
	mcp.AddTool(srv, &mcp.Tool{Name: name, Description: description},
		func(
			ctx context.Context,
			_ *mcp.CallToolRequest,
			args In,
		) (*mcp.CallToolResult, any, error) {
			if err := requireAdmin(ctx); err != nil {
				return nil, nil, err
			}
			msg, err := produce(ctx, args)
			if err != nil {
				return nil, nil, err
			}
			return mcptools.Result(msg)
		})
}
