package backlog

import (
	"net/http"
	"time"

	"github.com/xdoubleu/essentia/v4/pkg/contexttools"
	tpltools "github.com/xdoubleu/essentia/v4/pkg/tpl"
	"tools.xdoubleu.com/apps/backlog/internal/models"
	"tools.xdoubleu.com/internal/constants"
	sharedmodels "tools.xdoubleu.com/internal/models"
)

func currentBacklogUser(r *http.Request) *sharedmodels.User {
	return contexttools.GetValue[sharedmodels.User](
		r.Context(),
		constants.UserContextKey,
	)
}

func parseDateRange(r *http.Request) (time.Time, time.Time) {
	end := time.Now()
	start := end.AddDate(-1, 0, 0)

	if v := r.URL.Query().Get("from"); v != "" {
		if t, err := time.Parse(models.ProgressDateFormat, v); err == nil {
			start = t
		}
	}
	if v := r.URL.Query().Get("to"); v != "" {
		if t, err := time.Parse(models.ProgressDateFormat, v); err == nil {
			end = t
		}
	}
	return start, end
}

func (app *Backlog) rootHandler(w http.ResponseWriter, r *http.Request) error {
	user := currentBacklogUser(r)
	if user == nil {
		return httpError(http.StatusUnauthorized, "Sign in to access this page")
	}

	onboarded, err := app.HasCompletedOnboarding(r.Context(), user.ID)
	if err != nil {
		return err
	}
	if !onboarded {
		http.Redirect(w, r, "/onboarding", http.StatusSeeOther)
		return nil
	}

	data, err := app.Services.Backlog.GetSummary(r.Context(), user.ID)
	if err != nil {
		return err
	}

	tpltools.RenderWithPanic(app.Tpl, w, "root.html", data)
	return nil
}

func (app *Backlog) userBacklogHandler(w http.ResponseWriter, r *http.Request) error {
	targetUserID := r.PathValue("userID")

	data, err := app.Services.Backlog.GetSummary(r.Context(), targetUserID)
	if err != nil {
		return err
	}

	tpltools.RenderWithPanic(app.Tpl, w, "root.html", data)
	return nil
}

func (app *Backlog) refreshHandler(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	app.Services.WebSocket.ForceRun(id)
	w.WriteHeader(http.StatusNoContent)
}
