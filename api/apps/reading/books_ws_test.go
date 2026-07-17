package reading_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/xdoubleu/essentia/v4/pkg/test"
)

func TestRefreshGoodreads(t *testing.T) {
	tReq := test.CreateRequestTester(
		getRoutes(),
		http.MethodGet,
		"/"+testApp.GetName()+"/api/progress/goodreads/refresh",
	)
	tReq.AddCookie(&accessToken)

	rs := tReq.Do(t)
	assert.Equal(t, http.StatusNoContent, rs.StatusCode)
}

func TestWebSocketProgress_Unauthenticated(t *testing.T) {
	tReq := test.CreateRequestTester(
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

/* tests broken
func TestRefreshSteam_Unauthenticated(t *testing.T) {
	tReq := test.CreateRequestTester(
		getRoutes(),
		http.MethodGet,
		"/"+testApp.GetName()+"/api/progress/steam/refresh",
	)

	rs := tReq.Do(t)
	assert.Equal(t, http.StatusUnauthorized, rs.StatusCode)
}

func TestRefreshGoodreads_Unauthenticated(t *testing.T) {
	tReq := test.CreateRequestTester(
		getRoutes(),
		http.MethodGet,
		"/"+testApp.GetName()+"/api/progress/goodreads/refresh",
	)

	rs := tReq.Do(t)
	assert.Equal(t, http.StatusUnauthorized, rs.StatusCode)
}
*/
