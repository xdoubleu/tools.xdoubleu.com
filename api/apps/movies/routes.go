package movies

import (
	"net/http"

	connectv2 "connectrpc.com/connect/v2"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"tools.xdoubleu.com/apps/movies/connect"
	moviesmcp "tools.xdoubleu.com/apps/movies/mcp"
	"tools.xdoubleu.com/gen/movies/v1/moviesv1connect"
	iapp "tools.xdoubleu.com/internal/app"
)

func (a *Movies) Routes(prefix string, mux *http.ServeMux) {
	iapp.MountConnect(
		mux,
		a.Logger,
		func(next http.HandlerFunc) http.HandlerFunc {
			return a.services.Auth.AppAccess(prefix, next)
		},
		func(s *connectv2.Server) {
			moviesv1connect.RegisterMoviesServiceHandler(
				s,
				connect.NewHandler(a.services.Movies),
			)
		},
	)
}

// RegisterMCPTools exposes the app's read and write tools, scoped to the caller.
func (a *Movies) RegisterMCPTools(srv *mcp.Server) {
	moviesmcp.Register(srv, a.services.Movies)
}
