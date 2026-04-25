package main

import (
	"errors"
	"net/http"
	"strconv"

	httptools "github.com/xdoubleu/essentia/v3/pkg/communication/httptools"
	"github.com/xdoubleu/essentia/v3/pkg/contexttools"
	tpltools "github.com/xdoubleu/essentia/v3/pkg/tpl"
	"tools.xdoubleu.com/apps/backlog"
	"tools.xdoubleu.com/cmd/publish/internal/dtos"
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

	integrations, err := app.backlog.GetIntegrations(r.Context(), user.ID)
	if err != nil {
		http.Error(w, "Failed to load settings", http.StatusInternalServerError)
		return
	}

	var importedCount *int
	if v := r.URL.Query().Get("imported"); v != "" {
		if n, convErr := strconv.Atoi(v); convErr == nil {
			importedCount = &n
		}
	}

	tpltools.RenderWithPanic(app.tpl, w, "settings.html", map[string]any{
		"Integrations":  integrations,
		"Saved":         r.URL.Query().Get("saved") == "1",
		"ImportedCount": importedCount,
	})
}

func (app *Application) saveSettingsHandler(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	if user == nil {
		panic(errors.New("not signed in"))
	}

	var dto dtos.IntegrationsDto
	if err := httptools.ReadForm(r, &dto); err != nil {
		httptools.HandleError(w, r, err)
		return
	}

	if ok, errs := dto.Validate(); !ok {
		httptools.FailedValidationResponse(w, r, errs)
		return
	}

	integrations := backlog.Integrations{
		SteamAPIKey:     dto.SteamAPIKey,
		SteamUserID:     dto.SteamUserID,
		HardcoverAPIKey: dto.HardcoverAPIKey,
	}

	if err := app.backlog.SaveIntegrations(
		r.Context(), user.ID, integrations,
	); err != nil {
		http.Error(w, "Failed to save settings", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/settings?saved=1", http.StatusSeeOther)
}
