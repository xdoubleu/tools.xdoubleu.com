package feeds_test

import (
	"context"
	"net/http"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/xdoubleu/essentia/v4/pkg/database/postgres"
	"github.com/xdoubleu/essentia/v4/pkg/logging"

	"tools.xdoubleu.com/apps/feeds"
	"tools.xdoubleu.com/apps/feeds/internal/mocks"
	sharedmocks "tools.xdoubleu.com/internal/mocks"
	"tools.xdoubleu.com/internal/testhelper"
)

//nolint:gochecknoglobals //needed for tests
var testApp *feeds.Feeds

//nolint:gochecknoglobals //needed for tests
var userID = "4001e9cf-3fbe-4b09-863f-bd1654cfbf76"

//nolint:gochecknoglobals //needed for tests
var testDB postgres.DB

// mockWebFetch is testApp's external-content client; RSS/email ingest tests
// register canned responses on it.
//
//nolint:gochecknoglobals //needed for tests
var mockWebFetch *mocks.MockWebFetchClient

func TestMain(m *testing.M) {
	cfg := testhelper.NewTestConfig()
	cfg.EmailInboundDomain = "mail.example.com"
	cfg.EmailInboundSecret = emailWebhookSecret
	cfg.ResendAPIKey = "test-resend-key"

	postgresDB := testhelper.ConnectTestDB(cfg.DBDsn)
	testDB = postgresDB
	auth := sharedmocks.NewMockedAuthService(userID)

	mockWebFetch = mocks.NewMockWebFetchClient()
	testApp = feeds.NewInner(
		auth,
		logging.NewNopLogger(),
		cfg,
		postgresDB,
		mockWebFetch,
	)

	if _, err := postgresDB.Exec(
		context.Background(), "DROP SCHEMA IF EXISTS feeds CASCADE",
	); err != nil {
		panic(err)
	}

	if err := testApp.ApplyMigrations(context.Background(), postgresDB); err != nil {
		panic(err)
	}

	os.Exit(m.Run())
}

func getRoutes() http.Handler {
	return testhelper.BuildMux(testApp)
}

func TestGetDisplayName(t *testing.T) {
	assert.Equal(t, "Feeds", testApp.GetDisplayName())
}

func TestGetName(t *testing.T) {
	assert.Equal(t, "feeds", testApp.GetName())
}

func TestStart(t *testing.T) {
	assert.NoError(t, testApp.Start())
}
