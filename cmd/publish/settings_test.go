package main

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/xdoubleu/essentia/v3/pkg/test"
	"tools.xdoubleu.com/cmd/publish/internal/dtos"
)

func TestGetSettingsHandler(t *testing.T) {
	tReq := test.CreateRequestTester(
		testApp.Routes(),
		http.MethodGet,
		"/settings",
	)
	tReq.AddCookie(&accessToken)

	rs := tReq.Do(t)
	assert.Equal(t, http.StatusOK, rs.StatusCode)
}

func TestSaveSettingsHandler(t *testing.T) {
	tReq := test.CreateRequestTester(
		testApp.Routes(),
		http.MethodPost,
		"/settings",
	)
	tReq.AddCookie(&accessToken)
	tReq.SetFollowRedirect(false)
	tReq.SetContentType(test.FormContentType)
	tReq.SetData(dtos.IntegrationsDto{
		SteamAPIKey:  "test-steam-key",
		SteamUserID:  "test-steam-user",
		GoodreadsURL: "https://goodreads.com/user/123",
	})

	rs := tReq.Do(t)
	assert.Equal(t, http.StatusSeeOther, rs.StatusCode)
	assert.Equal(t, "/settings?saved=1", rs.Header.Get("Location"))
}

func TestSaveSettingsRoundTrip(t *testing.T) {
	routes := testApp.Routes()

	postReq := test.CreateRequestTester(routes, http.MethodPost, "/settings")
	postReq.AddCookie(&accessToken)
	postReq.SetFollowRedirect(false)
	postReq.SetContentType(test.FormContentType)
	postReq.SetData(dtos.IntegrationsDto{
		SteamAPIKey:  "round-trip-key",
		SteamUserID:  "round-trip-user",
		GoodreadsURL: "https://goodreads.com/user/123",
	})
	rs := postReq.Do(t)
	assert.Equal(t, http.StatusSeeOther, rs.StatusCode)

	getReq := test.CreateRequestTester(routes, http.MethodGet, "/settings")
	getReq.AddCookie(&accessToken)
	rs2 := getReq.Do(t)
	assert.Equal(t, http.StatusOK, rs2.StatusCode)
}
