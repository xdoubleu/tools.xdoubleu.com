package dashboard

import (
	"net/http"

	dashboardv1connect "tools.xdoubleu.com/gen/dashboard/v1/dashboardv1connect"
	iapp "tools.xdoubleu.com/internal/app"
)

// Routes registers only the two public, token-gated dashboard RPCs —
// deliberately NOT wrapped in auth middleware, since access is gated by the
// dashboard share token instead (resolved in connect_public.go). The
// owner-facing token CRUD (dashboard.v1.DashboardService) is registered
// separately in cmd/api/routes.go behind generic auth.Access, same as every
// other logged-in-user-scoped RPC (e.g. contacts) — see api/CLAUDE.md's
// "Public Dashboard Sharing" section for why it isn't gated by this app's
// own AppAccess.
func (a *Dashboard) Routes(_ string, mux *http.ServeMux) {
	scrub := iapp.ScrubInternalErrors(a.Logger)
	handler := &publicConnectHandler{app: a}

	gamesPath, gamesHandler := dashboardv1connect.NewPublicGamesDashboardServiceHandler(
		handler,
		scrub,
	)
	mux.Handle("POST "+gamesPath, gamesHandler)

	readingPath, readingHandler := dashboardv1connect.NewPublicReadingDashboardServiceHandler(
		handler,
		scrub,
	)
	mux.Handle("POST "+readingPath, readingHandler)
}
