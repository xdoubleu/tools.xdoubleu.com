package learningpaths

import (
	"log/slog"
	"net/http"
)

// todoistOAuthCallbackRoute completes the per-user Todoist OAuth flow
// (issue #1475) — the browser-facing leg Todoist's own redirect invokes
// with ?code=&state=, mirroring api/cmd/api/oauth_admin.go's shape for the
// admin-scoped GitHub/Sentry integrations. Mounted in routes.go, gated by
// Auth.Access.
func (a *LearningPaths) todoistOAuthCallbackRoute() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		state := r.URL.Query().Get("state")
		code := r.URL.Query().Get("code")

		_, err := a.services.Todoist.HandleCallback(r.Context(), state, code)
		if err != nil {
			a.Logger.ErrorContext(r.Context(), "todoist oauth callback failed",
				slog.Any("error", err))
			http.Redirect(
				w, r,
				a.Config.WebURL+"/learningpaths/settings?todoist_error=1",
				http.StatusFound,
			)
			return
		}

		http.Redirect(
			w, r,
			a.Config.WebURL+"/learningpaths/settings?todoist_connected=1",
			http.StatusFound,
		)
	}
}
