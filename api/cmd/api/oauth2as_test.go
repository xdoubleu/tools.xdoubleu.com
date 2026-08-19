package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	oauth2asTestCodeChallengeMethod = "S256"
	oauth2asTestResponseType        = "code"
)

// TestOAuth2MetadataHandler covers the hand-rolled RFC 8414 authorization
// server metadata document at /.well-known/oauth-authorization-server,
// wired up in oauth2as.go's oauth2MetadataHandler.
func TestOAuth2MetadataHandler(t *testing.T) {
	ts := connectServer(t)

	resp, err := http.Get(ts.URL + "/.well-known/oauth-authorization-server")
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var out map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&out))

	issuer, _ := out["issuer"].(string)
	assert.NotEmpty(t, issuer)
	assert.Equal(t, issuer+oauth2AuthorizePath, out["authorization_endpoint"])
	assert.Equal(t, issuer+oauth2TokenPath, out["token_endpoint"])
	assert.Equal(t, issuer+oauth2RegisterPath, out["registration_endpoint"])
	assert.Equal(t, []any{oauth2asTestResponseType}, out["response_types_supported"])
	assert.Equal(
		t, []any{oauth2asTestCodeChallengeMethod},
		out["code_challenge_methods_supported"],
	)
	assert.Equal(t, []any{"none"}, out["token_endpoint_auth_methods_supported"])
}

// TestOAuth2Metadata_PathInsertionAlias covers issue #1141: in production,
// APIURL (and therefore AuthIssuer, which defaults to it) has a path
// ("/api"), so RFC 8414/9728 require a discovering client to insert
// /.well-known/... *before* that path rather than trust the bare path
// TestOAuth2MetadataHandler above exercises. routes.go registers that "/api"
// alias unconditionally for both the AS metadata and the protected-resource
// metadata document, regardless of the configured APIURL/AuthIssuer shape.
func TestOAuth2Metadata_PathInsertionAlias(t *testing.T) {
	ts := connectServer(t)

	resp, err := http.Get(ts.URL + "/.well-known/oauth-authorization-server/api")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	resp2, err := http.Get(
		ts.URL + "/.well-known/oauth-protected-resource/api/apps/mcp",
	)
	require.NoError(t, err)
	defer resp2.Body.Close()
	assert.Equal(t, http.StatusOK, resp2.StatusCode)
}

