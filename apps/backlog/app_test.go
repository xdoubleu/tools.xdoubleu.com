package backlog_test

import (
	"context"
	"net/http"
	"os"
	"testing"
	"time"

	configtools "github.com/xdoubleu/essentia/v3/pkg/config"
	"github.com/xdoubleu/essentia/v3/pkg/database/postgres"
	"github.com/xdoubleu/essentia/v3/pkg/logging"
	"tools.xdoubleu.com/apps/backlog"
	"tools.xdoubleu.com/apps/backlog/internal/mocks"
	"tools.xdoubleu.com/apps/backlog/pkg/steam"
	"tools.xdoubleu.com/internal/config"
	sharedmocks "tools.xdoubleu.com/internal/mocks"
	"tools.xdoubleu.com/internal/templates"
)

var testApp *backlog.Backlog //nolint:gochecknoglobals //needed for tests

//nolint:gochecknoglobals //needed for tests
var userID = "4001e9cf-3fbe-4b09-863f-bd1654cfbf76"

//nolint:gochecknoglobals //needed for tests
var accessToken = http.Cookie{
	Name:  "accessToken",
	Value: "access",
}

func TestMain(m *testing.M) {
	var err error

	cfg := config.New(logging.NewNopLogger())
	cfg.Env = configtools.TestEnv
	cfg.Throttle = false

	postgresDB, err := postgres.Connect(
		logging.NewNopLogger(),
		cfg.DBDsn,
		25,
		"15m",
		5,
		15*time.Second,
		30*time.Second,
	)
	if err != nil {
		panic(err)
	}

	clients := backlog.Clients{
		SteamFactory: func(_ string) steam.Client { return mocks.NewMockSteamClient() },
		Goodreads:    mocks.NewMockGoodreadsClient(),
	}

	testApp = backlog.NewInner(
		context.Background(),
		sharedmocks.NewMockedAuthService(userID),
		logging.NewNopLogger(),
		cfg,
		postgresDB,
		clients,
		templates.LoadShared(cfg),
	)

	err = testApp.ApplyMigrations(context.Background(), postgresDB)
	if err != nil {
		panic(err)
	}

	err = testApp.SaveIntegrations(
		context.Background(),
		userID,
		//nolint:exhaustruct //intentionally empty to mark user as onboarded
		backlog.Integrations{},
	)
	if err != nil {
		panic(err)
	}

	os.Exit(m.Run())
}

func getRoutes() http.Handler {
	mux := http.NewServeMux()
	testApp.Routes(testApp.GetName(), mux)
	return mux
}
