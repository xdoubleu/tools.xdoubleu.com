package trains

import (
	"fmt"
	"net/http"

	"tools.xdoubleu.com/gen/trains/v1/trainsv1connect"
	iapp "tools.xdoubleu.com/internal/app"
)

// Routes registers trains.v1.TrainService behind trains AppAccess, plus the
// live-journey websocket at /trains/api/journeys/live.
func (a *Trains) Routes(prefix string, mux *http.ServeMux) {
	trainsPath, trainsHandler := trainsv1connect.NewTrainServiceHandler(
		&trainsConnectHandler{app: a},
		iapp.ScrubInternalErrors(a.Logger),
	)
	mux.Handle(
		fmt.Sprintf("POST %s", trainsPath),
		a.Auth.AppAccess(prefix, trainsHandler.ServeHTTP),
	)
	mux.HandleFunc(
		fmt.Sprintf("GET /%s/api/journeys/live", prefix),
		a.Auth.AppAccess(prefix, a.Services.JourneyWS.Handler()),
	)
}
