package feeds

import (
	"net/http"

	"tools.xdoubleu.com/gen/feeds/v1/feedsv1connect"
	iapp "tools.xdoubleu.com/internal/app"
)

func (a *Feeds) Routes(prefix string, mux *http.ServeMux) {
	handler := &feedsConnectHandler{app: a}
	scrub := iapp.ScrubInternalErrors(a.Logger)

	feedsPath, feedsHandler := feedsv1connect.NewFeedServiceHandler(handler, scrub)
	mux.Handle(
		"POST "+feedsPath,
		a.Services.Auth.AppAccess(prefix, feedsHandler.ServeHTTP),
	)

	a.emailRoutes(prefix, mux)
}
