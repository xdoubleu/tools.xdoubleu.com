package trains_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"connectrpc.com/connect/v2"
	"connectrpc.com/connect/v2/connecthttp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/trains"
	"tools.xdoubleu.com/apps/trains/internal/mocks"
	"tools.xdoubleu.com/gen/trains/v1/trainsv1connect"
	"tools.xdoubleu.com/internal/database/postgres"
	"tools.xdoubleu.com/internal/logging"
	sharedmocks "tools.xdoubleu.com/internal/mocks"
	"tools.xdoubleu.com/internal/testhelper"
)

//nolint:gochecknoglobals //shared across tests
var (
	testDB postgres.DB
	userID = "4001e9cf-3fbe-4b09-863f-bd1654cfbf76"
)

func TestMain(m *testing.M) {
	cfg := testhelper.NewTestConfig()
	pool := testhelper.ConnectTestDB(cfg.DBDsn)
	testDB = pool

	// Migrations don't touch iRail, so this app needs no client.
	migrator := trains.NewInner(
		context.Background(),
		sharedmocks.NewMockedAuthService(userID),
		logging.NewNopLogger(), cfg, pool, nil,
	)
	if err := migrator.ApplyMigrations(context.Background(), pool); err != nil {
		panic(err)
	}

	os.Exit(m.Run())
}

// newApp builds the app over the recorded iRail responses.
func newApp(t *testing.T) *trains.Trains {
	t.Helper()
	return trains.NewInner(
		context.Background(),
		sharedmocks.NewMockedAuthService(userID),
		logging.NewNopLogger(), testhelper.NewTestConfig(), testDB,
		mocks.NewFixtureClient(t),
	)
}

func newTrainsTestClient(t *testing.T) trainsv1connect.TrainServiceClient {
	t.Helper()
	mux := http.NewServeMux()
	a := newApp(t)
	a.Routes(a.GetName(), mux)
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	return trainsv1connect.NewTrainServiceClient(
		connect.NewClient(connecthttp.NewTransport(
			http.DefaultClient,
			ts.URL,
			connecthttp.WithReadMaxBytes(0),
			connecthttp.WithHTTPGet(),
		)),
	)
}

func connectCode(t *testing.T, err error) connect.Code {
	t.Helper()
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	return connErr.Code()
}

func TestGetName(t *testing.T) {
	a := newApp(t)
	assert.Equal(t, "trains", a.GetName())
	assert.Equal(t, "Trains", a.GetDisplayName())
	assert.Empty(t, a.GetDomain())
}

func TestNewAndStart(t *testing.T) {
	a := trains.New(
		context.Background(),
		sharedmocks.NewMockedAuthService(userID),
		logging.NewNopLogger(), testhelper.NewTestConfig(), testDB,
	)
	a.Routes("trains", http.NewServeMux())
	require.NoError(t, a.Start())
}
