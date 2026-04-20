package main

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/xdoubleu/essentia/v3/pkg/test"
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
	type settingsForm struct {
		TodoistAPIKey    string `form:"todoist_api_key"`
		TodoistProjectID string `form:"todoist_project_id"`
		SteamAPIKey      string `form:"steam_api_key"`
		SteamUserID      string `form:"steam_user_id"`
		GoodreadsURL     string `form:"goodreads_url"`
	}

	tReq := test.CreateRequestTester(
		testApp.Routes(),
		http.MethodPost,
		"/settings",
	)
	tReq.AddCookie(&accessToken)
	tReq.SetFollowRedirect(false)
	tReq.SetContentType(test.FormContentType)
	tReq.SetData(settingsForm{
		TodoistAPIKey:    "test-todoist-key",
		TodoistProjectID: "test-project-id",
		SteamAPIKey:      "test-steam-key",
		SteamUserID:      "test-steam-user",
		GoodreadsURL:     "https://goodreads.com/user/123",
	})

	rs := tReq.Do(t)
	assert.Equal(t, http.StatusSeeOther, rs.StatusCode)
	assert.Equal(t, "/settings?saved=1", rs.Header.Get("Location"))
}

func TestSaveSettingsRoundTrip(t *testing.T) {
	type settingsForm struct {
		TodoistAPIKey    string `form:"todoist_api_key"`
		TodoistProjectID string `form:"todoist_project_id"`
		SteamAPIKey      string `form:"steam_api_key"`
		SteamUserID      string `form:"steam_user_id"`
		GoodreadsURL     string `form:"goodreads_url"`
	}

	routes := testApp.Routes()

	postReq := test.CreateRequestTester(routes, http.MethodPost, "/settings")
	postReq.AddCookie(&accessToken)
	postReq.SetFollowRedirect(false)
	postReq.SetContentType(test.FormContentType)
	postReq.SetData(settingsForm{
		TodoistAPIKey:    "round-trip-key",
		TodoistProjectID: "round-trip-project",
		SteamAPIKey:      "",
		SteamUserID:      "",
		GoodreadsURL:     "",
	})
	rs := postReq.Do(t)
	assert.Equal(t, http.StatusSeeOther, rs.StatusCode)

	getReq := test.CreateRequestTester(routes, http.MethodGet, "/settings")
	getReq.AddCookie(&accessToken)
	rs2 := getReq.Do(t)
	assert.Equal(t, http.StatusOK, rs2.StatusCode)
}
