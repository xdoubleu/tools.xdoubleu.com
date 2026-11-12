package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/url"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	factoryv1 "tools.xdoubleu.com/gen/factory/v1"
	"tools.xdoubleu.com/internal/models"
	"tools.xdoubleu.com/internal/platform/oauth2as"
)

// approveAs approves a fresh authorization of clientID with the admin's session,
// delegating to accountID when it is set, and returns the redirect.
func approveAs(
	t *testing.T, baseURL, clientID, redirectURI, verifier string,
	session http.Cookie, accountID string,
) *url.URL {
	t.Helper()
	sum := sha256.Sum256([]byte(verifier))
	q := url.Values{
		"response_type":         {oauth2asTestResponseType},
		"client_id":             {clientID},
		"redirect_uri":          {redirectURI},
		"code_challenge":        {base64.RawURLEncoding.EncodeToString(sum[:])},
		"code_challenge_method": {oauth2asTestCodeChallengeMethod},
		"scope":                 {"offline_access"},
		"state":                 {"delegation-state-123"},
	}
	token := oauth2asConsentToken(t, baseURL, q, session)
	q.Set("consent", "allow")
	if accountID != "" {
		q.Set(oauth2as.AgentAccountParam, accountID)
	}
	req, err := http.NewRequestWithContext(
		context.Background(), http.MethodPost,
		baseURL+oauth2as.AuthorizePath+"?"+q.Encode(), nil,
	)
	require.NoError(t, err)
	req.AddCookie(&session)
	req.Header.Set(oauth2as.ConsentTokenHeader, token)
	client := &http.Client{
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	loc, err := url.Parse(resp.Header.Get("Location"))
	require.NoError(t, err)
	return loc
}

func newVerifier(t *testing.T) string {
	t.Helper()
	buf := make([]byte, 32)
	_, err := rand.Read(buf)
	require.NoError(t, err)
	return base64.RawURLEncoding.EncodeToString(buf)
}

func TestOAuth2Authorize_AdminDelegatesAClientToAnAgent(t *testing.T) {
	ts := connectServer(t)
	admin := newAgentAdmin(t, models.RoleAdmin)
	account := createAgent(t, admin, "Delegated agent").Account
	clientID, redirectURI := oauth2asRegisterTestClient(t, ts.URL)
	verifier := newVerifier(t)

	loc := approveAs(t, ts.URL, clientID, redirectURI, verifier, admin.session, account.Id)
	code := loc.Query().Get("code")
	require.NotEmpty(t, code, loc.String())

	resp, err := http.PostForm(ts.URL+oauth2as.TokenPath, url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {redirectURI},
		"client_id":     {clientID},
		"code_verifier": {verifier},
	})
	require.NoError(t, err)
	var out struct {
		AccessToken string `json:"access_token"`
	}
	defer func() { _ = resp.Body.Close() }()
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&out))
	require.NotEmpty(t, out.AccessToken)

	user, err := testApp.auth.ResolveToken(context.Background(), out.AccessToken)
	require.NoError(t, err)
	assert.Equal(t, account.UserId, user.ID)
	assert.Equal(t, models.RoleAgent, user.Role)

	_, err = agentAccountClient(t).RevokeAgentAccount(admin.ctx(),
		&factoryv1.RevokeAgentAccountRequest{Id: account.Id})
	require.NoError(t, err)
	_, err = testApp.auth.ResolveToken(context.Background(), out.AccessToken)
	assert.Error(t, err, "revoking the agent ends the delegated client's tokens")
}

func TestOAuth2Authorize_DelegationNeedsAnActiveAgentAndAnAdmin(t *testing.T) {
	ts := connectServer(t)
	admin := newAgentAdmin(t, models.RoleAdmin)
	clientID, redirectURI := oauth2asRegisterTestClient(t, ts.URL)

	for name, id := range map[string]string{
		"not a uuid": "agent-1", "unknown": uuid.NewString(),
	} {
		loc := approveAs(t, ts.URL, clientID, redirectURI, newVerifier(t), admin.session, id)
		assert.Equal(t, "invalid_request", loc.Query().Get("error"), name)
	}

	person := newAgentAdmin(t, models.RoleUser)
	account := createAgent(t, admin, "Not for people").Account
	loc := approveAs(
		t,
		ts.URL,
		clientID,
		redirectURI,
		newVerifier(t),
		person.session,
		account.Id,
	)
	assert.Equal(t, "access_denied", loc.Query().Get("error"))

	loc = approveAs(t, ts.URL, clientID, redirectURI, newVerifier(t), admin.session, "")
	assert.NotEmpty(t, loc.Query().Get("code"), "approving as yourself still works")
}
