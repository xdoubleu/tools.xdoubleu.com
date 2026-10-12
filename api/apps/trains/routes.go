package trains

import (
	"fmt"
	"net/http"

	"connectrpc.com/connect/v2"

	"tools.xdoubleu.com/gen/trains/v1/trainsv1connect"
	iapp "tools.xdoubleu.com/internal/app"
)

// Routes registers trains.v1.TrainService behind trains AppAccess, plus the
// live-journey websocket at /trains/api/journeys/live.
func (a *Trains) Routes(prefix string, mux *http.ServeMux) {
	iapp.MountConnect(
		mux,
		a.Logger,
		func(next http.HandlerFunc) http.HandlerFunc {
			return a.Auth.AppAccess(prefix, next)
		},
		func(s *connect.Server) {
			trainsv1connect.RegisterTrainServiceHandler(
				s,
				&trainsConnectHandler{app: a},
			)
		},
	)
	mux.HandleFunc(
		fmt.Sprintf("GET /%s/api/journeys/live", prefix),
		a.Auth.AppAccess(prefix, a.Services.JourneyWS.Handler()),
	)
}
