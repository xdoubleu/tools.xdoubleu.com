package games_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	"tools.xdoubleu.com/internal/testhelper"
)

func TestRefreshSteam(t *testing.T) {
	tReq := testhelper.CreateRequestTester(
		getRoutes(),
		http.MethodGet,
		"/"+testApp.GetName()+"/api/progress/steam/refresh",
	)
	tReq.AddCookie(&accessToken)

	rs := tReq.Do(t)
	assert.Equal(t, http.StatusNoContent, rs.StatusCode)
}

func TestWebSocketProgress_Unauthenticated(t *testing.T) {
	tReq := testhelper.CreateRequestTester(
		getRoutes(),
		http.MethodGet,
		"/"+testApp.GetName()+"/api/progress",
	)

	rs := tReq.Do(t)
	// WebSocket upgrade without auth returns 426 Upgrade Required
	// because auth middleware checks credentials before WebSocket handler
	// processes the upgrade
	assert.Equal(t, http.StatusUpgradeRequired, rs.StatusCode)
}
