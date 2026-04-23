package icsproxy_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	configtools "github.com/xdoubleu/essentia/v3/pkg/config"
	httptools "github.com/xdoubleu/essentia/v3/pkg/communication/httptools"
	"github.com/xdoubleu/essentia/v3/pkg/database/postgres"
	"github.com/xdoubleu/essentia/v3/pkg/logging"
	"github.com/stretchr/testify/require"
	"tools.xdoubleu.com/apps/icsproxy"
	"tools.xdoubleu.com/internal/config"
	sharedmocks "tools.xdoubleu.com/internal/mocks"
	"tools.xdoubleu.com/internal/templates"
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

	testApp = icsproxy.New(
		sharedmocks.NewMockedAuthService(userID),
		logging.NewNopLogger(),
		testCfg,
		postgresDB,
		templates.LoadShared(testCfg),
	)

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

func encodeForm(t *testing.T, dto any, extra url.Values) string {
	t.Helper()
	values, err := httptools.WriteForm(dto)
	require.NoError(t, err)
	for k, vs := range extra {
		values[k] = vs
	}
	return values.Encode()
}

func doRequest(t *testing.T, method, path, body string) *http.Response {
	t.Helper()
	var req *http.Request
	if body != "" {
		req = httptest.NewRequest(method, path,
			strings.NewReader(body))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	rr := httptest.NewRecorder()
	getRoutes().ServeHTTP(rr, req)
	return rr.Result()
}
