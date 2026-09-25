package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/oauth2as"
)

const routinesTestSecret = "routines-test-client-secret-0123456789"

// routinesClientToken mints a client_credentials token for the routines client.
func routinesClientToken(t *testing.T) string {
	t.Helper()
	ctx := context.Background()
	require.NoError(t, oauth2as.EnsureRoutinesClientSecret(
		ctx, testApp.db, routinesTestSecret, nil,
	))

	ts := connectServer(t)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		ts.URL+oauth2TokenPath,
		strings.NewReader(url.Values{"grant_type": {"client_credentials"}}.Encode()),
	)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(oauth2as.RoutinesClientID, routinesTestSecret)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var out struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&out))
	require.NotEmpty(t, out.AccessToken)
	assert.Empty(t, out.RefreshToken, "client_credentials never gets a refresh token")
	return out.AccessToken
}

// TestRoutinesClient_ObservabilityOnly: the service identity can use the
// observability tools but no app tool, which would expose user data.
func TestRoutinesClient_ObservabilityOnly(t *testing.T) {
	ctx := context.Background()
	session := appsMCPSession(t, routinesClientToken(t))

	//nolint:exhaustruct // only the tool name is required to call it
	allowed, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "get_job_stats",
	})
	require.NoError(t, err)
	assert.False(t, allowed.IsError, toolMessage(allowed, err))

	for _, name := range []string{"recipes_list_recipes", "books_get_library"} {
		//nolint:exhaustruct // only the tool name is required to call it
		res, callErr := session.CallTool(ctx, &mcp.CallToolParams{Name: name})
		assert.Containsf(t, strings.ToLower(toolMessage(res, callErr)),
			"access to", "expected %s to be denied", name)
	}
}

// TestGetAllUsers_ExcludesServiceUser keeps the service identity out of
// family invites and per-user jobs.
func TestGetAllUsers_ExcludesServiceUser(t *testing.T) {
	users, err := testApp.auth.GetAllUsers(context.Background())
	require.NoError(t, err)
	for _, u := range users {
		assert.NotEqual(t, oauth2as.RoutinesServiceUserID, u.ID)
	}
}
