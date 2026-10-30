package games

import (
	"net/http"

	"connectrpc.com/connect/v2"

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
		func(s *connect.Server) {
			gamesv1connect.RegisterGamesServiceHandler(
				s,
				&gamesConnectHandler{app: a},
			)
		},
	)
}
