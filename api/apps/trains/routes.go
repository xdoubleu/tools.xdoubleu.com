package trains

import (
	"fmt"
	"net/http"

	"tools.xdoubleu.com/gen/trains/v1/trainsv1connect"
	iapp "tools.xdoubleu.com/internal/app"
)

// Routes registers trains.v1.TrainService — journey search over the
// ingested timetable, gated by the trains app's own AppAccess (issue
// #1391, following #1390's app shell) — plus the per-journey live-update
// websocket at /trains/api/journeys/live (issue #1394), following the same
// wstools shape games/books use for job progress.
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
