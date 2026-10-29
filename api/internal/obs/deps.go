package obs

import (
	"log/slog"
	"net/http"

	"connectrpc.com/connect/v2"

	"tools.xdoubleu.com/gen/observability/v1/observabilityv1connect"
	iapp "tools.xdoubleu.com/internal/app"
	"tools.xdoubleu.com/internal/config"
	"tools.xdoubleu.com/internal/github"
	"tools.xdoubleu.com/internal/oauthconn"
	"tools.xdoubleu.com/internal/repositories"
	"tools.xdoubleu.com/internal/sentryapi"
)

type obsConnectHandler struct {
	deps *Deps
}

var _ observabilityv1connect.ObservabilityServiceHandler = (*obsConnectHandler)(nil)

// StorageScanRunner is the slice of the books app TriggerStorageScan needs.
type StorageScanRunner interface {
	StartStorageScan()
}

// Deps is the observability module's slice of the Application: the
// repositories, clients, logger and config its RPCs, MCP tools and admin OAuth
// routes need. main builds it from the wiring it already holds.
type Deps struct {
	Logger   *slog.Logger
	Config   config.Config
	AppUsersRepo *repositories.AppUsersRepository
	JobRunsRepo  *repositories.JobRunsRepository
	UsageRepo    *repositories.UsageRepository
	StorageRepo  *repositories.StorageSnapshotsRepository
	DBStatsRepo  *repositories.DBStatsRepository
	AutomatedActionsRepo *repositories.AutomatedActionsRepository
	LogsRepo             *repositories.LogsRepository
	NotificationSettingsRepo *repositories.NotificationSettingsRepository
	OAuthConnRepo    *repositories.OAuthConnectionsRepository
	TransactionLatencyRepo *repositories.TransactionLatencyRepository

	OAuthState   *oauthconn.StateStore
	GithubClient github.Client
	SentryClient sentryapi.Client

	// StorageScan starts the scheduled R2 walk on the books job queue, so a
	// request can trigger it without the request budget cutting it.
	StorageScan StorageScanRunner
	// AppNames lists every registered app's name, excluding dashboard, for the
	// unused-apps report.
	AppNames []string
}

// Register mounts the ObservabilityService handler on a Connect server with
// the app's access gate, matching the pre-extraction wiring.
func Register(
	mux *http.ServeMux,
	logger *slog.Logger,
	gate iapp.Gate,
	d *Deps,
) {
	iapp.MountConnect(
		mux,
		logger,
		gate,
		func(s *connect.Server) {
			observabilityv1connect.RegisterObservabilityServiceHandler(
				s, &obsConnectHandler{deps: d},
			)
		},
	)
}