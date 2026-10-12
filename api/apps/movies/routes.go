package movies

import (
	"net/http"

	"connectrpc.com/connect/v2"

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
		func(s *connect.Server) {
			moviesv1connect.RegisterMoviesServiceHandler(
				s,
				&moviesConnectHandler{app: a},
			)
		},
	)
}
