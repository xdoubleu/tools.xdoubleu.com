package taste

import (
	"net/http"

	connectv2 "connectrpc.com/connect/v2"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"tools.xdoubleu.com/apps/taste/connect"
	tastemcp "tools.xdoubleu.com/apps/taste/mcp"
	"tools.xdoubleu.com/gen/taste/v1/tastev1connect"
	iapp "tools.xdoubleu.com/internal/app"
)

func (a *Taste) Routes(prefix string, mux *http.ServeMux) {
	iapp.MountConnect(
		mux,
		a.Logger,
		func(next http.HandlerFunc) http.HandlerFunc {
			return a.auth.AppAccess(prefix, next)
		},
		func(s *connectv2.Server) {
			tastev1connect.RegisterTasteServiceHandler(
				s,
				a.handler(),
			)
		},
	)
}

func (a *Taste) handler() *connect.Handler {
	return connect.NewHandler(a.services, a.jobQueue)
}

// RegisterMCPTools exposes the consumption log and its stats, scoped to the
// caller.
func (a *Taste) RegisterMCPTools(srv *mcp.Server) {
	tastemcp.Register(srv, a.handler())
}
