package backlog_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/xdoubleu/essentia/v3/pkg/test"
)

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

func TestGoodreadsPage(t *testing.T) {
	tReq := test.CreateRequestTester(
		getRoutes(),
		http.MethodGet,
		"/"+testApp.GetName()+"/goodreads",
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
