package learningpaths_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"

	"tools.xdoubleu.com/apps/learningpaths/internal/repositories"
	"tools.xdoubleu.com/internal/crypto"
	sharedmodels "tools.xdoubleu.com/internal/models"
	"tools.xdoubleu.com/internal/todoist"
)

// TestTodoistOAuthCallback_InvalidStateRedirectsWithError exercises the
// callback route's error branch without any real network call — an
// unknown/expired state fails before the handler ever reaches
// oauth2.Config.Exchange (the leg that would need a live Todoist round trip).
func TestTodoistOAuthCallback_InvalidStateRedirectsWithError(t *testing.T) {
	srv := httptest.NewServer(getRoutes())
	defer srv.Close()

	httpClient := &http.Client{
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	req, err := http.NewRequestWithContext(
		t.Context(), http.MethodGet,
		srv.URL+"/learningpaths/oauth/todoist/callback?state=bogus&code=xyz",
		nil,
	)
	require.NoError(t, err)

	resp, err := httpClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusFound, resp.StatusCode)
	assert.Contains(t, resp.Header.Get("Location"), "todoist_error=1")
}

// TestTodoistOAuthCallback_SuccessRedirectsConnected exercises the callback
// route's success branch end to end, including the real token-exchange call
// — but against a local httptest server standing in for Todoist's token
// endpoint (stubbed via TodoistServiceForTest/SetOAuthConfigForTest, the
// same idea as cmd/api's admin OAuth tests stubbing GitHub/Sentry), not a
// real network call.
func TestTodoistOAuthCallback_SuccessRedirectsConnected(t *testing.T) {
	tokenSrv := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(
				`{"access_token":"new-todoist-token","token_type":"Bearer"}`,
			))
		}))
	defer tokenSrv.Close()

	todoistSvc := testApp.TodoistServiceForTest()
	origConf := todoist.OAuthConfig(
		testCfg.TodoistOAuthClientID, testCfg.TodoistOAuthClientSecret, testCfg.APIURL,
	)
	t.Cleanup(func() { todoistSvc.SetOAuthConfigForTest(origConf) })
	//nolint:exhaustruct //other fields unused by this test
	todoistSvc.SetOAuthConfigForTest(&oauth2.Config{
		ClientID: "id",
		Endpoint: oauth2.Endpoint{TokenURL: tokenSrv.URL},
	})

	authorizeURL := todoistSvc.AuthorizeURL(userID)
	parsed, err := url.Parse(authorizeURL)
	require.NoError(t, err)
	state := parsed.Query().Get("state")
	require.NotEmpty(t, state)

	srv := httptest.NewServer(getRoutes())
	defer srv.Close()

	httpClient := &http.Client{
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	req, err := http.NewRequestWithContext(
		t.Context(), http.MethodGet,
		srv.URL+"/learningpaths/oauth/todoist/callback?state="+state+"&code=good-code",
		nil,
	)
	require.NoError(t, err)

	resp, err := httpClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusFound, resp.StatusCode)
	assert.Contains(t, resp.Header.Get("Location"), "todoist_connected=1")

	testSealer, err := crypto.New(testCfg.EncryptionKey)
	require.NoError(t, err)
	oauthRepo := repositories.New(testDB, testSealer).OAuthConnections
	t.Cleanup(func() {
		_ = oauthRepo.Delete(t.Context(), userID, sharedmodels.OAuthProviderTodoist)
	})

	tok, _, getErr := oauthRepo.Get(
		t.Context(), userID, sharedmodels.OAuthProviderTodoist,
	)
	require.NoError(t, getErr)
	assert.Equal(t, "new-todoist-token", tok.AccessToken)
}
