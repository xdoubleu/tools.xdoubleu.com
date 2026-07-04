package games

import (
	"net/http"

	gamesv1connect "tools.xdoubleu.com/gen/games/v1/gamesv1connect"
)

func (a *Games) Routes(prefix string, mux *http.ServeMux) {
	mux.HandleFunc(
		"GET /"+prefix+"/api/progress",
		a.Services.Auth.Access(a.Services.WebSocket.Handler()),
	)
	mux.HandleFunc(
		"GET /"+prefix+"/api/progress/{id}/refresh",
		a.Services.Auth.Access(a.refreshHandler),
	)

	gamesPath, gamesHandler := gamesv1connect.NewGamesServiceHandler(
		&gamesConnectHandler{app: a},
	)
	mux.Handle(
		"POST "+gamesPath,
		a.Services.Auth.AppAccess(prefix, gamesHandler.ServeHTTP),
	)
}
