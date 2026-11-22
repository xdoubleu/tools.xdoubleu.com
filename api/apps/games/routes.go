package games

import (
	"net/http"

	connectv2 "connectrpc.com/connect/v2"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"tools.xdoubleu.com/apps/games/connect"
	gamesmcp "tools.xdoubleu.com/apps/games/mcp"
	gamesv1connect "tools.xdoubleu.com/gen/games/v1/gamesv1connect"
	iapp "tools.xdoubleu.com/internal/app"
)

func (a *Games) Routes(prefix string, mux *http.ServeMux) {
	mux.HandleFunc(
		"GET /"+prefix+"/api/progress",
		a.services.Auth.Access(a.services.WebSocket.Handler()),
	)
	mux.HandleFunc(
		"POST /"+prefix+"/api/progress/steam/refresh",
		a.services.Auth.AppAccess(prefix, a.refreshSteamHandler),
	)

	iapp.MountConnect(
		mux,
		a.Logger,
		func(next http.HandlerFunc) http.HandlerFunc {
			return a.services.Auth.AppAccess(prefix, next)
		},
		func(s *connectv2.Server) {
			gamesv1connect.RegisterGamesServiceHandler(
				s,
				a.connectHandler(),
			)
		},
	)
}

// RegisterMCPTools exposes the app's read-only RPCs as MCP tools, scoped to
// the caller.
func (a *Games) RegisterMCPTools(srv *mcp.Server) {
	gamesmcp.Register(srv, a.connectHandler())
}

func (a *Games) connectHandler() *connect.Handler {
	return connect.NewHandler(a.services)
}
