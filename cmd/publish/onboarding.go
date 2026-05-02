package main

import (
	"net/http"

	tpltools "github.com/xdoubleu/essentia/v4/pkg/tpl"
)

func (app *Application) onboardingHandler(w http.ResponseWriter, _ *http.Request) {
	tpltools.RenderWithPanic(app.tpl, w, "onboarding.html", nil)
}

func (app *Application) saveOnboardingHandler(w http.ResponseWriter, r *http.Request) {
	app.saveIntegrations(w, r, "/backlog")
}
