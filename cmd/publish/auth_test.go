package main

import (
	"net/http"
	"testing"

	"github.com/XDoubleU/essentia/pkg/test"
	"github.com/stretchr/testify/assert"
	"tools.xdoubleu.com/cmd/publish/internal/dtos"
)

func TestSignInHandler(t *testing.T) {
	tReq := test.CreateRequestTester(
		testApp.Routes(),
		http.MethodPost,
		"/api/auth/signin",
	)

	signInDto := dtos.SignInDto{
		Email:      "valid@example.com",
		Password:   "password",
		RememberMe: true,
	}

	tReq.SetFollowRedirect(false)

	tReq.SetContentType(test.FormContentType)
	tReq.SetData(signInDto)

	rs := tReq.Do(t)
	assert.Equal(t, http.StatusSeeOther, rs.StatusCode)
}

func TestSignOutHandler(t *testing.T) {
	tReq := test.CreateRequestTester(
		testApp.Routes(),
		http.MethodGet,
		"/api/auth/signout",
	)

	tReq.SetFollowRedirect(false)

	tReq.AddCookie(&accessToken)
	tReq.AddCookie(&refreshToken)

	rs := tReq.Do(t)
	assert.Equal(t, http.StatusSeeOther, rs.StatusCode)
}

func TestSignIn(t *testing.T) {
	tReq := test.CreateRequestTester(
		testApp.Routes(),
		http.MethodGet,
		"/",
	)

	rs := tReq.Do(t)
	assert.Equal(t, http.StatusOK, rs.StatusCode)
}

func TestRefreshTokens(t *testing.T) {
	tReq := test.CreateRequestTester(
		testApp.Routes(),
		http.MethodGet,
		"/",
	)

	tReq.AddCookie(&refreshToken)

	rs := tReq.Do(t)
	assert.Equal(t, http.StatusOK, rs.StatusCode)
}
