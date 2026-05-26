package shoppinglist

import (
	"net/http"

	"tools.xdoubleu.com/gen/shoppinglist/v1/shoppinglistv1connect"
)

func (a *ShoppingList) Routes(prefix string, mux *http.ServeMux) {
	shoppingPath, shoppingHandler := shoppinglistv1connect.NewShoppingListServiceHandler(
		&shoppingConnectHandler{app: a},
	)
	mux.Handle(
		"POST "+shoppingPath,
		a.services.Auth.AppAccess(prefix, shoppingHandler.ServeHTTP),
	)
}
