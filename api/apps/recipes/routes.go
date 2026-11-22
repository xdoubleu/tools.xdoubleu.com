package recipes

import (
	"net/http"

	connectv2 "connectrpc.com/connect/v2"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"tools.xdoubleu.com/apps/recipes/connect"
	recipesmcp "tools.xdoubleu.com/apps/recipes/mcp"
	"tools.xdoubleu.com/gen/recipes/v1/recipesv1connect"
	iapp "tools.xdoubleu.com/internal/app"
)

func (a *Recipes) Routes(prefix string, mux *http.ServeMux) {
	iapp.MountConnect(
		mux,
		a.Logger,
		func(next http.HandlerFunc) http.HandlerFunc {
			return a.services.Auth.AppAccess(prefix, next)
		},
		func(s *connectv2.Server) {
			recipesv1connect.RegisterRecipesServiceHandler(
				s,
				connect.NewHandler(a.services.Recipes),
			)
		},
	)
}

// RegisterMCPTools exposes the app's read RPCs, a video-text fetch and
// draft-preserving create/update write tools, scoped to the caller's family
// (docs/adr-0026-recipes-mcp-write-tool.md).
func (a *Recipes) RegisterMCPTools(srv *mcp.Server) {
	recipesmcp.Register(srv, a.services.Recipes, a.services.Videos)
}
