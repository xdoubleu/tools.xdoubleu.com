package taste

import (
	"net/http"

	"connectrpc.com/connect/v2"

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
		func(s *connect.Server) {
			tastev1connect.RegisterTasteServiceHandler(
				s,
				&tasteConnectHandler{app: a},
			)
		},
	)
}
