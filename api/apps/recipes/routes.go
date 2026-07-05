package recipes

import (
	"fmt"
	"net/http"

	"tools.xdoubleu.com/gen/recipes/v1/recipesv1connect"
	iapp "tools.xdoubleu.com/internal/app"
)

func (a *Recipes) Routes(prefix string, mux *http.ServeMux) {
	recipesPath, recipesHandler := recipesv1connect.NewRecipesServiceHandler(
		&recipesConnectHandler{app: a},
		iapp.ScrubInternalErrors(a.Logger),
	)
	mux.Handle(
		fmt.Sprintf("POST %s", recipesPath),
		a.services.Auth.AppAccess(prefix, recipesHandler.ServeHTTP),
	)
}
