package main

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/xdoubleu/essentia/v3/pkg/test"
	"tools.xdoubleu.com/cmd/publish/internal/dtos"
)

func TestSignInHandler(t *testing.T) {
	tReq := test.CreateRequestTester(
		testApp.Routes(),
		http.MethodPost,
		"/auth/signin",
	)

	signInDto := dtos.SignInDto{
		Email:      "valid@example.com",
		Password:   "password",
		RememberMe: true,
		Redirect:   "/",
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
		"/auth/signout",
	)

	tReq.SetFollowRedirect(false)

	tReq.AddCookie(&accessToken)
	tReq.AddCookie(&refreshToken)

	rs := tReq.Do(t)
	assert.Equal(t, http.StatusSeeOther, rs.StatusCode)
}

func TestSignInHandlerValidationFailure(t *testing.T) {
	tReq := test.CreateRequestTester(
		testApp.Routes(),
		http.MethodPost,
		"/auth/signin",
	)

	tReq.SetContentType(test.FormContentType)
	tReq.SetData(
		dtos.SignInDto{Email: "", Password: "", RememberMe: false, Redirect: "/"},
	)

	rs := tReq.Do(t)
	assert.Equal(t, http.StatusUnprocessableEntity, rs.StatusCode)
}

func TestSignInHandlerNoRememberMe(t *testing.T) {
	tReq := test.CreateRequestTester(
		testApp.Routes(),
		http.MethodPost,
		"/auth/signin",
	)

	tReq.SetFollowRedirect(false)
	tReq.SetContentType(test.FormContentType)
	tReq.SetData(dtos.SignInDto{
		Email:      "valid@example.com",
		Password:   "password",
		RememberMe: false,
		Redirect:   "/",
	})

	rs := tReq.Do(t)
	assert.Equal(t, http.StatusSeeOther, rs.StatusCode)
}

func TestSignOutHandlerNoRefreshToken(t *testing.T) {
	tReq := test.CreateRequestTester(
		testApp.Routes(),
		http.MethodGet,
		"/auth/signout",
	)

	tReq.SetFollowRedirect(false)
	tReq.AddCookie(&accessToken)
	// no refresh token cookie — exercises the refreshToken == nil branch

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
