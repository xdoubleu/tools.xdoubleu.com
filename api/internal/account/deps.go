package account

import (
	"log/slog"
	"net/http"

	"connectrpc.com/connect/v2"

	accessv1connect "tools.xdoubleu.com/gen/access/v1/accessv1connect"
	authv1connect "tools.xdoubleu.com/gen/auth/v1/authv1connect"
	dashboardv1connect "tools.xdoubleu.com/gen/dashboard/v1/dashboardv1connect"
	familyv1connect "tools.xdoubleu.com/gen/family/v1/familyv1connect"
	iapp "tools.xdoubleu.com/internal/app"
	"tools.xdoubleu.com/internal/auth"
	"tools.xdoubleu.com/internal/config"
	"tools.xdoubleu.com/internal/family"
	"tools.xdoubleu.com/internal/repositories"
)

// Deps is the account module's slice of the Application: the auth service,
// its repositories and config the auth/access/family/dashboard RPCs need.
type Deps struct {
	Auth     *auth.LocalService
	Config   config.Config
	AppUsers *repositories.AppUsersRepository
	Family   family.Service
	Shares   *repositories.ProfileSharesRepository
}

// Register mounts the account Connect handlers: AuthService is public
// (session endpoints), the rest are gated by the same app access gate the
// other apps use.
func Register(
	mux *http.ServeMux,
	logger *slog.Logger,
	authGate, accessGate iapp.Gate,
	d *Deps,
) {
	iapp.MountConnect(mux, logger, authGate,
		func(s *connect.Server) {
			authv1connect.RegisterAuthServiceHandler(s, &authConnectHandler{app: d})
		})
	iapp.MountConnect(mux, logger, accessGate,
		func(s *connect.Server) {
			accessv1connect.RegisterAccessServiceHandler(
				s, &accessConnectHandler{app: d},
			)
			familyv1connect.RegisterFamilyServiceHandler(
				s, &familyConnectHandler{app: d},
			)
			dashboardv1connect.RegisterDashboardServiceHandler(
				s, &dashboardConnectHandler{app: d},
			)
		})
}
