package backlog

import (
	"net/http"

	backlogv1connect "tools.xdoubleu.com/gen/backlog/v1/backlogv1connect"
)

func (app *Backlog) backlogRoutes(prefix string, mux *http.ServeMux) {
	mux.HandleFunc(
		"GET /"+prefix+"/api/progress",
		app.Services.Auth.Access(app.Services.WebSocket.Handler()),
	)
	mux.HandleFunc(
		"GET /"+prefix+"/api/progress/{id}/refresh",
		app.Services.Auth.Access(app.refreshHandler),
	)

	gamesPath, gamesHandler := backlogv1connect.NewGamesServiceHandler(
		&gamesConnectHandler{app: app},
	)
	mux.Handle(
		"POST "+gamesPath,
		app.Services.Auth.AppAccess(prefix, gamesHandler.ServeHTTP),
	)

	booksPath, booksHandler := backlogv1connect.NewBooksServiceHandler(
		&booksConnectHandler{app: app},
	)
	mux.Handle(
		"POST "+booksPath,
		app.Services.Auth.AppAccess(prefix, booksHandler.ServeHTTP),
	)
}

func (app *Backlog) Routes(prefix string, mux *http.ServeMux) {
	app.backlogRoutes(prefix, mux)
	app.coverRoutes(prefix, mux)
	app.koboRoutes(prefix, mux)
}
