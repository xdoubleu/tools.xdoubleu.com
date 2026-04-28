package recipes_test

import (
	"context"
	"net/http"
	"os"
	"testing"
	"time"

	configtools "github.com/xdoubleu/essentia/v3/pkg/config"
	"github.com/xdoubleu/essentia/v3/pkg/database/postgres"
	"github.com/xdoubleu/essentia/v3/pkg/logging"
	"tools.xdoubleu.com/apps/recipes"
	"tools.xdoubleu.com/internal/config"
	sharedmocks "tools.xdoubleu.com/internal/mocks"
	"tools.xdoubleu.com/internal/templates"
)

//nolint:gochecknoglobals //needed for tests
var testApp *recipes.Recipes

//nolint:gochecknoglobals //needed for tests
var testDB postgres.DB

//nolint:gochecknoglobals //needed for tests
var testCfg config.Config

//nolint:gochecknoglobals //needed for tests
var userID = "4001e9cf-3fbe-4b09-863f-bd1654cfbf76"

func TestMain(m *testing.M) {
	testCfg = config.New(logging.NewNopLogger())
	testCfg.Env = configtools.TestEnv

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

	testApp = recipes.New(
		sharedmocks.NewMockedAuthService(userID),
		logging.NewNopLogger(),
		testCfg,
		postgresDB,
		templates.LoadShared(testCfg),
		sharedmocks.NewMockedContactsService(),
	)

	// Drop the schema so the rewritten migration is applied from scratch.
	if _, err = postgresDB.Exec(
		context.Background(),
		"DROP SCHEMA IF EXISTS recipes CASCADE",
	); err != nil {
		panic(err)
	}

	if err = testApp.ApplyMigrations(context.Background(), postgresDB); err != nil {
		panic(err)
	}

	os.Exit(m.Run())
}

func getRoutes() http.Handler {
	mux := http.NewServeMux()
	testApp.Routes(testApp.GetName(), mux)
	return mux
}
