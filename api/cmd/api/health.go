package main

import (
	"context"
	"net/http"
	"time"

	httptools "github.com/xdoubleu/essentia/v4/pkg/communication/httptools"
)

const healthCheckTimeout = 5 * time.Second

// healthPath matches the literal gateway/internal/gateway/proxy.go routes
// unstripped to this api process rather than proxying it to the Next.js
// child — a routing contract between the two independently deployed
// binaries, not something shared via Go code across the module boundary.
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
