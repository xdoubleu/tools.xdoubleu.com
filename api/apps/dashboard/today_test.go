package dashboard_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"connectrpc.com/connect/v2"
	"connectrpc.com/connect/v2/connecthttp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/dashboard"
	"tools.xdoubleu.com/apps/movies"
	dashboardv1 "tools.xdoubleu.com/gen/dashboard/v1"
	"tools.xdoubleu.com/gen/dashboard/v1/dashboardv1connect"
	"tools.xdoubleu.com/internal/kit/logging"
	"tools.xdoubleu.com/internal/kit/testhelper"
	sharedmocks "tools.xdoubleu.com/internal/mocks"
	"tools.xdoubleu.com/internal/platform/family"
)

func newTodayClient(
	t *testing.T,
	app *dashboard.Dashboard,
) dashboardv1connect.TodayServiceClient {
	t.Helper()
	ts := httptest.NewServer(testhelper.BuildMux(app))
	t.Cleanup(ts.Close)
	return dashboardv1connect.NewTodayServiceClient(
		connect.NewClient(connecthttp.NewTransport(
			http.DefaultClient, ts.URL, connecthttp.WithReadMaxBytes(0),
		)),
	)
}

func newAdminDashboard() *dashboard.Dashboard {
	return dashboard.New(
		context.Background(),
		sharedmocks.NewMockedAdminAuthService(userID),
		logging.NewNopLogger(),
		testCfg,
		testDB,
		testGames,
		testBooks,
		testFeeds,
		testToday,
	)
}

func TestGetToday_AdminGetsEveryBlock(t *testing.T) {
	client := newTodayClient(t, newAdminDashboard())

	resp, err := client.GetToday(context.Background(), &dashboardv1.GetTodayRequest{})
	require.NoError(t, err)

	assert.NotNil(t, resp.Reading)
	assert.NotNil(t, resp.Feeds)
	assert.NotNil(t, resp.Meals)
	assert.NotNil(t, resp.Shopping)
	assert.NotNil(t, resp.Watching)
	assert.NotNil(t, resp.Learning)
	assert.NotNil(t, resp.Factory)
}

func TestGetToday_UserSeesOnlyGrantedApps(t *testing.T) {
	client := newTodayClient(t, testApp)

	resp, err := client.GetToday(context.Background(), &dashboardv1.GetTodayRequest{})
	require.NoError(t, err)

	assert.Nil(t, resp.Reading)
	assert.Nil(t, resp.Feeds)
	assert.Nil(t, resp.Factory)
}

func TestGetToday_FailedBlockIsLeftUnset(t *testing.T) {
	closed := testhelper.ConnectTestDB(testCfg.DBDsn)
	closed.Close()
	today := testToday
	today.Movies = movies.New(
		context.Background(),
		sharedmocks.NewMockedAdminAuthService(userID),
		logging.NewNopLogger(),
		testCfg,
		closed,
		family.NewFamilyRepository(closed),
	)
	app := dashboard.New(
		context.Background(),
		sharedmocks.NewMockedAdminAuthService(userID),
		logging.NewNopLogger(),
		testCfg,
		testDB,
		testGames,
		testBooks,
		testFeeds,
		today,
	)
	client := newTodayClient(t, app)

	resp, err := client.GetToday(context.Background(), &dashboardv1.GetTodayRequest{})
	require.NoError(t, err)

	assert.Nil(t, resp.Watching)
	assert.NotNil(t, resp.Meals)
}
