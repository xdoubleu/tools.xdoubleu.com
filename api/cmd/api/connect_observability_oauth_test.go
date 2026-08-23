package main

import (
	"context"
	"errors"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"

	observabilityv1 "tools.xdoubleu.com/gen/observability/v1"
	"tools.xdoubleu.com/internal/github"
	"tools.xdoubleu.com/internal/models"
	"tools.xdoubleu.com/internal/oauthconn"
)

// githubScopes is what a GitHub connection is authorized with today, read
// from the real config so these tests can't drift from it.
//
//nolint:gochecknoglobals // fixture mirroring production config
var githubScopes = github.OAuthConfig("", "", "").Scopes

func TestProviderOptionsError_DecryptFailed(t *testing.T) {
	err := providerOptionsError(models.ErrDecryptFailed)
	assert.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
	assert.NotErrorIs(
		t, err, models.ErrDecryptFailed,
		"the client-facing message must not leak the internal sentinel/cause",
	)
}

func TestProviderOptionsError_NotConnected(t *testing.T) {
	err := providerOptionsError(oauthconn.ErrNotConnected)
	assert.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
}

func TestProviderOptionsError_OtherError(t *testing.T) {
	err := providerOptionsError(errors.New("boom"))
	assert.Equal(t, connect.CodeInternal, connect.CodeOf(err))
}

func clearOAuthConnections(t *testing.T) {
	t.Helper()
	_, err := testApp.db.Exec(t.Context(), "DELETE FROM global.oauth_connections")
	require.NoError(t, err)
}

func TestListOAuthConnections_AsAdmin_NoneConnected(t *testing.T) {
	promoteToAdmin(t)
	t.Cleanup(func() { demoteToUser(t) })
	clearOAuthConnections(t)

	req := connect.NewRequest(&observabilityv1.ListOAuthConnectionsRequest{})
	setCookieOnRequest(req, accessToken)
	resp, err := observabilityClient(t).ListOAuthConnections(context.Background(), req)
	require.NoError(t, err)

	require.Len(t, resp.Msg.Connections, 3)
	for _, c := range resp.Msg.Connections {
		assert.False(t, c.Connected)
		assert.Empty(t, c.ConnectedBy)
	}
}

func TestListOAuthConnections_AsAdmin_OneConnected(t *testing.T) {
	promoteToAdmin(t)
	t.Cleanup(func() { demoteToUser(t) })
	clearOAuthConnections(t)

	require.NoError(t, testApp.oauthConnRepo.Upsert(
		t.Context(),
		models.OAuthProviderGithub,
		&oauth2.Token{ //nolint:exhaustruct // other fields unused in test
			AccessToken: "tok",
		},
		testUserID,
		githubScopes,
	))

	req := connect.NewRequest(&observabilityv1.ListOAuthConnectionsRequest{})
	setCookieOnRequest(req, accessToken)
	resp, err := observabilityClient(t).ListOAuthConnections(context.Background(), req)
	require.NoError(t, err)

	for _, c := range resp.Msg.Connections {
		if c.Provider == string(models.OAuthProviderGithub) {
			assert.True(t, c.Connected)
			assert.NotEmpty(t, c.ConnectedAt)
		} else {
			assert.False(t, c.Connected)
		}
	}
}

func TestListOAuthConnections_AsAdmin_StaleScope(t *testing.T) {
	promoteToAdmin(t)
	t.Cleanup(func() { demoteToUser(t) })
	clearOAuthConnections(t)

	tok := (&oauth2.Token{ //nolint:exhaustruct // other fields unused in test
		AccessToken: "tok",
	}).WithExtra(map[string]any{"scope": "org:read"})
	require.NoError(t, testApp.oauthConnRepo.Upsert(
		t.Context(), models.OAuthProviderSentry, tok, testUserID, nil,
	))

	req := connect.NewRequest(&observabilityv1.ListOAuthConnectionsRequest{})
	setCookieOnRequest(req, accessToken)
	resp, err := observabilityClient(t).ListOAuthConnections(context.Background(), req)
	require.NoError(t, err)

	for _, c := range resp.Msg.Connections {
		if c.Provider == string(models.OAuthProviderSentry) {
			assert.False(
				t,
				c.Connected,
				"a connection missing a currently-required scope must show as not connected",
			)
		}
	}
}

