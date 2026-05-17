package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xdoubleu/essentia/v4/pkg/test"
	"tools.xdoubleu.com/cmd/publish/internal/dtos"
	"tools.xdoubleu.com/internal/mocks"
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

func TestForgotPasswordGetHandler(t *testing.T) {
	tReq := test.CreateRequestTester(
		testApp.Routes(),
		http.MethodGet,
		"/auth/forgot-password",
	)

	rs := tReq.Do(t)
	assert.Equal(t, http.StatusOK, rs.StatusCode)
}

func TestForgotPasswordPostHandler(t *testing.T) {
	tReq := test.CreateRequestTester(
		testApp.Routes(),
		http.MethodPost,
		"/auth/forgot-password",
	)

	tReq.SetFollowRedirect(false)
	tReq.SetContentType(test.FormContentType)
	tReq.SetData(dtos.ForgotPasswordDto{Email: "user@example.com"})

	rs := tReq.Do(t)
	assert.Equal(t, http.StatusSeeOther, rs.StatusCode)
	assert.Contains(t, rs.Header.Get("Location"), "sent=1")
}

func TestForgotPasswordPostHandlerValidationFailure(t *testing.T) {
	tReq := test.CreateRequestTester(
		testApp.Routes(),
		http.MethodPost,
		"/auth/forgot-password",
	)

	tReq.SetContentType(test.FormContentType)
	tReq.SetData(dtos.ForgotPasswordDto{Email: ""})

	rs := tReq.Do(t)
	assert.Equal(t, http.StatusUnprocessableEntity, rs.StatusCode)
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

// mfaToken cookie value — the mock auth service accepts any non-empty string.
var mfaTokenCookie = http.Cookie{
	Name:  "mfaToken",
	Value: "access",
}

// mfaFactorIDCookie holds the factor UUID the mock always returns.
var mfaFactorIDCookie = http.Cookie{ //nolint:gochecknoglobals
	Name:  "mfaFactorID",
	Value: mocks.MockedFactorID.String(),
}

// doMFARequest sends a request directly in-process (no TCP server) so it
// avoids the rate-limit bucket used by httptest.NewServer-based requests.
func doMFARequest(
	t *testing.T,
	method, target string,
	formVals url.Values,
	cookies ...*http.Cookie,
) *httptest.ResponseRecorder {
	t.Helper()
	var body strings.Reader
	if formVals != nil {
		body = *strings.NewReader(formVals.Encode())
	}
	req := httptest.NewRequest(method, target, &body)
	if formVals != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	for _, c := range cookies {
		req.AddCookie(c)
	}
	rr := httptest.NewRecorder()
	testApp.Routes().ServeHTTP(rr, req)
	return rr
}

// ── mfaEnrollGetHandler ───────────────────────────────────────────────────────

func TestMFAEnrollGetHandler_NoToken(t *testing.T) {
	// Without mfaToken cookie → redirect to "/"
	rr := doMFARequest(t, http.MethodGet, "/auth/mfa/enroll", nil)
	assert.Equal(t, http.StatusSeeOther, rr.Code)
	assert.Equal(t, "/", rr.Header().Get("Location"))
}

func TestMFAEnrollGetHandler_WithToken(t *testing.T) {
	// With mfaToken cookie → enrolls TOTP and renders the enroll page
	rr := doMFARequest(t, http.MethodGet, "/auth/mfa/enroll", nil, &mfaTokenCookie)
	assert.Equal(t, http.StatusOK, rr.Code)
}

// ── mfaEnrollPostHandler ──────────────────────────────────────────────────────

func TestMFAEnrollPostHandler_NoToken(t *testing.T) {
	rr := doMFARequest(t, http.MethodPost, "/auth/mfa/enroll", url.Values{})
	assert.Equal(t, http.StatusSeeOther, rr.Code)
	assert.Equal(t, "/", rr.Header().Get("Location"))
}

func TestMFAEnrollPostHandler_InvalidFactorID(t *testing.T) {
	// mfaToken present but factor_id is not a valid UUID → redirect to enroll
	vals := url.Values{"factor_id": {"not-a-uuid"}, "code": {"123456"}}
	rr := doMFARequest(t, http.MethodPost, "/auth/mfa/enroll", vals, &mfaTokenCookie)
	assert.Equal(t, http.StatusSeeOther, rr.Code)
	assert.Equal(t, "/auth/mfa/enroll", rr.Header().Get("Location"))
}

func TestMFAEnrollPostHandler_ValidFactorID(t *testing.T) {
	// Valid factor_id and any code → mock VerifyFactor succeeds → completeMFA
	vals := url.Values{
		"factor_id": {mocks.MockedFactorID.String()},
		"code":      {"123456"},
	}
	rr := doMFARequest(t, http.MethodPost, "/auth/mfa/enroll", vals, &mfaTokenCookie)
	// completeMFA sets cookies and redirects to "/"
	assert.Equal(t, http.StatusSeeOther, rr.Code)
}

func TestMFAEnrollPostHandler_WithRememberMeAndRedirect(t *testing.T) {
	vals := url.Values{
		"factor_id": {mocks.MockedFactorID.String()},
		"code":      {"123456"},
	}
	rememberMe := http.Cookie{Name: "mfaRememberMe", Value: "1"}
	redirect := http.Cookie{Name: "mfaRedirect", Value: "/backlog"}
	rr := doMFARequest(
		t, http.MethodPost, "/auth/mfa/enroll", vals,
		&mfaTokenCookie, &rememberMe, &redirect,
	)
	assert.Equal(t, http.StatusSeeOther, rr.Code)
	assert.Equal(t, "/backlog", rr.Header().Get("Location"))
}

// ── mfaChallengeGetHandler ────────────────────────────────────────────────────

func TestMFAChallengeGetHandler_NoToken(t *testing.T) {
	rr := doMFARequest(t, http.MethodGet, "/auth/mfa/challenge", nil)
	assert.Equal(t, http.StatusSeeOther, rr.Code)
	assert.Equal(t, "/", rr.Header().Get("Location"))
}

func TestMFAChallengeGetHandler_WithToken(t *testing.T) {
	rr := doMFARequest(t, http.MethodGet, "/auth/mfa/challenge", nil, &mfaTokenCookie)
	assert.Equal(t, http.StatusOK, rr.Code)
}

// ── mfaChallengePostHandler ───────────────────────────────────────────────────

func TestMFAChallengePostHandler_NoToken(t *testing.T) {
	rr := doMFARequest(t, http.MethodPost, "/auth/mfa/challenge", url.Values{})
	assert.Equal(t, http.StatusSeeOther, rr.Code)
	assert.Equal(t, "/", rr.Header().Get("Location"))
}

func TestMFAChallengePostHandler_NoFactorID(t *testing.T) {
	// mfaToken present but no mfaFactorID cookie → redirect
	rr := doMFARequest(
		t, http.MethodPost, "/auth/mfa/challenge", url.Values{},
		&mfaTokenCookie,
	)
	assert.Equal(t, http.StatusSeeOther, rr.Code)
	assert.Equal(t, "/", rr.Header().Get("Location"))
}

func TestMFAChallengePostHandler_InvalidFactorID(t *testing.T) {
	badFactor := http.Cookie{Name: "mfaFactorID", Value: "not-a-uuid"}
	vals := url.Values{"code": {"123456"}}
	rr := doMFARequest(
		t, http.MethodPost, "/auth/mfa/challenge", vals,
		&mfaTokenCookie, &badFactor,
	)
	assert.Equal(t, http.StatusSeeOther, rr.Code)
}

func TestMFAChallengePostHandler_ValidSubmission(t *testing.T) {
	vals := url.Values{"code": {"123456"}}
	rr := doMFARequest(
		t, http.MethodPost, "/auth/mfa/challenge", vals,
		&mfaTokenCookie, &mfaFactorIDCookie,
	)
	// Mock VerifyFactor always succeeds → completeMFA → redirect
	assert.Equal(t, http.StatusSeeOther, rr.Code)
}

func TestMFAChallengePostHandler_WithRememberMe(t *testing.T) {
	vals := url.Values{"code": {"123456"}}
	rememberMe := http.Cookie{Name: "mfaRememberMe", Value: "1"}
	require.NotNil(t, &rememberMe)
	rr := doMFARequest(
		t, http.MethodPost, "/auth/mfa/challenge", vals,
		&mfaTokenCookie, &mfaFactorIDCookie, &rememberMe,
	)
	assert.Equal(t, http.StatusSeeOther, rr.Code)
}

func TestMFAChallengePostHandler_WithRedirectCookie(t *testing.T) {
	vals := url.Values{"code": {"123456"}}
	redirect := http.Cookie{Name: "mfaRedirect", Value: "/backlog/"}
	rr := doMFARequest(
		t, http.MethodPost, "/auth/mfa/challenge", vals,
		&mfaTokenCookie, &mfaFactorIDCookie, &redirect,
	)
	assert.Equal(t, http.StatusSeeOther, rr.Code)
	assert.Equal(t, "/backlog/", rr.Header().Get("Location"))
}
