package mealplans

import (
	"fmt"
	"net/http"

	"connectrpc.com/connect/v2"

	"tools.xdoubleu.com/gen/mealplans/v1/mealplansv1connect"
	iapp "tools.xdoubleu.com/internal/app"
)

func (a *MealPlans) Routes(prefix string, mux *http.ServeMux) {
	p := fmt.Sprintf("/%s", prefix)

	// iCal feed (public, no auth) — must precede ConnectRPC registration.
	mux.HandleFunc(
		fmt.Sprintf("GET %s/ical/", p),
		a.icalFeedHandler,
	)

	iapp.MountConnect(
		mux,
		a.Logger,
		func(next http.HandlerFunc) http.HandlerFunc {
			return a.services.Auth.AppAccess(prefix, next)
		},
		func(s *connect.Server) {
			mealplansv1connect.RegisterMealPlansServiceHandler(
				s,
				&mealplansConnectHandler{app: a},
			)
		},
	)
}
