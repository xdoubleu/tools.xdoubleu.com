package icsproxy

import (
	"fmt"
	"net/http"

	icsproxyv1connect "tools.xdoubleu.com/gen/icsproxy/v1/icsproxyv1connect"
	iapp "tools.xdoubleu.com/internal/app"
)

func (app *ICSProxy) Routes(prefix string, mux *http.ServeMux) {
	// ICS feed — NOT migratable to ConnectRPC (serves text/calendar)
	mux.HandleFunc(fmt.Sprintf("GET /%s/", prefix), app.feedHandler)

	path, handler := icsproxyv1connect.NewICSProxyServiceHandler(
		&icsProxyConnectHandler{app: app},
		iapp.ScrubInternalErrors(app.Logger),
	)
	mux.Handle(
		"POST "+path,
		app.services.Auth.AppAccess(prefix, handler.ServeHTTP),
	)
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

	filtered, err := app.services.Calendar.ApplyFilter(r.Context(), data, cfg)
	if err != nil {
		http.Error(w, "Failed to process calendar", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/calendar")
	_, err = w.Write(filtered)
	if err != nil {
		app.Logger.ErrorContext(
			r.Context(),
			"Failed to write filtered calendar",
			"error",
			err,
		)
	}
}
