package dashboard

import (
	"net/http"

	"connectrpc.com/connect/v2"

	dashboardv1connect "tools.xdoubleu.com/gen/dashboard/v1/dashboardv1connect"
	iapp "tools.xdoubleu.com/internal/app"
)

// Routes registers the two public dashboard RPCs, gated by share token rather
// than auth. The owner-facing token CRUD is registered in cmd/api/routes.go
// behind auth.Access.
func (a *Dashboard) Routes(_ string, mux *http.ServeMux) {
	iapp.MountConnect(mux, a.Logger, nil, func(s *connect.Server) {
		handler := &publicConnectHandler{app: a}
		dashboardv1connect.RegisterPublicGamesDashboardServiceHandler(s, handler)
		dashboardv1connect.RegisterPublicReadingDashboardServiceHandler(s, handler)
	})
}
