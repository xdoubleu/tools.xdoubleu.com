package main

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/xdoubleu/essentia/v3/pkg/test"
)

func TestGetOnboardingHandler(t *testing.T) {
	tReq := test.CreateRequestTester(
		testApp.Routes(),
		http.MethodGet,
		"/onboarding",
	)
	tReq.AddCookie(&accessToken)

	rs := tReq.Do(t)
	assert.Equal(t, http.StatusOK, rs.StatusCode)
}

func TestSaveOnboardingHandler(t *testing.T) {
	type onboardingForm struct {
		TodoistAPIKey    string `form:"todoist_api_key"`
		TodoistProjectID string `form:"todoist_project_id"`
		SteamAPIKey      string `form:"steam_api_key"`
		SteamUserID      string `form:"steam_user_id"`
		GoodreadsURL     string `form:"goodreads_url"`
	}

	tReq := test.CreateRequestTester(
		testApp.Routes(),
		http.MethodPost,
		"/onboarding",
	)
	tReq.AddCookie(&accessToken)
	tReq.SetFollowRedirect(false)
	tReq.SetContentType(test.FormContentType)
	tReq.SetData(onboardingForm{
		TodoistAPIKey:    "test-todoist-key",
		TodoistProjectID: "test-project-id",
		SteamAPIKey:      "test-steam-key",
		SteamUserID:      "test-steam-user",
		GoodreadsURL:     "https://goodreads.com/user/123",
	})

	rs := tReq.Do(t)
	assert.Equal(t, http.StatusSeeOther, rs.StatusCode)
	assert.Equal(t, "/goaltracker", rs.Header.Get("Location"))
}
