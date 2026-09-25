package learningpaths

import (
	"log/slog"
	"net/http"
)

// todoistOAuthCallbackRoute is the browser leg of the per-user Todoist OAuth
// flow (?code=&state=), mounted behind Auth.Access in routes.go.
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
