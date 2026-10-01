package main

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"

	"tools.xdoubleu.com/internal/github"
	"tools.xdoubleu.com/internal/logging"
	"tools.xdoubleu.com/internal/models"
)

// useDeniedGithub points the GitHub client at a server answering 403.
func useDeniedGithub(t *testing.T) {
	t.Helper()
	srv := jsonServer(t, http.StatusForbidden,
		`{"message":"Resource not accessible by integration"}`)
	github.SetBaseURL(srv.URL)
	t.Cleanup(func() { github.SetBaseURL("https://api.github.com") })
	testApp.githubClient = github.New(
		logging.NewNopLogger(),
		stubTok("tok"),
		testConfigJSON(t, map[string]string{"repo": "o/r"}),
	)
}

// connectGithub stores a GitHub connection whose echoed scope is granted.
func connectGithub(t *testing.T, granted string) {
	t.Helper()
	clearOAuthConnections(t)
	t.Cleanup(func() {
		// t.Context() is already canceled during cleanup.
		_, err := testApp.db.Exec(
			context.Background(), "DELETE FROM global.oauth_connections",
		)
		assert.NoError(t, err)
	})
	tok := (&oauth2.Token{ //nolint:exhaustruct // other fields unused in test
		AccessToken: "tok",
	}).WithExtra(map[string]any{"scope": granted})
	require.NoError(t, testApp.oauthConnRepo.Upsert(
		t.Context(), models.OAuthProviderGithub, tok, testUserID, githubScopes,
	))
}

func TestObservabilityGetSecurityAlerts_Denied_NamesMissingScope(t *testing.T) {
	promoteToAdmin(t)
	t.Cleanup(func() { demoteToUser(t) })
	useDeniedGithub(t)
	// GitHub's echo is comma-separated.
	connectGithub(t, "read:project,repo")

	resp, err := callSecurityAlerts(t)
	require.NoError(t, err)
	assert.True(t, resp.Msg.Configured)
	assert.Empty(t, resp.Msg.Alerts)
	assert.Contains(t, resp.Msg.Error, "security_events")
	assert.Contains(t, resp.Msg.Error, `"read:project,repo"`)
	assert.Contains(t, resp.Msg.Error, "/monitoring/connections")
	assert.Contains(t, resp.Msg.Error, "Resource not accessible by integration")
}

func TestObservabilityGetSecurityAlerts_DeniedWithScope_ReportsUpstream(
	t *testing.T,
) {
	promoteToAdmin(t)
	t.Cleanup(func() { demoteToUser(t) })
	useDeniedGithub(t)
	connectGithub(t, "repo,security_events,read:project")

	resp, err := callSecurityAlerts(t)
	require.NoError(t, err)
	assert.Contains(t, resp.Msg.Error, "GitHub denied access to security alerts")
	assert.Contains(t, resp.Msg.Error, "Resource not accessible by integration")
	assert.NotContains(t, resp.Msg.Error, "reconnect")
}

func TestObservabilityGetSecurityAlerts_UpstreamError_SetsError(t *testing.T) {
	promoteToAdmin(t)
	t.Cleanup(func() { demoteToUser(t) })

	srv := jsonServer(t, http.StatusInternalServerError, ``)
	github.SetBaseURL(srv.URL)
	t.Cleanup(func() { github.SetBaseURL("https://api.github.com") })
	testApp.githubClient = github.New(
		logging.NewNopLogger(),
		stubTok("tok"),
		testConfigJSON(t, map[string]string{"repo": "o/r"}),
	)

	resp, err := callSecurityAlerts(t)
	require.NoError(t, err)
	assert.Contains(t, resp.Msg.Error, "security alerts unavailable")
	assert.Contains(t, resp.Msg.Error, "500")
}
