package main

import (
	"net/http"
)

func (app *Application) onboardingHandler(w http.ResponseWriter, r *http.Request) {
	_ = OnboardingPage().Render(r.Context(), w)
}

func (app *Application) saveOnboardingHandler(w http.ResponseWriter, r *http.Request) {
	app.saveIntegrations(w, r, "/backlog")
}