// Regression for issue #1195: GitHub returns a normalized granted scope —
// just `repo`, since it subsumes the `security_events` also requested — and
// judging coverage by that echo reported a freshly-authorized connection as
// not connected, so the admin UI kept showing "Connect" no matter how many
// times the flow was completed.
func TestListOAuthConnections_NormalizedGrantedScope_ShowsConnected(t *testing.T) {
	promoteToAdmin(t)
	t.Cleanup(func() { demoteToUser(t) })
	clearOAuthConnections(t)

	tok := (&oauth2.Token{ //nolint:exhaustruct // other fields unused in test
		AccessToken: "tok",
	}).WithExtra(map[string]any{"scope": "repo"})
	require.NoError(t, testApp.oauthConnRepo.Upsert(
		t.Context(), models.OAuthProviderGithub, tok, testUserID, githubScopes,
	))

	req := connect.NewRequest(&observabilityv1.ListOAuthConnectionsRequest{})
	setCookieOnRequest(req, accessToken)
	resp, err := observabilityClient(t).ListOAuthConnections(context.Background(), req)
	require.NoError(t, err)

	for _, c := range resp.Msg.Connections {
		if c.Provider != string(models.OAuthProviderGithub) {
			continue
		}
		assert.True(
			t, c.Connected,
			"a connection authorized with every required scope must show as "+
				"connected even when the provider's echoed scope omits one",
		)
		assert.Equal(t, "repo", c.GrantedScope)
		assert.Equal(t, strings.Join(githubScopes, " "), c.RequestedScope)
		assert.Equal(t, strings.Join(githubScopes, " "), c.RequiredScope)
	}
}

// A stale connection still reports its scopes, so the reason it is being
// shown as not connected is visible without database access.
func TestListOAuthConnections_StaleScope_ReportsScopes(t *testing.T) {
	promoteToAdmin(t)
	t.Cleanup(func() { demoteToUser(t) })
	clearOAuthConnections(t)

	require.NoError(t, testApp.oauthConnRepo.Upsert(
		t.Context(),
		models.OAuthProviderGithub,
		&oauth2.Token{ //nolint:exhaustruct // other fields unused in test
			AccessToken: "tok",
		},
		testUserID,
		[]string{"repo"},
	))

	req := connect.NewRequest(&observabilityv1.ListOAuthConnectionsRequest{})
	setCookieOnRequest(req, accessToken)
	resp, err := observabilityClient(t).ListOAuthConnections(context.Background(), req)
	require.NoError(t, err)

	for _, c := range resp.Msg.Connections {
		if c.Provider != string(models.OAuthProviderGithub) {
			continue
		}
		assert.False(t, c.Connected)
		assert.Equal(t, "repo", c.RequestedScope)
		assert.Equal(t, strings.Join(githubScopes, " "), c.RequiredScope)
	}
}

func TestListOAuthConnections_NonAdmin(t *testing.T) {
	demoteToUser(t)

	req := connect.NewRequest(&observabilityv1.ListOAuthConnectionsRequest{})
	setCookieOnRequest(req, accessToken)
	_, err := observabilityClient(t).ListOAuthConnections(context.Background(), req)
	requirePermissionDenied(t, err)
}

func TestDisconnectOAuthConnection_AsAdmin(t *testing.T) {
	promoteToAdmin(t)
	t.Cleanup(func() { demoteToUser(t) })
	clearOAuthConnections(t)

	require.NoError(t, testApp.oauthConnRepo.Upsert(
		t.Context(),
		models.OAuthProviderGithub,
		&oauth2.Token{ //nolint:exhaustruct // other fields unused in test
			AccessToken: "tok",
		},
		testUserID,
		githubScopes,
	))

	req := connect.NewRequest(&observabilityv1.DisconnectOAuthConnectionRequest{
		Provider: string(models.OAuthProviderGithub),
	})
	setCookieOnRequest(req, accessToken)
	_, err := observabilityClient(
		t,
	).DisconnectOAuthConnection(context.Background(), req)
	require.NoError(t, err)

	_, _, err = testApp.oauthConnRepo.Get(t.Context(), models.OAuthProviderGithub)
	assert.Error(t, err, "connection should be gone after disconnect")
}

func TestDisconnectOAuthConnection_NonAdmin(t *testing.T) {
	demoteToUser(t)

	req := connect.NewRequest(&observabilityv1.DisconnectOAuthConnectionRequest{
		Provider: string(models.OAuthProviderGithub),
	})
	setCookieOnRequest(req, accessToken)
	_, err := observabilityClient(
		t,
	).DisconnectOAuthConnection(context.Background(), req)
	requirePermissionDenied(t, err)
}
