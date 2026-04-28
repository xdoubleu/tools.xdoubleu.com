package backlog

import "net/http"

func (app *Backlog) backlogRoutes(prefix string, mux *http.ServeMux) {
	mux.HandleFunc(
		"GET /"+prefix+"/{$}",
		app.Services.Auth.AppAccess(prefix, app.handle(app.rootHandler)),
	)
	mux.HandleFunc(
		"GET /"+prefix+"/users/{userID}",
		app.Services.Auth.AppAccess(prefix, app.handle(app.userBacklogHandler)),
	)
	mux.HandleFunc(
		"GET /"+prefix+"/steam",
		app.Services.Auth.AppAccess(prefix, app.handle(app.steamPageHandler)),
	)
	mux.HandleFunc(
		"GET /"+prefix+"/steam/distribution/{bucket}",
		app.Services.Auth.AppAccess(prefix, app.handle(app.steamDistributionHandler)),
	)
	mux.HandleFunc(
		"GET /"+prefix+"/books",
		app.Services.Auth.AppAccess(prefix, app.handle(app.booksPageHandler)),
	)
	mux.HandleFunc(
		"GET /"+prefix+"/books/search",
		app.Services.Auth.AppAccess(prefix, app.handle(app.booksSearchLibraryHandler)),
	)
	mux.HandleFunc(
		"GET /"+prefix+"/books/search/external",
		app.Services.Auth.AppAccess(prefix, app.handle(app.booksSearchExternalHandler)),
	)
	mux.HandleFunc(
		"POST /"+prefix+"/books",
		app.Services.Auth.AppAccess(prefix, app.handle(app.addBookHandler)),
	)
	mux.HandleFunc(
		"POST /"+prefix+"/books/{id}/status",
		app.Services.Auth.AppAccess(prefix, app.handle(app.updateBookStatusHandler)),
	)
	mux.HandleFunc(
		"POST /"+prefix+"/books/import",
		app.Services.Auth.AppAccess(prefix, app.handle(app.importBooksHandler)),
	)
	mux.HandleFunc(
		"POST /"+prefix+"/books/{id}/tags",
		app.Services.Auth.AppAccess(prefix, app.handle(app.toggleTagHandler)),
	)
	mux.HandleFunc(
		"GET /"+prefix+"/books/library",
		app.Services.Auth.AppAccess(prefix, app.handle(app.booksLibraryHandler)),
	)
	mux.HandleFunc(
		"GET /"+prefix+"/books/progress",
		app.Services.Auth.AppAccess(prefix, app.handle(app.booksProgressHandler)),
	)
	mux.HandleFunc(
		"GET /"+prefix+"/api/progress",
		app.Services.WebSocket.Handler(),
	)
	mux.HandleFunc(
		"GET /"+prefix+"/api/progress/{id}/refresh",
		app.refreshHandler,
	)
}

func (app *Backlog) Routes(prefix string, mux *http.ServeMux) {
	app.backlogRoutes(prefix, mux)
}
