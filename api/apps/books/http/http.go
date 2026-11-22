// Package http holds the books app's plain-HTTP adapters: the Kobo sync
// protocol, the Readium web publication routes and the cover proxy. Rules live
// in services; handlers authenticate, parse and write.
package http

import (
	"log/slog"
	"net/http"

	"tools.xdoubleu.com/apps/books/internal/services"
	"tools.xdoubleu.com/apps/books/kobo"
)

// Handlers serves the books app's non-Connect HTTP routes.
type Handlers struct {
	services *services.Services
	kobo     *kobo.Protocol
	logger   *slog.Logger
}

// New returns the HTTP handlers over svc and the Kobo protocol.
func New(
	svc *services.Services,
	protocol *kobo.Protocol,
	logger *slog.Logger,
) *Handlers {
	return &Handlers{services: svc, kobo: protocol, logger: logger}
}

// Register mounts every route under /{prefix}/.
func (app *Handlers) Register(prefix string, mux *http.ServeMux) {
	app.coverRoutes(prefix, mux)
	app.webPubRoutes(prefix, mux)
	app.koboRoutes(prefix, mux)
}
