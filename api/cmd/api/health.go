package main

import (
	"context"
	"net/http"
	"time"

	httptools "tools.xdoubleu.com/internal/communication/httptools"
)

const healthCheckTimeout = 5 * time.Second

// healthPath is checked directly by Kamal's own deploy-time healthcheck
// (config/deploy.api.yml's proxy.healthcheck.path), which hits this
// container over the internal Docker network — entirely bypassing
// kamal-proxy's public path-prefix routing, so it's unaffected by
// stripAPIPathPrefix (kamal_proxy_shim.go).
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
		map[string]string{"status": "ok"},
		nil,
	); err != nil {
		httptools.HandleError(w, r, err)
	}
}
