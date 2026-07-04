package mealplans

import (
	"fmt"
	"net/http"

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

	mealplansPath, mealplansHandler := mealplansv1connect.NewMealPlansServiceHandler(
		&mealplansConnectHandler{app: a},
		iapp.ScrubInternalErrors(a.Logger),
	)
	mux.Handle(
		"POST "+mealplansPath,
		a.services.Auth.AppAccess(prefix, mealplansHandler.ServeHTTP),
	)
}
