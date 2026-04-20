package main

import (
	"net/http"
	"os"
	"testing"
	"time"

	configtools "github.com/xdoubleu/essentia/v3/pkg/config"
	"github.com/xdoubleu/essentia/v3/pkg/database/postgres"
	"github.com/xdoubleu/essentia/v3/pkg/logging"
	"tools.xdoubleu.com/internal/config"
	"tools.xdoubleu.com/internal/mocks"
)

var testApp *Application           //nolint:gochecknoglobals //needed for tests
var testAppWithGitHub *Application //nolint:gochecknoglobals //needed for tests

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

	cfgWithGitHub := config.New(logging.NewNopLogger())
	cfgWithGitHub.Env = configtools.TestEnv
	cfgWithGitHub.Throttle = false
	cfgWithGitHub.GitHubToken = "test-token"
	cfgWithGitHub.GitHubRepo = "owner/repo"

	testAppWithGitHub = NewApplication(
		logging.NewNopLogger(),
		cfgWithGitHub,
		postgresDB,
		mocks.NewMockedGoTrueClient(),
	)

	os.Exit(m.Run())
}
