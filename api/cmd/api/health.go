package main

import (
	"context"
	"net/http"
	"strings"
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

	body := map[string]string{"status": "ok"}
	if len(app.disabledApps) > 0 {
		names := make([]string, len(app.disabledApps))
		for i, f := range app.disabledApps {
			names[i] = f.App
		}
		body["status"] = "degraded"
		body["disabled"] = strings.Join(names, ",")
	}

	if err := httptools.WriteJSON(
		w,
		http.StatusOK,
		body,
		nil,
	); err != nil {
		httptools.HandleError(w, r, err)
	}
}
