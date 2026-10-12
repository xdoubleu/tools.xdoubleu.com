package feeds

import (
	"net/http"

	"connectrpc.com/connect/v2"

	"tools.xdoubleu.com/gen/feeds/v1/feedsv1connect"
	iapp "tools.xdoubleu.com/internal/app"
)

func (a *Feeds) Routes(prefix string, mux *http.ServeMux) {
	iapp.MountConnect(
		mux,
		a.Logger,
		func(next http.HandlerFunc) http.HandlerFunc {
			return a.Services.Auth.AppAccess(prefix, next)
		},
		func(s *connect.Server) {
			feedsv1connect.RegisterFeedServiceHandler(
				s,
				&feedsConnectHandler{app: a},
			)
		},
	)

	a.emailRoutes(prefix, mux)
}
