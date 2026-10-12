package shoppinglist

import (
	"net/http"

	"connectrpc.com/connect/v2"

	"tools.xdoubleu.com/gen/shoppinglist/v1/shoppinglistv1connect"
	iapp "tools.xdoubleu.com/internal/app"
)

func (a *ShoppingList) Routes(prefix string, mux *http.ServeMux) {
	iapp.MountConnect(
		mux,
		a.Logger,
		func(next http.HandlerFunc) http.HandlerFunc {
			return a.services.Auth.AppAccess(prefix, next)
		},
		func(s *connect.Server) {
			shoppinglistv1connect.RegisterShoppingListServiceHandler(
				s,
				&shoppingConnectHandler{app: a},
			)
		},
	)
}
