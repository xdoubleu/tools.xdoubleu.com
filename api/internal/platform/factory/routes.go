package factory

import (
	"net/http"

	connectv2 "connectrpc.com/connect/v2"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	iapp "tools.xdoubleu.com/internal/app"
	fconnect "tools.xdoubleu.com/internal/platform/factory/connect"
	fhttp "tools.xdoubleu.com/internal/platform/factory/http"
	fmcp "tools.xdoubleu.com/internal/platform/factory/mcp"
)

func (a *Factory) Routes(prefix string, mux *http.ServeMux) {
	deps := &fconnect.Deps{
		Services:      a.services,
		Logger:        a.Logger,
		HeartbeatHold: a.heartbeatHold,
		StepUp:        a.stepUp,
	}
	webhook := &fhttp.Webhook{
		Jobs: a.services.Jobs, Logger: a.Logger, Secret: a.webhookSecret,
	}
	// GitHub signs its deliveries (Webhook checks it), so no cookie gate.
	mux.HandleFunc("POST /"+prefix+"/github-webhook", webhook.Handle)
	iapp.MountConnect(
		mux,
		a.Logger,
		func(next http.HandlerFunc) http.HandlerFunc {
			return a.services.Auth.AppAccess(prefix, next)
		},
		func(s *connectv2.Server) { fconnect.RegisterAdmin(s, deps) },
	)
	// The runner authenticates itself (pairing code, device token), so this
	// mount has no cookie gate.
	iapp.MountConnect(mux, a.Logger, nil, func(s *connectv2.Server) {
		fconnect.RegisterRunner(s, deps)
	})
}

// RegisterMCPTools exposes the board to agent accounts (docs/adr-0040).
func (a *Factory) RegisterMCPTools(srv *mcp.Server) {
	fmcp.Register(srv, a.services)
}
