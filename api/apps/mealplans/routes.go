package mealplans

import (
	"fmt"
	"net/http"

	connectv2 "connectrpc.com/connect/v2"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"tools.xdoubleu.com/apps/mealplans/connect"
	mealplanshttp "tools.xdoubleu.com/apps/mealplans/http"
	mealplansmcp "tools.xdoubleu.com/apps/mealplans/mcp"
	"tools.xdoubleu.com/gen/mealplans/v1/mealplansv1connect"
	iapp "tools.xdoubleu.com/internal/app"
)

func (a *MealPlans) Routes(prefix string, mux *http.ServeMux) {
	p := fmt.Sprintf("/%s", prefix)

	// iCal feed (public, no auth) — must precede ConnectRPC registration.
	mux.Handle(
		fmt.Sprintf("GET %s/ical/", p),
		mealplanshttp.NewHandler(a.services.Plans, a.Logger),
	)

	iapp.MountConnect(
		mux,
		a.Logger,
		func(next http.HandlerFunc) http.HandlerFunc {
			return a.services.Auth.AppAccess(prefix, next)
		},
		func(s *connectv2.Server) {
			mealplansv1connect.RegisterMealPlansServiceHandler(
				s,
				connect.NewHandler(a.services.Plans),
			)
		},
	)
}

// RegisterMCPTools exposes the app's read-only RPCs as MCP tools.
func (a *MealPlans) RegisterMCPTools(srv *mcp.Server) {
	mealplansmcp.Register(srv, a.services.Plans)
}
