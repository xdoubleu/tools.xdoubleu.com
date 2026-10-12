package taste

import (
	"fmt"
	"net/http"

	"tools.xdoubleu.com/gen/taste/v1/tastev1connect"
	iapp "tools.xdoubleu.com/internal/app"
)

func (a *Taste) Routes(prefix string, mux *http.ServeMux) {
	path, handler := tastev1connect.NewTasteServiceHandler(
		&tasteConnectHandler{app: a},
		iapp.ScrubInternalErrors(a.Logger),
	)
	mux.Handle(
		fmt.Sprintf("POST %s", path),
		a.auth.AppAccess(prefix, handler.ServeHTTP),
	)
}
