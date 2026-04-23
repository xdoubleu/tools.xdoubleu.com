package main

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/xdoubleu/essentia/v3/pkg/test"
	"tools.xdoubleu.com/cmd/publish/internal/dtos"
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
	tReq := test.CreateRequestTester(
		testApp.Routes(),
		http.MethodPost,
		"/onboarding",
	)
	tReq.AddCookie(&accessToken)
	tReq.SetFollowRedirect(false)
	tReq.SetContentType(test.FormContentType)
	tReq.SetData(dtos.IntegrationsDto{
		SteamAPIKey:     "test-steam-key",
		SteamUserID:     "test-steam-user",
		HardcoverAPIKey: "test-hardcover-key",
	})

	rs := tReq.Do(t)
	assert.Equal(t, http.StatusSeeOther, rs.StatusCode)
	assert.Equal(t, "/backlog", rs.Header.Get("Location"))
}
