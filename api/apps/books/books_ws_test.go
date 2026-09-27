package books_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	"tools.xdoubleu.com/internal/testhelper"
)

// Job runs are triggered through the admin-gated RPCs, not a GET route.
func TestRefreshRoute_Removed(t *testing.T) {
	tReq := testhelper.CreateRequestTester(
		getRoutes(),
		http.MethodGet,
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
	// WebSocket upgrade without auth returns 426 Upgrade Required
	// because auth middleware checks credentials before WebSocket handler
	// processes the upgrade
	assert.Equal(t, http.StatusUpgradeRequired, rs.StatusCode)
}
