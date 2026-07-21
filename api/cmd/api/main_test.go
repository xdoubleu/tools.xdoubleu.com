package main

import (
	"context"
	"encoding/base64"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/xdoubleu/essentia/v4/pkg/database/postgres"
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
	// A fixed test key so OAuth connection tests can round-trip through the
	// real AES-GCM sealer instead of the "encryption not configured" path.
	cfg.OAuthTokenEncKey = base64.StdEncoding.EncodeToString(make([]byte, 32))

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
