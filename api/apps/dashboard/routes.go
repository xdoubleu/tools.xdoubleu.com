package dashboard

import (
	"net/http"

	dashboardv1connect "tools.xdoubleu.com/gen/dashboard/v1/dashboardv1connect"
	iapp "tools.xdoubleu.com/internal/app"
)

// Routes registers the two public dashboard RPCs, gated by share token rather
// than auth. The owner-facing token CRUD is registered in cmd/api/routes.go
// behind auth.Access.
func (a *Dashboard) Routes(_ string, mux *http.ServeMux) {
	scrub := iapp.ScrubInternalErrors(a.Logger)
	handler := &publicConnectHandler{app: a}

	gamesPath, gamesHandler := dashboardv1connect.NewPublicGamesDashboardServiceHandler(
		handler,
		scrub,
	)
	mux.Handle("POST "+gamesPath, gamesHandler)

	readingPath, readingHandler := dashboardv1connect.
		NewPublicReadingDashboardServiceHandler(
			handler,
			scrub,
		)
	mux.Handle("POST "+readingPath, readingHandler)
}
