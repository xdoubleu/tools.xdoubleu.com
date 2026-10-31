package factory

import (
	"net/http"

	"connectrpc.com/connect/v2"

	"tools.xdoubleu.com/gen/factory/v1/factoryv1connect"
	iapp "tools.xdoubleu.com/internal/app"
)

func (a *Factory) Routes(prefix string, mux *http.ServeMux) {
	iapp.MountConnect(
		mux,
		a.Logger,
		func(next http.HandlerFunc) http.HandlerFunc {
			return a.services.Auth.AppAccess(prefix, next)
		},
		func(s *connect.Server) {
			factoryv1connect.RegisterTaskServiceHandler(
				s,
				&taskConnectHandler{app: a},
			)
		},
	)
}
