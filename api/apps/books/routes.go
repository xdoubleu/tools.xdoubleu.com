package books

import (
	"net/http"

	booksv1connect "tools.xdoubleu.com/gen/books/v1/booksv1connect"
)

func (a *Books) booksRoutes(prefix string, mux *http.ServeMux) {
	mux.HandleFunc(
		"GET /"+prefix+"/api/progress",
		a.Services.Auth.Access(a.Services.WebSocket.Handler()),
	)
	mux.HandleFunc(
		"GET /"+prefix+"/api/progress/{id}/refresh",
		a.Services.Auth.Access(a.refreshHandler),
	)

	handler := &booksConnectHandler{app: a}

	libraryPath, libraryHandler := booksv1connect.NewLibraryServiceHandler(handler)
	mux.Handle(
		"POST "+libraryPath,
		a.Services.Auth.AppAccess(prefix, libraryHandler.ServeHTTP),
	)

	filesPath, filesHandler := booksv1connect.NewBookFilesServiceHandler(handler)
	mux.Handle(
		"POST "+filesPath,
		a.Services.Auth.AppAccess(prefix, filesHandler.ServeHTTP),
	)

	koboPath, koboHandler := booksv1connect.NewKoboServiceHandler(handler)
	mux.Handle(
		"POST "+koboPath,
		a.Services.Auth.AppAccess(prefix, koboHandler.ServeHTTP),
	)

	catalogPath, catalogHandler := booksv1connect.NewCatalogServiceHandler(handler)
	mux.Handle(
		"POST "+catalogPath,
		a.Services.Auth.AppAccess(prefix, catalogHandler.ServeHTTP),
	)
}

func (a *Books) Routes(prefix string, mux *http.ServeMux) {
	a.booksRoutes(prefix, mux)
	a.coverRoutes(prefix, mux)
	a.koboRoutes(prefix, mux)
}
