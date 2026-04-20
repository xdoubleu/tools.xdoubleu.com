package main

import (
	"errors"
	"net/http"

	"github.com/xdoubleu/essentia/v3/pkg/contexttools"
	tpltools "github.com/xdoubleu/essentia/v3/pkg/tpl"
	goaltracker "tools.xdoubleu.com/apps/goaltracker"
	"tools.xdoubleu.com/internal/constants"
	"tools.xdoubleu.com/internal/models"
)

func currentUser(r *http.Request) *models.User {
	return contexttools.GetValue[models.User](r.Context(), constants.UserContextKey)
}

func (app *Application) settingsHandler(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	if user == nil {
		panic(errors.New("not signed in"))
	}

	integrations, err := app.goalTracker.GetIntegrations(r.Context(), user.ID)
	if err != nil {
		http.Error(w, "Failed to load settings", http.StatusInternalServerError)
		return
	}

	tpltools.RenderWithPanic(app.tpl, w, "settings.html", map[string]any{
		"Integrations": integrations,
		"Saved":        r.URL.Query().Get("saved") == "1",
	})
}

//nolint:dupl //similar to saveOnboardingHandler but redirects to /settings?saved=1
func (app *Application) saveSettingsHandler(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	if user == nil {
		panic(errors.New("not signed in"))
	}

	//nolint:mnd //no magic number
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	if err := r.ParseForm(); err != nil {
		http.Error(w, "request too large", http.StatusRequestEntityTooLarge)
		return
	}

	integrations := goaltracker.Integrations{
		TodoistAPIKey:    r.FormValue("todoist_api_key"),
		TodoistProjectID: r.FormValue("todoist_project_id"),
		SteamAPIKey:      r.FormValue("steam_api_key"),
		SteamUserID:      r.FormValue("steam_user_id"),
		GoodreadsURL:     r.FormValue("goodreads_url"),
	}

	if err := app.goalTracker.SaveIntegrations(
		r.Context(), user.ID, integrations,
	); err != nil {
		http.Error(w, "Failed to save settings", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/settings?saved=1", http.StatusSeeOther)
}
