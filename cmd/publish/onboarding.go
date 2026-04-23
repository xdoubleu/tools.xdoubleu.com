package main

import (
	"errors"
	"net/http"

	httptools "github.com/xdoubleu/essentia/v3/pkg/communication/httptools"
	tpltools "github.com/xdoubleu/essentia/v3/pkg/tpl"
	"tools.xdoubleu.com/apps/backlog"
	"tools.xdoubleu.com/cmd/publish/internal/dtos"
)

func (app *Application) onboardingHandler(w http.ResponseWriter, _ *http.Request) {
	tpltools.RenderWithPanic(app.tpl, w, "onboarding.html", nil)
}

func (app *Application) saveOnboardingHandler(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	if user == nil {
		panic(errors.New("not signed in"))
	}

	var dto dtos.IntegrationsDto
	if err := httptools.ReadForm(r, &dto); err != nil {
		httptools.HandleError(w, r, err)
		return
	}

	integrations := backlog.Integrations{
		SteamAPIKey:  dto.SteamAPIKey,
		SteamUserID:  dto.SteamUserID,
		GoodreadsURL: dto.GoodreadsURL,
	}

	if err := app.backlog.SaveIntegrations(
		r.Context(), user.ID, integrations,
	); err != nil {
		http.Error(w, "Failed to save settings", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/backlog", http.StatusSeeOther)
}
