package backlog_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/xdoubleu/essentia/v3/pkg/logging"
	"github.com/xdoubleu/essentia/v3/pkg/test"
	"tools.xdoubleu.com/apps/backlog"
	"tools.xdoubleu.com/apps/backlog/internal/mocks"
	"tools.xdoubleu.com/apps/backlog/pkg/hardcover"
	"tools.xdoubleu.com/apps/backlog/pkg/steam"
	sharedmocks "tools.xdoubleu.com/internal/mocks"
	"tools.xdoubleu.com/internal/templates"
)

func TestRootUnboarded(t *testing.T) {
	unonboardedID := "unboarded-user-000000000001"
	app := backlog.NewInner(
		context.Background(),
		sharedmocks.NewMockedAuthService(unonboardedID),
		logging.NewNopLogger(),
		testCfg,
		testDB,
		backlog.Clients{
			SteamFactory:     func(_ string) steam.Client { return mocks.NewMockSteamClient() },
			HardcoverFactory: func(_ string) hardcover.Client { return mocks.NewMockHardcoverClient() },
		},
		templates.LoadShared(testCfg),
	)

	mux := http.NewServeMux()
	app.Routes(app.GetName(), mux)

	tReq := test.CreateRequestTester(mux, http.MethodGet, "/"+app.GetName()+"/")
	tReq.AddCookie(&accessToken)
	tReq.SetFollowRedirect(false)

	rs := tReq.Do(t)
	assert.Equal(t, http.StatusSeeOther, rs.StatusCode)
	assert.Equal(t, "/onboarding", rs.Header.Get("Location"))
}

func TestRoot(t *testing.T) {
	tReq := test.CreateRequestTester(
		getRoutes(),
		http.MethodGet,
		"/"+testApp.GetName()+"/",
	)
	tReq.AddCookie(&accessToken)

	rs := tReq.Do(t)
	assert.Equal(t, http.StatusOK, rs.StatusCode)
}

func TestUserBacklog(t *testing.T) {
	tReq := test.CreateRequestTester(
		getRoutes(),
		http.MethodGet,
		"/"+testApp.GetName()+"/users/"+userID,
	)
	tReq.AddCookie(&accessToken)

	rs := tReq.Do(t)
	assert.Equal(t, http.StatusOK, rs.StatusCode)
}

func TestSteamPage(t *testing.T) {
	tReq := test.CreateRequestTester(
		getRoutes(),
		http.MethodGet,
		"/"+testApp.GetName()+"/steam",
	)
	tReq.AddCookie(&accessToken)

	rs := tReq.Do(t)
	assert.Equal(t, http.StatusOK, rs.StatusCode)
}

func TestSteamPageWithDateRange(t *testing.T) {
	tReq := test.CreateRequestTester(
		getRoutes(),
		http.MethodGet,
		"/"+testApp.GetName()+"/steam?from=2024-01-01&to=2024-12-31",
	)
	tReq.AddCookie(&accessToken)

	rs := tReq.Do(t)
	assert.Equal(t, http.StatusOK, rs.StatusCode)
}

func TestRefreshSteam(t *testing.T) {
	tReq := test.CreateRequestTester(
		getRoutes(),
		http.MethodGet,
		"/"+testApp.GetName()+"/api/progress/steam/refresh",
	)

	rs := tReq.Do(t)
	assert.Equal(t, http.StatusNoContent, rs.StatusCode)
}

func TestRefreshGoodreads(t *testing.T) {
	tReq := test.CreateRequestTester(
		getRoutes(),
		http.MethodGet,
		"/"+testApp.GetName()+"/api/progress/goodreads/refresh",
	)

	rs := tReq.Do(t)
	assert.Equal(t, http.StatusNoContent, rs.StatusCode)
}