// oauth2asRegisterTestClient registers a fresh dynamic client against the
// real /oauth2/register route (wired via app.oauth2as.store in routes.go)
// and returns its client ID and redirect URI.
func oauth2asRegisterTestClient(
	t *testing.T, ts string,
) (string, string) {
	t.Helper()
	redirectURI := "http://localhost:9999/callback"
	body, err := json.Marshal(map[string]any{
		"redirect_uris": []string{redirectURI},
		"client_name":   "cmd/api oauth2as test client",
	})
	require.NoError(t, err)

	resp, err := http.Post(
		ts+oauth2RegisterPath, "application/json", strings.NewReader(string(body)),
	)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	var out struct {
		ClientID string `json:"client_id"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&out))
	require.NotEmpty(t, out.ClientID)
	return out.ClientID, redirectURI
}

func oauth2asPKCEChallenge(t *testing.T) string {
	t.Helper()
	buf := make([]byte, 32)
	_, err := rand.Read(buf)
	require.NoError(t, err)
	sum := sha256.Sum256([]byte(base64.RawURLEncoding.EncodeToString(buf)))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

// TestOAuth2Authorize_SessionResolver_NoCookie covers
// app.oauth2SessionUserResolver's ok=false branch (oauth2as.go): consenting
// without a valid accessToken cookie must be rejected as unauthorized rather
// than silently granting a token for no one.
func TestOAuth2Authorize_SessionResolver_NoCookie(t *testing.T) {
	ts := connectServer(t)
	clientID, redirectURI := oauth2asRegisterTestClient(t, ts.URL)

	client := &http.Client{
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	q := url.Values{
		"response_type":         {oauth2asTestResponseType},
		"client_id":             {clientID},
		"redirect_uri":          {redirectURI},
		"code_challenge":        {oauth2asPKCEChallenge(t)},
		"code_challenge_method": {oauth2asTestCodeChallengeMethod},
		"scope":                 {"offline_access"},
		"state":                 {"no-cookie-state-123"},
		"consent":               {"allow"},
	}

	resp, err := client.Get(ts.URL + oauth2AuthorizePath + "?" + q.Encode())
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusSeeOther, resp.StatusCode)

	loc, err := url.Parse(resp.Header.Get("Location"))
	require.NoError(t, err)
	assert.Equal(t, "request_unauthorized", loc.Query().Get("error"))
}

// TestOAuth2Authorize_SessionResolver_ValidCookie_IssuesCode covers
// app.oauth2SessionUserResolver's ok=true branch: a real accessToken cookie
// resolves via the already-wired auth service and the flow completes with an
// authorization code.
func TestOAuth2Authorize_SessionResolver_ValidCookie_IssuesCode(t *testing.T) {
	ts := connectServer(t)
	clientID, redirectURI := oauth2asRegisterTestClient(t, ts.URL)

	client := &http.Client{
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	q := url.Values{
		"response_type":         {oauth2asTestResponseType},
		"client_id":             {clientID},
		"redirect_uri":          {redirectURI},
		"code_challenge":        {oauth2asPKCEChallenge(t)},
		"code_challenge_method": {oauth2asTestCodeChallengeMethod},
		"scope":                 {"offline_access"},
		"state":                 {"valid-cookie-state-123"},
		"consent":               {"allow"},
	}

	req, err := http.NewRequestWithContext(
		context.Background(), http.MethodGet,
		ts.URL+oauth2AuthorizePath+"?"+q.Encode(), nil,
	)
	require.NoError(t, err)
	req.AddCookie(&accessToken)

	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusSeeOther, resp.StatusCode)

	loc, err := url.Parse(resp.Header.Get("Location"))
	require.NoError(t, err)
	assert.Equal(t, "valid-cookie-state-123", loc.Query().Get("state"))
	assert.NotEmpty(t, loc.Query().Get("code"))
}

// TestOAuth2Authorize_SessionResolver_InvalidCookie covers the resolver's
// GetUser-fails sub-branch specifically (as opposed to no cookie at all).
func TestOAuth2Authorize_SessionResolver_InvalidCookie(t *testing.T) {
	ts := connectServer(t)
	clientID, redirectURI := oauth2asRegisterTestClient(t, ts.URL)

	client := &http.Client{
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	q := url.Values{
		"response_type":         {oauth2asTestResponseType},
		"client_id":             {clientID},
		"redirect_uri":          {redirectURI},
		"code_challenge":        {oauth2asPKCEChallenge(t)},
		"code_challenge_method": {oauth2asTestCodeChallengeMethod},
		"scope":                 {"offline_access"},
		"state":                 {"invalid-cookie-state-123"},
		"consent":               {"allow"},
	}

	req, err := http.NewRequestWithContext(
		context.Background(), http.MethodGet,
		ts.URL+oauth2AuthorizePath+"?"+q.Encode(), nil,
	)
	require.NoError(t, err)
	req.AddCookie(&http.Cookie{Name: "accessToken", Value: "not-a-real-token"})

	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusSeeOther, resp.StatusCode)

	loc, err := url.Parse(resp.Header.Get("Location"))
	require.NoError(t, err)
	assert.Equal(t, "request_unauthorized", loc.Query().Get("error"))
}

// TestOAuth2Authorize_NoConsentYet_RedirectsToWebConsentPage covers the
// no-consent-decision-yet branch of AuthorizeHandler as reached through the
// real cmd/api route (rather than oauth2as's own package tests, which build
// their own bare mux).
func TestOAuth2Authorize_NoConsentYet_RedirectsToWebConsentPage(t *testing.T) {
	ts := connectServer(t)
	clientID, redirectURI := oauth2asRegisterTestClient(t, ts.URL)

	client := &http.Client{
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	q := url.Values{
		"response_type":         {oauth2asTestResponseType},
		"client_id":             {clientID},
		"redirect_uri":          {redirectURI},
		"code_challenge":        {oauth2asPKCEChallenge(t)},
		"code_challenge_method": {oauth2asTestCodeChallengeMethod},
		"scope":                 {"offline_access"},
		"state":                 {uuid.NewString()},
	}

	resp, err := client.Get(ts.URL + oauth2AuthorizePath + "?" + q.Encode())
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusFound, resp.StatusCode)
	assert.Contains(t, resp.Header.Get("Location"), "/oauth/consent?")
}
