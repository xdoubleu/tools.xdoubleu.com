package recipes

import (
	"net/http"

	"connectrpc.com/connect/v2"

	"tools.xdoubleu.com/gen/recipes/v1/recipesv1connect"
	iapp "tools.xdoubleu.com/internal/app"
)

func (a *Recipes) Routes(prefix string, mux *http.ServeMux) {
	iapp.MountConnect(
		mux,
		a.Logger,
		func(next http.HandlerFunc) http.HandlerFunc {
			return a.services.Auth.AppAccess(prefix, next)
		},
		func(s *connect.Server) {
			recipesv1connect.RegisterRecipesServiceHandler(
				s,
				&recipesConnectHandler{app: a},
			)
		},
	)
}
