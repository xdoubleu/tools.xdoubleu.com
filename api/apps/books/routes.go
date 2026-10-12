package books

import (
	"net/http"

	"connectrpc.com/connect/v2"

	booksv1connect "tools.xdoubleu.com/gen/books/v1/booksv1connect"
	iapp "tools.xdoubleu.com/internal/app"
)

func (a *Books) booksRoutes(prefix string, mux *http.ServeMux) {
	mux.HandleFunc(
		"GET /"+prefix+"/api/progress",
		a.Services.Auth.Access(a.Services.WebSocket.Handler()),
	)

	iapp.MountConnect(
		mux,
		a.Logger,
		func(next http.HandlerFunc) http.HandlerFunc {
			return a.Services.Auth.AppAccess(prefix, next)
		},
		func(s *connect.Server) {
			handler := &booksConnectHandler{app: a}
			booksv1connect.RegisterLibraryServiceHandler(s, handler)
			booksv1connect.RegisterBookFilesServiceHandler(s, handler)
			booksv1connect.RegisterKoboServiceHandler(s, handler)
			booksv1connect.RegisterCatalogServiceHandler(s, handler)
		},
	)
}

func (a *Books) Routes(prefix string, mux *http.ServeMux) {
	a.booksRoutes(prefix, mux)
	a.coverRoutes(prefix, mux)
	a.webPubRoutes(prefix, mux)
	a.koboRoutes(prefix, mux)
}
