package main

import (
	"errors"
	"net/http"

	tpltools "github.com/xdoubleu/essentia/v3/pkg/tpl"
	"tools.xdoubleu.com/apps/backlog"
)

func (app *Application) onboardingHandler(w http.ResponseWriter, _ *http.Request) {
	tpltools.RenderWithPanic(app.tpl, w, "onboarding.html", nil)
}

func (app *Application) saveOnboardingHandler(w http.ResponseWriter, r *http.Request) {
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

	integrations := backlog.Integrations{
		SteamAPIKey:  r.FormValue("steam_api_key"),
		SteamUserID:  r.FormValue("steam_user_id"),
		GoodreadsURL: r.FormValue("goodreads_url"),
	}

	if err := app.backlog.SaveIntegrations(
		r.Context(), user.ID, integrations,
	); err != nil {
		http.Error(w, "Failed to save settings", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/backlog", http.StatusSeeOther)
}
