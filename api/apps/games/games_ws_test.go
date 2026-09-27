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
		http.MethodPost,
		"/"+testApp.GetName()+"/api/progress/steam/refresh",
	)
	tReq.AddCookie(&accessToken)

	rs := tReq.Do(t)
	assert.Equal(t, http.StatusNoContent, rs.StatusCode)
}

// A state-changing GET would be triggerable by a cross-site link.
func TestRefreshSteam_GetNotAllowed(t *testing.T) {
	tReq := testhelper.CreateRequestTester(
		getRoutes(),
		http.MethodGet,
		"/"+testApp.GetName()+"/api/progress/steam/refresh",
	)
	tReq.AddCookie(&accessToken)

	rs := tReq.Do(t)
	assert.Equal(t, http.StatusMethodNotAllowed, rs.StatusCode)
}

// Other jobs aren't user-triggerable.
func TestRefreshOtherJob_NotFound(t *testing.T) {
	tReq := testhelper.CreateRequestTester(
		getRoutes(),
		http.MethodPost,
		"/"+testApp.GetName()+"/api/progress/books-storage-scan/refresh",
	)
	tReq.AddCookie(&accessToken)

	rs := tReq.Do(t)
	assert.Equal(t, http.StatusNotFound, rs.StatusCode)
}

func TestWebSocketProgress_Unauthenticated(t *testing.T) {
	tReq := testhelper.CreateRequestTester(
		getRoutes(),
		http.MethodGet,
		"/"+testApp.GetName()+"/api/progress",
	)

	rs := tReq.Do(t)
	// Auth middleware rejects before the upgrade is processed.
	assert.Equal(t, http.StatusUpgradeRequired, rs.StatusCode)
}
