package icsproxy_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	configtools "github.com/xdoubleu/essentia/v4/pkg/config"
	"github.com/xdoubleu/essentia/v4/pkg/database/postgres"
	"github.com/xdoubleu/essentia/v4/pkg/logging"

	"tools.xdoubleu.com/apps/icsproxy"
	"tools.xdoubleu.com/internal/config"
	sharedmocks "tools.xdoubleu.com/internal/mocks"
	"tools.xdoubleu.com/internal/testhelper"
)

//nolint:gochecknoglobals //needed for tests
var testApp *icsproxy.ICSProxy

//nolint:gochecknoglobals //needed for tests
var testDB postgres.DB

//nolint:gochecknoglobals //needed for tests
var testCfg config.Config

//nolint:gochecknoglobals //needed for tests
var userID = "4001e9cf-3fbe-4b09-863f-bd1654cfbf76"

// sampleICS returns a minimal valid ICS feed with a single event.
func sampleICS() string {
	return "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//Test//Test//EN\r\n" +
		"BEGIN:VEVENT\r\nUID:test-uid-1\r\nSUMMARY:Team Meeting\r\n" +
		"DTSTART:20240115T090000Z\r\nDTEND:20240115T100000Z\r\n" +
		"END:VEVENT\r\n" +
		"END:VCALENDAR\r\n"
}

// calendarServer starts a test HTTP server returning static ICS content.
func calendarServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/calendar")
			_, _ = w.Write([]byte(sampleICS()))
		}),
	)
}

func TestMain(m *testing.M) {
	testCfg = config.New(logging.NewNopLogger())
	testCfg.Env = configtools.TestEnv

	postgresDB := testhelper.ConnectTestDB(testCfg.DBDsn)
	testDB = postgresDB

	testApp = icsproxy.New(
		sharedmocks.NewMockedAuthService(userID),
		logging.NewNopLogger(),
		testCfg,
		postgresDB,
	)

	if err := testApp.ApplyMigrations(context.Background(), postgresDB); err != nil {
		panic(err)
	}

	os.Exit(m.Run())
}

func getRoutes() http.Handler {
	return testhelper.BuildMux(testApp)
}

func TestGetDisplayName(t *testing.T) {
	assert.Equal(t, "ICS Proxy", testApp.GetDisplayName())
}
