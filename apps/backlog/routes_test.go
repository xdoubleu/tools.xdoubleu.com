package backlog_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/xdoubleu/essentia/v4/pkg/test"
)

func TestSteamDistributionHandler(t *testing.T) {
	tReq := test.CreateRequestTester(
		getRoutes(),
		http.MethodGet,
		"/"+testApp.GetName()+"/steam/distribution/0",
	)
	tReq.AddCookie(&accessToken)

	rs := tReq.Do(t)
	assert.Equal(t, http.StatusOK, rs.StatusCode)
}

func TestSteamDistributionHandlerInvalidBucket(t *testing.T) {
	tReq := test.CreateRequestTester(
		getRoutes(),
		http.MethodGet,
		"/"+testApp.GetName()+"/steam/distribution/999",
	)
	tReq.AddCookie(&accessToken)

	rs := tReq.Do(t)
	assert.Equal(t, http.StatusNotFound, rs.StatusCode)
}

func TestSteamDistributionHandlerNonNumericBucket(t *testing.T) {
	tReq := test.CreateRequestTester(
		getRoutes(),
		http.MethodGet,
		"/"+testApp.GetName()+"/steam/distribution/abc",
	)
	tReq.AddCookie(&accessToken)

	rs := tReq.Do(t)
	assert.Equal(t, http.StatusNotFound, rs.StatusCode)
}

func TestBooksSearchExternalHandler(t *testing.T) {
	tReq := test.CreateRequestTester(
		getRoutes(),
		http.MethodGet,
		"/"+testApp.GetName()+"/books/search/external?q=golang",
	)
	tReq.AddCookie(&accessToken)

	rs := tReq.Do(t)
	assert.Equal(t, http.StatusOK, rs.StatusCode)
}

func TestBooksSearchExternalHandlerEmptyQuery(t *testing.T) {
	tReq := test.CreateRequestTester(
		getRoutes(),
		http.MethodGet,
		"/"+testApp.GetName()+"/books/search/external",
	)
	tReq.AddCookie(&accessToken)

	rs := tReq.Do(t)
	assert.Equal(t, http.StatusOK, rs.StatusCode)
}
