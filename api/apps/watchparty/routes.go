package watchparty

import (
	"fmt"
	"net/http"

	watchpartyv1connect "tools.xdoubleu.com/gen/watchparty/v1/watchpartyv1connect"
)

func (app *WatchParty) Routes(prefix string, mux *http.ServeMux) {
	// WebSocket routes
	apiPrefix := fmt.Sprintf("/%s/api", prefix)
	app.wsRoutes(apiPrefix, mux)

	// ConnectRPC routes
	path, handler := watchpartyv1connect.NewRoomServiceHandler(
		&roomConnectHandler{app: app},
	)
	mux.Handle(
		"POST "+path,
		app.Services.Auth.Access(handler.ServeHTTP),
	)
}
