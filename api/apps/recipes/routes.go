package recipes

import (
	"fmt"
	"net/http"

	"tools.xdoubleu.com/gen/recipes/v1/recipesv1connect"
)

func (a *Recipes) Routes(prefix string, mux *http.ServeMux) {
	p := fmt.Sprintf("/%s", prefix)

	// ── iCal feed (public, no auth) ────────────────────────────────────────────
	// Must be registered before ConnectRPC as it's a special text/calendar endpoint.
	mux.HandleFunc(
		fmt.Sprintf("GET %s/ical/", p),
		a.icalFeedHandler,
	)

	recipesPath, recipesHandler := recipesv1connect.NewRecipesServiceHandler(
		&recipesConnectHandler{app: a},
	)
	mux.Handle(
		"POST "+recipesPath,
		a.services.Auth.AppAccess(prefix, recipesHandler.ServeHTTP),
	)

	mealplansPath, mealplansHandler := recipesv1connect.NewMealPlansServiceHandler(
		&mealplansConnectHandler{app: a},
	)
	mux.Handle(
		"POST "+mealplansPath,
		a.services.Auth.AppAccess(prefix, mealplansHandler.ServeHTTP),
	)
}
