package main

import (
	"context"
	"net/http"
	"time"

	httptools "tools.xdoubleu.com/internal/communication/httptools"
)

const healthCheckTimeout = 5 * time.Second

// healthPath is hit by Kamal's healthcheck over the Docker network, bypassing
// kamal-proxy, so stripAPIPathPrefix doesn't apply.
const healthPath = "/health"

func (app *Application) healthHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), healthCheckTimeout)
	defer cancel()

	if err := app.db.Ping(ctx); err != nil {
		http.Error(w, "database unreachable", http.StatusServiceUnavailable)
		return
	}

	if err := httptools.WriteJSON(
		w,
		http.StatusOK,
		healthBody(app.disabledApps),
		nil,
	); err != nil {
		httptools.HandleError(w, r, err)
	}
}

// healthBody reports degraded, still with a 200 so Kamal's healthcheck passes
// and a deploy with one broken app goes out.
func healthBody(disabled DisabledApps) map[string]any {
	if len(disabled) == 0 {
		return map[string]any{"status": "ok"}
	}
	return map[string]any{"status": "degraded", "disabled_apps": disabled.Names()}
}
