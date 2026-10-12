package watchparty

import (
	"fmt"
	"net/http"

	"connectrpc.com/connect/v2"

	watchpartyv1connect "tools.xdoubleu.com/gen/watchparty/v1/watchpartyv1connect"
	iapp "tools.xdoubleu.com/internal/app"
)

func (app *WatchParty) Routes(prefix string, mux *http.ServeMux) {
	apiPrefix := fmt.Sprintf("/%s/api", prefix)
	app.wsRoutes(apiPrefix, mux)

	iapp.MountConnect(
		mux,
		app.Logger,
		func(next http.HandlerFunc) http.HandlerFunc {
			return app.Services.Auth.AppAccess(prefix, next)
		},
		func(s *connect.Server) {
			watchpartyv1connect.RegisterRoomServiceHandler(
				s,
				&roomConnectHandler{app: app},
			)
		},
	)
}
