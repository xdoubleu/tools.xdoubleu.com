package reading

import (
	"net/http"

	readingv1connect "tools.xdoubleu.com/gen/reading/v1/readingv1connect"
	iapp "tools.xdoubleu.com/internal/app"
)

func (a *Reading) booksRoutes(prefix string, mux *http.ServeMux) {
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

	libraryPath, libraryHandler := readingv1connect.NewLibraryServiceHandler(
		handler,
		scrub,
	)
	mux.Handle(
		"POST "+libraryPath,
		a.Services.Auth.AppAccess(prefix, libraryHandler.ServeHTTP),
	)

	filesPath, filesHandler := readingv1connect.NewBookFilesServiceHandler(
		handler,
		scrub,
	)
	mux.Handle(
		"POST "+filesPath,
		a.Services.Auth.AppAccess(prefix, filesHandler.ServeHTTP),
	)

	koboPath, koboHandler := readingv1connect.NewKoboServiceHandler(handler, scrub)
	mux.Handle(
		"POST "+koboPath,
		a.Services.Auth.AppAccess(prefix, koboHandler.ServeHTTP),
	)

	catalogPath, catalogHandler := readingv1connect.NewCatalogServiceHandler(
		handler,
		scrub,
	)
	mux.Handle(
		"POST "+catalogPath,
		a.Services.Auth.AppAccess(prefix, catalogHandler.ServeHTTP),
	)

	feedsPath, feedsHandler := readingv1connect.NewFeedServiceHandler(
		handler,
		scrub,
	)
	mux.Handle(
		"POST "+feedsPath,
		a.Services.Auth.AppAccess(prefix, feedsHandler.ServeHTTP),
	)

	// Public shareable-profile RPCs — deliberately NOT wrapped in auth
	// middleware; access is gated by the profile share token instead.
	publicPath, publicHandler := readingv1connect.NewPublicLibraryServiceHandler(
		&publicConnectHandler{app: a},
		scrub,
	)
	mux.Handle("POST "+publicPath, publicHandler)
}

// legacyPrefix is the app's pre-rename URL prefix. Kobo devices registered
// before the rename have "/books/kobo/{token}" baked into their firmware's
// api_endpoint, and device metadata may hold "/books/…" cover URLs — both
// route groups therefore stay mounted under the old prefix permanently so
// no device ever needs to be set up again.
const legacyPrefix = "books"

func (a *Reading) Routes(prefix string, mux *http.ServeMux) {
	a.booksRoutes(prefix, mux)
	a.coverRoutes(prefix, mux)
	a.koboRoutes(prefix, mux)
	if prefix != legacyPrefix {
		a.coverRoutes(legacyPrefix, mux)
		a.koboRoutes(legacyPrefix, mux)
	}
}
