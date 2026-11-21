package shoppinglist

import (
	"net/http"

	connectv2 "connectrpc.com/connect/v2"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"tools.xdoubleu.com/apps/shoppinglist/connect"
	shoppingmcp "tools.xdoubleu.com/apps/shoppinglist/mcp"
	"tools.xdoubleu.com/gen/shoppinglist/v1/shoppinglistv1connect"
	iapp "tools.xdoubleu.com/internal/app"
)

func (a *ShoppingList) Routes(prefix string, mux *http.ServeMux) {
	iapp.MountConnect(
		mux,
		a.Logger,
		func(next http.HandlerFunc) http.HandlerFunc {
			return a.services.Auth.AppAccess(prefix, next)
		},
		func(s *connectv2.Server) {
			shoppinglistv1connect.RegisterShoppingListServiceHandler(
				s,
				connect.NewHandler(a.services.Shopping),
			)
		},
	)
}

// RegisterMCPTools exposes the app's read-only RPCs as MCP tools.
func (a *ShoppingList) RegisterMCPTools(srv *mcp.Server) {
	shoppingmcp.Register(srv, a.services.Shopping)
}
