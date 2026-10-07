package movies

import (
	"fmt"
	"net/http"

	"tools.xdoubleu.com/gen/movies/v1/moviesv1connect"
	iapp "tools.xdoubleu.com/internal/app"
)

func (a *Movies) Routes(prefix string, mux *http.ServeMux) {
	moviesPath, moviesHandler := moviesv1connect.NewMoviesServiceHandler(
		&moviesConnectHandler{app: a},
		iapp.ScrubInternalErrors(a.Logger),
	)
	mux.Handle(
		fmt.Sprintf("POST %s", moviesPath),
		a.services.Auth.AppAccess(prefix, moviesHandler.ServeHTTP),
	)
}
