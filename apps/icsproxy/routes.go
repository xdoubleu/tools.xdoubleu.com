package icsproxy

import (
	"fmt"
	"net/http"
)

func (app *ICSProxy) Routes(prefix string, mux *http.ServeMux) {

	mux.HandleFunc(
		fmt.Sprintf("GET /%s", prefix),
		app.services.Auth.TemplateAccess(app.indexHandler),
	)

	mux.HandleFunc(
		fmt.Sprintf("POST /%s/preview", prefix),
		app.services.Auth.TemplateAccess(app.previewHandler),
	)

	mux.HandleFunc(
		fmt.Sprintf("POST /%s/create", prefix),
		app.services.Auth.TemplateAccess(app.createHandler),
	)

	// Edit existing filters
	mux.HandleFunc(
		fmt.Sprintf("GET /%s/edit/", prefix),
		app.services.Auth.TemplateAccess(app.editHandler),
	)

	// Delete existing filters
	mux.HandleFunc(
		fmt.Sprintf("POST /%s/delete/", prefix),
		app.services.Auth.TemplateAccess(app.deleteHandler),
	)

	// Feed endpoint (must stay last)
	mux.HandleFunc(fmt.Sprintf("GET /%s/", prefix), app.feedHandler)
}

func (app *ICSProxy) feedHandler(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	var token string

	_, err := fmt.Sscanf(path, "/icsproxy/%s", &token)
	if err != nil {
		http.Error(w, "Invalid feed URL", http.StatusBadRequest)
		return
	}

	// strip ".ics"
	if len(token) > 4 && token[len(token)-4:] == ".ics" {
		token = token[:len(token)-4]
	}

	cfg, ok := app.services.Calendar.LoadConfig(r.Context(), token)
	if !ok {
		http.Error(w, "Feed not found", http.StatusNotFound)
		return
	}

	data, err := app.services.Calendar.FetchICS(r.Context(), cfg.SourceURL)
	if err != nil {
		http.Error(w, "Failed to fetch source calendar", http.StatusBadGateway)
		return
	}

	filtered, err := app.services.Calendar.ApplyFilter(data, cfg)
	if err != nil {
		http.Error(w, "Failed to process calendar", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/calendar")
	_, err = w.Write(filtered)
	if err != nil {
		app.logger.Error("Failed to write filtered calendar", "error", err)
	}
}
