package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"

	"tools.xdoubleu.com/internal/models"
)

// withStubProvider temporarily points the "digitalocean" provider entry at a
// fixed oauth2.Config (e.g. an httptest server), restoring the real one
// afterwards, so callback-exchange tests never hit the real network.
func withStubProvider(t *testing.T, conf *oauth2.Config) {
	t.Helper()
	original := oauthProviders["digitalocean"]
	oauthProviders["digitalocean"] = oauthProviderDef{
		provider: original.provider,
		conf:     func(*Application) *oauth2.Config { return conf },
	}
	t.Cleanup(func() { oauthProviders["digitalocean"] = original })
}

func TestOAuthStartRoute_UnknownProvider(t *testing.T) {
	promoteToAdmin(t)
	t.Cleanup(func() { demoteToUser(t) })

	rr := doInProcess(
		t,
		http.MethodGet,
		"/admin/oauth/bogus/start",
		"",
		"",
		&accessToken,
	)
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestOAuthStartRoute_NonAdmin(t *testing.T) {
	demoteToUser(t)

	rr := doInProcess(
		t,
		http.MethodGet,
		"/admin/oauth/github/start",
		"",
		"",
		&accessToken,
	)
	assert.Equal(t, http.StatusSeeOther, rr.Code)
	assert.Equal(t, "/", rr.Header().Get("Location"))
}

func TestOAuthStartRoute_AsAdmin(t *testing.T) {
	promoteToAdmin(t)
	t.Cleanup(func() { demoteToUser(t) })

	rr := doInProcess(
		t,
		http.MethodGet,
		"/admin/oauth/digitalocean/start",
		"",
		"",
		&accessToken,
	)
	require.Equal(t, http.StatusFound, rr.Code)

	loc, err := rr.Result().Location()
	require.NoError(t, err)
	assert.Equal(t, "cloud.digitalocean.com", loc.Host)

	state := loc.Query().Get("state")
	require.NotEmpty(t, state)
	provider, userID, ok := testApp.oauthState.Consume(state)
	require.True(t, ok)
	assert.Equal(t, models.OAuthProviderDigitalOcean, provider)
	assert.Equal(t, testUserID, userID)
}

func TestOAuthCallbackRoute_UnknownProvider(t *testing.T) {
	promoteToAdmin(t)
	t.Cleanup(func() { demoteToUser(t) })

	rr := doInProcess(
		t,
		http.MethodGet,
		"/admin/oauth/bogus/callback",
		"",
		"",
		&accessToken,
	)
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestOAuthCallbackRoute_InvalidState(t *testing.T) {
	promoteToAdmin(t)
	t.Cleanup(func() { demoteToUser(t) })

	rr := doInProcess(
		t, http.MethodGet,
		"/admin/oauth/github/callback?state=does-not-exist&code=x",
		"", "", &accessToken,
	)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestOAuthCallbackRoute_ProviderMismatch(t *testing.T) {
	promoteToAdmin(t)
	t.Cleanup(func() { demoteToUser(t) })

	state := testApp.oauthState.New(models.OAuthProviderGithub, testUserID)
	rr := doInProcess(
		t, http.MethodGet,
		"/admin/oauth/digitalocean/callback?state="+state+"&code=x",
		"", "", &accessToken,
	)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestOAuthCallbackRoute_ExchangeFailure(t *testing.T) {
	promoteToAdmin(t)
	t.Cleanup(func() { demoteToUser(t) })
	clearOAuthConnections(t)

	srv := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
		}))
	t.Cleanup(srv.Close)
	withStubProvider(
		t,
		&oauth2.Config{ //nolint:exhaustruct // other fields unused in test
			//nolint:exhaustruct // other fields unused in test
			Endpoint: oauth2.Endpoint{
				TokenURL: srv.URL,
			},
		},
	)

	state := testApp.oauthState.New(models.OAuthProviderDigitalOcean, testUserID)
	rr := doInProcess(
		t, http.MethodGet,
		"/admin/oauth/digitalocean/callback?state="+state+"&code=bad-code",
		"", "", &accessToken,
	)
	require.Equal(t, http.StatusFound, rr.Code)
	loc, err := rr.Result().Location()
	require.NoError(t, err)
	assert.Contains(t, loc.RawQuery, "oauth_error=digitalocean")

	_, _, getErr := testApp.oauthConnRepo.Get(
		t.Context(),
		models.OAuthProviderDigitalOcean,
	)
	assert.Error(t, getErr, "a failed exchange must not store a connection")
}

func TestOAuthCallbackRoute_Success(t *testing.T) {
	promoteToAdmin(t)
	t.Cleanup(func() { demoteToUser(t) })
	clearOAuthConnections(t)

	srv := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(
				`{"access_token":"new-token","token_type":"Bearer"}`,
			))
		}))
	t.Cleanup(srv.Close)
	withStubProvider(
		t,
		&oauth2.Config{ //nolint:exhaustruct // other fields unused in test
			//nolint:exhaustruct // other fields unused in test
			Endpoint: oauth2.Endpoint{
				TokenURL: srv.URL,
			},
		},
	)

	state := testApp.oauthState.New(models.OAuthProviderDigitalOcean, testUserID)
	rr := doInProcess(
		t, http.MethodGet,
		"/admin/oauth/digitalocean/callback?state="+state+"&code=good-code",
		"", "", &accessToken,
	)
	require.Equal(t, http.StatusFound, rr.Code)
	loc, err := rr.Result().Location()
	require.NoError(t, err)
	assert.Contains(t, loc.RawQuery, "oauth_connected=digitalocean")

	tok, _, getErr := testApp.oauthConnRepo.Get(
		t.Context(),
		models.OAuthProviderDigitalOcean,
	)
	require.NoError(t, getErr)
	assert.Equal(t, "new-token", tok.AccessToken)
}
