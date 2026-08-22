package oauth2as_test

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestOAuth2Flow_NoScopeRequested_StillIssuesRefreshToken is the regression
// test for issue #1177: MCP clients routinely send no scope parameter, and
// before grantOfflineAccess that meant an empty GrantedScope, no refresh
// token, and a forced interactive re-authentication once the access token
// expired an hour later.
func TestOAuth2Flow_NoScopeRequested_StillIssuesRefreshToken(t *testing.T) {
	srv := newOAuth2asTestServer(t)
	client := srv.registerClient(t)

	verifier, challenge := pkcePair(t)
	code := srv.authorizeAndGetCodeWithScope(
		t, client, challenge, "no-scope-state-1", "",
	)

	resp, out := srv.exchangeToken(t, url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {client.RedirectURIs[0]},
		"client_id":     {client.ID},
		"code_verifier": {verifier},
	})
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.NotEmpty(t, out.AccessToken)
	require.NotEmpty(t, out.RefreshToken)
	assert.Equal(t, "bearer", strings.ToLower(out.TokenType))

	// And that refresh token must actually work, so the client can stay
	// authenticated without another trip through the consent screen.
	refreshResp, refreshOut := srv.exchangeToken(t, url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {out.RefreshToken},
		"client_id":     {client.ID},
	})
	require.Equal(t, http.StatusOK, refreshResp.StatusCode)
	assert.NotEmpty(t, refreshOut.AccessToken)
	assert.NotEmpty(t, refreshOut.RefreshToken)
}

// TestConsentInfoHandler_ReportsEffectiveScope: the consent screen must show
// the scope approving will actually grant, not the raw request parameter,
// now that offline_access is added on top of whatever the client asked for.
func TestConsentInfoHandler_ReportsEffectiveScope(t *testing.T) {
	srv := newOAuth2asTestServer(t)
	client := srv.registerClient(t)

	requestedScopes := map[string]string{
		"no scope requested":       "",
		"offline_access requested": "offline_access",
	}

	for name, requested := range requestedScopes {
		t.Run(name, func(t *testing.T) {
			q := url.Values{"client_id": {client.ID}}
			if requested != "" {
				q.Set("scope", requested)
			}

			resp, err := http.Get(srv.ts.URL + "/oauth2/consent-info?" + q.Encode())
			require.NoError(t, err)
			defer resp.Body.Close()
			require.Equal(t, http.StatusOK, resp.StatusCode)

			var out struct {
				Scope string `json:"scope"`
			}
			require.NoError(t, json.NewDecoder(resp.Body).Decode(&out))
			assert.Equal(t, "offline_access", out.Scope)
		})
	}
}
