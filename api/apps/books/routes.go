package books

import (
	"net/http"

	booksv1connect "tools.xdoubleu.com/gen/books/v1/booksv1connect"
	iapp "tools.xdoubleu.com/internal/app"
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
	scrub := iapp.ScrubInternalErrors(a.Logger)

	libraryPath, libraryHandler := booksv1connect.NewLibraryServiceHandler(
		handler,
		scrub,
	)
	mux.Handle(
		"POST "+libraryPath,
		a.Services.Auth.AppAccess(prefix, libraryHandler.ServeHTTP),
	)

	filesPath, filesHandler := booksv1connect.NewBookFilesServiceHandler(
		handler,
		scrub,
	)
	mux.Handle(
		"POST "+filesPath,
		a.Services.Auth.AppAccess(prefix, filesHandler.ServeHTTP),
	)

	koboPath, koboHandler := booksv1connect.NewKoboServiceHandler(handler, scrub)
	mux.Handle(
		"POST "+koboPath,
		a.Services.Auth.AppAccess(prefix, koboHandler.ServeHTTP),
	)

	catalogPath, catalogHandler := booksv1connect.NewCatalogServiceHandler(
		handler,
		scrub,
	)
	mux.Handle(
		"POST "+catalogPath,
		a.Services.Auth.AppAccess(prefix, catalogHandler.ServeHTTP),
	)

	// Public shareable-profile RPCs — deliberately NOT wrapped in auth
	// middleware; access is gated by the profile share token instead.
	publicPath, publicHandler := booksv1connect.NewPublicLibraryServiceHandler(
		&publicConnectHandler{app: a},
		scrub,
	)
	mux.Handle("POST "+publicPath, publicHandler)
}

func (a *Books) Routes(prefix string, mux *http.ServeMux) {
	a.booksRoutes(prefix, mux)
	a.coverRoutes(prefix, mux)
	a.koboRoutes(prefix, mux)
}
