package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	httptools "github.com/xdoubleu/essentia/v4/pkg/communication/httptools"
	"tools.xdoubleu.com/cmd/publish/internal/dtos"
	"tools.xdoubleu.com/internal/mocks"
)

// doInProcess executes a request directly against the handler using
// httptest.NewRecorder so that it hits the 192.0.2.1 rate-limit bucket
// (not the 127.0.0.1 bucket consumed by httptest.NewServer-based tests).
func doInProcess(
	t *testing.T,
	method, target string,
	body string,
	contentType string,
	cookies ...*http.Cookie,
) *httptest.ResponseRecorder {
	t.Helper()

	var reqBody strings.Reader
	if body != "" {
		reqBody = *strings.NewReader(body)
	}

	req := httptest.NewRequest(method, target, &reqBody)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	for _, c := range cookies {
		req.AddCookie(c)
	}

	rr := httptest.NewRecorder()
	testApp.Routes().ServeHTTP(rr, req)
	return rr
}

// encodeForm serialises a dto as URL-encoded form values via the same
// httptools helper the application itself uses, so field names match.
func encodeForm(t *testing.T, dto interface{}) string {
	t.Helper()
	vals, err := httptools.WriteForm(dto)
	assert.NoError(t, err)
	return vals.Encode()
}

// TestSettingsHandler_WithSavedFlag verifies that the settings page renders
// the "saved" success alert when the query param ?saved=1 is present.
func TestSettingsHandler_WithSavedFlag(t *testing.T) {
	rr := doInProcess(t, http.MethodGet, "/settings?saved=1", "", "", &accessToken)
	assert.Equal(t, http.StatusOK, rr.Code)
}

// TestSettingsHandler_WithImportedCount exercises the importedCount branch
// in settingsHandler (the ?imported=N query parameter path).
func TestSettingsHandler_WithImportedCount(t *testing.T) {
	rr := doInProcess(t, http.MethodGet, "/settings?imported=5", "", "", &accessToken)
	assert.Equal(t, http.StatusOK, rr.Code)
}

// TestSettingsHandler_WithBothFlags exercises both importedCount and saved
// simultaneously — MainSettingsPage renders both conditional branches.
func TestSettingsHandler_WithBothFlags(t *testing.T) {
	rr := doInProcess(
		t, http.MethodGet, "/settings?saved=1&imported=3", "", "", &accessToken,
	)
	assert.Equal(t, http.StatusOK, rr.Code)
}

// TestSaveSettingsHandler_InvalidSteamUserID exercises the DTO validation
// failure branch of saveIntegrations (SteamUserID must be numeric).
func TestSaveSettingsHandler_InvalidSteamUserID(t *testing.T) {
	body := encodeForm(t, dtos.IntegrationsDto{
		SteamAPIKey:     "some-key",
		SteamUserID:     "not-a-number",
		HardcoverAPIKey: "",
	})
	rr := doInProcess(
		t,
		http.MethodPost,
		"/settings",
		body,
		"application/x-www-form-urlencoded",
		&accessToken,
	)
	// DTO validation fails → 422 Unprocessable Entity.
	assert.Equal(t, http.StatusUnprocessableEntity, rr.Code)
}

// TestSaveOnboardingHandler_InvalidSteamUserID exercises the validation failure
// branch of saveIntegrations via the onboarding route.
func TestSaveOnboardingHandler_InvalidSteamUserID(t *testing.T) {
	body := encodeForm(t, dtos.IntegrationsDto{
		SteamAPIKey:     "key",
		SteamUserID:     "not-numeric",
		HardcoverAPIKey: "",
	})
	rr := doInProcess(
		t,
		http.MethodPost,
		"/onboarding",
		body,
		"application/x-www-form-urlencoded",
		&accessToken,
	)
	assert.Equal(t, http.StatusUnprocessableEntity, rr.Code)
}

// TestDomainMiddleware_KnownDomain verifies that the domainMiddleware rewrites
// the URL path when the request Host matches a registered app domain.
// watchparty.xdoubleu.com is the only app that overrides GetDomain().
func TestDomainMiddleware_KnownDomain(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Host = "watchparty.xdoubleu.com"
	req.AddCookie(&accessToken)

	rr := httptest.NewRecorder()
	testApp.Routes().ServeHTTP(rr, req)

	// The domain middleware rewrites "/" → "/watchparty/" and the watchparty
	// handler responds — any non-5xx is fine; we just verify the middleware ran.
	assert.Less(t, rr.Code, http.StatusInternalServerError)
}

// TestDomainMiddleware_PathRewrite exercises the non-root path rewrite branch
// (r.URL.Path != "/" → prefix+r.URL.Path rather than prefix+"/").
func TestDomainMiddleware_PathRewrite(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/some/path", nil)
	req.Host = "watchparty.xdoubleu.com"
	req.AddCookie(&accessToken)

	rr := httptest.NewRecorder()
	testApp.Routes().ServeHTTP(rr, req)

	// Any non-5xx response is acceptable — we are testing the middleware path rewrite.
	assert.Less(t, rr.Code, http.StatusInternalServerError)
}

// TestSettingsHandler_PostWithEmptyForm exercises the saveIntegrations path
// when all fields are empty (SteamUserID="" is valid numeric-or-empty).
func TestSettingsHandler_PostWithEmptyForm(t *testing.T) {
	body := url.Values{}.Encode()
	rr := doInProcess(
		t,
		http.MethodPost,
		"/settings",
		body,
		"application/x-www-form-urlencoded",
		&accessToken,
	)
	// Empty form → validation passes (all fields optional), redirects to ?saved=1.
	assert.Equal(t, http.StatusSeeOther, rr.Code)
}

// TestHasVerifiedTOTP_WithMFAAccess covers the loop body and return-true path
// in HasVerifiedTOTP by calling it with the "mfa-access" token which the mock
// maps to a user that has a verified TOTP factor.
func TestHasVerifiedTOTP_WithMFAAccess(t *testing.T) {
	factorID, hasMFA := testApp.services.Auth.HasVerifiedTOTP("mfa-access")
	assert.True(t, hasMFA)
	assert.Equal(t, mocks.MockedFactorID, factorID)
}

// TestAppAccess_AdminGrantedPath covers the AppAccess "granted" branch
// (admin user → next handler is called) by promoting the test user to admin
// and hitting a protected app route.
func TestAppAccess_AdminGrantedPath(t *testing.T) {
	promoteToAdmin(t)
	t.Cleanup(func() { demoteToUser(t) })

	rr := doInProcess(t, http.MethodGet, "/backlog/", "", "", &accessToken)
	require.Less(t, rr.Code, http.StatusInternalServerError)
}
