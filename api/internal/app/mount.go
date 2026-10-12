package app

import (
	"log/slog"
	"net/http"

	"connectrpc.com/connect/v2"
	"connectrpc.com/connect/v2/connecthttp"
)

// Gate wraps an HTTP handler, e.g. with auth middleware.
type Gate func(http.HandlerFunc) http.HandlerFunc

// MountConnect serves the services added by register on mux as POST routes,
// each procedure behind gate (nil mounts them ungated) and with
// [ScrubInternalErrors] applied.
func MountConnect(
	mux *http.ServeMux,
	logger *slog.Logger,
	gate Gate,
	register func(*connect.Server),
) {
	server := connect.NewServer(ScrubInternalErrors(logger))
	register(server)
	connecthttp.Mount(gatedMux{mux: mux, gate: gate}, server)
}

type gatedMux struct {
	mux  *http.ServeMux
	gate Gate
}

func (m gatedMux) Handle(pattern string, handler http.Handler) {
	if m.gate != nil {
		handler = m.gate(handler.ServeHTTP)
	}
	m.mux.Handle("POST "+pattern, handler)
}
