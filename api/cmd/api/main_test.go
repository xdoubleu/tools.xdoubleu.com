package main

import (
	"context"
	"net/http"
	"os"
	"testing"

	"github.com/xdoubleu/essentia/v4/pkg/logging"

	"tools.xdoubleu.com/internal/mocks"
	"tools.xdoubleu.com/internal/testhelper"
)

var testApp *Application //nolint:gochecknoglobals //needed for tests

//nolint:gochecknoglobals //needed for tests
var accessToken = http.Cookie{
	Name:  "accessToken",
	Value: "access",
}

func TestMain(m *testing.M) {
	var err error

	cfg := testhelper.NewTestConfig()

	postgresDB, err := newDBPool(logging.NewNopLogger(), cfg.DBDsn)
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
