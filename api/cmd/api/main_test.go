package main

import (
	"context"
	"net/http"
	"os"
	"testing"
	"time"

	configtools "github.com/xdoubleu/essentia/v4/pkg/config"
	"github.com/xdoubleu/essentia/v4/pkg/database/postgres"
	"github.com/xdoubleu/essentia/v4/pkg/logging"

	"tools.xdoubleu.com/internal/config"
	"tools.xdoubleu.com/internal/mocks"
)

var testApp *Application //nolint:gochecknoglobals //needed for tests

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

	testApp = NewApplication(
		logging.NewNopLogger(),
		cfg,
		postgresDB,
		mocks.NewMockedGoTrueClient(),
	)

	if _, err = postgresDB.Exec(
		context.Background(),
		"DELETE FROM global.contacts WHERE owner_user_id = $1 OR contact_user_id = $1",
		testUserID,
	); err != nil {
		panic(err)
	}

	os.Exit(m.Run())
}
