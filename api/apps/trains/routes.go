package trains

import (
	"fmt"
	"net/http"

	connectv2 "connectrpc.com/connect/v2"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"tools.xdoubleu.com/apps/trains/connect"
	trainsmcp "tools.xdoubleu.com/apps/trains/mcp"
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
		func(s *connectv2.Server) {
			trainsv1connect.RegisterTrainServiceHandler(
				s,
				connect.NewHandler(a.Services),
			)
		},
	)
	mux.HandleFunc(
		fmt.Sprintf("GET /%s/api/journeys/live", prefix),
		a.Auth.AppAccess(prefix, a.journeyWS.Handler()),
	)
}

// RegisterMCPTools exposes the trains read RPCs, which are public timetable
// data for any caller with trains access.
func (a *Trains) RegisterMCPTools(srv *mcp.Server) {
	trainsmcp.Register(srv, a.Services)
}
