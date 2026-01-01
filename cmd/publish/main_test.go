package main

import (
	"net/http"
	"os"
	"testing"
	"time"

	configtools "github.com/XDoubleU/essentia/pkg/config"
	"github.com/XDoubleU/essentia/pkg/database/postgres"
	"github.com/XDoubleU/essentia/pkg/logging"
	"tools.xdoubleu.com/internal/config"
	"tools.xdoubleu.com/internal/mocks"
)

var testApp *Application //nolint:gochecknoglobals //needed for tests

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

	testApp = NewApplication(
		logging.NewNopLogger(),
		cfg,
		postgresDB,
		mocks.NewMockedGoTrueClient(),
	)

	os.Exit(m.Run())
}
