package goaltracker_test

import (
	"net/http"
	"os"
	"testing"
	"time"

	configtools "github.com/xdoubleu/essentia/v2/pkg/config"
	"github.com/xdoubleu/essentia/v2/pkg/database/postgres"
	"github.com/xdoubleu/essentia/v2/pkg/logging"
	"tools.xdoubleu.com/apps/goaltracker"
	"tools.xdoubleu.com/apps/goaltracker/internal/mocks"
	"tools.xdoubleu.com/internal/config"
	sharedmocks "tools.xdoubleu.com/internal/mocks"
)

var testApp *goaltracker.GoalTracker //nolint:gochecknoglobals //needed for tests

var goalID = "123" //nolint:gochecknoglobals //needed for tests

//nolint:gochecknoglobals //needed for tests
var userID = "4001e9cf-3fbe-4b09-863f-bd1654cfbf76"

//nolint:gochecknoglobals //needed for tests
var accessToken = http.Cookie{
	Name:  "accessToken",
	Value: "access",
}

//nolint:gochecknoglobals //needed for tests
var refreshToken = http.Cookie{
	Name:  "refreshToken",
	Value: "refresh",
}

func TestMain(m *testing.M) {
	var err error

	cfg := config.New(logging.NewNopLogger())
	cfg.Env = configtools.TestEnv
	cfg.Throttle = false
	cfg.SupabaseUserID = "4001e9cf-3fbe-4b09-863f-bd1654cfbf76"

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

	clients := goaltracker.Clients{
		Todoist:   mocks.NewMockTodoistClient(),
		Steam:     mocks.NewMockSteamClient(),
		Goodreads: mocks.NewMockGoodreadsClient(),
	}

	testApp = goaltracker.NewInner(
		sharedmocks.NewMockedAuthService(userID),
		logging.NewNopLogger(),
		cfg,
		postgresDB,
		clients,
	)

	err = testApp.ApplyMigrations(postgresDB)
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
