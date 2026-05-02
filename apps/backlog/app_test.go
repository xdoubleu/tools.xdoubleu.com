package backlog_test

import (
	"context"
	"net/http"
	"os"
	"testing"
	"time"

	configtools "github.com/xdoubleu/essentia/v4/pkg/config"
	"github.com/xdoubleu/essentia/v4/pkg/database/postgres"
	"github.com/xdoubleu/essentia/v4/pkg/logging"
	"tools.xdoubleu.com/apps/backlog"
	"tools.xdoubleu.com/apps/backlog/internal/mocks"
	"tools.xdoubleu.com/apps/backlog/pkg/hardcover"
	"tools.xdoubleu.com/apps/backlog/pkg/steam"
	"tools.xdoubleu.com/internal/config"
	sharedmocks "tools.xdoubleu.com/internal/mocks"
	"tools.xdoubleu.com/internal/templates"
)

var testApp *backlog.Backlog //nolint:gochecknoglobals //needed for tests

//nolint:gochecknoglobals //needed for tests
var userID = "4001e9cf-3fbe-4b09-863f-bd1654cfbf76"

//nolint:gochecknoglobals //needed for tests
var testCfg config.Config

//nolint:gochecknoglobals //needed for tests
var testDB postgres.DB

//nolint:gochecknoglobals //needed for tests
var accessToken = http.Cookie{
	Name:  "accessToken",
	Value: "access",
}

func TestMain(m *testing.M) {
	var err error

	testCfg = config.New(logging.NewNopLogger())
	testCfg.Env = configtools.TestEnv
	testCfg.Throttle = false

	postgresDB, err := postgres.Connect(
		logging.NewNopLogger(),
		testCfg.DBDsn,
		25,
		"15m",
		5,
		15*time.Second,
		30*time.Second,
	)
	if err != nil {
		panic(err)
	}
	testDB = postgresDB

	clients := backlog.Clients{
		SteamFactory: func(_ string) steam.Client {
			return mocks.NewMockSteamClient()
		},
		HardcoverFactory: func(_ string) hardcover.Client {
			return mocks.NewMockHardcoverClient()
		},
	}

	testApp = backlog.NewInner(
		context.Background(),
		sharedmocks.NewMockedAuthService(userID),
		logging.NewNopLogger(),
		testCfg,
		postgresDB,
		clients,
		templates.LoadShared(testCfg),
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
