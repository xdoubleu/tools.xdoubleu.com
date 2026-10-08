package podcasts

import (
	"fmt"
	"net/http"

	"tools.xdoubleu.com/gen/podcasts/v1/podcastsv1connect"
	iapp "tools.xdoubleu.com/internal/app"
)

func (a *Podcasts) Routes(prefix string, mux *http.ServeMux) {
	path, handler := podcastsv1connect.NewPodcastsServiceHandler(
		&podcastsConnectHandler{app: a},
		iapp.ScrubInternalErrors(a.Logger),
	)
	mux.Handle(
		fmt.Sprintf("POST %s", path),
		a.services.Auth.AppAccess(prefix, handler.ServeHTTP),
	)
}
