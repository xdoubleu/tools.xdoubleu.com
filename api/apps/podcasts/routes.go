package podcasts

import (
	"net/http"

	"connectrpc.com/connect/v2"

	"tools.xdoubleu.com/gen/podcasts/v1/podcastsv1connect"
	iapp "tools.xdoubleu.com/internal/app"
)

func (a *Podcasts) Routes(prefix string, mux *http.ServeMux) {
	iapp.MountConnect(
		mux,
		a.Logger,
		func(next http.HandlerFunc) http.HandlerFunc {
			return a.services.Auth.AppAccess(prefix, next)
		},
		func(s *connect.Server) {
			podcastsv1connect.RegisterPodcastsServiceHandler(
				s,
				&podcastsConnectHandler{app: a},
			)
		},
	)
}
